package state

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendLifecycleEventVerified(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("event-log-change")
	require.NoError(t, SaveChange(root, change))

	recorded, err := AppendLifecycleEvent(root, change, LifecycleEvent{
		EventID:    "event-1",
		OccurredAt: time.Date(2026, 5, 24, 1, 2, 3, 0, time.UTC),
		Command:    "run",
		EventType:  "state.transitioned",
		Action:     "advanced",
		Result:     "advanced",
		GateID:     "G_plan",
		EvidenceRefs: map[string]string{
			"plan-audit": "verification/plan-audit.yaml",
			"blank":      " ",
		},
		BeforeState: model.StateS1Plan,
		AfterState:  model.StateS2Implement,
	})
	require.NoError(t, err)

	assert.Equal(t, 1, recorded.Version)
	assert.Equal(t, "event-log-change", recorded.ChangeSlug)
	assert.NotEmpty(t, recorded.CorrelationID)
	assert.Equal(t, "cli", recorded.ActorKind)
	assert.Equal(t, "G_plan", recorded.GateID)
	assert.Equal(t, map[string]string{"plan-audit": "verification/plan-audit.yaml"}, recorded.EvidenceRefs)

	events, err := ReadLifecycleEvents(root, change)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "event-1", events[0].EventID)
	assert.Equal(t, "state.transitioned", events[0].EventType)

	eventPath, err := LifecycleEventLogPath(root, change)
	require.NoError(t, err)
	expectedPath, err := NormalizePath(filepath.Join(root, "artifacts", "changes", change.Slug, "events", LifecycleEventLogFileName))
	require.NoError(t, err)
	assert.Equal(t, expectedPath, eventPath)
}

func TestReadLifecycleEventsMissingLogIsEmpty(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("missing-event-log")
	require.NoError(t, SaveChange(root, change))

	events, err := ReadLifecycleEvents(root, change)
	require.NoError(t, err)
	assert.Empty(t, events)
}
