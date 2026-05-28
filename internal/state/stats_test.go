package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectRepoStatsSummarizesArchivesCodebaseMapAndExplorations(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	now := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)

	active := model.NewChange("active")
	require.NoError(t, SaveChange(root, active))

	archived := model.NewChange("archived-change")
	require.NoError(t, SaveChange(root, archived))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", archived.Slug), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "artifacts", "changes", archived.Slug, "change.yaml"), []byte("id: x"), 0o644))
	_, err := ArchiveChange(root, archived, model.ChangeStatusDone)
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(CodebaseMapDir(root), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(CodebaseMapDir(root), "STACK.md"), []byte("# Stack"), 0o644))
	require.NoError(t, os.Chtimes(filepath.Join(CodebaseMapDir(root), "STACK.md"), now, now))

	stats, err := CollectRepoStats(root, now)
	require.NoError(t, err)
	require.Len(t, stats.ActiveChanges, 1)
	assert.Equal(t, 1, stats.ArchiveCount)
	assert.Equal(t, 1, stats.CodebaseMap.PresentDocs)
	assert.Equal(t, 0, stats.CodebaseMap.PopulatedDocs)
	assert.Equal(t, "partial", stats.CodebaseMap.Freshness)
	assert.Contains(t, stats.CodebaseMap.MissingDocs, "INTEGRATIONS.md")
	assert.Contains(t, stats.CodebaseMap.ScaffoldOnlyDocs, "STACK.md")
}

func TestCollectRepoStatsCountsWorktreeArchiveAfterWorktreeMetadataIsRemoved(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	now := time.Now().UTC()
	change := model.NewChange("stats-worktree-archive")
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))
	_, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	stats, err := CollectRepoStats(root, now)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.ArchiveCount)
}

func TestCollectRepoStatsMarksScaffoldOnlyCodebaseMapIncomplete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	now := time.Now().UTC()
	require.NoError(t, os.MkdirAll(CodebaseMapDir(root), 0o755))
	for _, name := range repoCodebaseMapDocs {
		require.NoError(t, os.WriteFile(filepath.Join(CodebaseMapDir(root), name), []byte("# "+name+"\n\n- Notes:\n"), 0o644))
	}

	stats, err := CollectRepoStats(root, now)
	require.NoError(t, err)
	assert.Equal(t, len(repoCodebaseMapDocs), stats.CodebaseMap.PresentDocs)
	assert.Equal(t, 0, stats.CodebaseMap.PopulatedDocs)
	assert.Equal(t, "scaffold_only", stats.CodebaseMap.Freshness)
	require.Len(t, stats.CodebaseMap.ScaffoldOnlyDocs, len(repoCodebaseMapDocs))
}
