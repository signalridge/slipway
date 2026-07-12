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

func TestRenameNoReplaceNeverOverwritesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.txt")
	newPath := filepath.Join(dir, "new.txt")
	require.NoError(t, os.WriteFile(oldPath, []byte("old"), 0o600))
	require.NoError(t, os.WriteFile(newPath, []byte("user destination"), 0o600))
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer root.Close()

	err = renameNoReplaceRoots(root, root, "old.txt", "new.txt")
	require.Error(t, err)
	if !atomicNoReplaceAvailableForTest() {
		assert.ErrorIs(t, err, ErrFileTransactionNoReplaceUnsupported)
	}
	oldContent, readErr := os.ReadFile(oldPath)
	require.NoError(t, readErr)
	assert.Equal(t, "old", string(oldContent))
	newContent, readErr := os.ReadFile(newPath)
	require.NoError(t, readErr)
	assert.Equal(t, "user destination", string(newContent))
}

func TestTransactionLeaseOwnsGuardAndStageIdentities(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	t.Run("guard", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "managed.txt")
		require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
		root, err := os.OpenRoot(dir)
		require.NoError(t, err)
		lease := &transactionIdentityLease{}
		t.Cleanup(func() { require.NoError(t, root.Close()) })
		t.Cleanup(func() { require.NoError(t, lease.close()) })
		filesystem := &transactionFilesystem{rootPath: dir, root: root, identityLease: lease}

		snapshot, err := filesystem.snapshotGuard(path, fileTransactionOpWrite, lease)
		require.NoError(t, err)
		require.NotNil(t, snapshot.identity)
		assert.True(t, transactionLeaseOwnsFileIdentity(lease, snapshot.identity), "guard identity must have a live owned file handle")
	})

	t.Run("stage before failed install cleanup", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "managed.txt")
		require.NoError(t, os.WriteFile(path, []byte("user destination"), 0o600))
		root, err := os.OpenRoot(dir)
		require.NoError(t, err)
		lease := &transactionIdentityLease{}
		t.Cleanup(func() { require.NoError(t, root.Close()) })
		t.Cleanup(func() { require.NoError(t, lease.close()) })
		observed := false
		stopCleanup := errors.New("stop after identity observation")
		filesystem := &transactionFilesystem{
			rootPath:      dir,
			root:          root,
			identityLease: lease,
			hooks: FileTransactionHooks{DuringQuarantineCleanup: func(_, recovery string) error {
				observed = true
				info, err := os.Lstat(recovery)
				require.NoError(t, err)
				assert.True(t, transactionLeaseOwnsFileIdentity(lease, info), "stage identity must remain owned before failed-install cleanup")
				return stopCleanup
			}},
		}

		_, applied, err := filesystem.writeFileAtomicNoReplace(path, []byte("transaction"), 0o600, fileSnapshot{}, lease)
		require.ErrorIs(t, err, stopCleanup)
		assert.False(t, applied)
		assert.True(t, observed)
	})
}

func transactionLeaseOwnsFileIdentity(lease *transactionIdentityLease, expected os.FileInfo) bool {
	for _, closer := range lease.closers {
		file, ok := closer.(*os.File)
		if !ok {
			continue
		}
		info, err := file.Stat()
		if err == nil && os.SameFile(info, expected) {
			return true
		}
	}
	return false
}

