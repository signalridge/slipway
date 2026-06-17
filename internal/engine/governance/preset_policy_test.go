package governance

import (
	"os"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePresetPolicyFromConfig_LightPresetEnablesLowCeremonyPolicy(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	change := model.NewChange("light-policy")
	change.WorkflowPreset = model.WorkflowPresetLight

	policy := resolvePresetPolicyFromConfig(cfg, change)

	assert.Equal(t, model.WorkflowPresetLight, policy.EffectivePreset)
	assert.True(t, policy.ReviewAutoPassEnabled)
	assert.True(t, policy.VerifyAutoPassEnabled)
	assert.Equal(t, 2, policy.MaxPlanAuditIterations)
	require.NotNil(t, policy.Overrides)
	assert.Equal(t, model.ControlModeAdvisory, policy.Overrides.ModeOverrides[model.ControlIndependentReview])
	assert.Equal(t, model.ControlModeAdvisory, policy.Overrides.ModeOverrides[model.ControlSecurityReview])
}

func TestResolvePresetPolicyFromConfig_GuardrailNoLongerFloorsLightToStandard(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	change := model.NewChange("guardrail-light")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.GuardrailDomain = model.GuardrailDomainAuthAuthZ

	policy := resolvePresetPolicyFromConfig(cfg, change)

	assert.Equal(t, model.WorkflowPresetLight, policy.EffectivePreset,
		"guardrail domain must no longer force preset upgrade")
	assert.Empty(t, policy.UpgradeReasons,
		"no upgrade reasons when guardrail domain does not force upgrade")
}

func TestResolvePresetPolicyFromConfig_MinPresetFloorsConfirmedPreset(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Governance.MinPreset = model.WorkflowPresetStrict
	change := model.NewChange("min-floor")
	change.WorkflowPreset = model.WorkflowPresetLight

	policy := resolvePresetPolicyFromConfig(cfg, change)

	assert.Equal(t, model.WorkflowPresetStrict, policy.EffectivePreset)
	assert.True(t, policy.CloseoutRefreshRequired)
	assert.Contains(t, policy.UpgradeReasons, "project_min_preset=strict")
}

func TestResolvePresetPolicy_MalformedConfigFailsClosed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Write a corrupt .slipway.yaml that exists but is not valid YAML.
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("{{invalid yaml"), 0o644))

	change := model.NewChange("fail-closed-test")
	change.WorkflowPreset = model.WorkflowPresetLight

	_, err := ResolvePresetPolicy(root, change)
	assert.Error(t, err, "malformed .slipway.yaml must cause a fail-closed error, not silent default fallback")
	assert.Contains(t, err.Error(), "fail-closed")
}

func TestResolvePresetPolicy_MissingConfigUsesDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// No .slipway.yaml at all — should succeed with defaults.

	change := model.NewChange("no-config")
	change.WorkflowPreset = model.WorkflowPresetLight

	policy, err := ResolvePresetPolicy(root, change)
	require.NoError(t, err)
	assert.Equal(t, model.WorkflowPresetLight, policy.EffectivePreset)
}

func TestResolvePresetPolicyUsesCanonicalConfigForBoundWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	worktreeRoot := t.TempDir()
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("governance:\n  min_preset: strict\n"), 0o644))
	require.NoError(t, os.WriteFile(state.ConfigPath(worktreeRoot), []byte("governance:\n  min_preset: light\n"), 0o644))

	change := model.NewChange("bound-config")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.WorktreePath = worktreeRoot

	policy, err := ResolvePresetPolicy(root, change)
	require.NoError(t, err)
	assert.Equal(t, model.WorkflowPresetStrict, policy.EffectivePreset)
}

func TestResolvePresetPolicyBoundWorkspaceMissingLocalCopyStillUsesCanonicalConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	worktreeRoot := t.TempDir()
	require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte("governance:\n  min_preset: strict\n"), 0o644))

	change := model.NewChange("bound-config-missing-local-copy")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.WorktreePath = worktreeRoot

	policy, err := ResolvePresetPolicy(root, change)
	require.NoError(t, err)
	assert.Equal(t, model.WorkflowPresetStrict, policy.EffectivePreset)
}

func TestResolvePresetPolicyFromConfig_StrictPresetBehavior(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	change := model.NewChange("strict-test")
	change.WorkflowPreset = model.WorkflowPresetStrict

	policy := resolvePresetPolicyFromConfig(cfg, change)

	assert.Equal(t, model.WorkflowPresetStrict, policy.EffectivePreset)
	assert.True(t, policy.CloseoutRefreshRequired, "strict implies closeout refresh")
	assert.False(t, policy.ReviewAutoPassEnabled, "strict disables review auto-pass")
	assert.False(t, policy.VerifyAutoPassEnabled, "strict disables verify auto-pass")
	require.NotNil(t, policy.Overrides)
	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlIndependentReview])
	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlSecurityReview])
	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlRollbackRequired])
	assert.Equal(t, model.SignalLevelMedium, policy.Overrides.SecurityReviewBlastRadius)
}

func TestResolvePresetPolicyFromConfig_LightNeverDemotesClarificationOrExploration(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	change := model.NewChange("light-failclosed")
	change.WorkflowPreset = model.WorkflowPresetLight

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	// Clarification and research must never appear as advisory in light
	// preset overrides. Their built-in defaults (blocking) must remain.
	_, hasClarification := policy.Overrides.ModeOverrides[model.ControlClarification]
	_, hasResearch := policy.Overrides.ModeOverrides[model.ControlResearch]
	assert.False(t, hasClarification, "light must not override clarification mode")
	assert.False(t, hasResearch, "light must not override research mode")
}

