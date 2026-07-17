//go:build windows

package runstore

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestNamedMutexWriterLockPinsOwningThreadUntilUnlock(t *testing.T) {
	handle, err := windows.CreateMutex(nil, false, nil)
	require.NoError(t, err)
	require.NotZero(t, handle)

	lock := &namedMutexWriterLock{handle: handle}
	t.Cleanup(func() {
		if lock.threadLocked {
			assert.NoError(t, lock.unlock())
		}
		assert.NoError(t, lock.close())
	})

	locked, err := lock.tryLock()
	require.NoError(t, err)
	require.True(t, locked)
	require.True(t, lock.threadLocked)

	ownerThread := windows.GetCurrentThreadId()
	for range 100 {
		runtime.Gosched()
	}
	assert.Equal(t, ownerThread, windows.GetCurrentThreadId())

	require.NoError(t, lock.unlock())
	assert.False(t, lock.threadLocked)
}
