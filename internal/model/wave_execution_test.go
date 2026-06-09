package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWavePlanWaveParallelRequiresMultipleTasks(t *testing.T) {
	t.Parallel()

	multi := WavePlanWave{
		WaveIndex: 1,
		Parallel:  true,
		Tasks:     []WavePlanTask{{TaskID: "t-01"}, {TaskID: "t-02"}},
	}
	require.NoError(t, multi.Validate(1, map[string]struct{}{}))

	single := WavePlanWave{
		WaveIndex: 1,
		Parallel:  true,
		Tasks:     []WavePlanTask{{TaskID: "t-01"}},
	}
	err := single.Validate(1, map[string]struct{}{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2 tasks")

	serial := WavePlanWave{
		WaveIndex: 1,
		Parallel:  false,
		Tasks:     []WavePlanTask{{TaskID: "t-01"}},
	}
	require.NoError(t, serial.Validate(1, map[string]struct{}{}))
}

func TestWaveDispatchModeIsValid(t *testing.T) {
	t.Parallel()

	assert.True(t, WaveDispatchParallel.IsValid())
	assert.True(t, WaveDispatchDegradedSequential.IsValid())
	assert.False(t, WaveDispatchMode("bogus").IsValid())
	assert.False(t, WaveDispatchMode("").IsValid())
}

func TestWaveRunValidateDispatchMode(t *testing.T) {
	t.Parallel()

	base := WaveRun{WaveIndex: 1, RunSummaryVersion: 1, Verdict: WaveVerdictPass}
	require.NoError(t, base.Validate(1), "empty dispatch_mode is allowed")

	parallel := base
	parallel.DispatchMode = WaveDispatchParallel
	require.NoError(t, parallel.Validate(1))

	degraded := base
	degraded.DispatchMode = WaveDispatchDegradedSequential
	require.NoError(t, degraded.Validate(1))

	bad := base
	bad.DispatchMode = WaveDispatchMode("nope")
	err := bad.Validate(1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dispatch_mode")
}
