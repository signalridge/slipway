package progression

import (
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
)

// PlanGateResult captures the result of a plan gate evaluation with iteration tracking.
type PlanGateResult struct {
	Blocked                  bool
	Blockers                 []model.ReasonCode
	NextPlanAuditIterations  int
	LastCheckerFeedback      string
	ClearLastCheckerFeedback bool
	Stalled                  bool
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
		Reason:    "governance_gates_passed",
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

type AdvanceOptions struct {
	SkipAutoPass bool
	// Command names the mutating surface that requested advancement. It is
	// recorded in lifecycle trace events only; it does not affect progression.
	Command string
	// Auto enables config/flag-driven auto-advancement. When true, a pending
	// workflow-preset confirmation is auto-confirmed UPGRADE-ONLY to the
	// suggested/effective preset (never auto-downgraded) so advancement
	// continues without a manual preset hard-stop. Auto introduces no
	// force-close, bypass, or private-attestation path: every evidence gate and
	// guardrail-domain control still blocks exactly as in the non-auto path.
	Auto bool
}

// Advance advances a change through its lifecycle.
// All changes are governed and start at S1_PLAN.
func Advance(root, slug string, opts ...AdvanceOptions) (AdvanceSummary, error) {
	return AdvanceGoverned(root, slug, opts...)
}
