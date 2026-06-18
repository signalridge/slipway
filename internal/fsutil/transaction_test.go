package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyFileTransactionRollsBackAppliedWriteOnLaterFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	existingPath := filepath.Join(dir, "existing.yaml")
	newPath := filepath.Join(dir, "new.yaml")
	require.NoError(t, os.WriteFile(existingPath, []byte("old"), 0o644))

	writeErr := errors.New("write failed")
	ops := []FileTransactionOp{
		WriteFileTransactionOp(existingPath, []byte("new"), 0o644),
		WriteFileTransactionOp(newPath, []byte("created"), 0o644),
	}

	err := applyFileTransaction(ops, fileTransactionHooks{
		writeFile: func(path string, data []byte, perm os.FileMode) error {
			if path == newPath {
				return writeErr
			}
			return WriteFileAtomic(path, data, perm)
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)

	got, readErr := os.ReadFile(existingPath)
	require.NoError(t, readErr)
	assert.Equal(t, "old", string(got))

	_, statErr := os.Stat(newPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestApplyFileTransactionRestoresRemovedFileOnLaterFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	removedPath := filepath.Join(dir, "verification.yaml")
	laterPath := filepath.Join(dir, "change.yaml")
	require.NoError(t, os.WriteFile(removedPath, []byte("evidence"), 0o640))

	writeErr := errors.New("state save failed")
	ops := []FileTransactionOp{
		RemoveFileTransactionOp(removedPath),
		WriteFileTransactionOp(laterPath, []byte("state"), 0o644),
	}

	err := applyFileTransaction(ops, fileTransactionHooks{
		writeFile: func(path string, data []byte, perm os.FileMode) error {
			if path == laterPath {
				return writeErr
			}
			return WriteFileAtomic(path, data, perm)
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)

	got, readErr := os.ReadFile(removedPath)
	require.NoError(t, readErr)
	assert.Equal(t, "evidence", string(got))

	info, statErr := os.Stat(removedPath)
	require.NoError(t, statErr)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	}
}

func TestApplyFileTransactionRestoresRemovedTreeOnLaterFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	treePath := filepath.Join(dir, "generated")
	nestedPath := filepath.Join(treePath, "references", "guide.md")
	laterPath := filepath.Join(dir, "later.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(nestedPath), 0o755))
	require.NoError(t, os.WriteFile(nestedPath, []byte("managed"), 0o640))

	writeErr := errors.New("later write failed")
	ops := []FileTransactionOp{
		RemoveAllTransactionOp(treePath),
		WriteFileTransactionOp(laterPath, []byte("state"), 0o644),
	}

	err := applyFileTransaction(ops, fileTransactionHooks{
		writeFile: func(path string, data []byte, perm os.FileMode) error {
			if path == laterPath {
				return writeErr
			}
			return WriteFileAtomic(path, data, perm)
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)

	got, readErr := os.ReadFile(nestedPath)
	require.NoError(t, readErr)
	assert.Equal(t, "managed", string(got))

	info, statErr := os.Stat(nestedPath)
	require.NoError(t, statErr)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	}
}

func TestApplyFileTransactionRollsBackAppliedWriteOnLaterSnapshotFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createdPath := filepath.Join(dir, "created.yaml")
	directoryPath := filepath.Join(dir, "not-a-file")
	require.NoError(t, os.Mkdir(directoryPath, 0o755))

	err := ApplyFileTransaction([]FileTransactionOp{
		WriteFileTransactionOp(createdPath, []byte("created"), 0o644),
		WriteFileTransactionOp(directoryPath, []byte("later"), 0o644),
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "path is a directory")

	_, statErr := os.Stat(createdPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestApplyFileTransactionReportsRollbackFailurePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	createdPath := filepath.Join(dir, "created.yaml")
	laterPath := filepath.Join(dir, "change.yaml")

	writeErr := errors.New("state save failed")
	rollbackErr := errors.New("rollback remove failed")
	ops := []FileTransactionOp{
		WriteFileTransactionOp(createdPath, []byte("created"), 0o644),
		WriteFileTransactionOp(laterPath, []byte("state"), 0o644),
	}

	err := applyFileTransaction(ops, fileTransactionHooks{
		writeFile: func(path string, data []byte, perm os.FileMode) error {
			if path == laterPath {
				return writeErr
			}
			return WriteFileAtomic(path, data, perm)
		},
		removeFile: func(path string) error {
			if path == createdPath {
				return rollbackErr
			}
			return os.Remove(path)
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, writeErr)
	assert.ErrorIs(t, err, rollbackErr)
	assert.ErrorContains(t, err, createdPath)
}
