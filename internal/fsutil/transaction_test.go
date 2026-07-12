package fsutil

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyFileTransactionRollsBackAppliedAndFailingWrites(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	t.Run("later operation fails", func(t *testing.T) {
		dir := t.TempDir()
		first := filepath.Join(dir, "first.txt")
		later := filepath.Join(dir, "later.txt")
		require.NoError(t, os.WriteFile(first, []byte("before"), 0o640))
		failure := errors.New("later operation failed")

		err := ApplyFileTransactionWithHooks([]FileTransactionOp{
			WriteFileTransactionOp(first, []byte("transaction"), 0o600),
			WriteFileTransactionOp(later, []byte("later"), 0o600),
		}, FileTransactionHooks{BeforeMutation: func(path, _ string) error {
			if path == later {
				return failure
			}
			return nil
		}})

		require.Error(t, err)
		assert.ErrorIs(t, err, failure)
		content, readErr := os.ReadFile(first)
		require.NoError(t, readErr)
		assert.Equal(t, "before", string(content))
		assert.Equal(t, os.FileMode(0o640), fileMode(t, first))
		assert.NoFileExists(t, later)
		requireNoRecoveryArtifacts(t, dir)
	})

	t.Run("failing mutation already committed", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "managed.txt")
		require.NoError(t, os.WriteFile(path, []byte("before"), 0o640))
		failure := errors.New("post-write sync report failed")

		err := ApplyFileTransactionWithHooks([]FileTransactionOp{
			WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		}, FileTransactionHooks{AfterMutation: func(original, _ string) error {
			if original == path {
				return failure
			}
			return nil
		}})

		require.Error(t, err)
		assert.ErrorIs(t, err, failure)
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "before", string(content))
		assert.Equal(t, os.FileMode(0o640), fileMode(t, path))
		requireNoRecoveryArtifacts(t, dir)
	})
}

func TestApplyFileTransactionRestoresRemovedSnapshotKinds(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	t.Run("regular file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "evidence.txt")
		later := filepath.Join(dir, "later.txt")
		require.NoError(t, os.WriteFile(path, []byte("evidence"), 0o640))
		err := failLaterTransaction(path, later, RemoveFileTransactionOp(path))
		require.Error(t, err)
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "evidence", string(content))
		assert.Equal(t, os.FileMode(0o640), fileMode(t, path))
		requireNoRecoveryArtifacts(t, dir)
	})

	t.Run("symbolic link", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symbolic links may require elevated privileges")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "current")
		later := filepath.Join(dir, "later.txt")
		require.NoError(t, os.Symlink("target-a", path))
		err := failLaterTransaction(path, later, RemoveFileTransactionOp(path))
		require.Error(t, err)
		target, readErr := os.Readlink(path)
		require.NoError(t, readErr)
		assert.Equal(t, "target-a", target)
		requireNoRecoveryArtifacts(t, dir)
	})

	t.Run("nested directory tree", func(t *testing.T) {
		dir := t.TempDir()
		tree := filepath.Join(dir, "tree")
		nested := filepath.Join(tree, "nested")
		file := filepath.Join(nested, "managed.txt")
		later := filepath.Join(dir, "later.txt")
		require.NoError(t, os.MkdirAll(nested, 0o700))
		require.NoError(t, os.WriteFile(file, []byte("managed"), 0o640))
		require.NoError(t, os.Chmod(nested, 0o555))
		require.NoError(t, os.Chmod(tree, 0o555))
		t.Cleanup(func() {
			_ = os.Chmod(tree, 0o700)
			_ = os.Chmod(nested, 0o700)
		})

		err := failLaterTransaction(tree, later, RemoveAllTransactionOp(tree))
		require.Error(t, err)
		content, readErr := os.ReadFile(file)
		require.NoError(t, readErr)
		assert.Equal(t, "managed", string(content))
		assertReadOnlyDirectoryMode(t, tree)
		assertReadOnlyDirectoryMode(t, nested)
		require.NoError(t, os.Chmod(tree, 0o700))
		require.NoError(t, os.Chmod(nested, 0o700))
		requireNoRecoveryArtifacts(t, dir)
	})
}

