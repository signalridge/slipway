//go:build !windows

package fsutil

import (
	"errors"
	"fmt"
	"os"
)

// RestrictToOwner is a no-op where callers already enforce private POSIX modes.
func RestrictToOwner(file *os.File) error {
	return restrictToOwner(file)
}

func restrictToOwner(file *os.File) error {
	if file == nil {
		return errors.New("restrict owner access: nil file")
	}
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect owner access target: %w", err)
	}
	if !info.IsDir() && !info.Mode().IsRegular() {
		return errors.New("owner access target is not a regular file or directory")
	}
	if err := file.Chmod(info.Mode().Perm() & 0o700); err != nil {
		return fmt.Errorf("restrict owner access: %w", err)
	}
	return nil
}

func ownerProtectionIsPrivate(_ *os.File, mode os.FileMode) bool {
	return mode.Perm()&0o077 == 0
}
