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
}

func TestResolvePresetPolicyFromConfig_GuardrailFloorsLightToStandard(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	change := model.NewChange("guardrail-light")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.GuardrailDomain = model.GuardrailDomainAuthAuthZ

	policy := resolvePresetPolicyFromConfig(cfg, change)

	assert.Equal(t, model.WorkflowPresetStandard, policy.EffectivePreset)
	assert.Contains(t, policy.UpgradeReasons, "guardrail_domain=auth_authz")
	require.NotNil(t, policy.Overrides)
	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlDomainReview])
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
	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlRollbackRequired])
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
	}
	change := model.NewChange("strict-project-override")
	change.WorkflowPreset = model.WorkflowPresetStrict

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	// Project override (applied after preset defaults) takes precedence.
	assert.Equal(t, model.ControlModeAdvisory, policy.Overrides.ModeOverrides[model.ControlIndependentReview],
		"project per-control override must take precedence over strict preset default")
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

func TestResolvePresetPolicyFromConfig_RollbackRequiredFailClosedForSchemaMigration(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	// Project config explicitly disables rollback-required.
	cfg.Governance.DisabledControls = []model.ControlID{model.ControlRollbackRequired}
	change := model.NewChange("rollback-failclosed")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.GuardrailDomain = model.GuardrailDomainSchemaDataMigration

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	// rollback-required must be re-escalated to blocking and removed from
	// disabled list for release-sensitive guardrail domains.
	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlRollbackRequired],
		"rollback-required must be fail-closed for schema_data_migration")
	for _, id := range policy.Overrides.DisabledControls {
		assert.NotEqual(t, model.ControlRollbackRequired, id,
			"rollback-required must not be in disabled list for schema_data_migration")
	}
}

func TestResolvePresetPolicyFromConfig_RollbackRequiredFailClosedForIrreversibleOps(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Governance.DisabledControls = []model.ControlID{model.ControlRollbackRequired}
	change := model.NewChange("rollback-failclosed-irrev")
	change.WorkflowPreset = model.WorkflowPresetLight
	change.GuardrailDomain = model.GuardrailDomainIrreversibleOps

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	assert.Equal(t, model.ControlModeBlocking, policy.Overrides.ModeOverrides[model.ControlRollbackRequired],
		"rollback-required must be fail-closed for irreversible_operations")
}

func TestResolvePresetPolicyFromConfig_RollbackNotEscalatedForNonSensitiveDomain(t *testing.T) {
	t.Parallel()

	cfg := model.DefaultConfig()
	cfg.Governance.DisabledControls = []model.ControlID{model.ControlRollbackRequired}
	change := model.NewChange("rollback-nonsensitive")
	change.WorkflowPreset = model.WorkflowPresetStandard
	change.GuardrailDomain = model.GuardrailDomainPrivacyPII

	policy := resolvePresetPolicyFromConfig(cfg, change)
	require.NotNil(t, policy.Overrides)

	// For non-rollback-sensitive domains, project config can still disable
	// rollback-required (it's advisory by default for standard).
	assert.Contains(t, policy.Overrides.DisabledControls, model.ControlRollbackRequired,
		"rollback-required should remain disableable for non-sensitive domains")
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
