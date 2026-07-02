package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigExecutionForcedParallelDefault(t *testing.T) {
	t.Parallel()

	assert.True(t, ConfigExecution{}.ForcedParallel(), "unset defaults to forced")
	assert.True(t, ConfigExecution{Parallelization: ParallelizationForced}.ForcedParallel())
	assert.False(t, ConfigExecution{Parallelization: ParallelizationOff}.ForcedParallel())
}

func TestConfigGovernanceIsZero(t *testing.T) {
	t.Parallel()

	assert.True(t, ConfigGovernance{}.IsZero(), "empty governance config should be zero")

	autoProvision := false
	tests := []struct {
		name string
		cfg  ConfigGovernance
	}{
		{
			name: "preset",
			cfg:  ConfigGovernance{DefaultPreset: WorkflowPresetStrict},
		},
		{
			name: "policy pack",
			cfg:  ConfigGovernance{PolicyPacks: []PolicyPack{{Name: "local", Path: "policy.yaml"}}},
		},
		{
			name: "control mode",
			cfg:  ConfigGovernance{Controls: map[ControlID]ControlMode{ControlIndependentReview: ControlModeAdvisory}},
		},
		{
			name: "disabled control",
			cfg:  ConfigGovernance{DisabledControls: []ControlID{ControlSecurityReview}},
		},
		{
			name: "threshold",
			cfg: ConfigGovernance{
				Thresholds: ConfigGovernanceThresholds{IndependentReviewBlastRadius: SignalLevelMedium},
			},
		},
		{
			name: "auto provision pointer",
			cfg:  ConfigGovernance{AutoProvisionWorktree: &autoProvision},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.False(t, tt.cfg.IsZero())
		})
	}
}

func TestConfigValidateParallelization(t *testing.T) {
	t.Parallel()

	for _, v := range []string{"", ParallelizationForced, ParallelizationOff} {
		cfg := DefaultConfig()
		cfg.Execution.Parallelization = v
		require.NoErrorf(t, cfg.Validate(), "value %q should be valid", v)
	}

	cfg := DefaultConfig()
	cfg.Execution.Parallelization = "sometimes"
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parallelization")
	assert.Contains(t, err.Error(), "unset")
}

func TestConfigParallelizationYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Execution.Parallelization = ParallelizationOff

	out, err := cfg.ToYAML()
	require.NoError(t, err)

	back, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.Equal(t, ParallelizationOff, back.Execution.Parallelization)
	assert.False(t, back.Execution.ForcedParallel())
}

func TestConfigExecutionAutoEnabledZeroValue(t *testing.T) {
	t.Parallel()

	assert.False(t, ConfigExecution{}.AutoEnabled(), "zero value defaults to off")
}

func TestConfigExecutionAutoDefaultOffAbsentOnRoundTrip(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfigYAML([]byte("defaults:\n  artifact_schema: expanded\n"))
	require.NoError(t, err)
	assert.False(t, cfg.Execution.AutoEnabled(), "auto defaults to off when unset")

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.NotContains(t, string(out), "auto:", "auto key omitted when disabled")
}

func TestConfigExecutionAutoEnabledYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfigYAML([]byte("execution:\n  auto: true\n"))
	require.NoError(t, err)
	assert.True(t, cfg.Execution.AutoEnabled(), "auto: true parses as enabled")

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "auto: true", "auto: true emitted when enabled")

	back, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.True(t, back.Execution.AutoEnabled(), "auto round-trips losslessly")
}

// TestConfigToYAMLPersistsIsolatedGovernancePointer guards against ToYAML
// dropping auto_provision_worktree when it is the ONLY governance key set — the
// silent-loss path `config set governance.auto_provision_worktree` exposes.
func TestConfigToYAMLPersistsIsolatedGovernancePointer(t *testing.T) {
	t.Parallel()

	for _, want := range []bool{false, true} {
		cfg := DefaultConfig()
		v := want
		cfg.Governance.AutoProvisionWorktree = &v

		out, err := cfg.ToYAML()
		require.NoError(t, err)
		assert.Contains(t, string(out), "auto_provision_worktree", "isolated governance pointer must be emitted")

		back, err := ParseConfigYAML(out)
		require.NoError(t, err)
		require.NotNil(t, back.Governance.AutoProvisionWorktree, "auto_provision_worktree must survive the round-trip")
		assert.Equal(t, want, *back.Governance.AutoProvisionWorktree)
		assert.Equal(t, want, back.Governance.AutoProvisionWorktreeEnabled())
	}
}

// TestConfigToYAMLPersistsIsolatedContextRecentWork guards against ToYAML
// dropping context.recent_work when it is the only context leaf set — the
// predicate used to omit recent_work even though ProjectContext.IsZero() counts
// it.
func TestConfigToYAMLPersistsIsolatedContextRecentWork(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Context.RecentWork = "shipped PR1"

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "recent_work", "isolated context.recent_work must be emitted")

	back, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.Equal(t, "shipped PR1", back.Context.RecentWork)
}

