package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindRepoRootStartsAtCurrentWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/repo\n"), 0o644))
	nested := filepath.Join(root, "internal", "cmd", "tool")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	t.Chdir(nested)

	got, err := FindRepoRoot("")
	require.NoError(t, err)
	require.Equal(t, root, got)
}

func TestFindRepoRootWalksFromProvidedStart(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/repo\n"), 0o644))
	nested := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	got, err := FindRepoRoot(nested)
	require.NoError(t, err)
	require.Equal(t, root, got)
}

func TestFindRepoRootReportsMissingGoModule(t *testing.T) {
	t.Parallel()

	start := filepath.Join(t.TempDir(), "a", "b")
	require.NoError(t, os.MkdirAll(start, 0o755))

	_, err := FindRepoRoot(start)
	require.ErrorContains(t, err, "could not find repository root containing go.mod")
}
