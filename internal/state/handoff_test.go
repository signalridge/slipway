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

func TestWriteHandoffCreatesHeaderWithoutLifecycleSnapshotAndPreservesNarrative(t *testing.T) {
	root := t.TempDir()
	change := model.NewChange("demo")
	change.WorktreePath = root
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/feature/demo\n"), 0o644))

	first, err := WriteHandoff(root, change, HandoffWriteOptions{
		Now:          time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC),
		SessionOwner: "agent-a",
		Section:      "Next Session Focus",
		SectionBody:  "Implement the command layer.",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, first.Header.Generation)
	assert.Equal(t, "feature/demo", first.Header.GitBranch)
	assert.Equal(t, "fresh", first.Header.Staleness)

	raw, err := os.ReadFile(first.Path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "Implement the command layer.")
	assert.NotContains(t, string(raw), "current_state")
	assert.NotContains(t, string(raw), "next_skill")
	assert.NotContains(t, string(raw), "next_command")

	second, err := WriteHandoff(root, change, HandoffWriteOptions{
		Now:          time.Date(2026, 6, 23, 1, 3, 3, 0, time.UTC),
		SessionOwner: "agent-a",
	})
	require.NoError(t, err)
	assert.Equal(t, 2, second.Header.Generation)
	assert.Contains(t, second.Narrative, "Implement the command layer.")
}

func TestReadHandoffReportsStaleWhenLifecycleAdvancedAfterWrite(t *testing.T) {
	root := t.TempDir()
	change := model.NewChange("demo")
	change.WorktreePath = root

	writtenAt := time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC)
	_, err := WriteHandoff(root, change, HandoffWriteOptions{Now: writtenAt})
	require.NoError(t, err)
	_, err = AppendLifecycleEvent(root, change, LifecycleEvent{
		OccurredAt: writtenAt.Add(time.Minute),
		EventType:  "state.transitioned",
		Command:    "run",
		Result:     "advanced",
	})
	require.NoError(t, err)

	doc, err := ReadHandoff(root, change)
	require.NoError(t, err)
	assert.Equal(t, "stale", doc.Header.Staleness)
}

func TestHandoffRejectsInvalidEmbeddedChangeSlugBeforePathDerivation(t *testing.T) {
	root := t.TempDir()
	change := model.NewChange("../x")

	_, err := WriteHandoff(root, change, HandoffWriteOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handoff change slug")

	_, err = ReadHandoff(root, change)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid handoff change slug")

	escapedPath := filepath.Join(GitRuntimeDir(root), "x", "handoff.md")
	_, statErr := os.Stat(escapedPath)
	assert.True(t, os.IsNotExist(statErr), "invalid embedded slug must not escape changes/<slug>")
}

func TestHandoffBriefIsBoundedDescriptor(t *testing.T) {
	doc := HandoffDocument{
		Path: "/repo/.git/slipway/runtime/changes/demo/handoff.md",
		Header: HandoffHeader{
			Slug:         "demo",
			Generation:   3,
			SessionOwner: "agent-a",
			UpdatedAt:    time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC),
			Staleness:    "fresh",
		},
		Narrative: "## Next Session Focus\nFinish command tests.\n",
	}
	brief := HandoffBrief(doc)
	assert.Contains(t, brief, "slug=demo")
	assert.Contains(t, brief, "generation=3")
	assert.Contains(t, brief, "Finish command tests.")
	assert.NotContains(t, brief, "current_state")
	assert.NotContains(t, brief, "next_skill")
}

func TestHandoffHeaderKeysExcludeLifecycleAuthorityFields(t *testing.T) {
	keys := HandoffHeaderKeys()
	raw, err := json.Marshal(keys)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "current_state")
	assert.NotContains(t, string(raw), "substep")
	assert.NotContains(t, string(raw), "next_skill")
	assert.NotContains(t, string(raw), "next_command")
}
