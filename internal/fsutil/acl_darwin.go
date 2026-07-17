//go:build darwin

package fsutil

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	darwinFileSecurityMagic = 0x012cc16d
	darwinNoACL             = ^uint32(0)
	darwinNoID              = ^uint32(0) - 100
)

type darwinFileSecurity struct {
	Magic      uint32
	Owner      [16]byte
	Group      [16]byte
	EntryCount uint32
	Flags      uint32
}

// RestrictToOwner removes extended ACL entries and retains only owner POSIX
// permissions on the already-open file or directory.
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
		return fmt.Errorf("restrict owner access mode: %w", err)
	}
	if err := removeDarwinExtendedACL(file); err != nil {
		return fmt.Errorf("restrict owner access ACL: %w", err)
	}
	info, err = file.Stat()
	if err != nil {
		return fmt.Errorf("verify owner access mode: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("verify owner access mode: permissions are %04o", info.Mode().Perm())
	}
	if err := verifyDarwinExtendedACLAbsent(file); err != nil {
		return fmt.Errorf("verify owner access ACL: %w", err)
	}
	return nil
}

func ownerProtectionIsPrivate(file *os.File, mode os.FileMode) bool {
	return mode.Perm()&0o077 == 0 && verifyDarwinExtendedACLAbsent(file) == nil
}

func removeDarwinExtendedACL(file *os.File) error {
	security := darwinFileSecurity{
		Magic:      darwinFileSecurityMagic,
		EntryCount: darwinNoACL,
	}
	minusOne := int(-1)
	_, _, errno := unix.Syscall6(
		unix.SYS_FCHMOD_EXTENDED,
		file.Fd(),
		uintptr(darwinNoID),
		uintptr(darwinNoID),
		uintptr(minusOne),
		uintptr(unsafe.Pointer(&security)),
		0,
	)
	runtime.KeepAlive(file)
	runtime.KeepAlive(&security)
	if errno != 0 {
		return errno
	}
	return nil
}

func verifyDarwinExtendedACLAbsent(file *os.File) error {
	var stat unix.Stat_t
	var security darwinFileSecurity
	size := uintptr(unsafe.Sizeof(security))
	_, _, errno := unix.Syscall6(
		unix.SYS_FSTAT64_EXTENDED,
		file.Fd(),
		uintptr(unsafe.Pointer(&stat)),
		uintptr(unsafe.Pointer(&security)),
		uintptr(unsafe.Pointer(&size)),
		0,
		0,
	)
	runtime.KeepAlive(file)
	runtime.KeepAlive(&stat)
	runtime.KeepAlive(&security)
	if errno != 0 {
		return errno
	}
	if size != unsafe.Sizeof(security) {
		return fmt.Errorf("extended ACL data requires %d bytes", size)
	}
	if security.Magic != darwinFileSecurityMagic {
		return fmt.Errorf("unexpected file security magic %#x", security.Magic)
	}
	if security.EntryCount != darwinNoACL {
		return fmt.Errorf("extended ACL remains present with %d entries", security.EntryCount)
	}
	return nil
}
