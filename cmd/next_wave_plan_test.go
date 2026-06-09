package cmd

import (
	"os"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWavePlanViewFromModelSurfacesParallel(t *testing.T) {
	t.Parallel()

	plan := model.WavePlan{
		Version:    model.WavePlanVersion,
		TotalTasks: 3,
		Waves: []model.WavePlanWave{
			{WaveIndex: 1, Parallel: true, Tasks: []model.WavePlanTask{{TaskID: "t-01"}, {TaskID: "t-02"}}},
			{WaveIndex: 2, Parallel: false, Tasks: []model.WavePlanTask{{TaskID: "t-03"}}},
		},
	}

	view := wavePlanViewFromModel(plan, true)
	require.NotNil(t, view)
	require.Len(t, view.Waves, 2)
	assert.True(t, view.Waves[0].Parallel, "multi-task wave is surfaced as parallel")
	assert.False(t, view.Waves[1].Parallel, "single-task wave is not parallel")
}

func TestAuthoritativeWavePlanViewReDerivesParallelFromCurrentConfig(t *testing.T) {
	t.Parallel()

	t.Run("stale persisted false becomes parallel by default", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		change := model.NewChange("stale-wave-plan-default")
		change.CurrentState = model.StateS2Execute
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, state.SaveWavePlan(root, change.Slug, model.WavePlan{
			Version: model.WavePlanVersion,
			GeneratedAt: time.Date(2026, 6, 9, 1, 0, 0, 0,
				time.UTC),
			TotalTasks: 2,
			Waves: []model.WavePlanWave{{
				WaveIndex: 1,
				Parallel:  false,
				Tasks: []model.WavePlanTask{
					{TaskID: "t-01"},
					{TaskID: "t-02"},
				},
			}},
		}))

		view := authoritativeWavePlanView(root, change)
		require.NotNil(t, view)
		require.Empty(t, view.ParseError)
		require.Len(t, view.Waves, 1)
		assert.True(t, view.Waves[0].Parallel)
	})

	t.Run("parallelization off suppresses stale persisted true", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		change := model.NewChange("stale-wave-plan-off")
		change.CurrentState = model.StateS2Execute
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, os.WriteFile(
			state.ConfigPath(root),
			[]byte("execution:\n  parallelization: off\n"),
			0o644,
		))
		require.NoError(t, state.SaveWavePlan(root, change.Slug, model.WavePlan{
			Version: model.WavePlanVersion,
			GeneratedAt: time.Date(2026, 6, 9, 1, 0, 0, 0,
				time.UTC),
			TotalTasks: 2,
			Waves: []model.WavePlanWave{{
				WaveIndex: 1,
				Parallel:  true,
				Tasks: []model.WavePlanTask{
					{TaskID: "t-01"},
					{TaskID: "t-02"},
				},
			}},
		}))

		view := authoritativeWavePlanView(root, change)
		require.NotNil(t, view)
		require.Empty(t, view.ParseError)
		require.Len(t, view.Waves, 1)
		assert.False(t, view.Waves[0].Parallel)
	})
}
