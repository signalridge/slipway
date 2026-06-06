package governance

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/signalridge/slipway/internal/engine/control"
	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// GovernanceHealthCheck represents one governance health diagnostic.
type GovernanceHealthCheck struct {
	Name             string                  `json:"name"`
	Status           string                  `json:"status"` // OK, WARN, FAIL
	Message          string                  `json:"message"`
	TraceabilityGaps []model.TraceabilityGap `json:"traceability_gaps,omitempty"`
}

// GovernanceHealthReport contains all governance health checks for a change.
type GovernanceHealthReport struct {
	Slug    string                  `json:"slug"`
	Checks  []GovernanceHealthCheck `json:"checks"`
	Healthy bool                    `json:"healthy"`
}

// CollectGovernanceHealth runs governance health checks for a change.
func CollectGovernanceHealth(root string, change model.Change) GovernanceHealthReport {
	snap, err := LoadSnapshot(root, change.Slug)
	if err != nil {
		return snapshotReadFailureReport(root, change, err)
	}
	return CollectGovernanceHealthWithSnapshot(root, change, snap)
}

// CollectGovernanceHealthWithSnapshot runs governance health checks against the
// provided snapshot, allowing callers to validate freshly recomputed state
// without reloading stale on-disk data.
func CollectGovernanceHealthWithSnapshot(root string, change model.Change, snap model.GovernanceSnapshot) GovernanceHealthReport {
	slug := change.Slug
	var checks []GovernanceHealthCheck
	healthy := true

	// 1. controls_config
	check := checkControlsConfig(root, change)
	checks = append(checks, check)
	if check.Status == "FAIL" {
		healthy = false
	}

	check = checkPolicyPacks(root, change)
	checks = append(checks, check)

	// 2. signal_freshness
	check = SignalFreshnessCheck(snap)
	checks = append(checks, check)
	if check.Status == "FAIL" {
		healthy = false
	}

	// 3. traceability_coherence
	check = checkTraceabilityCoherence(snap)
	checks = append(checks, check)
	if check.Status == "FAIL" {
		healthy = false
	}

	// 4. signal_control_coherence
	check = checkSignalControlCoherence(root, change, snap)
	checks = append(checks, check)
	if check.Status == "FAIL" {
		healthy = false
	}

	// 5. worktree_binding
	check = checkWorktreeBinding(root, change)
	checks = append(checks, check)
	if check.Status == "FAIL" {
		healthy = false
	}

	return GovernanceHealthReport{
		Slug:    slug,
		Checks:  checks,
		Healthy: healthy,
	}
}

func checkPolicyPacks(root string, change model.Change) GovernanceHealthCheck {
	cfgPath, err := state.ConfigPathForChange(root, change)
	if err != nil {
		return GovernanceHealthCheck{
			Name:    "policy_packs",
			Status:  "WARN",
			Message: fmt.Sprintf("skipped policy pack checks: resolve .slipway.yaml path error: %v", err),
		}
	}
	cfg, err := model.LoadConfig(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return GovernanceHealthCheck{Name: "policy_packs", Status: "OK", Message: "no policy packs configured"}
		}
		return GovernanceHealthCheck{
			Name:    "policy_packs",
			Status:  "WARN",
			Message: fmt.Sprintf("skipped policy pack checks because .slipway.yaml is invalid: %v", err),
		}
	}
	if len(cfg.Governance.PolicyPacks) == 0 {
		return GovernanceHealthCheck{Name: "policy_packs", Status: "OK", Message: "no policy packs configured"}
	}

	var warnings []string
	for _, pack := range cfg.Governance.PolicyPacks {
		if pack.Mode != "" && pack.Mode != model.ControlModeAdvisory {
			warnings = append(warnings, fmt.Sprintf("policy pack %q is not advisory", pack.Name))
			continue
		}
		packPath := strings.TrimSpace(pack.Path)
		if !filepath.IsAbs(packPath) {
			packPath = filepath.Join(filepath.Dir(cfgPath), packPath)
		}
		if _, err := LoadAdvisoryPolicyPack(pack.Name, packPath); err != nil {
			warnings = append(warnings, err.Error())
		}
	}
	if len(warnings) > 0 {
		return GovernanceHealthCheck{
			Name:    "policy_packs",
			Status:  "WARN",
			Message: "advisory policy pack warnings: " + strings.Join(warnings, "; "),
		}
	}
	return GovernanceHealthCheck{
		Name:    "policy_packs",
		Status:  "OK",
		Message: fmt.Sprintf("%d advisory policy pack(s) parsed", len(cfg.Governance.PolicyPacks)),
	}
}