func TestRootedAtomicWritePreservesConcurrentDestinationChanges(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	t.Run("missing destination is recreated after planning", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "managed.txt")
		injected := false
		err := ApplyFileTransactionWithinWithHooks(root, []FileTransactionOp{
			WriteFileTransactionOp(path, []byte("transaction"), 0o600).WithExpectedMissing(),
		}, FileTransactionHooks{AfterGuardBeforeQuarantine: func(original, _ string) error {
			if original == path && !injected {
				injected = true
				return os.WriteFile(path, []byte("user recreation"), 0o600)
			}
			return nil
		}})

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrFileTransactionConcurrentEdit)
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "user recreation", string(content))
	})

	t.Run("missing parent appears after planning", func(t *testing.T) {
		root := t.TempDir()
		parent := filepath.Join(root, "new-parent")
		path := filepath.Join(parent, "managed.txt")
		injected := false
		err := ApplyFileTransactionWithinWithHooks(root, []FileTransactionOp{
			WriteFileTransactionOp(path, []byte("transaction"), 0o600).WithExpectedMissing(),
		}, FileTransactionHooks{AfterGuardBeforeQuarantine: func(original, _ string) error {
			if original != path || injected {
				return nil
			}
			injected = true
			if err := os.Mkdir(parent, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(parent, "user.txt"), []byte("user parent"), 0o600)
		}})

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrFileTransactionConcurrentEdit)
		content, readErr := os.ReadFile(filepath.Join(parent, "user.txt"))
		require.NoError(t, readErr)
		assert.Equal(t, "user parent", string(content))
		assert.NoFileExists(t, path)
	})

	t.Run("existing destination changes before quarantine", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "managed.txt")
		require.NoError(t, os.WriteFile(path, []byte("planned"), 0o600))
		injected := false
		err := ApplyFileTransactionWithinWithHooks(root, []FileTransactionOp{
			WriteFileTransactionOp(path, []byte("transaction"), 0o600).WithExpectedSHA256(testSHA256([]byte("planned"))),
		}, FileTransactionHooks{AfterGuardBeforeQuarantine: func(original, _ string) error {
			if original == path && !injected {
				injected = true
				return os.WriteFile(path, []byte("user edit"), 0o600)
			}
			return nil
		}})

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrFileTransactionConcurrentEdit)
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "user edit", string(content))
		recovery := collectRecoveryErrors(err)
		require.NotEmpty(t, recovery)
		assert.True(t, recovery[0].Reattached)
	})

	t.Run("destination is recreated after quarantine", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "managed.txt")
		require.NoError(t, os.WriteFile(path, []byte("planned"), 0o600))
		injected := false
		err := ApplyFileTransactionWithinWithHooks(root, []FileTransactionOp{
			WriteFileTransactionOp(path, []byte("transaction"), 0o600).WithExpectedSHA256(testSHA256([]byte("planned"))),
		}, FileTransactionHooks{AfterQuarantineBeforeValidation: func(original, _ string) error {
			if original == path && !injected {
				injected = true
				return os.WriteFile(path, []byte("user recreation"), 0o600)
			}
			return nil
		}})

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "user recreation", string(content))
		assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
	})
}

func TestRootTransactionParentSwapDoesNotEscapeRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links may require elevated privileges")
	}
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	t.Run("normal mutation", func(t *testing.T) {
		root := t.TempDir()
		managed := filepath.Join(root, "managed")
		original := managed + "-original"
		path := filepath.Join(managed, "value.txt")
		outside := t.TempDir()
		require.NoError(t, os.Mkdir(managed, 0o700))
		require.NoError(t, os.WriteFile(path, []byte("inside"), 0o600))
		injected := false

		err := ApplyFileTransactionWithinWithHooks(root, []FileTransactionOp{
			WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		}, FileTransactionHooks{AfterGuardBeforeQuarantine: func(originalPath, _ string) error {
			if originalPath != path || injected {
				return nil
			}
			injected = true
			if err := os.Rename(managed, original); err != nil {
				return err
			}
			return os.Symlink(outside, managed)
		}})

		require.Error(t, err)
		content, readErr := os.ReadFile(filepath.Join(original, "value.txt"))
		require.NoError(t, readErr)
		assert.Equal(t, "inside", string(content))
		entries, readDirErr := os.ReadDir(outside)
		require.NoError(t, readDirErr)
		assert.Empty(t, entries)
	})

	t.Run("rollback", func(t *testing.T) {
		root := t.TempDir()
		managed := filepath.Join(root, "managed")
		original := managed + "-original"
		path := filepath.Join(managed, "value.txt")
		later := filepath.Join(root, "later.txt")
		outside := t.TempDir()
		require.NoError(t, os.Mkdir(managed, 0o700))
		require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
		failure := errors.New("later failed")
		guardCalls := 0

		err := ApplyFileTransactionWithinWithHooks(root, []FileTransactionOp{
			WriteFileTransactionOp(path, []byte("transaction"), 0o600),
			WriteFileTransactionOp(later, []byte("later"), 0o600),
		}, FileTransactionHooks{
			BeforeMutation: failPath(later, failure),
			AfterGuardBeforeQuarantine: func(originalPath, _ string) error {
				if originalPath != path {
					return nil
				}
				guardCalls++
				if guardCalls != 2 {
					return nil
				}
				if err := os.Rename(managed, original); err != nil {
					return err
				}
				return os.Symlink(outside, managed)
			},
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, failure)
		assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
		content, readErr := os.ReadFile(filepath.Join(original, "value.txt"))
		require.NoError(t, readErr)
		assert.Equal(t, "transaction", string(content))
		entries, readDirErr := os.ReadDir(outside)
		require.NoError(t, readDirErr)
		assert.Empty(t, entries)
		assert.Contains(t, err.Error(), path)
		assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
	})
}