func TestFileTransactionPreconditionsPreservePlannedUserPaths(t *testing.T) {
	dir := t.TempDir()
	created := filepath.Join(dir, "created.txt")
	require.NoError(t, os.WriteFile(created, []byte("user"), 0o600))
	err := ApplyFileTransaction([]FileTransactionOp{
		WriteFileTransactionOp(created, []byte("managed"), 0o600).WithExpectedMissing(),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionPrecondition)
	content, readErr := os.ReadFile(created)
	require.NoError(t, readErr)
	assert.Equal(t, "user", string(content))

	guarded := filepath.Join(dir, "guarded.txt")
	require.NoError(t, os.WriteFile(guarded, []byte("planned"), 0o600))
	hash := testSHA256([]byte("planned"))
	require.NoError(t, os.WriteFile(guarded, []byte("user edit"), 0o600))
	err = ApplyFileTransaction([]FileTransactionOp{
		RemoveFileTransactionOp(guarded).WithExpectedSHA256(hash),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionPrecondition)
	content, readErr = os.ReadFile(guarded)
	require.NoError(t, readErr)
	assert.Equal(t, "user edit", string(content))
}

func TestRollbackConcurrentEditWindowsPreserveUserBytes(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	tests := []struct {
		name        string
		hooks       func(path string) FileTransactionHooks
		wantAtPath  string
		wantInStore string
	}{
		{
			name: "after guard snapshot before quarantine",
			hooks: func(path string) FileTransactionHooks {
				calls := 0
				return FileTransactionHooks{AfterGuardBeforeQuarantine: func(original, _ string) error {
					if original != path {
						return nil
					}
					calls++
					if calls == 2 {
						return os.WriteFile(path, []byte("user-guard"), 0o600)
					}
					return nil
				}}
			},
			wantAtPath: "user-guard",
		},
		{
			name: "after quarantine before validation",
			hooks: func(path string) FileTransactionHooks {
				calls := 0
				return FileTransactionHooks{AfterQuarantineBeforeValidation: func(original, _ string) error {
					if original != path {
						return nil
					}
					calls++
					if calls == 2 {
						return os.WriteFile(path, []byte("user-after-quarantine"), 0o600)
					}
					return nil
				}}
			},
			wantAtPath: "user-after-quarantine",
		},
		{
			name: "after validation before restore",
			hooks: func(path string) FileTransactionHooks {
				return FileTransactionHooks{AfterValidationBeforeRestore: func(original, _ string) error {
					if original == path {
						return os.WriteFile(path, []byte("user-before-restore"), 0o600)
					}
					return nil
				}}
			},
			wantAtPath: "user-before-restore",
		},
		{
			name: "during exclusive file restore",
			hooks: func(path string) FileTransactionHooks {
				return FileTransactionHooks{DuringExclusiveRestore: func(original, _ string) error {
					if original != path {
						return nil
					}
					if err := os.Remove(path); err != nil {
						return err
					}
					return os.WriteFile(path, []byte("user-during-restore"), 0o600)
				}}
			},
			wantAtPath: "user-during-restore",
		},
		{
			name: "after restore before post-validation",
			hooks: func(path string) FileTransactionHooks {
				return FileTransactionHooks{AfterRestoreBeforePostValidation: func(original, _ string) error {
					if original == path {
						return os.WriteFile(path, []byte("user-after-restore"), 0o600)
					}
					return nil
				}}
			},
			wantAtPath: "user-after-restore",
		},
		{
			name: "destination changes during quarantine cleanup",
			hooks: func(path string) FileTransactionHooks {
				injected := false
				return FileTransactionHooks{DuringQuarantineCleanup: func(original, _ string) error {
					if original == path && !injected {
						injected = true
						return os.WriteFile(path, []byte("user-during-cleanup"), 0o600)
					}
					return nil
				}}
			},
			wantAtPath: "user-during-cleanup",
		},
		{
			name: "quarantine changes during cleanup",
			hooks: func(path string) FileTransactionHooks {
				injected := false
				return FileTransactionHooks{DuringQuarantineCleanup: func(original, recovery string) error {
					if original == path && !injected {
						injected = true
						return os.WriteFile(recovery, []byte("user-in-recovery"), 0o600)
					}
					return nil
				}}
			},
			wantAtPath:  "before",
			wantInStore: "user-in-recovery",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "managed.txt")
			later := filepath.Join(dir, "later.txt")
			require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
			failure := errors.New("later operation failed")
			hooks := test.hooks(path)
			originalBefore := hooks.BeforeMutation
			hooks.BeforeMutation = func(original, recovery string) error {
				if original == later {
					return failure
				}
				if originalBefore != nil {
					return originalBefore(original, recovery)
				}
				return nil
			}

			err := ApplyFileTransactionWithHooks([]FileTransactionOp{
				WriteFileTransactionOp(path, []byte("transaction"), 0o600),
				WriteFileTransactionOp(later, []byte("later"), 0o600),
			}, hooks)

			require.Error(t, err)
			assert.ErrorIs(t, err, failure)
			assert.ErrorIs(t, err, ErrFileTransactionConcurrentEdit)
			assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
			assert.Contains(t, err.Error(), path)
			content, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, test.wantAtPath, string(content))
			recoveries := collectRecoveryErrors(err)
			require.NotEmpty(t, recoveries)
			assertReportedRecoveryArtifactsPrivate(t, err, recoveries)
			if test.wantInStore != "" {
				assertRecoveryContains(t, recoveries, test.wantInStore)
			}
		})
	}
}

