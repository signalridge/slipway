//go:build darwin || linux

package fsutil

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func openFileNoFollow(root *os.Root, name string) (*os.File, error) {
	directory, err := root.Open(".")
	if err != nil {
		return nil, fmt.Errorf("open no-follow parent: %w", err)
	}
	defer directory.Close()

	// O_NONBLOCK keeps a raced replacement with a FIFO from stalling before
	// callers can validate the opened handle.
	fd, err := unix.Openat(int(directory.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	runtime.KeepAlive(directory)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("invalid file descriptor")
	}
	return file, nil
}
