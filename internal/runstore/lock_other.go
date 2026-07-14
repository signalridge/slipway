//go:build !unix && !windows

package runstore

import "errors"

type unsupportedWriterLock struct{}

func openRunWriterLock(_ *runHandle) (runWriterLock, error) {
	return nil, errors.New("run journal locking is unsupported on this platform")
}

func (*unsupportedWriterLock) tryLock() (bool, error) { return false, nil }
func (*unsupportedWriterLock) unlock() error          { return nil }
func (*unsupportedWriterLock) close() error           { return nil }
