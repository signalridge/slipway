//go:build !windows

package fsutil

import "os"

// RestrictToOwner is a no-op where callers already enforce private POSIX modes.
func RestrictToOwner(path string) error {
	return restrictToOwner(path)
}

func restrictToOwner(string) error {
	return nil
}

func ownerProtectionIsPrivate(_ string, mode os.FileMode) bool {
	return mode.Perm()&0o077 == 0
}
