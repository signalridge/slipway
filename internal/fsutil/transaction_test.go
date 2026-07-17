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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func applyFileTransactionForTest(ops []FileTransactionOp) error {
	return applyFileTransactionAt("", ops, fileTransactionHooks{}, false)
}

func applyFileTransactionWithHooksForTest(ops []FileTransactionOp, hooks fileTransactionHooks) error {
	return applyFileTransactionAt("", ops, hooks, false)
}

func applyFileTransactionWithinWithHooksForTest(root string, ops []FileTransactionOp, hooks fileTransactionHooks) error {
	return applyFileTransactionAt(root, ops, hooks, true)
}

func TestApplyFileTransactionRollsBackAppliedAndFailingWrites(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	t.Run("later operation fails", func(t *testing.T) {
		dir := t.TempDir()
		first := filepath.Join(dir, "first.txt")
		later := filepath.Join(dir, "later.txt")
		require.NoError(t, os.WriteFile(first, []byte("before"), 0o640))
		failure := errors.New("later operation failed")

		err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
			WriteFileTransactionOp(first, []byte("transaction"), 0o600),
			WriteFileTransactionOp(later, []byte("later"), 0o600),
		}, fileTransactionHooks{BeforeMutation: func(path, _ string) error {
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
		assertFileMode(t, first, 0o640)
		assert.NoFileExists(t, later)
		requireNoRecoveryArtifacts(t, dir)
	})

	t.Run("failing mutation already committed", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "managed.txt")
		require.NoError(t, os.WriteFile(path, []byte("before"), 0o640))
		failure := errors.New("post-write sync report failed")

		err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
			WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		}, fileTransactionHooks{AfterMutation: func(original, _ string) error {
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
		assertFileMode(t, path, 0o640)
		requireNoRecoveryArtifacts(t, dir)
	})
}

func TestRollbackReattachesOriginalFileIdentityAndMetadata(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	later := filepath.Join(dir, "later.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o640))
	fixedTime := time.Unix(1_700_000_000, 0)
	require.NoError(t, os.Chtimes(path, fixedTime, fixedTime))
	before, err := os.Stat(path)
	require.NoError(t, err)
	failure := errors.New("later operation failed")

	err = applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, fileTransactionHooks{BeforeMutation: failPath(later, failure)})
	require.Error(t, err)
	assert.ErrorIs(t, err, failure)
	after, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.True(t, os.SameFile(before, after), "rollback must reattach the quarantined object")
	assert.Equal(t, before.Mode().Perm(), after.Mode().Perm())
	assert.Equal(t, before.ModTime(), after.ModTime())
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "before", string(content))
	requireNoRecoveryArtifacts(t, dir)
}

func TestApplyFileTransactionRestoresRemovedSnapshotKinds(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
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
		assertFileMode(t, path, 0o640)
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
}

