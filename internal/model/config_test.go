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
