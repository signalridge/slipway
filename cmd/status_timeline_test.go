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
