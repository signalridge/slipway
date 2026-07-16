package fsutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReadFileNoSymlink reads at most maxBytes from a regular file after rejecting direct symlink paths.
func ReadFileNoSymlink(path string, maxBytes int64) ([]byte, error) {
	if maxBytes < 0 {
		return nil, fmt.Errorf("read limit must be non-negative")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Dir(absolute))
	if err != nil {
		return nil, err
	}
	defer root.Close()

	name := filepath.Base(absolute)
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refuse symlink file path %q", path)
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("refuse non-regular file path %q", path)
	}
	if before.Size() > maxBytes {
		return nil, fmt.Errorf("file %q exceeds %d-byte read limit", path, maxBytes)
	}

	file, err := openFileNoFollow(root, name)
	if err != nil {
		return nil, fmt.Errorf("open no-follow file %q: %w", name, err)
	}
	defer file.Close()

	opened, statErr := file.Stat()
	current, lstatErr := root.Lstat(name)
	if statErr != nil {
		return nil, statErr
	}
	if lstatErr != nil {
		return nil, lstatErr
	}
	if !opened.Mode().IsRegular() || current.Mode()&os.ModeSymlink != 0 || !current.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, current) {
		return nil, fmt.Errorf("file path %q changed or is not the opened regular file", path)
	}

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file %q exceeds %d-byte read limit", path, maxBytes)
	}
	openedAfter, statErr := file.Stat()
	currentAfter, lstatErr := root.Lstat(name)
	if statErr != nil {
		return nil, statErr
	}
	if lstatErr != nil {
		return nil, lstatErr
	}
	if !os.SameFile(opened, openedAfter) || currentAfter.Mode()&os.ModeSymlink != 0 || !currentAfter.Mode().IsRegular() || !os.SameFile(openedAfter, currentAfter) {
		return nil, fmt.Errorf("file path %q changed while reading", path)
	}
	return data, nil
}
