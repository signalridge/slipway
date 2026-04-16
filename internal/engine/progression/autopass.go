package progression

import (
	"fmt"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
)

const (
	autoPassReasonNoBlockingReviewObligations  = "no_blocking_review_obligations"
	autoPassReasonNoBlockingReleaseObligations = "no_blocking_release_obligations"
)

func attemptAutoPassSequence(
	root string,
	change model.Change,
	fromState model.WorkflowState,
	startState model.WorkflowState,
) (AdvanceSummary, bool, error) {
	if startState != model.StateS3Review && startState != model.StateS4Verify {
		return AdvanceSummary{}, false, nil
	}

	candidate := change
	candidate.PlanSubStep = model.PlanSubStepNone
	current := startState
	autoPassed := make([]model.AutoPassedState, 0, 2)

	for {
		candidate.CurrentState = current
		policy, err := governance.ResolvePresetPolicy(root, candidate)
		if err != nil {
			return AdvanceSummary{}, false, err
		}
		eligible, reason, err := autoPassEligibleForState(root, candidate, policy)
		if err != nil {
			return AdvanceSummary{}, false, err
		}
		if !eligible {
			break
		}
		autoPassed = append(autoPassed, model.AutoPassedState{
			State:  current,
			Reason: reason,
		})
		if current == model.StateS4Verify {
			candidate.CurrentState = model.StateS4Verify
			candidate.LastAutoPassedStates = autoPassed
			summary, err := saveChangeAndReturn(root, candidate, AdvanceSummary{
				Action:           "done_ready",
				FromState:        fromState,
				ToState:          model.StateS4Verify,
				Reason:           "auto_pass_complete",
				Message:          "All governance gates passed. Run `slipway done` to finalize.",
				Blockers:         []model.ReasonCode{model.NewReasonCode("run_slipway_done_to_finalize", "")},
				AutoPassedStates: autoPassed,
			})
			return summary, true, err
		}
		current = model.StateS4Verify
	}

	if len(autoPassed) == 0 {
		return AdvanceSummary{}, false, nil
	}

	candidate.CurrentState = current
	candidate.LastAutoPassedStates = autoPassed
	summary, err := saveChangeAndReturn(root, candidate, AdvanceSummary{
		Action:           "advanced",
		FromState:        fromState,
		ToState:          current,
		Reason:           "auto_pass_partial",
		Message:          fmt.Sprintf("Advanced to %s.", current),
		AutoPassedStates: autoPassed,
	})
	return summary, true, err
}

// AutoPassEligibility reports which states are eligible for auto-pass without
// persisting any state change. It evaluates from the change's current state
// forward so callers never see eligibility for states already advanced past.
func AutoPassEligibility(root string, change model.Change) ([]model.AutoPassedState, error) {
	if change.CurrentState != model.StateS3Review && change.CurrentState != model.StateS4Verify {
		return nil, nil
	}
	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return nil, err
	}
	var eligible []model.AutoPassedState
	candidate := change
	for current := change.CurrentState; current != ""; {
		candidate.CurrentState = current
		ok, reason, err := autoPassEligibleForState(root, candidate, policy)
		if err != nil {
			return nil, err
		}
		if ok {
			eligible = append(eligible, model.AutoPassedState{State: current, Reason: reason})
		} else {
			break
		}
		if current == model.StateS4Verify {
			break
		}
		current = model.StateS4Verify
	}
	return eligible, nil
}

func autoPassEligibleForState(root string, change model.Change, policy governance.PresetPolicy) (bool, string, error) {
	if change.CurrentState != model.StateS3Review && change.CurrentState != model.StateS4Verify {
		return false, "", nil
	}

	switch change.CurrentState {
	case model.StateS3Review:
		reviewAuthority, err := EvaluateReviewAuthority(root, change)
		if err != nil {
			return false, "", err
		}
		readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
		if err != nil {
			return false, "", err
		}
		if !policy.ReviewAutoPassEnabled || hasUnsatisfiedBlockingAction(readiness.RequiredActions, model.ControlScopeReview) {
			return false, "", nil
		}
		if len(reviewAuthority.Blockers) > 0 {
			return false, "", nil
		}
		return true, autoPassReasonNoBlockingReviewObligations, nil
	case model.StateS4Verify:
		shipAuthority, err := EvaluateShipAuthority(root, change)
		if err != nil {
			return false, "", err
		}
		if !policy.VerifyAutoPassEnabled || hasUnsatisfiedBlockingAction(shipAuthority.Actions, model.ControlScopeRelease) {
			return false, "", nil
		}
		if shipAuthority.Result.Status != model.GateStatusApproved {
			return false, "", nil
		}
		return true, autoPassReasonNoBlockingReleaseObligations, nil
	default:
		return false, "", nil
	}
}

func hasUnsatisfiedBlockingAction(actions []governance.RequiredAction, scope model.ControlScope) bool {
	for _, action := range actions {
		if action.Scope == scope && action.Mode == model.ControlModeBlocking && !action.Satisfied {
			return true
		}
	}
	return false
}
