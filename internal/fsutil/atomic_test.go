package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsWindowsSharingViolation(t *testing.T) {
	t.Parallel()

	assert.True(t, isWindowsSharingViolation(syscall.Errno(32)), "ERROR_SHARING_VIOLATION should match")
	assert.True(t, isWindowsSharingViolation(syscall.Errno(5)), "ERROR_ACCESS_DENIED should match")
	assert.False(t, isWindowsSharingViolation(syscall.Errno(13)), "unrelated errno should not match")
	assert.False(t, isWindowsSharingViolation(errors.New("boom")), "plain error should not match")
}

func TestRenameWithRetryRetriesTransientWindowsSharingViolation(t *testing.T) {
	t.Parallel()

	renameCalls := 0
	sleepCalls := 0
	err := renameWithRetryFunc("old", "new", "windows", func(_, _ string) error {
		renameCalls++
		if renameCalls == 1 {
			return syscall.Errno(32)
		}
		return nil
	}, func(delay time.Duration) {
		sleepCalls++
		assert.Equal(t, renameRetryBaseDelay, delay)
	})

	require.NoError(t, err)
	assert.Equal(t, 2, renameCalls, "initial failure plus one successful retry")
	assert.Equal(t, 1, sleepCalls)
}

func TestRenameWithRetrySurfacesErrorAfterWindowsRetryBudget(t *testing.T) {
	t.Parallel()

	renameCalls := 0
	sleepCalls := 0
	err := renameWithRetryFunc("old", "new", "windows", func(_, _ string) error {
		renameCalls++
		return syscall.Errno(32)
	}, func(delay time.Duration) {
		sleepCalls++
		assert.Equal(t, renameRetryBaseDelay, delay)
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, syscall.Errno(32))
	assert.Equal(t, renameRetryAttempts+1, renameCalls, "initial rename plus bounded retries")
	assert.Equal(t, renameRetryAttempts, sleepCalls)
}

func TestWriteFileAtomicCreatesAndOverwrites(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "state.yaml")

	require.NoError(t, WriteFileAtomic(target, []byte("first"), 0o644))
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "first", string(got))

	require.NoError(t, WriteFileAtomic(target, []byte("second"), 0o644))
	got, err = os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "second", string(got))
}

func TestWriteFileAtomicReplacesContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "state.yaml")

	require.NoError(t, os.WriteFile(target, []byte("old"), 0o644))
	require.NoError(t, WriteFileAtomic(target, []byte("new"), 0o644))

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
}
