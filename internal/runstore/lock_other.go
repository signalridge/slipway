//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd && !solaris && !windows

package runstore

import (
	"errors"
	"os"
)

func tryLockFile(_ *os.File) (bool, error) {
	return false, errors.New("run journal locking is unsupported on this platform")
}

func unlockFile(_ *os.File) error { return nil }
