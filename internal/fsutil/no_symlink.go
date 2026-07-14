package fsutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ReadFileNoSymlink reads a regular file after rejecting direct symlink paths.
func ReadFileNoSymlink(path string) ([]byte, error) {
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
	return io.ReadAll(file)
}
