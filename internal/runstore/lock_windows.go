//go:build windows

package runstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

type namedMutexWriterLock struct {
	handle windows.Handle
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
	result, err := windows.WaitForSingleObject(lock.handle, 0)
	if err != nil {
		return false, err
	}
	switch result {
	case windows.WAIT_OBJECT_0, windows.WAIT_ABANDONED:
		return true, nil
	case uint32(windows.WAIT_TIMEOUT):
		return false, nil
	default:
		return false, fmt.Errorf("unexpected mutex wait result %#x", result)
	}
}

func (lock *namedMutexWriterLock) unlock() error {
	return windows.ReleaseMutex(lock.handle)
}

func (lock *namedMutexWriterLock) close() error {
	if lock.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(lock.handle)
	lock.handle = 0
	return err
}
