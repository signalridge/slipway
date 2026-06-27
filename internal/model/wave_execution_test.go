package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWavePlanNormalizeDefaultsRunSummaryVersion(t *testing.T) {
	t.Parallel()

	plan := WavePlan{
		Version: WavePlanVersion,
		GeneratedAt: time.Date(
			2026,
			6,
			22,
			1,
			2,
			3,
			0,
			time.UTC,
		),
		TotalTasks: 1,
		Waves: []WavePlanWave{{
			WaveIndex: 1,
			Tasks:     []WavePlanTask{{TaskID: "t-01"}},
		}},
	}

	plan.Normalize()
	assert.Equal(t, 1, plan.RunSummaryVersion)
	require.NoError(t, plan.Validate())
}

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

func TestNormalizePublicPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "slash path", raw: "internal/wave.go", want: "internal/wave.go"},
		{name: "windows separators", raw: `internal\wave.go`, want: "internal/wave.go"},
		{name: "dot segments", raw: `./internal/wave/../model/wave_execution.go`, want: "internal/model/wave_execution.go"},
		{name: "blank", raw: "  ", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, NormalizePublicPath(tt.raw))
		})
	}
}

func TestPublicPathValidationHelpers(t *testing.T) {
	t.Parallel()

	assert.True(t, PublicPathIsAbs("/tmp/file"))
	assert.True(t, PublicPathIsAbs(`C:\tmp\file`))
	assert.False(t, PublicPathIsAbs(`cmd\run.go`))
	assert.True(t, PublicPathHasParentTraversal(`cmd\..\run.go`))
	assert.False(t, PublicPathHasParentTraversal(`cmd\run.go`))
}

func TestWaveDispatchModesFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record VerificationRecord
		want   map[int]WaveDispatchMode
	}{
		{
			name: "references token",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=2:degraded_sequential"},
			},
			want: map[int]WaveDispatchMode{2: WaveDispatchDegradedSequential},
		},
		{
			name: "notes token is ignored",
			record: VerificationRecord{
				Notes: "no degraded `dispatch_mode:wave=1:degraded_sequential` was needed",
			},
		},
		{
			name: "no tokens",
			record: VerificationRecord{
				References: []string{"tool:go-test"},
				Notes:      "all waves used the default path",
			},
		},
		{
			name: "malformed token is ignored",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=1"},
			},
		},
		{
			name: "invalid wave index is ignored",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=0:degraded_sequential"},
			},
		},
		{
			name: "invalid mode is ignored",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=1:sequential"},
			},
		},
		{
			name: "conflicting mode is dropped",
			record: VerificationRecord{
				References: []string{
					"dispatch_mode:wave=1:degraded_sequential",
					"dispatch_mode:wave=1:parallel_subagents",
				},
			},
		},
		{
			name: "conflicting wave stays dropped after later valid token",
			record: VerificationRecord{
				References: []string{
					"dispatch_mode:wave=1:degraded_sequential",
					"dispatch_mode:wave=1:parallel_subagents",
					"dispatch_mode:wave=1:degraded_sequential",
				},
			},
		},
		{
			name: "valid token survives unrelated invalid token",
			record: VerificationRecord{
				References: []string{
					"dispatch_mode:wave=1:degraded_sequential",
					"dispatch_mode:wave=2:sequential",
				},
			},
			want: map[int]WaveDispatchMode{1: WaveDispatchDegradedSequential},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := WaveDispatchModesFromVerification(tt.record)
			require.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExecutorAgentHandlesFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record VerificationRecord
		want   map[int]map[string]string
	}{
		{
			name: "single handle",
			record: VerificationRecord{
				References: []string{"executor_agent:wave=1:task=t-01:agent-abc"},
			},
			want: map[int]map[string]string{1: {"t-01": "agent-abc"}},
		},
		{
			name: "multiple waves and tasks",
			record: VerificationRecord{
				References: []string{
					"executor_agent:wave=1:task=t-01:agent-a",
					"executor_agent:wave=1:task=t-02:agent-b",
					"executor_agent:wave=2:task=t-03:agent-c",
				},
			},
			want: map[int]map[string]string{
				1: {"t-01": "agent-a", "t-02": "agent-b"},
				2: {"t-03": "agent-c"},
			},
		},
		{
			name: "colon inside handle is preserved",
			record: VerificationRecord{
				References: []string{"executor_agent:wave=1:task=t-01:agent:nested:id"},
			},
			want: map[int]map[string]string{1: {"t-01": "agent:nested:id"}},
		},
		{
			name: "conflicting handles for the same wave and task collapse to empty",
			record: VerificationRecord{
				References: []string{
					"executor_agent:wave=1:task=t-01:agent-old",
					"executor_agent:wave=1:task=t-01:agent-new",
				},
			},
			// REQ-005 requires exactly one handle per planned task; two different
			// handles are ambiguous, so the parser fails closed to an empty handle
			// that ExecutorAgentBlockers treats as missing.
			want: map[int]map[string]string{1: {"t-01": ""}},
		},
		{
			name: "a repeated identical handle is idempotent",
			record: VerificationRecord{
				References: []string{
					"executor_agent:wave=1:task=t-01:agent-a",
					"executor_agent:wave=1:task=t-01:agent-a",
				},
			},
			want: map[int]map[string]string{1: {"t-01": "agent-a"}},
		},
		{
			name: "unrelated reference is ignored",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=1:parallel_subagents", "tool:go-test"},
			},
		},
		{
			name: "missing handle is ignored",
			record: VerificationRecord{
				References: []string{"executor_agent:wave=1:task=t-01:"},
			},
		},
		{
			name: "missing task segment is ignored",
			record: VerificationRecord{
				References: []string{"executor_agent:wave=1:t-01:agent-a"},
			},
		},
		{
			name: "invalid wave index is ignored",
			record: VerificationRecord{
				References: []string{"executor_agent:wave=0:task=t-01:agent-a"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ExecutorAgentHandlesFromVerification(tt.record)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
