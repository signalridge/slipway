package cmd

import (
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusViewIncludesLifecycleTimeline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("timeline-status")
	change.CurrentState = model.StateS1Plan
	change.PlanSubStep = model.PlanSubStepBundle
	require.NoError(t, state.SaveChange(root, change))
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventID:     "event-status-1",
		OccurredAt:  time.Date(2026, 5, 24, 1, 2, 3, 0, time.UTC),
		Command:     "run",
		EventType:   "state.transitioned",
		Action:      "advanced",
		Result:      "advanced",
		BeforeState: model.StateS0Intake,
		AfterState:  model.StateS1Plan,
	})
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.Len(t, view.Timeline, 1)
	assert.Equal(t, "event-status-1", view.Timeline[0].EventID)
	assert.Equal(t, "state.transitioned", view.Timeline[0].EventType)
	assert.Equal(t, "2026-05-24T01:02:03Z", view.Timeline[0].OccurredAt)
}

func TestStatusTimelineMarksDuplicateTransitionReplay(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("timeline-duplicate-transition")
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))

	evidenceRefs := map[string]string{
		"wave-orchestration": "artifacts/changes/timeline-duplicate-transition/verification/wave-orchestration.yaml",
	}
	firstTransition := state.LifecycleEvent{
		EventID:      "event-transition-1",
		OccurredAt:   time.Date(2026, 6, 15, 16, 24, 40, 0, time.UTC),
		Command:      "run",
		EventType:    "state.transitioned",
		Action:       "advanced",
		Reason:       "state_progression",
		Result:       "advanced",
		BeforeState:  model.StateS2Execute,
		AfterState:   model.StateS3Review,
		EvidenceRefs: evidenceRefs,
	}
	_, err := state.AppendLifecycleEvent(root, change, firstTransition)
	require.NoError(t, err)
	_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventID:      "event-wave-evidence-1",
		OccurredAt:   time.Date(2026, 6, 15, 16, 24, 41, 0, time.UTC),
		Command:      "run",
		EventType:    "skill.evidence_recorded",
		Action:       "advanced",
		Reason:       "verification_evidence_consumed",
		Result:       "recorded",
		BeforeState:  model.StateS2Execute,
		AfterState:   model.StateS3Review,
		SkillID:      "wave-orchestration",
		EvidenceRefs: evidenceRefs,
	})
	require.NoError(t, err)

	duplicateTransition := firstTransition
	duplicateTransition.EventID = "event-transition-2"
	duplicateTransition.OccurredAt = time.Date(2026, 6, 15, 16, 30, 23, 0, time.UTC)
	_, err = state.AppendLifecycleEvent(root, change, duplicateTransition)
	require.NoError(t, err)
	_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventID:     "event-recovery-1",
		OccurredAt:  time.Date(2026, 6, 15, 17, 11, 12, 0, time.UTC),
		Command:     "run",
		EventType:   "state.transitioned",
		Action:      "advanced",
		Reason:      "stale_evidence_recovery_started",
		Result:      "advanced",
		BeforeState: model.StateS4Verify,
		AfterState:  model.StateS1Plan,
	})
	require.NoError(t, err)
	_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventID:     "event-plan-to-execute-1",
		OccurredAt:  time.Date(2026, 6, 15, 17, 16, 38, 0, time.UTC),
		Command:     "run",
		EventType:   "state.transitioned",
		Action:      "advanced",
		Reason:      "state_progression",
		Result:      "advanced",
		BeforeState: model.StateS1Plan,
		AfterState:  model.StateS2Execute,
	})
	require.NoError(t, err)
	reexecutedTransition := firstTransition
	reexecutedTransition.EventID = "event-transition-3"
	reexecutedTransition.OccurredAt = time.Date(2026, 6, 15, 17, 21, 35, 0, time.UTC)
	_, err = state.AppendLifecycleEvent(root, change, reexecutedTransition)
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.Len(t, view.Timeline, 6)
	assert.Equal(t, "state.transitioned", view.Timeline[0].EventType)
	assert.Equal(t, "skill.evidence_recorded", view.Timeline[1].EventType)
	assert.Equal(t, "state.transition.replayed", view.Timeline[2].EventType)
	assert.Equal(t, "event-transition-2", view.Timeline[2].EventID)
	assert.Equal(t, model.StateS2Execute, view.Timeline[2].FromState)
	assert.Equal(t, model.StateS3Review, view.Timeline[2].ToState)
	assert.Equal(t, "state.transitioned", view.Timeline[5].EventType)
	assert.Equal(t, "event-transition-3", view.Timeline[5].EventID)
}

func TestStatusTimelineKeepsSubstepTransitionDistinct(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("timeline-substep-transition")
	change.CurrentState = model.StateS0Intake
	change.IntakeSubStep = model.IntakeSubStepConfirm
	require.NoError(t, state.SaveChange(root, change))
	_, err := state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventID:       "event-substep-1",
		OccurredAt:    time.Date(2026, 6, 15, 16, 3, 58, 0, time.UTC),
		Command:       "run",
		EventType:     "state.substep_transitioned",
		Action:        "advanced",
		Reason:        "clarification_complete",
		Result:        "advanced",
		BeforeState:   model.StateS0Intake,
		AfterState:    model.StateS0Intake,
		BeforeSubStep: string(model.IntakeSubStepClarify),
		AfterSubStep:  string(model.IntakeSubStepConfirm),
	})
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.Len(t, view.Timeline, 1)
	assert.Equal(t, "state.substep_transitioned", view.Timeline[0].EventType)
	assert.Equal(t, model.StateS0Intake, view.Timeline[0].FromState)
	assert.Equal(t, model.StateS0Intake, view.Timeline[0].ToState)
}
