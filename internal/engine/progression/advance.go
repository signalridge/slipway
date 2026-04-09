package progression

import (
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// PlanGateResult captures the result of a plan gate evaluation with iteration tracking.
type PlanGateResult struct {
	Blocked  bool
	Blockers []model.ReasonCode
}

func blockedAdvanceSummary(fromState model.WorkflowState, blockers []model.ReasonCode) AdvanceSummary {
	return AdvanceSummary{
		Action:    "blocked",
		FromState: fromState,
		Blockers:  blockers,
	}
}

func doneReadyAdvanceSummary(fromState model.WorkflowState, message string) AdvanceSummary {
	return AdvanceSummary{
		Action:    "done_ready",
		FromState: fromState,
		Message:   message,
		Blockers:  []model.ReasonCode{model.NewReasonCode("run_slipway_done_to_finalize", "")},
	}
}

func saveChangeAndReturn(root string, change model.Change, summary AdvanceSummary) (AdvanceSummary, error) {
	if err := state.SaveChange(root, change); err != nil {
		return AdvanceSummary{}, err
	}
	return summary, nil
}

func saveBlockedChange(root string, change model.Change, fromState model.WorkflowState, blockers []model.ReasonCode) (AdvanceSummary, error) {
	return saveChangeAndReturn(root, change, blockedAdvanceSummary(fromState, blockers))
}

// Advance advances a change through its lifecycle.
// All changes are governed and start at S1_PLAN.
func Advance(root, slug string) (AdvanceSummary, error) {
	return AdvanceGoverned(root, slug)
}
