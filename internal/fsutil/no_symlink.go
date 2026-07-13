package fsutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// ReadFileNoSymlink reads a regular file after rejecting direct symlink paths.
func ReadFileNoSymlink(path string) ([]byte, error) {
	if err := requireRegularNonSymlink(path, false); err != nil {
		return nil, err
	}
	return os.ReadFile(path) // #nosec G304 -- direct symlink targets are rejected with Lstat before reading.
}

func requireRegularNonSymlink(path string, allowMissing bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		if allowMissing && errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refuse symlink file path %q", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refuse non-regular file path %q", path)
	}
	return nil
}
