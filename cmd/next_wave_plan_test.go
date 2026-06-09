package cmd

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
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

	view := wavePlanViewFromModel(plan)
	require.NotNil(t, view)
	require.Len(t, view.Waves, 2)
	assert.True(t, view.Waves[0].Parallel, "multi-task wave is surfaced as parallel")
	assert.False(t, view.Waves[1].Parallel, "single-task wave is not parallel")
}
