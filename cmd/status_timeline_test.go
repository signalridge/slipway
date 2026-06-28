package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
		EventID:     "event-status-2",
		OccurredAt:  time.Date(2026, 5, 24, 1, 3, 3, 0, time.UTC),
		Command:     "run",
		EventType:   "state.transitioned",
		Action:      "advanced",
		Result:      "advanced",
		BeforeState: model.StateS1Plan,
		AfterState:  model.StateS2Implement,
	})
	require.NoError(t, err)

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.Len(t, view.Timeline, 2)
	assert.Equal(t, "event-status-1", view.Timeline[0].EventID)
	assert.Equal(t, "state.transitioned", view.Timeline[0].EventType)
	assert.Equal(t, "2026-05-24T01:02:03Z", view.Timeline[0].OccurredAt)
	assert.Equal(t, model.StateS2Implement, view.Timeline[1].ToState)
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
		BeforeState:  model.StateS2Implement,
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
		BeforeState:  model.StateS2Implement,
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
		EventID:     "event-repair-1",
		OccurredAt:  time.Date(2026, 6, 15, 17, 11, 12, 0, time.UTC),
		Command:     "run",
		EventType:   "state.blocked",
		Action:      "blocked",
		Reason:      "stale_evidence_requires_review_alignment",
		Result:      "blocked",
		BeforeState: model.StateS3Review,
		AfterState:  model.StateS3Review,
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
		AfterState:  model.StateS2Implement,
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
	assert.Equal(t, model.StateS2Implement, view.Timeline[2].FromState)
	assert.Equal(t, model.StateS3Review, view.Timeline[2].ToState)
	assert.Equal(t, "state.transitioned", view.Timeline[5].EventType)
	assert.Equal(t, "event-transition-3", view.Timeline[5].EventID)
}

func TestStatusTimelineMarksDuplicateTransitionReplayOutsideRawTail(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	const displayLimit = 3
	change := model.NewChange("timeline-duplicate-transition-tail-context")
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))

	firstTransition := state.LifecycleEvent{
		EventID:     "event-transition-context",
		OccurredAt:  time.Date(2026, 6, 15, 16, 24, 40, 0, time.UTC),
		Command:     "run",
		EventType:   "state.transitioned",
		Action:      "advanced",
		Reason:      "state_progression",
		Result:      "advanced",
		BeforeState: model.StateS2Implement,
		AfterState:  model.StateS3Review,
	}
	_, err := state.AppendLifecycleEvent(root, change, firstTransition)
	require.NoError(t, err)
	for i := 0; i < displayLimit*4+1; i++ {
		_, err = state.AppendLifecycleEvent(root, change, state.LifecycleEvent{
			EventID:     fmt.Sprintf("event-tail-gap-%02d", i),
			OccurredAt:  time.Date(2026, 6, 15, 16, 25+i, 0, 0, time.UTC),
			Command:     "run",
			EventType:   "state.blocked",
			Action:      "blocked",
			Reason:      "waiting_for_evidence",
			Result:      "blocked",
			BeforeState: model.StateS3Review,
			AfterState:  model.StateS3Review,
		})
		require.NoError(t, err)
	}
	duplicateTransition := firstTransition
	duplicateTransition.EventID = "event-transition-duplicate"
	duplicateTransition.OccurredAt = time.Date(2026, 6, 15, 17, 0, 0, 0, time.UTC)
	_, err = state.AppendLifecycleEvent(root, change, duplicateTransition)
	require.NoError(t, err)

	timeline, err := buildStatusTimelineWithReadContext(newStateReadContext(root), change, displayLimit)
	require.NoError(t, err)
	require.Len(t, timeline, displayLimit)
	last := timeline[len(timeline)-1]
	assert.Equal(t, "event-transition-duplicate", last.EventID)
	assert.Equal(t, "state.transition.replayed", last.EventType)
	assert.Equal(t, "replayed", last.Result)
}

func TestStatusTimelineIgnoresMalformedPrefixOutsidePredecessorContextBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	const displayLimit = 3
	readLimit := displayLimit * 4
	change := model.NewChange("timeline-malformed-prefix-outside-context")
	change.CurrentState = model.StateS3Review
	require.NoError(t, state.SaveChange(root, change))

	firstTransition := state.LifecycleEvent{
		EventID:     "event-transition-too-old",
		OccurredAt:  time.Date(2026, 6, 15, 16, 24, 40, 0, time.UTC),
		Command:     "run",
		EventType:   "state.transitioned",
		Action:      "advanced",
		Reason:      "state_progression",
		Result:      "advanced",
		BeforeState: model.StateS2Implement,
		AfterState:  model.StateS3Review,
	}
	path, err := state.LifecycleEventLogPath(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	payload := "{not-json}\n" + statusTimelineLifecycleEventLine(t, firstTransition)
	for i := 0; i < readLimit*4+readLimit+1; i++ {
		payload += statusTimelineLifecycleEventLine(t, state.LifecycleEvent{
			EventID:     fmt.Sprintf("event-tail-gap-%02d", i),
			OccurredAt:  time.Date(2026, 6, 15, 16, 25+i, 0, 0, time.UTC),
			Command:     "run",
			EventType:   "state.blocked",
			Action:      "blocked",
			Reason:      "waiting_for_evidence",
			Result:      "blocked",
			BeforeState: model.StateS3Review,
			AfterState:  model.StateS3Review,
		})
	}
	duplicateTransition := firstTransition
	duplicateTransition.EventID = "event-transition-duplicate"
	duplicateTransition.OccurredAt = time.Date(2026, 6, 15, 17, 0, 0, 0, time.UTC)
	payload += statusTimelineLifecycleEventLine(t, duplicateTransition)
	require.NoError(t, os.WriteFile(path, []byte(payload), 0o644))

	timeline, err := buildStatusTimelineWithReadContext(newStateReadContext(root), change, displayLimit)
	require.NoError(t, err)
	require.Len(t, timeline, displayLimit)
	last := timeline[len(timeline)-1]
	assert.Equal(t, "event-transition-duplicate", last.EventID)
	assert.Equal(t, "state.transitioned", last.EventType)
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

func statusTimelineLifecycleEventLine(t *testing.T, event state.LifecycleEvent) string {
	t.Helper()
	event.Version = 1
	if event.ChangeSlug == "" {
		event.ChangeSlug = "timeline-malformed-prefix-outside-context"
	}
	raw, err := json.Marshal(event)
	require.NoError(t, err)
	return string(raw) + "\n"
}
