package fsutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateLockAcquireWritesMetaAndReleaseCleansIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "locks", "state.lock")
	l := NewStateLock(lockPath)

	ctx := context.Background()
	held, err := l.Acquire(ctx, 2*time.Second, "slipway test")
	require.NoError(t, err)

	meta, err := l.ReadMeta()
	require.NoError(t, err)
	assert.Equal(t, "slipway test", meta.Command)
	assert.NotZero(t, meta.HolderPID)
	assert.False(t, meta.AcquiredAt.IsZero())

	require.NoError(t, held.Release())

	_, err = os.Stat(l.LockPath() + ".meta")
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestStateLockAcquireTimeout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "locks", "state.lock")
	require.NoError(t, os.MkdirAll(filepath.Dir(lockPath), 0o755))

	other := flock.New(lockPath)
	require.NoError(t, other.Lock())
	defer func() { _ = other.Unlock() }()

	l := NewStateLock(lockPath)
	_, err := l.Acquire(context.Background(), 30*time.Millisecond, "slipway timeout")
	require.ErrorIs(t, err, ErrLockTimeout)
}

func TestConcurrentLocksDifferentPathsDoNotBlock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockA := NewStateLock(filepath.Join(dir, "requestA", "state.lock"))
	lockB := NewStateLock(filepath.Join(dir, "requestB", "state.lock"))

	ctx := context.Background()
	heldA, err := lockA.Acquire(ctx, 2*time.Second, "cmd-A")
	require.NoError(t, err)
	defer func() { _ = heldA.Release() }()

	// Lock B on a different path should succeed immediately.
	heldB, err := lockB.Acquire(ctx, 100*time.Millisecond, "cmd-B")
	require.NoError(t, err)
	defer func() { _ = heldB.Release() }()

	metaA, err := lockA.ReadMeta()
	require.NoError(t, err)
	assert.Equal(t, "cmd-A", metaA.Command)

	metaB, err := lockB.ReadMeta()
	require.NoError(t, err)
	assert.Equal(t, "cmd-B", metaB.Command)
}

func TestConcurrentLocksSamePathBlocks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "requestA", "state.lock")

	lockFirst := NewStateLock(lockPath)
	ctx := context.Background()
	held, err := lockFirst.Acquire(ctx, 2*time.Second, "first")
	require.NoError(t, err)
	defer func() { _ = held.Release() }()

	// Second acquire on the same path should timeout.
	lockSecond := NewStateLock(lockPath)
	_, err = lockSecond.Acquire(ctx, 50*time.Millisecond, "second")
	require.ErrorIs(t, err, ErrLockTimeout)
}

func TestConcurrentLocksGoroutineIsolation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPathA := filepath.Join(dir, "requestA", "state.lock")
	lockPathB := filepath.Join(dir, "requestB", "state.lock")

	ctx := context.Background()
	lockA := NewStateLock(lockPathA)
	heldA, err := lockA.Acquire(ctx, 2*time.Second, "goroutine-A")
	require.NoError(t, err)
	defer func() { _ = heldA.Release() }()

	// Goroutine acquires lock B while A is held — should succeed immediately.
	errCh := make(chan error, 1)
	go func() {
		lockB := NewStateLock(lockPathB)
		heldB, err := lockB.Acquire(ctx, 200*time.Millisecond, "goroutine-B")
		if err != nil {
			errCh <- err
			return
		}
		errCh <- heldB.Release()
	}()
	require.NoError(t, <-errCh)

	// Goroutine attempts lock A while A is held — should timeout.
	go func() {
		lockA2 := NewStateLock(lockPathA)
		_, err := lockA2.Acquire(ctx, 50*time.Millisecond, "goroutine-A2")
		errCh <- err
	}()
	err = <-errCh
	require.ErrorIs(t, err, ErrLockTimeout)
}

func TestCleanupStaleLockRequiresDeadHolderAndAge(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "locks", "state.lock")
	l := NewStateLock(lockPath)

	require.NoError(t, os.MkdirAll(filepath.Dir(lockPath), 0o755))
	require.NoError(t, os.WriteFile(lockPath, []byte(""), 0o644))

	old := time.Now().Add(-5 * time.Minute)
	err := l.WriteMeta(LockMeta{
		HolderPID:  12345,
		AcquiredAt: old,
		Command:    "slipway do",
	})
	require.NoError(t, err)

	cleaned, err := l.CleanupStale(2*time.Minute, time.Now(), func(pid int) bool {
		return false
	})
	require.NoError(t, err)
	assert.True(t, cleaned)

	_, err = os.Stat(lockPath)
	if runtime.GOOS == "windows" {
		require.NoError(t, err)
		return
	}
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestCleanupUnheldAnchorWithoutMetaRemovesOnlyUnlockedAnchor(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("windows cannot safely remove a locked anchor file while proving ownership")
	}

	dir := t.TempDir()
	lockPath := filepath.Join(dir, "locks", "state.lock")
	lock := NewStateLock(lockPath)

	require.NoError(t, os.MkdirAll(filepath.Dir(lockPath), 0o755))
	require.NoError(t, os.WriteFile(lockPath, []byte(""), 0o644))

	cleaned, err := lock.CleanupUnheldAnchorWithoutMeta()
	require.NoError(t, err)
	assert.True(t, cleaned)
	assert.NoFileExists(t, lockPath)
}

func TestCleanupUnheldAnchorWithoutMetaPreservesAnchorsWithMeta(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "locks", "state.lock")
	lock := NewStateLock(lockPath)

	require.NoError(t, os.MkdirAll(filepath.Dir(lockPath), 0o755))
	require.NoError(t, os.WriteFile(lockPath, []byte(""), 0o644))
	require.NoError(t, lock.WriteMeta(LockMeta{
		HolderPID:  12345,
		AcquiredAt: time.Now().UTC(),
		Command:    "slipway active",
	}))

	cleaned, err := lock.CleanupUnheldAnchorWithoutMeta()
	require.NoError(t, err)
	assert.False(t, cleaned)
	assert.FileExists(t, lockPath)
	assert.FileExists(t, lockPath+".meta")
}

func TestCleanupUnheldAnchorWithoutMetaPreservesHeldAnchor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "locks", "state.lock")
	require.NoError(t, os.MkdirAll(filepath.Dir(lockPath), 0o755))

	held := flock.New(lockPath)
	require.NoError(t, held.Lock())
	defer func() { _ = held.Unlock() }()

	cleaned, err := NewStateLock(lockPath).CleanupUnheldAnchorWithoutMeta()
	require.NoError(t, err)
	assert.False(t, cleaned)
	assert.FileExists(t, lockPath)
}

func TestHeldLockReleaseJoinsUnlockAndRemoveErrors(t *testing.T) {
	t.Parallel()

	unlockErr := errors.New("unlock failed")
	removeErr := errors.New("remove meta failed")

	held := &HeldLock{
		stateLock: &StateLock{metaPath: "/tmp/test.lock.meta"},
		unlock: func() error {
			return unlockErr
		},
		removeMeta: func(path string) error {
			assert.Equal(t, "/tmp/test.lock.meta", path)
			return removeErr
		},
	}

	err := held.Release()
	require.Error(t, err)
	assert.ErrorIs(t, err, unlockErr)
	assert.ErrorIs(t, err, removeErr)
}
