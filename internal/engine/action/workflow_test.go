package action

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestWorkflowPathWithoutDiscovery(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []model.WorkflowState{
		model.StateS0Intake,
		model.StateS1Plan,
		model.StateS2Implement,
		model.StateS3Review,
		model.StateDone,
	}, WorkflowPath(false))
}

func TestWorkflowPathWithDiscovery(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []model.WorkflowState{
		model.StateS0Intake,
		model.StateS1Plan,
		model.StateS2Implement,
		model.StateS3Review,
		model.StateDone,
	}, WorkflowPath(true))
}

func TestCanFinalizeDone(t *testing.T) {
	t.Parallel()
	assert.True(t, CanFinalizeDone(model.StateS3Review))
	assert.False(t, CanFinalizeDone(model.StateS2Implement))
}
