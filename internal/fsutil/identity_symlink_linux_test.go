//go:build linux

package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenSymlinkIdentityPinsSymlinkObject(t *testing.T) {
	testOpenSymlinkIdentityPinsSymlinkObject(t)
}

func TestOpenSymlinkIdentityRejectsOtherEntries(t *testing.T) {
	testOpenSymlinkIdentityRejectsOtherEntries(t)
}

func testOpenSymlinkIdentityPinsSymlinkObject(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "first.txt"), []byte("first"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "second.txt"), []byte("second"), 0o600))
	root, err := os.OpenRoot(directory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	require.NoError(t, root.Symlink("first.txt", "link"))

	expected, err := root.Lstat("link")
	require.NoError(t, err)
	identity, err := openSymlinkIdentity(root, "link")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, identity.Close()) })
	opened, err := identity.Stat()
	require.NoError(t, err)
	assert.NotZero(t, opened.Mode()&os.ModeSymlink)
	assert.True(t, os.SameFile(expected, opened))
	heldTarget, err := readSymlinkIdentity(root, "link", identity)
	require.NoError(t, err)
	assert.Equal(t, "first.txt", heldTarget)

	require.NoError(t, root.Remove("link"))
	require.NoError(t, root.Symlink("second.txt", "link"))
	recreated, err := root.Lstat("link")
	require.NoError(t, err)
	assert.NotZero(t, recreated.Mode()&os.ModeSymlink)
	assert.False(t, os.SameFile(opened, recreated))
	heldTarget, err = readSymlinkIdentity(root, "link", identity)
	require.NoError(t, err)
	assert.Equal(t, "first.txt", heldTarget)
	target, err := root.Readlink("link")
	require.NoError(t, err)
	assert.Equal(t, "second.txt", target)
}

func testOpenSymlinkIdentityRejectsOtherEntries(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "regular.txt"), []byte("regular"), 0o600))
	require.NoError(t, os.Mkdir(filepath.Join(directory, "nested"), 0o700))
	require.NoError(t, os.Symlink("../regular.txt", filepath.Join(directory, "nested", "link")))
	root, err := os.OpenRoot(directory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })

	identity, err := openSymlinkIdentity(root, "regular.txt")
	require.Error(t, err)
	assert.Nil(t, identity)
	identity, err = openSymlinkIdentity(root, filepath.Join("nested", "link"))
	require.Error(t, err)
	assert.Nil(t, identity)
}