func checkWorktreeBinding(root string, change model.Change) GovernanceHealthCheck {
	validation, err := state.ValidateChangeWorktree(root, change)
	if err != nil {
		return GovernanceHealthCheck{
			Name:    "worktree_binding",
			Status:  "FAIL",
			Message: fmt.Sprintf("worktree validation error: %v", err),
		}
	}
	if len(validation.Blockers) > 0 {
		return GovernanceHealthCheck{
			Name:    "worktree_binding",
			Status:  "FAIL",
			Message: strings.Join(model.ReasonSpecs(validation.Blockers), ", "),
		}
	}
	return GovernanceHealthCheck{
		Name:    "worktree_binding",
		Status:  "OK",
		Message: "worktree binding valid",
	}
}

func checkControlsConfig(root string, change model.Change) GovernanceHealthCheck {
	// Validate governance config from .slipway.yaml (sole source).
	cfgPath, err := state.ConfigPathForChange(root, change)
	if err != nil {
		return GovernanceHealthCheck{
			Name:    "controls_config",
			Status:  "FAIL",
			Message: fmt.Sprintf("resolve .slipway.yaml path error: %v", err),
		}
	}
	var warnings []string
	cfg, cfgErr := model.LoadConfig(cfgPath)
	if cfgErr != nil {
		// Distinguish "file missing" (OK — no custom config) from "file exists but corrupt" (FAIL).
		if _, statErr := os.Stat(cfgPath); statErr == nil {
			return GovernanceHealthCheck{
				Name:    "controls_config",
				Status:  "FAIL",
				Message: fmt.Sprintf(".slipway.yaml parse error: %v", cfgErr),
			}
		}
		// File does not exist — no custom governance config, which is fine.
	} else {
		for id, mode := range cfg.Governance.Controls {
			if !id.IsValid() {
				warnings = append(warnings, fmt.Sprintf("unknown control_id in governance.controls: %q", id))
			}
			if !mode.IsValid() {
				warnings = append(warnings, fmt.Sprintf("invalid mode for governance.controls.%s: %q (must be blocking or advisory)", id, mode))
			}
		}
		for _, id := range cfg.Governance.DisabledControls {
			if !id.IsValid() {
				warnings = append(warnings, fmt.Sprintf("unknown control_id in governance.disabled_controls: %q", id))
			}
		}
		if t := cfg.Governance.Thresholds.IndependentReviewBlastRadius; t != "" && !t.IsValid() {
			warnings = append(warnings, fmt.Sprintf("invalid signal level for governance.thresholds.independent_review_blast_radius: %q", t))
		}
		if t := cfg.Governance.Thresholds.WorktreeBlastRadius; t != "" && !t.IsValid() {
			warnings = append(warnings, fmt.Sprintf("invalid signal level for governance.thresholds.worktree_blast_radius: %q", t))
		}
	}
	if len(warnings) > 0 {
		return GovernanceHealthCheck{
			Name:    "controls_config",
			Status:  "WARN",
			Message: fmt.Sprintf("parsed with warnings: %s", strings.Join(warnings, "; ")),
		}
	}
	return GovernanceHealthCheck{
		Name:    "controls_config",
		Status:  "OK",
		Message: "parsed OK",
	}
}

