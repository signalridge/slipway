//go:build windows

package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenameNoReplaceAtWindowsPreservesExistingDestination(t *testing.T) {
	root := t.TempDir()
	sourceDirectory := filepath.Join(root, "source")
	destinationDirectory := filepath.Join(root, "destination")
	require.NoError(t, os.MkdirAll(sourceDirectory, 0o700))
	require.NoError(t, os.MkdirAll(destinationDirectory, 0o700))
	sourceRoot, err := os.OpenRoot(sourceDirectory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sourceRoot.Close()) })
	destinationRoot, err := os.OpenRoot(destinationDirectory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, destinationRoot.Close()) })

	sourcePath := filepath.Join(sourceDirectory, "source.txt")
	destinationPath := filepath.Join(destinationDirectory, "destination.txt")
	require.NoError(t, os.WriteFile(sourcePath, []byte("source"), 0o600))
	require.NoError(t, os.WriteFile(destinationPath, []byte("destination"), 0o600))

	err = renameNoReplaceRoots(sourceRoot, destinationRoot, "source.txt", "destination.txt")
	require.Error(t, err)
	sourceContent, readErr := os.ReadFile(sourcePath)
	require.NoError(t, readErr)
	destinationContent, readErr := os.ReadFile(destinationPath)
	require.NoError(t, readErr)
	assert.Equal(t, "source", string(sourceContent))
	assert.Equal(t, "destination", string(destinationContent))
}

func TestRenameNoReplaceAtWindowsMovesIntoMissingDestination(t *testing.T) {
	root := t.TempDir()
	sourceDirectory := filepath.Join(root, "source")
	destinationDirectory := filepath.Join(root, "destination")
	require.NoError(t, os.MkdirAll(sourceDirectory, 0o700))
	require.NoError(t, os.MkdirAll(destinationDirectory, 0o700))
	sourceRoot, err := os.OpenRoot(sourceDirectory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sourceRoot.Close()) })
	destinationRoot, err := os.OpenRoot(destinationDirectory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, destinationRoot.Close()) })

	sourcePath := filepath.Join(sourceDirectory, "source.txt")
	destinationPath := filepath.Join(destinationDirectory, "destination.txt")
	require.NoError(t, os.WriteFile(sourcePath, []byte("source"), 0o600))

	require.NoError(t, renameNoReplaceRoots(sourceRoot, destinationRoot, "source.txt", "destination.txt"))
	_, err = os.Stat(sourcePath)
	assert.ErrorIs(t, err, os.ErrNotExist)
	destinationContent, err := os.ReadFile(destinationPath)
	require.NoError(t, err)
	assert.Equal(t, "source", string(destinationContent))
}
