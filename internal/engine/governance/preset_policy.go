package governance

import (
	"fmt"
	"os"
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
		overrides.ModeOverrides[model.ControlSecurityReview] = model.ControlModeAdvisory
		overrides.ModeOverrides[model.ControlWorktreeIsolation] = model.ControlModeAdvisory
		overrides.ModeOverrides[model.ControlRollbackRequired] = model.ControlModeAdvisory
	case model.WorkflowPresetStrict:
		overrides.ModeOverrides[model.ControlIndependentReview] = model.ControlModeBlocking
		overrides.ModeOverrides[model.ControlSecurityReview] = model.ControlModeBlocking
		overrides.ModeOverrides[model.ControlWorktreeIsolation] = model.ControlModeBlocking
		overrides.ModeOverrides[model.ControlRollbackRequired] = model.ControlModeBlocking
		overrides.ModeOverrides[model.ControlDomainReview] = model.ControlModeBlocking
		overrides.IndependentReviewBlastRadius = model.SignalLevelMedium
		overrides.SecurityReviewBlastRadius = model.SignalLevelMedium
		overrides.WorktreeBlastRadius = model.SignalLevelMedium
	}

	for id, mode := range cfg.Governance.Controls {
		overrides.ModeOverrides[id] = mode
	}
	overrides.DisabledControls = append([]model.ControlID(nil), cfg.Governance.DisabledControls...)
	overrides.DisabledControls = append(overrides.DisabledControls, change.CallerDisabledCtrls...)
	for id, mode := range change.CallerControlModes {
		overrides.ModeOverrides[id] = mode
	}
	if cfg.Governance.Thresholds.IndependentReviewBlastRadius.IsValid() {
		overrides.IndependentReviewBlastRadius = cfg.Governance.Thresholds.IndependentReviewBlastRadius
	}
	if cfg.Governance.Thresholds.SecurityReviewBlastRadius.IsValid() {
		overrides.SecurityReviewBlastRadius = cfg.Governance.Thresholds.SecurityReviewBlastRadius
	}
	if cfg.Governance.Thresholds.WorktreeBlastRadius.IsValid() {
		overrides.WorktreeBlastRadius = cfg.Governance.Thresholds.WorktreeBlastRadius
	}
	if change.CallerIndependentReviewBlastRadius.IsValid() {
		overrides.IndependentReviewBlastRadius = change.CallerIndependentReviewBlastRadius
	}
	if change.CallerWorktreeBlastRadius.IsValid() {
		overrides.WorktreeBlastRadius = change.CallerWorktreeBlastRadius
	}
	if effective != model.WorkflowPresetLight {
		if mode, ok := overrides.ModeOverrides[model.ControlSecurityReview]; ok && mode != model.ControlModeBlocking {
			overrides.ModeOverrides[model.ControlSecurityReview] = model.ControlModeBlocking
		}
	}
	if effective == model.WorkflowPresetStrict {
		overrides.SecurityReviewBlastRadius = model.SignalLevelMedium
	}
	applyFailClosedGuardrailOverrides(change, overrides)

	if len(overrides.ModeOverrides) == 0 &&
		len(overrides.DisabledControls) == 0 &&
		overrides.IndependentReviewBlastRadius == "" &&
		overrides.SecurityReviewBlastRadius == "" &&
		overrides.WorktreeBlastRadius == "" {
		return nil
	}
	return overrides
}

func applyFailClosedGuardrailOverrides(change model.Change, overrides *control.ControlOverrides) {
	if overrides == nil || strings.TrimSpace(change.GuardrailDomain) == "" {
		return
	}
	failClosed := map[model.ControlID]struct{}{
		model.ControlDomainReview: {},
	}
	if guardrailDomainRequiresRollback(change.GuardrailDomain) {
		failClosed[model.ControlRollbackRequired] = struct{}{}
	}
	for id := range failClosed {
		overrides.ModeOverrides[id] = model.ControlModeBlocking
	}
	overrides.DisabledControls = removeControls(overrides.DisabledControls, failClosed)
}

func guardrailDomainRequiresRollback(domain string) bool {
	switch strings.TrimSpace(domain) {
	case model.GuardrailDomainSchemaDataMigration, model.GuardrailDomainIrreversibleOps:
		return true
	default:
		return false
	}
}

func removeControls(values []model.ControlID, blocked map[model.ControlID]struct{}) []model.ControlID {
	if len(values) == 0 || len(blocked) == 0 {
		return values
	}
	out := values[:0]
	for _, value := range values {
		if _, found := blocked[value]; found {
			continue
		}
		out = append(out, value)
	}
	return out
}
