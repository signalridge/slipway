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
