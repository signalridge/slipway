package action

import (
	"github.com/signalridge/slipway/internal/model"
)

// WorkflowPath returns the ordered workflow states for a governed change.
// The outer state path is always the same regardless of needsDiscovery.
// NeedsDiscovery only affects PlanSubStep progression within S1_PLAN.
func WorkflowPath(_ bool) []model.WorkflowState {
	return []model.WorkflowState{
		model.StateS0Intake,
		model.StateS1Plan,
		model.StateS2Implement,
		model.StateS3Review,
		model.StateDone,
	}
}

func CanFinalizeDone(state model.WorkflowState) bool {
	return state == model.StateS3Review
}
