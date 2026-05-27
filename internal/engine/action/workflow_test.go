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
		model.StateS2Execute,
		model.StateS3Review,
		model.StateS4Verify,
		model.StateDone,
	}, WorkflowPath(false))
}

func TestWorkflowPathWithDiscovery(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []model.WorkflowState{
		model.StateS0Intake,
		model.StateS1Plan,
		model.StateS2Execute,
		model.StateS3Review,
		model.StateS4Verify,
		model.StateDone,
	}, WorkflowPath(true))
}

func TestCanFinalizeDone(t *testing.T) {
	t.Parallel()
	assert.True(t, CanFinalizeDone(model.StateS4Verify))
	assert.False(t, CanFinalizeDone(model.StateS2Execute))
}
