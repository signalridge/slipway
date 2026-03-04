package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindProjectRootFindsNearestSplnParent(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".spln"), 0o755))
	start := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(start, 0o755))

	got, err := FindProjectRoot(start)
	require.NoError(t, err)
	assert.Equal(t, root, got)
}

func TestFindProjectRootReturnsErrorWhenMissing(t *testing.T) {
	root := t.TempDir()
	start := filepath.Join(root, "x", "y")
	require.NoError(t, os.MkdirAll(start, 0o755))

	_, err := FindProjectRoot(start)
	require.ErrorIs(t, err, ErrProjectRootNotFound)
}
