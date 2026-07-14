//go:build windows

package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestrictToOwnerSetsProtectedUserAndSystemDACL(t *testing.T) {
	tests := []struct {
		name      string
		directory bool
	}{
		{name: "file"},
		{name: "directory", directory: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "private")
			if test.directory {
				require.NoError(t, os.Mkdir(path, 0o700))
			} else {
				require.NoError(t, os.WriteFile(path, []byte("private"), 0o600))
			}
			var file *os.File
			var err error
			if test.directory {
				file, err = os.Open(path)
			} else {
				file, err = os.OpenFile(path, os.O_RDWR, 0)
			}
			require.NoError(t, err)
			defer file.Close()

			require.NoError(t, RestrictToOwner(file))
			private, err := ownerACLIsPrivate(file)
			require.NoError(t, err)
			assert.True(t, private)
		})
	}
}

func TestRestrictToOwnerRemainsBoundToOpenedObjectAfterPathReplacement(t *testing.T) {
	directory := t.TempDir()
	root, err := os.OpenRoot(directory)
	require.NoError(t, err)
	defer root.Close()

	path := filepath.Join(directory, "private")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	original, err := root.Open("private")
	require.NoError(t, err)
	defer original.Close()

	require.NoError(t, root.Rename("private", "detached"))
	require.NoError(t, os.WriteFile(path, []byte("replacement"), 0o600))
	replacement, err := root.Open("private")
	require.NoError(t, err)
	defer replacement.Close()

	require.NoError(t, RestrictToOwner(original))
	originalPrivate, err := ownerACLIsPrivate(original)
	require.NoError(t, err)
	assert.True(t, originalPrivate)
	replacementPrivate, err := ownerACLIsPrivate(replacement)
	require.NoError(t, err)
	assert.False(t, replacementPrivate, "replacement pathname entry must not receive the opened object's DACL")
}

// TestRestrictToOwnerWorksWithReadOnlyRootOpenDirectoryHandle covers the
// transaction path: os.Root.Open returns a FILE_GENERIC_READ directory handle
// that ReOpenFile cannot reliably reopen with WRITE_DAC.
func TestRestrictToOwnerWorksWithReadOnlyRootOpenDirectoryHandle(t *testing.T) {
	parent := t.TempDir()
	root, err := os.OpenRoot(parent)
	require.NoError(t, err)
	defer root.Close()

	require.NoError(t, root.Mkdir("private", 0o700))
	handle, err := root.Open("private")
	require.NoError(t, err)
	defer handle.Close()

	require.NoError(t, RestrictToOwner(handle))
	private, err := ownerACLIsPrivate(handle)
	require.NoError(t, err)
	assert.True(t, private)
}
