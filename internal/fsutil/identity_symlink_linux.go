//go:build linux

package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"golang.org/x/sys/unix"
)

func openSymlinkIdentity(root *os.Root, name string) (*os.File, error) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return nil, fmt.Errorf("symlink identity requires one leaf name: %q", name)
	}

	directory, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open symlink identity parent: %w", err)
	}
	defer directory.Close()

	fd, err := unix.Openat(int(directory.Fd()), name, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
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
	for size := 256; size <= 1<<20; size *= 2 {
		buffer := make([]byte, size)
		count, err := unix.Readlinkat(int(identity.Fd()), "", buffer)
		runtime.KeepAlive(identity)
		if err != nil {
			return "", fmt.Errorf("read pinned symlink: %w", err)
		}
		if count < len(buffer) {
			return string(buffer[:count]), nil
		}
	}
	return "", fmt.Errorf("read pinned symlink: target exceeds 1 MiB")
}

func validateSymlinkTransactionIdentity(_ os.FileInfo) error {
	return nil
}
