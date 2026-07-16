//go:build windows

package runstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/windows"
)

type namedMutexWriterLock struct {
	handle       windows.Handle
	threadLocked bool
}

func openRunWriterLock(run *runHandle) (runWriterLock, error) {
	canonical := strings.ToLower(filepath.Clean(run.store.commonDir))
	digest := sha256.Sum256([]byte(canonical + "\x00" + run.id))
	name, err := windows.UTF16PtrFromString("Global\\slipway-run-" + hex.EncodeToString(digest[:]))
	if err != nil {
		return nil, fmt.Errorf("encode run writer mutex name: %w", err)
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, fmt.Errorf("create run writer mutex: %w", err)
	}
	if handle == 0 {
		return nil, errors.New("create run writer mutex returned an invalid handle")
	}
	return &namedMutexWriterLock{handle: handle}, nil
}

func (lock *namedMutexWriterLock) tryLock() (bool, error) {
	if lock.threadLocked {
		return false, errors.New("run writer mutex is already owned")
	}

	// Windows mutex ownership belongs to the waiting OS thread, not to the Go
	// goroutine. Pin before waiting so ReleaseMutex runs on the same thread even
	// if the goroutine blocks or yields while the transaction is in progress.
	runtime.LockOSThread()
	result, err := windows.WaitForSingleObject(lock.handle, 0)
	if err != nil {
		runtime.UnlockOSThread()
		return false, err
	}
	switch result {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
		lock.threadLocked = true
		return true, nil
	case uint32(windows.WAIT_TIMEOUT):
		runtime.UnlockOSThread()
		return false, nil
	default:
		runtime.UnlockOSThread()
		return false, fmt.Errorf("unexpected mutex wait result %#x", result)
	}
}

// Windows named mutexes do not provide a shared mode. Read-only callers retain
// the same cross-process commit-boundary exclusion as writers on this platform.
func (lock *namedMutexWriterLock) tryReadLock() (bool, error) {
	return lock.tryLock()
}

func (lock *namedMutexWriterLock) unlock() error {
	if !lock.threadLocked {
		return errors.New("run writer mutex is not owned")
	}
	err := windows.ReleaseMutex(lock.handle)
	lock.threadLocked = false
	runtime.UnlockOSThread()
	return err
}

func (lock *namedMutexWriterLock) close() error {
	if lock.threadLocked {
		return errors.New("close run writer mutex while owned")
	}
	if lock.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(lock.handle)
	lock.handle = 0
	return err
}
