//go:build unix

package runstore

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type directoryWriterLock struct {
	file *os.File
}

func openRunWriterLock(run *runHandle) (runWriterLock, error) {
	file, err := run.root.Open(".")
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.IsDir() || !os.SameFile(info, run.identity) {
		_ = file.Close()
		return nil, errors.Join(errors.New("opened run directory changed before locking"), err)
	}
	return &directoryWriterLock{file: file}, nil
}

func (lock *directoryWriterLock) tryLock() (bool, error) {
	err := unix.Flock(int(lock.file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return false, nil
	}
	return false, err
}

func (lock *directoryWriterLock) unlock() error {
	return unix.Flock(int(lock.file.Fd()), unix.LOCK_UN)
}

func (lock *directoryWriterLock) close() error {
	if lock.file == nil {
		return nil
	}
	err := lock.file.Close()
	lock.file = nil
	if err != nil {
		return fmt.Errorf("close run directory lock: %w", err)
	}
	return nil
}
