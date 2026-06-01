package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelocateGovernedBundleMovesBetweenCanonicalRoots(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	worktreeDir := t.TempDir()

	change := model.NewChange("bundle-move")

	fromDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(fromDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fromDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	moved := change
	moved.NeedsDiscovery = true
	moved.WorktreePath = worktreeDir

	require.NoError(t, RelocateGovernedBundle(root, change, moved))

	_, err := os.Stat(filepath.Join(fromDir, "tasks.md"))
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(filepath.Join(worktreeDir, "artifacts", "changes", change.Slug, "tasks.md"))
	require.NoError(t, err)
}

func TestRelocateGovernedBundleNoopsWhenRootsMatch(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("bundle-stays")

	fromDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(fromDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fromDir, "intent.md"), []byte("ok"), 0o644))

	require.NoError(t, RelocateGovernedBundle(root, change, change))

	_, err := os.Stat(filepath.Join(fromDir, "intent.md"))
	require.NoError(t, err)
}

func TestRelocateGovernedBundleRejectsNonEmptyTarget(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	worktreeDir := t.TempDir()

	change := model.NewChange("bundle-conflict")

	fromDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	toDir := filepath.Join(worktreeDir, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(fromDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fromDir, "tasks.md"), []byte("# Tasks\n"), 0o644))
	require.NoError(t, os.MkdirAll(toDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(toDir, "existing.md"), []byte("occupied"), 0o644))

	moved := change
	moved.NeedsDiscovery = true
	moved.WorktreePath = worktreeDir

	err := RelocateGovernedBundle(root, change, moved)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target already exists")
}

func TestRelocateGovernedBundleSeedsVisibleTargetAuthority(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	worktreeDir := addGitWorktree(t, root, "bundle-visible-authority")

	change := model.NewChange("bundle-visible-authority")
	require.NoError(t, SaveChange(root, change))
	fromDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.WriteFile(filepath.Join(fromDir, "tasks.md"), []byte("# Tasks\n"), 0o644))

	moved := change
	moved.NeedsDiscovery = true
	moved.WorktreePath = worktreeDir
	moved.WorktreeBranch = "bundle-visible-authority"

	require.NoError(t, RelocateGovernedBundle(root, change, moved))

	loaded, err := LoadChange(root, change.Slug)
	require.NoError(t, err)
	wantWorktree, err := NormalizePath(worktreeDir)
	require.NoError(t, err)
	assert.Equal(t, wantWorktree, loaded.WorktreePath)
	assert.Equal(t, "bundle-visible-authority", loaded.WorktreeBranch)

	_, err = os.Stat(filepath.Join(worktreeDir, ".slipway.yaml"))
	require.NoError(t, err)
	_, err = os.Stat(WorkspaceScopeMarkerPath(worktreeDir))
	require.NoError(t, err)
}
