package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGovernedBundleDirUsesRepoRootForL2(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("l2-bundle")
	bundleDir, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	expectedRoot, normErr := NormalizePath(root)
	require.NoError(t, normErr)
	assert.Equal(t, filepath.Join(expectedRoot, "artifacts", "changes", change.Slug), bundleDir)
}

func TestGovernedBundleDirUsesDedicatedWorktreeForL3(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	worktreeDir := t.TempDir()

	change := model.NewChange("l3-bundle")
	change.NeedsDiscovery = true
	change.WorktreePath = worktreeDir

	bundleDir, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	expectedWorktree, normErr := NormalizePath(worktreeDir)
	require.NoError(t, normErr)
	assert.Equal(t, filepath.Join(expectedWorktree, "artifacts", "changes", change.Slug), bundleDir)
}

func TestGovernedBundleDirFallsBackToProjectRootWithoutWorktree(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	change := model.NewChange("discovery-no-worktree")
	change.NeedsDiscovery = true

	bundleDir, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	expectedRoot, normErr := NormalizePath(root)
	require.NoError(t, normErr)
	assert.Equal(t, filepath.Join(expectedRoot, "artifacts", "changes", change.Slug), bundleDir)
}

func TestResolveChangePathsUsesRepoScopedCodebaseMapAndArtifactArchive(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	change := model.NewChange("repo-scoped-context")
	paths, err := ResolveChangePaths(root, change)
	require.NoError(t, err)

	expectedRoot, normErr := NormalizePath(root)
	require.NoError(t, normErr)
	assert.Equal(t, filepath.Join(expectedRoot, "artifacts", "codebase"), paths.CodebaseMapDir)
	assert.Equal(t, filepath.Join(expectedRoot, "artifacts", "changes", "archived", change.Slug), paths.GovernedBundleArchive)
}

func TestResolveChangePathsPrefersBoundWorktreeEvenWithoutDiscovery(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	worktreeDir := t.TempDir()

	change := model.NewChange("bound-worktree-root")
	change.WorktreePath = worktreeDir

	paths, err := ResolveChangePaths(root, change)
	require.NoError(t, err)

	expectedWorktree, normErr := NormalizePath(worktreeDir)
	require.NoError(t, normErr)
	assert.Equal(t, expectedWorktree, paths.WorkspaceRoot)
	assert.Equal(t, filepath.Join(expectedWorktree, "artifacts", "changes", change.Slug), paths.GovernedBundleDir)
}

func TestResolveChangePathsUsesScopeRelativePathInsideBoundWorktree(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
	worktreeRoot := addGitWorktree(t, root, "scope-path-branch")
	require.NoError(t, os.MkdirAll(filepath.Join(worktreeRoot, "services", "billing"), 0o755))

	change := model.NewChange("scope-bound-worktree")
	change.WorktreePath = worktreeRoot

	paths, err := ResolveChangePaths(scopeRoot, change)
	require.NoError(t, err)

	expectedWorkspace, normErr := NormalizePath(filepath.Join(worktreeRoot, "services", "billing"))
	require.NoError(t, normErr)
	assert.Equal(t, expectedWorkspace, paths.WorkspaceRoot)
	assert.Equal(t, filepath.Join(expectedWorkspace, "artifacts", "changes", change.Slug), paths.GovernedBundleDir)
}

func TestDisplayPathReturnsProjectRelativePath(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	target := filepath.Join(root, "artifacts", "changes", "demo", "change.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.WriteFile(target, []byte("demo"), 0o644))

	got := DisplayPath(root, target)
	assert.Equal(t, "artifacts/changes/demo/change.yaml", got)
}

func TestDisplayPathReturnsDotForProjectRoot(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)

	assert.Equal(t, ".", DisplayPath(root, root))
}

func TestDisplayPathReturnsAbsolutePathOutsideProject(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	outside := t.TempDir()

	expected, err := NormalizePath(outside)
	require.NoError(t, err)
	assert.Equal(t, filepath.ToSlash(expected), DisplayPath(root, outside))
}

func TestRelocateGovernedBundleMovesBetweenProjectAndWorktreeRoots(t *testing.T) {
	t.Parallel()
	root := createRuntimeLayout(t)
	worktreeDir := t.TempDir()

	fromChange := model.NewChange("bundle-move")
	fromPaths, err := ResolveChangePaths(root, fromChange)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(fromPaths.GovernedBundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(fromPaths.GovernedBundleDir, "intent.md"), []byte("proposal"), 0o644))

	toChange := fromChange
	toChange.NeedsDiscovery = true
	toChange.WorktreePath = worktreeDir

	require.NoError(t, RelocateGovernedBundle(root, fromChange, toChange))

	toPaths, err := ResolveChangePaths(root, toChange)
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(toPaths.GovernedBundleDir, "intent.md"))
	require.NoError(t, err)
	_, err = os.Stat(fromPaths.GovernedBundleDir)
	assert.True(t, os.IsNotExist(err))
}
