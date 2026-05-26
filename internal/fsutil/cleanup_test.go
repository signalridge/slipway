package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanupAtomicTempArtifacts(t *testing.T) {
	root := t.TempDir()
	files := []string{
		filepath.Join(root, ".tmp-state-1"),
		filepath.Join(root, ".tmp-change-2"),
		filepath.Join(root, "real.yaml"),
	}
	for _, file := range files {
		require.NoError(t, os.WriteFile(file, []byte("x"), 0o644))
	}

	deleted, err := CleanupAtomicTempArtifacts(root)
	require.NoError(t, err)
	assert.Len(t, deleted, 2)

	_, err = os.Stat(filepath.Join(root, ".tmp-state-1"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(root, "real.yaml"))
	require.NoError(t, err)
}
