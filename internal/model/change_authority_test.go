package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangeAuthorityTransitionClearsSubstepsForDestinationState(t *testing.T) {
	t.Parallel()

	change := NewChange("authority-transition")
	change.CurrentState = StateS1Plan
	change.IntakeSubStep = IntakeSubStepConfirm
	change.PlanSubStep = PlanSubStepAudit
	change.LastAutoPassedStates = []AutoPassedState{{State: StateS3Review, Reason: "no_blocking_review_obligations"}}

	cleared := change.TransitionTo(StateS2Implement)
	if change.ClearAutoPassHistory() {
		cleared = append(cleared, "last_auto_passed_states")
	}

	assert.Equal(t, StateS2Implement, change.CurrentState)
	assert.Equal(t, IntakeSubStepNone, change.IntakeSubStep)
	assert.Equal(t, PlanSubStepNone, change.PlanSubStep)
	assert.Nil(t, change.LastAutoPassedStates)
	assert.Equal(t, []string{"intake_substep", "plan_substep", "last_auto_passed_states"}, cleared)
}

func TestChangeAuthorityEnterPlanningSeedsDiscoveryAwarePlanSubstep(t *testing.T) {
	t.Parallel()

	change := NewChange("authority-enter-planning")
	change.IntakeSubStep = IntakeSubStepConfirm

	cleared := change.EnterPlanning(true)

	assert.Equal(t, StateS1Plan, change.CurrentState)
	assert.True(t, change.NeedsDiscovery)
	assert.Equal(t, IntakeSubStepNone, change.IntakeSubStep)
	assert.Equal(t, PlanSubStepResearch, change.PlanSubStep)
	assert.Equal(t, []string{"intake_substep"}, cleared)
}

func TestChangeAuthorityRecordEvidenceRefNormalizesAndClears(t *testing.T) {
	t.Parallel()

	change := NewChange("authority-evidence-ref")

	assert.True(t, change.RecordEvidenceRef(" plan-audit ", " feedback "))
	assert.Equal(t, "feedback", change.EvidenceRefs["plan-audit"])
	assert.False(t, change.RecordEvidenceRef("plan-audit", "feedback"))
	assert.True(t, change.RecordEvidenceRef("plan-audit", ""))
	assert.NotContains(t, change.EvidenceRefs, "plan-audit")
}