const signalFreshnessStaleAfter = 30 * time.Minute

// SignalFreshnessCheck evaluates whether a persisted governance snapshot is stale.
func SignalFreshnessCheck(snap model.GovernanceSnapshot) GovernanceHealthCheck {
	if snap.Version == 0 {
		return GovernanceHealthCheck{
			Name:    "signal_freshness",
			Status:  "WARN",
			Message: "no governance snapshot computed yet",
		}
	}

	age := time.Since(snap.ComputedAt)
	if age > signalFreshnessStaleAfter {
		return GovernanceHealthCheck{
			Name:    "signal_freshness",
			Status:  "WARN",
			Message: fmt.Sprintf("computed %s ago (may be stale)", age.Round(time.Minute)),
		}
	}

	return GovernanceHealthCheck{
		Name:    "signal_freshness",
		Status:  "OK",
		Message: fmt.Sprintf("computed %s ago", age.Round(time.Minute)),
	}
}

func checkTraceabilityCoherence(snap model.GovernanceSnapshot) GovernanceHealthCheck {
	if snap.Version == 0 {
		return GovernanceHealthCheck{
			Name:    "traceability_coherence",
			Status:  "WARN",
			Message: "no snapshot to check",
		}
	}

	switch snap.Traceability.Status {
	case model.TraceabilityStatusOK:
		return GovernanceHealthCheck{
			Name:    "traceability_coherence",
			Status:  "OK",
			Message: snap.Traceability.Message,
		}
	case model.TraceabilityStatusWarning:
		return GovernanceHealthCheck{
			Name:             "traceability_coherence",
			Status:           "WARN",
			Message:          snap.Traceability.Message,
			TraceabilityGaps: snap.Traceability.Gaps,
		}
	default:
		return GovernanceHealthCheck{
			Name:             "traceability_coherence",
			Status:           "FAIL",
			Message:          snap.Traceability.Message,
			TraceabilityGaps: snap.Traceability.Gaps,
		}
	}
}

