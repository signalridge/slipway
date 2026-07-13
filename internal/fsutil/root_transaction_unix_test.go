//go:build darwin || linux

package fsutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

const guardSnapshotRlimitHelper = "SLIPWAY_TEST_GUARD_SNAPSHOT_RLIMIT_HELPER"

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

func TestFailedPreflightGuardSnapshotReleasesScopedHandles(t *testing.T) {
	if os.Getenv(guardSnapshotRlimitHelper) != "1" {
		command := exec.Command(os.Args[0], "-test.run=^TestFailedPreflightGuardSnapshotReleasesScopedHandles$")
		command.Env = append(os.Environ(), guardSnapshotRlimitHelper+"=1")
		output, err := command.CombinedOutput()
		require.NoError(t, err, string(output))
		return
	}

	root := t.TempDir()
	managed := filepath.Join(root, "managed.txt")
	tree := filepath.Join(root, "large-tree")
	require.NoError(t, os.WriteFile(managed, []byte("before"), 0o600))
	require.NoError(t, os.Mkdir(tree, 0o700))
	for index := range 128 {
		require.NoError(t, os.WriteFile(filepath.Join(tree, fmt.Sprintf("item-%03d", index)), []byte("tree"), 0o600))
	}

	var original unix.Rlimit
	require.NoError(t, unix.Getrlimit(unix.RLIMIT_NOFILE, &original))
	if original.Cur < 64 {
		t.Skipf("RLIMIT_NOFILE is already too small for deterministic preflight exercise: %d", original.Cur)
	}
	limited := original
	limited.Cur = 64
	require.NoError(t, unix.Setrlimit(unix.RLIMIT_NOFILE, &limited))
	t.Cleanup(func() { require.NoError(t, unix.Setrlimit(unix.RLIMIT_NOFILE, &original)) })

	err := ApplyFileTransactionWithin(root, []FileTransactionOp{
		WriteFileTransactionOp(managed, []byte("transaction"), 0o600),
		RemoveAllTransactionOp(tree),
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, unix.EMFILE), "large guard snapshot must hit the scoped descriptor limit: %v", err)
	var transactionErr *FileTransactionError
	assert.False(t, errors.As(err, &transactionErr), "preflight must fail before rollback is needed")
	content, readErr := os.ReadFile(managed)
	require.NoError(t, readErr)
	assert.Equal(t, "before", string(content))
	requireNoRecoveryArtifacts(t, root)
}
