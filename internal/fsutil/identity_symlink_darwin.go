//go:build darwin

package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

//go:cgo_import_dynamic libc_freadlink freadlink "/usr/lib/libSystem.B.dylib"

var libcFreadlinkTrampolineAddr uintptr

//go:linkname syscallSyscall syscall.syscall
func syscallSyscall(fn, a1, a2, a3 uintptr) (r1, r2 uintptr, err syscall.Errno)

func openSymlinkIdentity(root *os.Root, name string) (*os.File, error) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return nil, fmt.Errorf("symlink identity requires one leaf name: %q", name)
	}

	directory, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open symlink identity parent: %w", err)
	}
	defer directory.Close()

	fd, err := unix.Openat(int(directory.Fd()), name, unix.O_SYMLINK|unix.O_CLOEXEC, 0)
	runtime.KeepAlive(directory)
	if err != nil {
		return nil, fmt.Errorf("open symlink identity %s: %w", name, err)
	}
	identity := os.NewFile(uintptr(fd), name)
	if identity == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("open symlink identity %s: invalid file descriptor", name)
	}
	if err := validateOpenedSymlinkIdentity(root, name, identity); err != nil {
		_ = identity.Close()
		return nil, err
	}
	return identity, nil
}

func validateOpenedSymlinkIdentity(root *os.Root, name string, identity *os.File) error {
	opened, err := identity.Stat()
	if err != nil {
		return fmt.Errorf("stat symlink identity %s: %w", name, err)
	}
	current, err := root.Lstat(name)
	if err != nil {
		return fmt.Errorf("inspect symlink identity %s: %w", name, err)
	}
	if opened.Mode()&os.ModeSymlink == 0 || current.Mode()&os.ModeSymlink == 0 || !os.SameFile(opened, current) {
		return fmt.Errorf("symlink identity %s changed while opening", name)
	}
	return nil
}

func readSymlinkIdentity(identity *os.File) (string, error) {
	target, err := readSymlinkIdentityFD(identity.Fd())
	runtime.KeepAlive(identity)
	return target, err
}

func readSymlinkIdentityFD(fd uintptr) (string, error) {
	for size := 256; size <= 1<<20; size *= 2 {
		buffer := make([]byte, size)
		// #nosec G103 -- freadlink requires a native pointer; buffer bounds and
		// lifetime are fixed by len(buffer) and the synchronous syscall below.
		count, _, errno := syscallSyscall(libcFreadlinkTrampolineAddr, fd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
		if errno != 0 {
			return "", fmt.Errorf("read pinned symlink: %w", errno)
		}
		if int(count) < len(buffer) {
			return string(buffer[:count]), nil
		}
	}
	return "", fmt.Errorf("read pinned symlink: target exceeds 1 MiB")
}

func validateSymlinkTransactionIdentity(_ os.FileInfo) error {
	return nil
}
