//go:build darwin

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

func readSymlinkIdentity(root *os.Root, name string, identity *os.File) (string, error) {
	// freadlink is unavailable before macOS 13. Keep the leaf handle pinned,
	// read through the anchored parent, and reject any identity change visible
	// before or after readlinkat.
	if err := validateOpenedSymlinkIdentity(root, name, identity); err != nil {
		return "", fmt.Errorf("read pinned symlink: %w", err)
	}
	target, err := root.Readlink(name)
	if err != nil {
		return "", fmt.Errorf("read pinned symlink: %w", err)
	}
	if err := validateOpenedSymlinkIdentity(root, name, identity); err != nil {
		return "", fmt.Errorf("read pinned symlink: %w", err)
	}
	return target, nil
}

func validateSymlinkTransactionIdentity(_ os.FileInfo) error {
	return nil
}