func TestFileTransactionPreconditionsPreservePlannedUserPaths(t *testing.T) {
	dir := t.TempDir()
	created := filepath.Join(dir, "created.txt")
	require.NoError(t, os.WriteFile(created, []byte("user"), 0o600))
	err := applyFileTransactionForTest([]FileTransactionOp{
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
	err = applyFileTransactionForTest([]FileTransactionOp{
		RemoveFileTransactionOp(guarded).WithExpectedSHA256(hash),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionPrecondition)
	content, readErr = os.ReadFile(guarded)
	require.NoError(t, readErr)
	assert.Equal(t, "user edit", string(content))
}

func TestFileTransactionPreflightsAllOperationsBeforeFirstMutation(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	later := filepath.Join(dir, "later.txt")
	require.NoError(t, os.WriteFile(first, []byte("first-before"), 0o600))
	require.NoError(t, os.WriteFile(later, []byte("later-before"), 0o600))

	var mutationPaths []string
	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(first, []byte("first-after"), 0o600),
		WriteFileTransactionOp(later, []byte("later-after"), 0o600).
			WithExpectedSHA256(testSHA256([]byte("different-content"))),
	}, fileTransactionHooks{
		BeforeMutation: func(path, _ string) error {
			mutationPaths = append(mutationPaths, path)
			return nil
		},
	})
	require.ErrorIs(t, err, ErrFileTransactionPrecondition)
	assert.Empty(t, mutationPaths, "preflight failure must happen before the first mutation hook")
	firstContent, readErr := os.ReadFile(first)
	require.NoError(t, readErr)
	assert.Equal(t, "first-before", string(firstContent))
	laterContent, readErr := os.ReadFile(later)
	require.NoError(t, readErr)
	assert.Equal(t, "later-before", string(laterContent))
}

func TestRollbackConcurrentEditWindowsPreserveUserBytes(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	tests := []struct {
		name        string
		hooks       func(path string) fileTransactionHooks
		wantAtPath  string
		wantInStore string
	}{
		{
			name: "after guard snapshot before quarantine",
			hooks: func(path string) fileTransactionHooks {
				calls := 0
				return fileTransactionHooks{AfterGuardBeforeQuarantine: func(original, _ string) error {
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
			hooks: func(path string) fileTransactionHooks {
				calls := 0
				return fileTransactionHooks{AfterQuarantineBeforeValidation: func(original, _ string) error {
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
			hooks: func(path string) fileTransactionHooks {
				return fileTransactionHooks{AfterValidationBeforeRestore: func(original, _ string) error {
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
			hooks: func(path string) fileTransactionHooks {
				return fileTransactionHooks{DuringExclusiveRestore: func(original, _ string) error {
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
			hooks: func(path string) fileTransactionHooks {
				return fileTransactionHooks{AfterRestoreBeforePostValidation: func(original, _ string) error {
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
			hooks: func(path string) fileTransactionHooks {
				injected := false
				return fileTransactionHooks{DuringQuarantineCleanup: func(original, _ string) error {
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
			hooks: func(path string) fileTransactionHooks {
				injected := false
				return fileTransactionHooks{DuringQuarantineCleanup: func(original, recovery string) error {
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

			err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
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
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	later := filepath.Join(dir, "later.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	failure := errors.New("later operation failed")
	guardCalls := 0
	identityReused := false

	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, fileTransactionHooks{
		BeforeMutation: failPath(later, failure),
		AfterGuardBeforeQuarantine: func(original, _ string) error {
			if original != path {
				return nil
			}
			guardCalls++
			if guardCalls != 2 {
				return nil
			}
			beforeReplacement, err := snapshotFileIdentity(path)
			if err != nil {
				return err
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte("transaction"), 0o600); err != nil {
				return err
			}
			afterReplacement, err := snapshotFileIdentity(path)
			if err != nil {
				return err
			}
			identityReused = os.SameFile(beforeReplacement, afterReplacement)
			return nil
		},
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
	assert.False(t, identityReused, "the transaction identity lease must keep a removed object from being reused")
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "transaction", string(content))
	assertReportedRecoveryArtifactsPrivate(t, err, collectRecoveryErrors(err))
}

func TestFailedInstallPinsStageBeforeCleanupValidation(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	stageCleanupObserved := false
	identityReused := false

	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
	}, fileTransactionHooks{
		AfterQuarantineBeforeValidation: func(original, _ string) error {
			if original != path {
				return nil
			}
			return os.WriteFile(path, []byte("user destination"), 0o600)
		},
		DuringQuarantineCleanup: func(original, recovery string) error {
			if original != path || filepath.Base(recovery) != transactionStageItem {
				return nil
			}
			stageCleanupObserved = true
			beforeReplacement, err := snapshotFileIdentity(recovery)
			if err != nil {
				return err
			}
			if err := os.Remove(recovery); err != nil {
				return err
			}
			if err := os.WriteFile(recovery, []byte("transaction"), 0o600); err != nil {
				return err
			}
			afterReplacement, err := snapshotFileIdentity(recovery)
			if err != nil {
				return err
			}
			identityReused = os.SameFile(beforeReplacement, afterReplacement)
			return nil
		},
	})

	require.Error(t, err)
	assert.True(t, stageCleanupObserved)
	assert.False(t, identityReused, "the staged identity must remain pinned throughout failed-install cleanup")
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "user destination", string(content))
	recoveries := collectRecoveryErrors(err)
	require.NotEmpty(t, recoveries)
	assertRecoveryContains(t, recoveries, "transaction")
	assertRecoveryContains(t, recoveries, "before")
	assertReportedRecoveryArtifactsPrivate(t, err, recoveries)
}

func TestCleanupPreservesEntryReplacedAfterValidation(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	hookObserved := false
	identityReused := false

	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
	}, fileTransactionHooks{AfterQuarantineValidationBeforeRelocation: func(original, recovery string) error {
		if original != path {
			return nil
		}
		hookObserved = true
		beforeReplacement, err := snapshotFileIdentity(recovery)
		if err != nil {
			return err
		}
		if err := os.Remove(recovery); err != nil {
			return err
		}
		if err := os.WriteFile(recovery, []byte("before"), 0o600); err != nil {
			return err
		}
		afterReplacement, err := snapshotFileIdentity(recovery)
		if err != nil {
			return err
		}
		identityReused = os.SameFile(beforeReplacement, afterReplacement)
		return nil
	}})

	require.Error(t, err)
	var cleanupErr *FileTransactionCleanupError
	require.ErrorAs(t, err, &cleanupErr)
	assert.True(t, hookObserved)
	assert.False(t, identityReused, "the validated quarantine identity must remain pinned through relocation")
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "transaction", string(content), "the requested mutation was already committed")
	recoveries := collectRecoveryErrors(err)
	require.NotEmpty(t, recoveries)
	assertRecoveryContains(t, recoveries, "before")
	assertReportedRecoveryArtifactsPrivate(t, err, recoveries)
}

func TestRollbackPreservesUserEditsInsideQuarantineAndAtDestination(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	later := filepath.Join(dir, "later.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	failure := errors.New("later operation failed")
	quarantineCalls := 0

	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, fileTransactionHooks{
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
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	later := filepath.Join(dir, "later.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	failure := errors.New("later operation failed")
	swapAttempted := false
	var swapErr error

	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, fileTransactionHooks{
		BeforeMutation: failPath(later, failure),
		DuringQuarantineCleanup: func(original, recovery string) error {
			if original != path || swapAttempted {
				return nil
			}
			swapAttempted = true
			directory := filepath.Dir(recovery)
			swapErr = os.Rename(directory, directory+"-moved")
			if swapErr != nil {
				if runtime.GOOS == "windows" {
					return nil
				}
				return swapErr
			}
			if err := os.Mkdir(directory, 0o700); err != nil {
				return err
			}
			return os.WriteFile(recovery, []byte("user quarantine namespace"), 0o600)
		},
	})

	require.Error(t, err)
	require.True(t, swapAttempted)
	if runtime.GOOS == "windows" {
		// Windows blocks the directory rename while the transaction lease holds
		// a descendant handle, so the namespace replacement never occurs.
		require.Error(t, swapErr, "the transaction identity lease must block quarantine namespace replacement")
		assert.ErrorIs(t, err, failure)
		assert.NotErrorIs(t, err, ErrFileTransactionRollbackPrecondition)
		content, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "before", string(content))
		requireNoRecoveryArtifacts(t, dir)
		return
	}
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
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "created.txt")
	later := filepath.Join(dir, "later.txt")
	failure := errors.New("later operation failed")

	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600).WithExpectedMissing(),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, fileTransactionHooks{
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
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "managed.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	failure := errors.New("mutation reported failure")

	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
	}, fileTransactionHooks{AfterMutation: func(original, _ string) error {
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

func TestRollbackPreservesConcurrentSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links may require elevated privileges")
	}
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "link")
	later := filepath.Join(dir, "later")
	require.NoError(t, os.Symlink("original-target", path))
	failure := errors.New("later failed")
	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		RemoveFileTransactionOp(path),
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, fileTransactionHooks{
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
}

func TestCommittedTransactionReportsDestinationChangeDuringCleanup(t *testing.T) {
	if !atomicNoReplaceAvailableForTest() {
		t.Skip("atomic no-replace rename intentionally fails closed on this platform")
	}
	path := filepath.Join(t.TempDir(), "managed.txt")
	require.NoError(t, os.WriteFile(path, []byte("before"), 0o600))
	injected := false
	err := applyFileTransactionWithHooksForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("transaction"), 0o600),
	}, fileTransactionHooks{DuringQuarantineCleanup: func(original, _ string) error {
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
	if atomicNoReplaceAvailableForTest() {
		t.Skip("this platform supplies an atomic no-replace rename")
	}
	path := filepath.Join(t.TempDir(), "managed.txt")
	err := applyFileTransactionForTest([]FileTransactionOp{
		WriteFileTransactionOp(path, []byte("managed"), 0o600).WithExpectedMissing(),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrFileTransactionNoReplaceUnsupported)
	assert.NoFileExists(t, path)
}

func failLaterTransaction(path, later string, operation FileTransactionOp) error {
	failure := errors.New("later operation failed")
	return applyFileTransactionWithHooksForTest([]FileTransactionOp{
		operation,
		WriteFileTransactionOp(later, []byte("later"), 0o600),
	}, fileTransactionHooks{BeforeMutation: failPath(later, failure)})
}

func failPath(path string, failure error) fileTransactionHook {
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
		info, err := os.Lstat(filepath.Dir(recovery.RecoveryPath))
		require.NoError(t, err, recovery.RecoveryPath)
		assert.True(t, info.IsDir())
		assert.Zero(t, info.Mode()&os.ModeSymlink)
		directory, openErr := os.Open(filepath.Dir(recovery.RecoveryPath))
		require.NoError(t, openErr)
		assert.True(
			t,
			ownerProtectionIsPrivate(directory, info.Mode()),
			"recovery directory must be private: %s",
			recovery.RecoveryPath,
		)
		require.NoError(t, directory.Close())
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

// snapshotFileIdentity captures identity from an open handle so the returned
// FileInfo remains a historical snapshot after the pathname is replaced.
func snapshotFileIdentity(path string) (os.FileInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	info, statErr := file.Stat()
	closeErr := file.Close()
	if statErr != nil || closeErr != nil {
		return nil, errors.Join(statErr, closeErr)
	}
	return info, nil
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

func assertFileMode(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	actual := fileMode(t, path)
	if runtime.GOOS == "windows" {
		assert.Equal(t, expected&0o200, actual&0o200, "Windows only exposes the read-only attribute through os.FileMode")
		return
	}
	assert.Equal(t, expected, actual)
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