func TestRollbackDetectsIdenticalContentRecreationByIdentity(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	later := filepath.Join(dir, "later.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	failure := errors.New("later operation failed")
	guardCalls := 0

	err := ApplyFileTransactionWithHooks([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, FileTransactionHooks{
		BeforeMutation: failPath(later, failure),
		AfterGuardBeforeQuarantine: func(original, _ string) error {
			if original != path {
				return nil
			}
			guardCalls++
			if guardCalls != 2 {
				return nil
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			return os.WriteFile(path, []byte("transaction"), 0o600)
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "transaction", string(content))
	assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
}

func TestRollbackPreservesUserEditsInsideQuarantineAndAtDestination(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	later := filepath.Join(dir, "later.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	failure := errors.New("later operation failed")
	quarantineCalls := 0

	err := ApplyFileTransactionWithHooks([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, FileTransactionHooks{
		BeforeMutation: failPath(later, failure),
		AfterQuarantineBeforeValidation: func(original, recovery string) error {
			if original != path {
				return nil
			}
			quarantineCalls++
			if quarantineCalls != 2 {
				return nil
			}
			if err := os.WriteFile(recovery, []byte("user recovery edit"), 0o600); err != nil {
				return err
			}
			return os.WriteFile(path, []byte("user destination edit"), 0o600)
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, failure)
	assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
	destination, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "user destination edit", string(destination))
	recoveries := collectRecoveryErrors(err)
	assertReportedRecoveryArtifactsPrivate(t, err, recoveries)
	assertRecoveryContains(t, recoveries, "user recovery edit")
}

func TestCleanupPreservesSwappedQuarantineNamespace(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	later := filepath.Join(dir, "later.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	failure := errors.New("later operation failed")
	injected := false

	err := ApplyFileTransactionWithHooks([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, FileTransactionHooks{
		BeforeMutation: failPath(later, failure),
		DuringQuarantineCleanup: func(original, recovery string) error {
			if original != path || injected {
				return nil
			}
			injected = true
			directory := filepath.Dir(recovery)
			if err := os.Rename(directory, directory+"-moved"); err != nil {
				return err
			}
			if err := os.Mkdir(directory, 0o700); err != nil {
				return err
			}
			return os.WriteFile(recovery, []byte("user quarantine namespace"), 0o600)
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "before", string(content))
	recoveries := collectRecoveryErrors(err)
	assertReportedRecoveryArtifactsPrivate(t, err, recoveries)
	assertRecoveryContains(t, recoveries, "user quarantine namespace")
	assertRecoveryContains(t, recoveries, "transaction")
}

func TestRollbackPreservesConcurrentRecreationOfMissingPreState(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "created.txt")
	later := filepath.Join(dir, "later.txt")
	failure := errors.New("later operation failed")

	err := ApplyFileTransactionWithHooks([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600).WithExpectedMissing(),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, FileTransactionHooks{
		BeforeMutation: func(original, _ string) error {
			if original == later {
				return failure
			}
			return nil
		},
		AfterQuarantineBeforeValidation: func(original, _ string) error {
			if original == path {
				return os.WriteFile(path, []byte("user recreation"), 0o600)
			}
			return nil
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "user recreation", string(content))
	assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
}

func TestRollbackOfFailingMutationPreservesUserEdit(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	failure := errors.New("mutation reported failure")

	err := ApplyFileTransactionWithHooks([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
	}, FileTransactionHooks{AfterMutation: func(original, _ string) error {
		if original != path {
			return nil
		}
		if err := os.WriteFile(path, []byte("user after failing mutation"), 0o600); err != nil {
			return err
		}
		return failure
	}})

	require.Error(t, err)
	assert.ErrorIs(t, err, failure)
	assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "user after failing mutation", string(content))
	assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
}

func TestRollbackPreservesConcurrentSymlinkAndNestedTree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links may require elevated privileges")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	t.Run("symbolic link", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "link")
		later := filepath.Join(dir, "later")
		require.NoError(t, os.Symlink("original-target", path))
		failure := errors.New("later failed")
		err := ApplyFileTransactionWithHooks([]FileTransactionOp{
			RemoveFileTransactionOp(path),
			WriteFileTransactionOp(later, []byte("later"), 0o600),
		}, FileTransactionHooks{
			BeforeMutation: failPath(later, failure),
			AfterValidationBeforeRestore: func(original, _ string) error {
				if original == path {
					return os.Symlink("user-target", path)
				}
				return nil
			},
		})
		require.Error(t, err)
		target, readErr := os.Readlink(path)
		require.NoError(t, readErr)
		assert.Equal(t, "user-target", target)
		assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
	})

	t.Run("nested directory appears during restore", func(t *testing.T) {
		dir := t.TempDir()
		tree := filepath.Join(dir, "tree")
		nested := filepath.Join(tree, "nested")
		managed := filepath.Join(nested, "managed.txt")
		later := filepath.Join(dir, "later")
		require.NoError(t, os.MkdirAll(nested, 0o700))
		require.NoError(t, os.WriteFile(managed, []byte("managed"), 0o600))
		failure := errors.New("later failed")
		err := ApplyFileTransactionWithHooks([]FileTransactionOp{
			RemoveAllTransactionOp(tree),
			WriteFileTransactionOp(later, []byte("later"), 0o600),
		}, FileTransactionHooks{
			BeforeMutation: failPath(later, failure),
			DuringExclusiveRestore: func(original, _ string) error {
				if original != tree {
					return nil
				}
				if err := os.Mkdir(nested, 0o700); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(nested, "user.txt"), []byte("user tree"), 0o600)
			},
		})
		require.Error(t, err)
		content, readErr := os.ReadFile(filepath.Join(nested, "user.txt"))
		require.NoError(t, readErr)
		assert.Equal(t, "user tree", string(content))
		assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
	})
}

func TestCommittedTransactionReportsDestinationChangeDuringCleanup(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	path := filepath.Join(t.TempDir(), "managed.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	injected := false
	err := ApplyFileTransactionWithHooks([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
	}, FileTransactionHooks{DuringQuarantineCleanup: func(original, _ string) error {
		if original == path && !injected {
			injected = true
			return os.WriteFile(path, []byte("user during commit cleanup"), 0o600)
		}
		return nil
	}})

	require.Error(t, err)
	var cleanupErr *FileTransactionCleanupError
	assert.ErrorAs(t, err, &cleanupErr)
	assert.ErrorIs(t, err, ErrFileTransactionConcurrentEdit)
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "user during commit cleanup", string(content))
}

func TestFileTransactionFailsClosedWhenNoReplaceIsUnavailable(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		t.Skip("this platform supplies an atomic no-replace rename")
	}
	path := filepath.Join(t.TempDir(), "managed.txt")
	err := ApplyFileTransaction([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("managed"), 0o600).WithExpectedMissing(),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionNoReplaceUnsupported)
	assert.NoFileExists(t, path)
}

func failLaterTransaction(path, later string, operation FileTransactionOp) error {
	failure := errors.New("later operation failed")
	return ApplyFileTransactionWithHooks([]FileTransactionOp{
		operation,
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, FileTransactionHooks{BeforeMutation: failPath(later, failure)})
}

func failPath(path string, failure error) FileTransactionHook {
	return func(original, _ string) error {
		if original == path {
			return failure
		}
		return nil
	}
}

func collectRecoveryErrors(err error) []*FileTransactionRecoveryError {
	var recoveries []*FileTransactionRecoveryError
	seen := map[*FileTransactionRecoveryError]struct{}{}
	var visit func(error)
	visit = func(current error) {
		if current == nil {
			return
		}
		if recovery, ok := current.(*FileTransactionRecoveryError); ok {
			if _, exists := seen[recovery]; !exists {
				seen[recovery] = struct{}{}
				recoveries = append(recoveries, recovery)
			}
		}
		switch wrapped := current.(type) {
		case interface{ Unwrap() []error }:
			for _, child := range wrapped.Unwrap() {
				visit(child)
			}
		case interface{ Unwrap() error }:
			visit(wrapped.Unwrap())
		}
	}
	visit(err)
	return recoveries
}

func assertReportedRecoveryArtifactsPrivate(t *testing.T, transactionErr error, recoveries []*FileTransactionRecoveryError) {
	t.Helper()
	require.NotEmpty(t, recoveries)
	for _, recovery := range recoveries {
		assert.Contains(t, transactionErr.Error(), recovery.OriginalPath)
		if recovery.RecoveryPath == "" || recovery.Reattached {
			continue
		}
		assert.Contains(t, transactionErr.Error(), recovery.RecoveryPath)
		info, err := os.Stat(filepath.Dir(recovery.RecoveryPath))
		require.NoError(t, err, recovery.RecoveryPath)
		assert.True(t, info.IsDir())
		assert.Zero(t, info.Mode().Perm()&0o077, "recovery directory must be private: %s", recovery.RecoveryPath)
	}
}

func assertRecoveryContains(t *testing.T, recoveries []*FileTransactionRecoveryError, expected string) {
	t.Helper()
	for _, recovery := range recoveries {
		if recovery.Reattached || recovery.RecoveryPath == "" {
			continue
		}
		content, err := os.ReadFile(recovery.RecoveryPath)
		if err == nil && string(content) == expected {
			return
		}
	}
	require.Fail(t, "expected recovery content was not reported", expected)
}

func requireNoRecoveryArtifacts(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".slipway-recovery-*"))
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func testSHA256(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

func assertReadOnlyDirectoryMode(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS == "windows" {
		assert.Zero(t, info.Mode().Perm()&0o200)
		return
	}
	assert.Equal(t, os.FileMode(0o555), info.Mode().Perm())
}

func TestRecoveryErrorNamesBothPaths(t *testing.T) {
	err := &FileTransactionRecoveryError{
		OriginalPath: "/repo/managed.txt",
		RecoveryPath: "/repo/.slipway-recovery-token/snapshot",
		Rollback:     true,
		Cause:        errors.New("destination occupied"),
	}
	assert.ErrorIs(t, err, ErrFileTransactionConcurrentEdit)
	assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
	assert.True(t, strings.Contains(err.Error(), err.OriginalPath))
	assert.True(t, strings.Contains(err.Error(), err.RecoveryPath))
}