func TestResolvePresetPolicyFromConfig_StrictSetsDomainReviewBlocking(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	change := model.NewChange("strict-domain")
	change.WorkflowPreset = model.WorkflowPresetStrict

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlDomainReview],
		"strict must explicitly set domain-review to blocking")
}

func TestResolvePresetPolicyFromConfig_ProjectOverrideCanWeakenPresetDefault(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Governance.Controls = map[model.ControlID]model.ControlMode{
		model.ControlIndependentReview: model.ControlModeAdvisory,
		model.ControlSecurityReview:    model.ControlModeAdvisory,
	}
	change := model.NewChange("strict-project-override")
	change.WorkflowPreset = model.WorkflowPresetStrict

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	// Project override (applied after preset defaults) takes precedence.
	assert.Equal(t, model.ControlModeAdvisory, policy.Overrides.ModeOverrides[model.ControlIndependentReview],
		"project per-control override must take precedence over strict preset default")
	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlSecurityReview],
		"security-review must remain blocking under strict even when project config tries to weaken it")
}

func TestResolvePresetPolicyFromConfig_StandardSecurityReviewOverrideCannotWeakenBlocking(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Governance.Controls = map[model.ControlID]model.ControlMode{
		model.ControlSecurityReview: model.ControlModeAdvisory,
	}
	change := model.NewChange("standard-security-override")
	change.WorkflowPreset = model.WorkflowPresetStandard

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlSecurityReview],
		"security-review must remain blocking under standard even when project config tries to weaken it")
}

func TestResolvePresetPolicyFromConfig_StrictSecurityReviewThresholdCannotBeWeakened(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Governance.Thresholds.SecurityReviewBlastRadius = model.SignalLevelHigh
	change := model.NewChange("strict-security-threshold")
	change.WorkflowPreset = model.WorkflowPresetStrict

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	assert.Equal(t, model.SignalLevelMedium, policy.Overrides.SecurityReviewBlastRadius,
		"strict preset must select security-review at medium blast radius even when config requests high")
}

func TestResolvePresetPolicyFromConfig_PendingConfirmationUseSuggested(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	change := model.NewChange("pending-test")
	// No confirmed preset, only a suggestion.
	change.SuggestedWorkflowPreset = model.WorkflowPresetLight

	policy := resolvePresetPolicyFromConfig(cfg, change)

	assert.True(t, policy.PendingConfirmation)
	assert.Equal(t, model.WorkflowPresetLight, policy.EffectivePreset)
	// Auto-pass should be disabled while confirmation is pending.
	assert.False(t, policy.ReviewAutoPassEnabled, "auto-pass disabled while pending confirmation")
	assert.False(t, policy.VerifyAutoPassEnabled, "auto-pass disabled while pending confirmation")
}

func TestResolvePresetPolicyFromConfig_GuardrailDomainForcesFailClosedControls(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Governance.DisabledControls = []model.ControlID{
		model.ControlDomainReview,
		model.ControlRollbackRequired,
	}
	cfg.Governance.Controls = map[model.ControlID]model.ControlMode{
		model.ControlDomainReview:     model.ControlModeAdvisory,
		model.ControlRollbackRequired: model.ControlModeAdvisory,
	}
	change := model.NewChange("guardrail-fail-closed")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.GuardrailDomain = model.GuardrailDomainSchemaDataMigration

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlDomainReview],
		"guardrail domains must force domain-review blocking")
	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlRollbackRequired],
		"schema/irreversible guardrail domains must force rollback-required blocking")
	assert.NotContains(t, policy.Overrides.DisabledControls, model.ControlDomainReview,
		"fail-closed domain-review must not be disabled by project config")
	assert.NotContains(t, policy.Overrides.DisabledControls, model.ControlRollbackRequired,
		"fail-closed rollback-required must not be disabled by project config")
}

func TestResolvePresetPolicyFromConfig_NonRollbackGuardrailDoesNotForceRollback(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Governance.DisabledControls = []model.ControlID{model.ControlRollbackRequired}
	change := model.NewChange("guardrail-auth")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.GuardrailDomain = model.GuardrailDomainAuthAuthZ

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlDomainReview],
		"all guardrail domains must force domain-review blocking")
	_, hasRollbackMode := policy.Overrides.ModeOverrides[model.ControlRollbackRequired]
	assert.False(t, hasRollbackMode,
		"non-rollback guardrail domains must not force rollback-required blocking")
	assert.Contains(t, policy.Overrides.DisabledControls, model.ControlRollbackRequired,
		"non-rollback guardrail domains may still leave rollback-required disabled")
}

func TestResolvePresetPolicyFromConfig_LightPlanAuditIterationFloor(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Execution.MaxPlanAuditIterations = 1
	change := model.NewChange("light-floor")
	change.WorkflowPreset = model.WorkflowPresetLight

	policy := resolvePresetPolicyFromConfig(cfg, change)

	// "1 is too hostile for light; keep one retry loop" — floor at 2.
	assert.Equal(t, 2, policy.MaxPlanAuditIterations,
		"light preset must floor plan-audit iterations at 2, not honor config value of 1")
}

func TestResolvePresetPolicyFromConfig_StandardHonorsLowIterationConfig(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Execution.MaxPlanAuditIterations = 1
	change := model.NewChange("standard-low-iter")
	change.WorkflowPreset = model.WorkflowPresetStandard

	policy := resolvePresetPolicyFromConfig(cfg, change)

	// Standard preset does not impose a floor — it respects project config.
	assert.Equal(t, 1, policy.MaxPlanAuditIterations,
		"standard preset should honor project config iteration count")
}
