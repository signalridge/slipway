package cmd

import (
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateReadContextCachesLoadedChangeWithinInvocation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("read-context-cache")
	change.Description = "before"
	require.NoError(t, state.SaveChange(root, change))

	readCtx := newStateReadContext(root)
	first, err := readCtx.loadChange(change.Slug)
	require.NoError(t, err)
	assert.Equal(t, "before", first.Description)

	change.Description = "after"
	require.NoError(t, state.SaveChange(root, change))

	second, err := readCtx.loadChange(change.Slug)
	require.NoError(t, err)
	assert.Equal(t, "before", second.Description)

	fresh, err := state.LoadChangeFast(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, "after", fresh.Description)
}

func TestStateReadContextReloadChangeInvalidatesDependentCaches(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("read-context-reload")
	change.Description = "before"
	require.NoError(t, state.SaveChange(root, change))

	readCtx := newStateReadContext(root)
	loaded, err := readCtx.loadChange(change.Slug)
	require.NoError(t, err)
	_, err = readCtx.resolvedPaths(loaded)
	require.NoError(t, err)
	_, err = readCtx.verificationRecords(loaded)
	require.NoError(t, err)
	readCtx.execution[change.Slug] = executionContext{}

	require.Contains(t, readCtx.changes, change.Slug)
	require.Contains(t, readCtx.paths, change.Slug)
	require.Contains(t, readCtx.verifications, change.Slug)
	require.Contains(t, readCtx.execution, change.Slug)

	change.Description = "after"
	require.NoError(t, state.SaveChange(root, change))

	reloaded, err := readCtx.reloadChange(change.Slug)
	require.NoError(t, err)
	assert.Equal(t, "after", reloaded.Description)
	require.Contains(t, readCtx.changes, change.Slug)
	assert.Equal(t, "after", readCtx.changes[change.Slug].Description)
	assert.NotContains(t, readCtx.paths, change.Slug)
	assert.NotContains(t, readCtx.verifications, change.Slug)
	assert.NotContains(t, readCtx.execution, change.Slug)
}

func TestStateReadContextResolvesPathsOncePerInvocation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("read-context-paths")
	require.NoError(t, state.SaveChange(root, change))

	readCtx := newStateReadContext(root)
	loaded, err := readCtx.loadChange(change.Slug)
	require.NoError(t, err)
	first, err := readCtx.resolvedPaths(loaded)
	require.NoError(t, err)
	second, err := readCtx.resolvedPaths(loaded)
	require.NoError(t, err)

	assert.Equal(t, first, second)
	expectedBundleDir, err := state.NormalizePath(filepath.Join(root, "artifacts", "changes", change.Slug))
	require.NoError(t, err)
	assert.Equal(t, expectedBundleDir, first.GovernedBundleDir)
}
