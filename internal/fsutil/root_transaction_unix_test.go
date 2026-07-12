//go:build darwin || linux

package fsutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestTreeSnapshotRejectsSpecialFilesWithoutMutation(t *testing.T) {
	root := t.TempDir()
	tree := filepath.Join(root, "tree")
	fifo := filepath.Join(tree, "pipe")
	require.NoError(t, os.Mkdir(tree, 0o700))
	require.NoError(t, unix.Mkfifo(fifo, 0o600))

	err := ApplyFileTransactionWithin(root, []FileTransactionOp{RemoveAllTransactionOp(tree)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink or special file")
	info, statErr := os.Lstat(fifo)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Mode()&os.ModeNamedPipe)
}
