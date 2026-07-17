package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLstatTopLevelDoesNotTraverseProtectedChildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission traps are not reliable on Windows")
	}
	directory := t.TempDir()
	protected := filepath.Join(directory, "protected")
	require.NoError(t, os.Mkdir(protected, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(protected, "secret"), []byte("do not read"), 0o600))
	require.NoError(t, os.Chmod(protected, 0))
	t.Cleanup(func() { _ = os.Chmod(protected, 0o700) })
	unreadable := filepath.Join(directory, "unreadable")
	require.NoError(t, os.WriteFile(unreadable, []byte("opaque"), 0o600))
	require.NoError(t, os.Chmod(unreadable, 0))
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o600) })
	require.NoError(t, os.Symlink(protected, filepath.Join(directory, "linked")))

	entries, err := LstatTopLevel(directory)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	assert.Equal(t, []string{"linked", "protected", "unreadable"}, []string{entries[0].Name, entries[1].Name, entries[2].Name})
	assert.NotZero(t, entries[0].Mode&os.ModeSymlink)
	assert.True(t, entries[1].Mode.IsDir())
	assert.True(t, entries[2].Mode.IsRegular())
}

func TestLstatTopLevelRejectsSymlinkedDirectoryAndReturnsEmptyForMissing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target")
	require.NoError(t, os.Mkdir(target, 0o700))
	linked := filepath.Join(root, "linked")
	require.NoError(t, os.Symlink(target, linked))

	_, err := LstatTopLevel(linked)
	require.Error(t, err)
	entries, err := LstatTopLevel(filepath.Join(root, "missing"))
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.NotNil(t, entries)
}
