package fsutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

func TestWriteFileAtomicCrashSafetyOldOrNewVisibilityAndRepairableTemps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows denies rename while another reader has the target open")
	}
	t.Parallel()
	dir := t.TempDir()
	target := filepath.Join(dir, "state.yaml")
	oldData := bytes.Repeat([]byte("A"), 1024)
	newData := bytes.Repeat([]byte("B"), 2048)

	require.NoError(t, os.WriteFile(target, oldData, 0o644))

	writerErr := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			payload := oldData
			if i%2 == 1 {
				payload = newData
			}
			if err := WriteFileAtomic(target, payload, 0o644); err != nil {
				writerErr <- err
				return
			}
		}
	}()

readLoop:
	for {
		select {
		case <-done:
			break readLoop
		default:
			got, err := os.ReadFile(target)
			require.NoError(t, err)
			if !bytes.Equal(got, oldData) && !bytes.Equal(got, newData) {
				t.Fatalf("atomic visibility violated: observed non old/new content length=%d", len(got))
			}
		}
	}

	select {
	case err := <-writerErr:
		require.NoError(t, err)
	default:
	}

	staleTemp := filepath.Join(dir, ".tmp-state.yaml-stale")
	require.NoError(t, os.WriteFile(staleTemp, []byte("stale"), 0o644))

	deleted, err := CleanupAtomicTempArtifactsOlderThan(dir, 0, time.Now().UTC())
	require.NoError(t, err)
	assert.Contains(t, deleted, staleTemp)

	final, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(final, oldData) || bytes.Equal(final, newData))
}

func TestCleanupAtomicTempArtifactsOlderThanSkipsFreshTemps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	freshTemp := filepath.Join(dir, ".tmp-state.yaml-fresh")
	staleTemp := filepath.Join(dir, ".tmp-state.yaml-stale")
	require.NoError(t, os.WriteFile(freshTemp, []byte("fresh"), 0o644))
	require.NoError(t, os.WriteFile(staleTemp, []byte("stale"), 0o644))

	now := time.Date(2026, 4, 9, 1, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(freshTemp, now.Add(-time.Second), now.Add(-time.Second)))
	require.NoError(t, os.Chtimes(staleTemp, now.Add(-5*time.Minute), now.Add(-5*time.Minute)))

	deleted, err := CleanupAtomicTempArtifactsOlderThan(dir, 2*time.Minute, now)
	require.NoError(t, err)
	assert.Contains(t, deleted, staleTemp)
	assert.NotContains(t, deleted, freshTemp)

	_, err = os.Stat(freshTemp)
	require.NoError(t, err)
	_, err = os.Stat(staleTemp)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