func TestConfigSubagentsResolveSlots(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfigYAML([]byte(`
subagents:
  default:
    type: skills
    name: sliphub
    session_instructions: Keep outputs concise.
    timeout: 20m
  fix:
    type: native
    name: repair-agent
    session_instructions: Fix accepted findings only.
  verify:
    name: verifier-hub
`))
	require.NoError(t, err)

	review := cfg.ResolveSubagent(SubagentSlotReview)
	assert.Equal(t, SubagentTypeSkills, review.Type)
	assert.Equal(t, "sliphub", review.Name)
	assert.Equal(t, "Keep outputs concise.", review.SessionInstructions)
	assert.Equal(t, "20m", review.Timeout)
	require.NotNil(t, review.EngineBoundary)
	assert.True(t, review.EngineBoundary.ReadOnly)
	assert.Equal(t, "deny", review.EngineBoundary.MutationPolicy)

	fix := cfg.ResolveSubagent(SubagentSlotFix)
	assert.Equal(t, SubagentTypeNative, fix.Type)
	assert.Equal(t, "repair-agent", fix.Name)
	assert.Equal(t, "Fix accepted findings only.", fix.SessionInstructions)
	assert.Equal(t, "20m", fix.Timeout)
	require.NotNil(t, fix.EngineBoundary)
	assert.False(t, fix.EngineBoundary.ReadOnly)
	assert.Equal(t, "allow", fix.EngineBoundary.MutationPolicy)

	verify := cfg.ResolveSubagent(SubagentSlotVerify)
	assert.Equal(t, SubagentTypeSkills, verify.Type, "type inherits from default")
	assert.Equal(t, "verifier-hub", verify.Name)
	assert.Equal(t, "Keep outputs concise.", verify.SessionInstructions)
}

func TestConfigSubagentsResolveReadOnlyBoundaryWithoutUserConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	for _, slot := range []SubagentSlotName{
		SubagentSlotPlanAudit,
		SubagentSlotReview,
		SubagentSlotVerify,
	} {
		got := cfg.ResolveSubagent(slot)
		assert.Equal(t, SubagentTypeNative, got.Type, slot)
		assert.Empty(t, got.Name, slot)
		assert.Empty(t, got.SessionInstructions, slot)
		assert.Empty(t, got.Timeout, slot)
		require.NotNil(t, got.EngineBoundary, slot)
		assert.True(t, got.EngineBoundary.ReadOnly, slot)
		assert.Equal(t, "deny", got.EngineBoundary.MutationPolicy, slot)
	}

	assert.True(
		t,
		cfg.ResolveSubagent(SubagentSlotExecutor).IsZero(),
		"unconfigured executor should not emit a writable directive",
	)
	assert.True(
		t,
		cfg.ResolveSubagent(SubagentSlotFix).IsZero(),
		"unconfigured fix should not emit a writable directive",
	)
}

func TestConfigSubagentsTypeOverrideDoesNotInheritCrossProviderName(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfigYAML([]byte(`
subagents:
  default:
    type: skills
    name: sliphub
  plan_audit:
    type: native
`))
	require.NoError(t, err)

	planAudit := cfg.ResolveSubagent(SubagentSlotPlanAudit)
	assert.Equal(t, SubagentTypeNative, planAudit.Type)
	assert.Empty(t, planAudit.Name, "provider type override must not inherit a target from a different provider family")

	_, err = ParseConfigYAML([]byte(`
subagents:
  default:
    name: native-default
  review:
    type: skills
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subagents.review.name")
	assert.Contains(t, err.Error(), "default.name is not inherited across provider families")
}

func TestConfigSubagentsValidateProviderTypeAndName(t *testing.T) {
	t.Parallel()

	_, err := ParseConfigYAML([]byte(`
subagents:
  executor:
    type: webhook
    name: exec
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subagents.executor.type")
	assert.Contains(t, err.Error(), "native, mcp, skills")

	_, err = ParseConfigYAML([]byte(`
subagents:
  review:
    type: skills
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subagents.review.name")
	assert.Contains(t, err.Error(), "skills")

	_, err = ParseConfigYAML([]byte(`
subagents:
  default:
    type: skills
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subagents.default.name")
	assert.Contains(t, err.Error(), "skills")

	_, err = ParseConfigYAML([]byte(`
subagents:
  verify:
    type: mcp
`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subagents.verify.name")
	assert.Contains(t, err.Error(), "mcp")
}

func TestConfigSubagentsRejectRemovedAndSubstepKeys(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "provider profiles",
			body: `
subagent_provider_profiles:
  sliphub:
    provider: skills
`,
			want: "subagent_provider_profiles",
		},
		{
			name: "review substep",
			body: `
subagents:
  security_review:
    type: skills
    name: sliphub
`,
			want: "security_review",
		},
		{
			name: "old profile field",
			body: `
subagents:
  review:
    profile: sliphub
`,
			want: "profile",
		},
		{
			name: "old prompt field",
			body: `
subagents:
  review:
    prompt: run reviews
`,
			want: "prompt",
		},
		{
			name: "tool policy",
			body: `
subagents:
  review:
    tool_policy: restricted
`,
			want: "tool_policy",
		},
		{
			name: "old model field",
			body: `
subagents:
  review:
    model: gpt-5
`,
			want: "model",
		},
		{
			name: "allowed skills",
			body: `
subagents:
  review:
    allowed_skills: []
`,
			want: "allowed_skills",
		},
		{
			name: "allowed mcp servers",
			body: `
subagents:
  review:
    allowed_mcp_servers: []
`,
			want: "allowed_mcp_servers",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfigYAML([]byte(tt.body))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestConfigSubagentsYAMLRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Subagents.Default = SubagentSlot{
		Type:                SubagentTypeNative,
		Name:                "general-agent",
		SessionInstructions: "Use the current change bundle.",
	}
	cfg.Subagents.Review = SubagentSlot{
		Type:    SubagentTypeSkills,
		Name:    "sliphub",
		Timeout: "30m",
	}

	out, err := cfg.ToYAML()
	require.NoError(t, err)
	assert.Contains(t, string(out), "subagents:")
	assert.Contains(t, string(out), "session_instructions")

	back, err := ParseConfigYAML(out)
	require.NoError(t, err)
	assert.Equal(t, "sliphub", back.ResolveSubagent(SubagentSlotReview).Name)
	assert.Equal(t, "Use the current change bundle.", back.ResolveSubagent(SubagentSlotReview).SessionInstructions)
}
