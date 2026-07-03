package state

import (
	"encoding/json"
	"os"
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

func TestReadLifecycleEventTailWithPredecessorTransitionIncludesContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("tail-event-log-context")
	require.NoError(t, SaveChange(root, change))

	path, err := LifecycleEventLogPath(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	payload := lifecycleEventLineWithType(t, "transition-context", change.Slug, "state.transitioned") +
		lifecycleEventLineWithType(t, "older-non-transition", change.Slug, "skill.evidence_recorded") +
		lifecycleEventLineWithType(t, "tail-non-transition", change.Slug, "state.blocked") +
		lifecycleEventLineWithType(t, "tail-transition", change.Slug, "state.transitioned")
	require.NoError(t, os.WriteFile(path, []byte(payload), 0o644))

	events, err := ReadLifecycleEventTailWithPredecessorTransitionFromPath(path, 2)
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, "transition-context", events[0].EventID)
	assert.Equal(t, "tail-non-transition", events[1].EventID)
	assert.Equal(t, "tail-transition", events[2].EventID)
}

func TestReadLifecycleEventTailWithPredecessorTransitionIncludesBoundaryContext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("tail-event-log-context-boundary")
	require.NoError(t, SaveChange(root, change))

	path, err := LifecycleEventLogPath(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	const limit = 2
	payload := lifecycleEventLineWithType(t, "transition-context-boundary", change.Slug, "state.transitioned")
	for i := 1; i < lifecyclePredecessorContextLimit(limit); i++ {
		payload += lifecycleEventLineWithType(t, "context-non-transition-"+string(rune('a'+i)), change.Slug, "skill.evidence_recorded")
	}
	payload += lifecycleEventLineWithType(t, "tail-non-transition", change.Slug, "state.blocked") +
		lifecycleEventLineWithType(t, "tail-transition", change.Slug, "state.transitioned")
	require.NoError(t, os.WriteFile(path, []byte(payload), 0o644))

	events, err := ReadLifecycleEventTailWithPredecessorTransitionFromPath(path, limit)
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, "transition-context-boundary", events[0].EventID)
	assert.Equal(t, "tail-non-transition", events[1].EventID)
	assert.Equal(t, "tail-transition", events[2].EventID)
}

func TestReadLifecycleEventTailWithPredecessorTransitionIgnoresContextOutsideBudget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("tail-event-log-context-budget")
	require.NoError(t, SaveChange(root, change))

	path, err := LifecycleEventLogPath(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	const limit = 2
	payload := "{not-json}\n" +
		lifecycleEventLineWithType(t, "transition-context-too-old", change.Slug, "state.transitioned")
	for i := 0; i < lifecyclePredecessorContextLimit(limit)+1; i++ {
		payload += lifecycleEventLineWithType(t, "context-non-transition-"+string(rune('a'+i)), change.Slug, "skill.evidence_recorded")
	}
	payload += lifecycleEventLineWithType(t, "tail-non-transition", change.Slug, "state.blocked") +
		lifecycleEventLineWithType(t, "tail-transition", change.Slug, "state.transitioned")
	require.NoError(t, os.WriteFile(path, []byte(payload), 0o644))

	events, err := ReadLifecycleEventTailWithPredecessorTransitionFromPath(path, limit)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, "tail-non-transition", events[0].EventID)
	assert.Equal(t, "tail-transition", events[1].EventID)
}

func TestReadLifecycleEventTailWithPredecessorTransitionFailsOnMalformedContextLine(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	change := model.NewChange("tail-event-log-context-malformed")
	require.NoError(t, SaveChange(root, change))

	path, err := LifecycleEventLogPath(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	payload := lifecycleEventLineWithType(t, "transition-context", change.Slug, "state.transitioned") +
		"{not-json}\n" +
		lifecycleEventLineWithType(t, "tail-non-transition", change.Slug, "state.blocked") +
		lifecycleEventLineWithType(t, "tail-transition", change.Slug, "state.transitioned")
	require.NoError(t, os.WriteFile(path, []byte(payload), 0o644))

	_, err = ReadLifecycleEventTailWithPredecessorTransitionFromPath(path, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode lifecycle event log context line")
}

func lifecycleEventLineWithType(t *testing.T, eventID string, slug string, eventType string) string {
	t.Helper()
	raw, err := json.Marshal(LifecycleEvent{
		Version:    1,
		EventID:    eventID,
		ChangeSlug: slug,
		OccurredAt: time.Date(2026, 6, 1, 1, 2, 3, 0, time.UTC),
		EventType:  eventType,
	})
	require.NoError(t, err)
	return string(raw) + "\n"
}