func TestRootTransactionRejectsParentSwapAfterQuarantineValidation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory replacement behavior differs on Windows")
	}
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	root := t.TempDir()
	managed := filepath.Join(root, "managed")
	moved := managed + "-moved"
	path := filepath.Join(managed, "value.txt")
	later := filepath.Join(root, "later.txt")
	require.NoError(t, os.Mkdir(managed, 0o700))
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	failure := errors.New("later failed")
	injected := false

	err := ApplyFileTransactionWithinWithHooks(root, []FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, FileTransactionHooks{
		BeforeMutation: failPath(later, failure),
		AfterValidationBeforeRestore: func(original, _ string) error {
			if original != path || injected {
				return nil
			}
			injected = true
			if err := os.Rename(managed, moved); err != nil {
				return err
			}
			if err := os.Mkdir(managed, 0o700); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(managed, "user.txt"), []byte("user parent"), 0o600)
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
	content, readErr := os.ReadFile(filepath.Join(managed, "user.txt"))
	require.NoError(t, readErr)
	assert.Equal(t, "user parent", string(content))
	assert.NoFileExists(t, path)
	assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
}

func TestRootTransactionTreeRestoreStaysOnOwnedDirectoryHandle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links may require elevated privileges")
	}
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	root := t.TempDir()
	tree := filepath.Join(root, "tree")
	nested := filepath.Join(tree, "nested")
	managed := filepath.Join(nested, "managed.txt")
	later := filepath.Join(root, "later.txt")
	outside := t.TempDir()
	require.NoError(t, os.MkdirAll(nested, 0o700))
	require.NoError(t, os.WriteFile(managed, []byte("managed"), 0o600))
	failure := errors.New("later failed")

	err := ApplyFileTransactionWithinWithHooks(root, []FileTransactionOp{
		RemoveAllTransactionOp(tree),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, FileTransactionHooks{
		BeforeMutation: failPath(later, failure),
		DuringExclusiveRestore: func(original, _ string) error {
			if original != tree {
				return nil
			}
			if err := os.Remove(tree); err != nil {
				return err
			}
			return os.Symlink(outside, tree)
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, failure)
	linkTarget, linkErr := os.Readlink(tree)
	require.NoError(t, linkErr)
	assert.Equal(t, outside, linkTarget)
	entries, readErr := os.ReadDir(outside)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
	assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
}

func TestTreeSnapshotRejectsSymlinksWithoutMutation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links may require elevated privileges")
	}
	root := t.TempDir()
	tree := filepath.Join(root, "tree")
	require.NoError(t, os.Mkdir(tree, 0o700))
	require.NoError(t, os.Symlink("target", filepath.Join(tree, "link")))

	err := ApplyFileTransactionWithin(root, []FileTransactionOp{RemoveAllTransactionOp(tree)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink or special file")
	target, readErr := os.Readlink(filepath.Join(tree, "link"))
	require.NoError(t, readErr)
	assert.Equal(t, "target", target)
}

func atomicNoReplaceAvailableForTest() bool {
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		return true
	default:
		return false
	}
}
