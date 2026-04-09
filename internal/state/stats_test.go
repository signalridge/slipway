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
	assert.Equal(t, "partial", stats.CodebaseMap.Freshness)
	assert.Contains(t, stats.CodebaseMap.MissingDocs, "INTEGRATIONS.md")
}
