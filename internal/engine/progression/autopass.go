package progression

import (
	"fmt"

	"github.com/signalridge/slipway/internal/engine/governance"
	"github.com/signalridge/slipway/internal/model"
)

const (
	autoPassReasonNoBlockingReviewObligations  = "no_blocking_review_obligations"
	autoPassReasonNoBlockingReleaseObligations = "no_blocking_release_obligations" // #nosec G101 -- auto-pass reason constants are lifecycle status strings, not credentials.
)

func attemptAutoPassSequence(
	root string,
	change model.Change,
	fromState model.WorkflowState,
	startState model.WorkflowState,
) (AdvanceSummary, bool, error) {
	if startState != model.StateS3Review {
		return AdvanceSummary{}, false, nil
	}

	candidate := change
	candidate.PlanSubStep = model.PlanSubStepNone
	autoPassed := make([]model.AutoPassedState, 0, 2)

	policy, err := governance.ResolvePresetPolicy(root, candidate)
	if err != nil {
		return AdvanceSummary{}, false, err
	}
	candidate.CurrentState = startState

	// Review authority is a pure function of (root, change-on-disk) and no state
	// mutation occurs before the review-skill stamp below, so it is computed once
	// here and threaded through both the eligibility check and the stamp rather
	// than recomputed inside each.
	reviewAuthority, err := EvaluateReviewAuthority(root, candidate)
	if err != nil {
		return AdvanceSummary{}, false, err
	}

	eligible, reason, err := autoPassReviewEligible(root, candidate, policy, reviewAuthority)
	if err != nil {
		return AdvanceSummary{}, false, err
	}
	if !eligible {
		return AdvanceSummary{}, false, nil
	}
	stampResult, err := stampAutoPassedSkillDigests(root, candidate, reviewAuthority)
	if err != nil {
		return AdvanceSummary{}, false, err
	}
	if len(stampResult.Blockers) > 0 {
		summary := blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(stampResult.Blockers))
		summary.AutoPassedStates = autoPassed
		return summary, true, nil
	}
	autoPassed = append(autoPassed, model.AutoPassedState{
		State:  model.StateS3Review,
		Reason: reason,
	})

	// Ship authority is likewise pure. The only barrier between here and the
	// ship-skill stamp below is the review stamp already applied above, so it is
	// computed once and reused for both the eligibility check and the stamp.
	shipAuthority, err := EvaluateShipAuthority(root, candidate)
	if err != nil {
		return AdvanceSummary{}, false, err
	}

	shipEligible, shipReason := autoPassShipEligible(policy, shipAuthority)
	if !shipEligible {
		candidate.CurrentState = model.StateS3Review
		candidate.LastAutoPassedStates = autoPassed
		summary, err := saveChangeAndReturn(root, candidate, AdvanceSummary{
			Action:           "advanced",
			FromState:        fromState,
			ToState:          model.StateS3Review,
			Reason:           "auto_pass_partial",
			Message:          fmt.Sprintf("Advanced to %s.", model.StateS3Review),
			AutoPassedStates: autoPassed,
		})
		return summary, true, err
	}
	stampResult, err = stampPassingSkillDigests(root, candidate, shipAuthorityPassingSkills(shipAuthority))
	if err != nil {
		return AdvanceSummary{}, false, err
	}
	if len(stampResult.Blockers) > 0 {
		summary := blockedAdvanceSummary(fromState, model.ReasonCodesFromSpecs(stampResult.Blockers))
		summary.AutoPassedStates = autoPassed
		return summary, true, nil
	}
	autoPassed = append(autoPassed, model.AutoPassedState{
		State:  model.StateS3Review,
		Reason: shipReason,
	})
	candidate.CurrentState = model.StateS3Review
	candidate.LastAutoPassedStates = autoPassed
	summary, err := saveChangeAndReturn(root, candidate, AdvanceSummary{
		Action:           "done_ready",
		FromState:        fromState,
		ToState:          model.StateS3Review,
		Reason:           "auto_pass_complete",
		Message:          "All governance gates passed. Run `slipway done` to finalize.",
		Blockers:         []model.ReasonCode{model.NewReasonCode("run_slipway_done_to_finalize", "")},
		AutoPassedStates: autoPassed,
	})
	return summary, true, err
}

// AutoPassEligibility reports which states are eligible for auto-pass without
// persisting any state change. It evaluates from the change's current state
// forward so callers never see eligibility for states already advanced past.
func AutoPassEligibility(root string, change model.Change) ([]model.AutoPassedState, error) {
	if change.CurrentState != model.StateS3Review {
		return nil, nil
	}
	policy, err := governance.ResolvePresetPolicy(root, change)
	if err != nil {
		return nil, err
	}
	var eligible []model.AutoPassedState
	candidate := change
	candidate.CurrentState = change.CurrentState
	reviewAuthority, err := EvaluateReviewAuthority(root, candidate)
	if err != nil {
		return nil, err
	}
	ok, reason, err := autoPassReviewEligible(root, candidate, policy, reviewAuthority)
	if err != nil {
		return nil, err
	}
	if ok {
		eligible = append(eligible, model.AutoPassedState{State: model.StateS3Review, Reason: reason})
	}
	shipAuthority, err := EvaluateShipAuthority(root, candidate)
	if err != nil {
		return nil, err
	}
	ok, reason = autoPassShipEligible(policy, shipAuthority)
	if ok {
		eligible = append(eligible, model.AutoPassedState{State: model.StateS3Review, Reason: reason})
	}
	return eligible, nil
}

func autoPassReviewEligible(root string, change model.Change, policy governance.PresetPolicy, reviewAuthority ReviewAuthority) (bool, string, error) {
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
}

func autoPassShipEligible(policy governance.PresetPolicy, shipAuthority ShipAuthority) (bool, string) {
	if !policy.VerifyAutoPassEnabled || hasUnsatisfiedBlockingAction(shipAuthority.Actions, model.ControlScopeRelease) {
		return false, ""
	}
	if shipAuthority.Result.Status != model.GateStatusApproved {
		return false, ""
	}
	return true, autoPassReasonNoBlockingReleaseObligations
}

func stampAutoPassedSkillDigests(root string, change model.Change, reviewAuthority ReviewAuthority) (skillDigestStampResult, error) {
	if change.CurrentState == model.StateS3Review {
		result, err := stampPassingSkillDigests(root, change, reviewAuthority.PassingSkills)
		if err != nil {
			return skillDigestStampResult{}, err
		}
		return result, nil
	}
	return skillDigestStampResult{}, nil
}

func hasUnsatisfiedBlockingAction(actions []governance.RequiredAction, scope model.ControlScope) bool {
	for _, action := range actions {
		if action.Scope == scope && action.Mode == model.ControlModeBlocking && !action.Satisfied {
			return true
		}
	}
	return false
}
