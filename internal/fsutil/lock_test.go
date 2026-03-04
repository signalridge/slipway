package fsutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gofrs/flock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateLockAcquireWritesMetaAndReleaseCleansIt(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".spln", "state.lock")
	l := NewStateLock(lockPath)

	ctx := context.Background()
	held, err := l.Acquire(ctx, 2*time.Second, "spln test")
	require.NoError(t, err)

	meta, err := l.ReadMeta()
	require.NoError(t, err)
	assert.Equal(t, "spln test", meta.Command)
	assert.NotZero(t, meta.HolderPID)
	assert.False(t, meta.AcquiredAt.IsZero())

	require.NoError(t, held.Release())

	_, err = os.Stat(l.MetaPath())
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestStateLockAcquireTimeout(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".spln", "state.lock")
	require.NoError(t, os.MkdirAll(filepath.Dir(lockPath), 0o755))

	other := flock.New(lockPath)
	require.NoError(t, other.Lock())
	defer func() { _ = other.Unlock() }()

	l := NewStateLock(lockPath)
	_, err := l.Acquire(context.Background(), 30*time.Millisecond, "spln timeout")
	require.ErrorIs(t, err, ErrLockTimeout)
}

func TestCleanupStaleLockRequiresDeadHolderAndAge(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".spln", "state.lock")
	l := NewStateLock(lockPath)

	require.NoError(t, os.MkdirAll(filepath.Dir(lockPath), 0o755))
	require.NoError(t, os.WriteFile(lockPath, []byte(""), 0o644))

	old := time.Now().Add(-5 * time.Minute)
	err := l.WriteMeta(LockMeta{
		HolderPID:  12345,
		AcquiredAt: old,
		Command:    "spln do",
	})
	require.NoError(t, err)

	cleaned, err := l.CleanupStale(2*time.Minute, time.Now(), func(pid int) bool {
		return false
	})
	require.NoError(t, err)
	assert.True(t, cleaned)

	_, err = os.Stat(lockPath)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}
