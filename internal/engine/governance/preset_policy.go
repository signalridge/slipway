package governance

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/signalridge/slipway/internal/engine/control"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

type PresetPolicy struct {
	ConfirmedPreset         model.WorkflowPreset
	SuggestedPreset         model.WorkflowPreset
	EffectivePreset         model.WorkflowPreset
	PendingConfirmation     bool
	UpgradeReasons          []string
	CloseoutRefreshRequired bool
	ReviewAutoPassEnabled   bool
	VerifyAutoPassEnabled   bool
	MaxPlanAuditIterations  int
	Overrides               *control.ControlOverrides
}

func ResolvePresetPolicy(root string, change model.Change) (PresetPolicy, error) {
	cfgPath, err := state.ConfigPathForChange(root, change)
	if err != nil {
		return PresetPolicy{}, fmt.Errorf("resolve governance config path: %w", err)
	}
	cfg, err := model.LoadConfig(cfgPath)
	if err != nil {
		// File missing is fine — use built-in defaults.
		// File exists but corrupt is a fail-closed error: a malformed config
		// could silently disable min_preset, default_preset, and control
		// overrides, degrading governance open instead of closed.
		if !os.IsNotExist(err) {
			return PresetPolicy{}, fmt.Errorf("governance config parse error (fail-closed): %w", err)
		}
		return resolvePresetPolicyFromConfig(model.DefaultConfig(), change), nil
	}
	return resolvePresetPolicyFromConfig(cfg, change), nil
}

func resolvePresetPolicyFromConfig(cfg model.Config, change model.Change) PresetPolicy {
	pending := change.WorkflowPresetConfirmationPending()
	confirmed := change.ConfirmedWorkflowPreset()
	suggested := model.WorkflowPreset("")
	if change.SuggestedWorkflowPreset.IsValid() {
		suggested = change.SuggestedWorkflowPreset
	}

	base := confirmed
	if pending && suggested.IsValid() {
		base = suggested
	}
	if !base.IsValid() {
		base = model.WorkflowPresetStandard
	}

	effective := base
	var upgradeReasons []string
	if cfg.Governance.MinPreset.IsValid() && effective.Rank() < cfg.Governance.MinPreset.Rank() {
		effective = cfg.Governance.MinPreset
		upgradeReasons = append(upgradeReasons, "project_min_preset="+string(cfg.Governance.MinPreset))
	}
	if strings.TrimSpace(change.GuardrailDomain) != "" && effective.Rank() < model.WorkflowPresetStandard.Rank() {
		effective = model.WorkflowPresetStandard
		upgradeReasons = append(upgradeReasons, "guardrail_domain="+change.GuardrailDomain)
	}

	overrides := buildPresetOverrides(change, cfg, effective)

	// Fail-safe OR: quality_mode=full and preset=strict both independently
	// trigger closeout refresh. This is the ONLY point where QualityMode
	// feeds into governance logic — it must not expand beyond this.
	closeoutRefreshRequired := change.RequiresCloseoutRefresh() || effective == model.WorkflowPresetStrict
	reviewAutoPassEnabled := effective == model.WorkflowPresetLight && !pending
	verifyAutoPassEnabled := effective == model.WorkflowPresetLight && !pending && !closeoutRefreshRequired
	maxPlanAuditIterations := cfg.Execution.MaxPlanAuditIterations
	if effective == model.WorkflowPresetLight {
		// Light preset: fixed at 2 (lighter than standard's default 3,
		// but never below 2 — "1 is too hostile; keep one retry loop").
		maxPlanAuditIterations = 2
	}

	return PresetPolicy{
		ConfirmedPreset:         confirmed,
		SuggestedPreset:         suggested,
		EffectivePreset:         effective,
		PendingConfirmation:     pending,
		UpgradeReasons:          upgradeReasons,
		CloseoutRefreshRequired: closeoutRefreshRequired,
		ReviewAutoPassEnabled:   reviewAutoPassEnabled,
		VerifyAutoPassEnabled:   verifyAutoPassEnabled,
		MaxPlanAuditIterations:  maxPlanAuditIterations,
		Overrides:               overrides,
	}
}