func checkSignalControlCoherence(root string, change model.Change, snap model.GovernanceSnapshot) GovernanceHealthCheck {
	if snap.Version == 0 {
		return GovernanceHealthCheck{
			Name:    "signal_control_coherence",
			Status:  "WARN",
			Message: "no snapshot to check",
		}
	}

	worktreeValidation, err := state.ValidateChangeWorktree(root, change)
	if err != nil {
		return GovernanceHealthCheck{
			Name:    "signal_control_coherence",
			Status:  "FAIL",
			Message: fmt.Sprintf("worktree validation error: %v", err),
		}
	}
	if len(worktreeValidation.Blockers) > 0 {
		return GovernanceHealthCheck{
			Name:    "signal_control_coherence",
			Status:  "WARN",
			Message: fmt.Sprintf("skipped because worktree binding is invalid: %s", strings.Join(model.ReasonSpecs(worktreeValidation.Blockers), ", ")),
		}
	}

	executionSummaryCtx, err := state.LoadRelevantExecutionSummaryContext(root, change)
	if err != nil {
		return GovernanceHealthCheck{
			Name:    "signal_control_coherence",
			Status:  "FAIL",
			Message: fmt.Sprintf("execution summary invalid: %v", err),
		}
	}
	if len(executionSummaryCtx.Issues) > 0 {
		return GovernanceHealthCheck{
			Name:    "signal_control_coherence",
			Status:  "FAIL",
			Message: fmt.Sprintf("execution summary blockers require refresh: %s", strings.Join(executionSummaryCtx.Issues, ", ")),
		}
	}

	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		return GovernanceHealthCheck{
			Name:    "signal_control_coherence",
			Status:  "FAIL",
			Message: fmt.Sprintf("bundle path: %v", err),
		}
	}

	expected, _, err := deriveGovernanceControls(root, change, bundleDir, nil)
	if err != nil {
		return GovernanceHealthCheck{
			Name:    "signal_control_coherence",
			Status:  "FAIL",
			Message: fmt.Sprintf("governance config: %v", err),
		}
	}

	activeMap := map[model.ControlID]model.ControlMode{}
	for _, ctrl := range snap.ActiveControls {
		activeMap[ctrl.ControlID] = ctrl.Mode
	}

	expectedMap := map[model.ControlID]model.ControlMode{}
	var missing []string
	var staleMode []string
	for _, ctrl := range expected.ActiveControls {
		expectedMap[ctrl.ControlID] = ctrl.Mode
		if mode, ok := activeMap[ctrl.ControlID]; !ok {
			missing = append(missing, string(ctrl.ControlID))
		} else if mode != ctrl.Mode {
			staleMode = append(staleMode, fmt.Sprintf("%s(snapshot=%s,expected=%s)", ctrl.ControlID, mode, ctrl.Mode))
		}
	}

	var unexpected []string
	for _, ctrl := range snap.ActiveControls {
		if _, ok := expectedMap[ctrl.ControlID]; !ok {
			unexpected = append(unexpected, string(ctrl.ControlID))
		}
	}

	if len(missing) > 0 || len(unexpected) > 0 || len(staleMode) > 0 {
		slices.Sort(missing)
		slices.Sort(unexpected)
		slices.Sort(staleMode)
		parts := make([]string, 0, 3)
		if len(missing) > 0 {
			parts = append(parts, fmt.Sprintf("expected controls missing: %v", missing))
		}
		if len(unexpected) > 0 {
			parts = append(parts, fmt.Sprintf("unexpected active controls: %v", unexpected))
		}
		if len(staleMode) > 0 {
			parts = append(parts, fmt.Sprintf("stale control mode: %v", staleMode))
		}
		return GovernanceHealthCheck{
			Name:    "signal_control_coherence",
			Status:  "FAIL",
			Message: strings.Join(parts, "; "),
		}
	}

	return GovernanceHealthCheck{
		Name:    "signal_control_coherence",
		Status:  "OK",
		Message: "active controls match current signals",
	}
}

// PreviewGovernanceSnapshot computes the current governance sidecar payload
// without mutating the on-disk snapshot.
func PreviewGovernanceSnapshot(
	root string,
	change model.Change,
	bundleDir string,
) (model.GovernanceSnapshot, error) {
	return buildGovernanceSnapshot(root, change, bundleDir, model.GovernanceSnapshot{})
}

// RecomputeGovernanceSnapshot runs signal detection, traceability, and control evaluation,
// then persists the snapshot if it has materially changed.
func RecomputeGovernanceSnapshot(
	root string,
	change model.Change,
	bundleDir string,
) (model.GovernanceSnapshot, error) {
	existing, err := loadExistingSnapshotForRecompute(root, change.Slug)
	if err != nil {
		return model.GovernanceSnapshot{}, err
	}
	snap, err := buildGovernanceSnapshot(root, change, bundleDir, existing)
	if err != nil {
		return model.GovernanceSnapshot{}, err
	}
	if err := SaveSnapshot(root, change.Slug, snap); err != nil {
		return snap, err
	}

	return snap, nil
}

func buildGovernanceSnapshot(
	root string,
	change model.Change,
	bundleDir string,
	existing model.GovernanceSnapshot,
) (model.GovernanceSnapshot, error) {
	deriveResult, traceability, err := deriveGovernanceControls(root, change, bundleDir, existing.ActiveControls)
	if err != nil {
		return model.GovernanceSnapshot{}, err
	}

	now := time.Now().UTC()
	snap := model.GovernanceSnapshot{
		Version:        model.GovernanceSnapshotVersion,
		Summary:        deriveResult.Summary,
		Observations:   deriveResult.Observations,
		Traceability:   traceability,
		ActiveControls: deriveResult.ActiveControls,
		ComputedAt:     now,
	}

	return snap, nil
}

