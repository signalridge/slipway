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

func TestWaveDispatchModesFromVerification(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		record  VerificationRecord
		want    map[int]WaveDispatchMode
		wantErr string
	}{
		{
			name: "references token",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=2:degraded_sequential"},
			},
			want: map[int]WaveDispatchMode{2: WaveDispatchDegradedSequential},
		},
		{
			name: "notes token with punctuation",
			record: VerificationRecord{
				Notes: "degraded `dispatch_mode:wave=1:degraded_sequential`, then continued",
			},
			want: map[int]WaveDispatchMode{1: WaveDispatchDegradedSequential},
		},
		{
			name: "no tokens",
			record: VerificationRecord{
				References: []string{"tool:go-test"},
				Notes:      "all waves used the default path",
			},
		},
		{
			name: "malformed token",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=1"},
			},
			wantErr: "invalid wave dispatch reference",
		},
		{
			name: "invalid wave index",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=0:degraded_sequential"},
			},
			wantErr: "wave index must be >= 1",
		},
		{
			name: "invalid mode",
			record: VerificationRecord{
				References: []string{"dispatch_mode:wave=1:sequential"},
			},
			wantErr: "invalid dispatch_mode",
		},
		{
			name: "conflicting mode",
			record: VerificationRecord{
				References: []string{
					"dispatch_mode:wave=1:degraded_sequential",
					"dispatch_mode:wave=1:parallel",
				},
			},
			wantErr: "conflicting dispatch_mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := WaveDispatchModesFromVerification(tt.record)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