func buildPresetOverrides(change model.Change, cfg model.Config, effective model.WorkflowPreset) *control.ControlOverrides {
	overrides := &control.ControlOverrides{
		ModeOverrides: make(map[model.ControlID]model.ControlMode),
	}

	// Preset-driven control mode defaults per the truth table in
	// docs/dynamic-workflow-path-plan.md.
	//
	// Clarification and research are NEVER demoted by any preset.
	// Their built-in defaults (blocking) are authoritative; the preset
	// layer does not touch them.
	switch effective {
	case model.WorkflowPresetLight:
		overrides.ModeOverrides[model.ControlIndependentReview] = model.ControlModeAdvisory
		overrides.ModeOverrides[model.ControlWorktreeIsolation] = model.ControlModeAdvisory
		overrides.ModeOverrides[model.ControlRollbackRequired] = model.ControlModeAdvisory
	case model.WorkflowPresetStrict:
		overrides.ModeOverrides[model.ControlIndependentReview] = model.ControlModeBlocking
		overrides.ModeOverrides[model.ControlWorktreeIsolation] = model.ControlModeBlocking
		overrides.ModeOverrides[model.ControlRollbackRequired] = model.ControlModeBlocking
		overrides.ModeOverrides[model.ControlDomainReview] = model.ControlModeBlocking
		overrides.IndependentReviewBlastRadius = model.SignalLevelMedium
		overrides.WorktreeBlastRadius = model.SignalLevelMedium
	}

	for id, mode := range cfg.Governance.Controls {
		overrides.ModeOverrides[id] = mode
	}
	overrides.DisabledControls = append([]model.ControlID(nil), cfg.Governance.DisabledControls...)
	if cfg.Governance.Thresholds.IndependentReviewBlastRadius.IsValid() {
		overrides.IndependentReviewBlastRadius = cfg.Governance.Thresholds.IndependentReviewBlastRadius
	}
	if cfg.Governance.Thresholds.WorktreeBlastRadius.IsValid() {
		overrides.WorktreeBlastRadius = cfg.Governance.Thresholds.WorktreeBlastRadius
	}

	// Guardrail-domain protections remain fail-closed in this wave.
	if strings.TrimSpace(change.GuardrailDomain) != "" {
		overrides.ModeOverrides[model.ControlDomainReview] = model.ControlModeBlocking
		overrides.DisabledControls = removeDisabledControl(overrides.DisabledControls, model.ControlDomainReview)

		// rollback-required is fail-closed for release-sensitive guardrail
		// domains. A project config must not be able to disable rollback
		// readiness for schema migrations or irreversible operations.
		if isRollbackSensitiveDomain(change.GuardrailDomain) {
			overrides.ModeOverrides[model.ControlRollbackRequired] = model.ControlModeBlocking
			overrides.DisabledControls = removeDisabledControl(overrides.DisabledControls, model.ControlRollbackRequired)
		}
	}

	if len(overrides.ModeOverrides) == 0 &&
		len(overrides.DisabledControls) == 0 &&
		overrides.IndependentReviewBlastRadius == "" &&
		overrides.WorktreeBlastRadius == "" {
		return nil
	}
	return overrides
}

// isRollbackSensitiveDomain returns true for guardrail domains where
// rollback-required must remain fail-closed regardless of project config.
func isRollbackSensitiveDomain(domain string) bool {
	switch strings.TrimSpace(domain) {
	case model.GuardrailDomainSchemaDataMigration, model.GuardrailDomainIrreversibleOps:
		return true
	default:
		return false
	}
}

func removeDisabledControl(disabled []model.ControlID, target model.ControlID) []model.ControlID {
	filtered := make([]model.ControlID, 0, len(disabled))
	for _, id := range disabled {
		if id == target {
			continue
		}
		filtered = append(filtered, id)
	}
	slices.Sort(filtered)
	return filtered
}