func deriveGovernanceControls(
	root string,
	change model.Change,
	bundleDir string,
	existingControls []model.ControlActivation,
) (control.DeriveControlsResult, model.TraceabilitySummary, error) {
	traceability := EvaluateTraceability(TraceabilityInput{
		BundleDir:      bundleDir,
		Slug:           change.Slug,
		SchemaName:     change.ArtifactSchema,
		LifecycleState: change.CurrentState,
	})

	presetPolicy, err := ResolvePresetPolicy(root, change)
	if err != nil {
		return control.DeriveControlsResult{}, traceability, err
	}

	// Light effective preset: downgrade non-intent blocking gaps to advisory.
	// Uses EffectivePreset (not raw WorkflowPreset) so that min_preset and
	// guardrail-domain upgrades are respected.
	if presetPolicy.EffectivePreset == model.WorkflowPresetLight {
		traceability = downgradeAuditGapsForLightPreset(traceability)
	}

	overrides := presetPolicy.Overrides
	policySource := model.BuiltinPolicySource
	if overrides != nil {
		policySource = model.OverridePolicySource
	}
	executionSummary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
	if err != nil {
		return control.DeriveControlsResult{}, traceability, err
	}
	latestRunVersion := 0
	taskRuns := map[string]model.TaskRun{}
	if executionSummary != nil {
		latestRunVersion = executionSummary.RunSummaryVersion
		taskRuns = executionSummary.TaskRunMap()
	}

	deriveResult := control.DeriveControls(control.DeriveControlsInput{
		GuardrailDomain:     change.GuardrailDomain,
		NeedsDiscovery:      change.NeedsDiscovery,
		ExecutionRunVersion: latestRunVersion,
		TaskResults:         taskRuns,
		PlannedTargetFiles:  loadPlannedTargetFiles(bundleDir),
		Traceability:        traceability,
		ExistingControls:    existingControls,
		PolicySource:        policySource,
		Overrides:           overrides,
	})
	return deriveResult, traceability, nil
}

func loadPlannedTargetFiles(bundleDir string) []string {
	if strings.TrimSpace(bundleDir) == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(bundleDir, "tasks.md"))
	if err != nil {
		return nil
	}
	plan, err := wave.ParseTaskPlan(string(raw))
	if err != nil {
		return nil
	}

	files := make([]string, 0)
	for _, task := range plan.Tasks {
		files = append(files, task.TargetFiles...)
	}
	if len(files) == 0 {
		return nil
	}
	return files
}

func loadExistingSnapshotForRecompute(root, slug string) (model.GovernanceSnapshot, error) {
	existing, err := LoadSnapshot(root, slug)
	if err != nil {
		if _, backupErr := BackupUnreadableSnapshot(root, slug, time.Now().UTC()); backupErr != nil {
			return model.GovernanceSnapshot{}, backupErr
		}
		return model.GovernanceSnapshot{}, nil
	}
	return existing, nil
}

func snapshotReadFailureReport(root string, change model.Change, err error) GovernanceHealthReport {
	checks := make([]GovernanceHealthCheck, 0, 5)
	configCheck := checkControlsConfig(root, change)
	checks = append(checks, configCheck)
	message := fmt.Sprintf("governance_audit_data_unavailable: %v", err)
	for _, name := range []string{
		"signal_freshness",
		"traceability_coherence",
		"signal_control_coherence",
	} {
		checks = append(checks, GovernanceHealthCheck{
			Name:    name,
			Status:  "WARN",
			Message: message,
		})
	}
	worktreeCheck := checkWorktreeBinding(root, change)
	checks = append(checks, worktreeCheck)
	healthy := configCheck.Status != "FAIL" && worktreeCheck.Status != "FAIL"
	return GovernanceHealthReport{
		Slug:    change.Slug,
		Checks:  checks,
		Healthy: healthy,
	}
}
