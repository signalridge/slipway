package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// WriteFileAtomic writes the file using temp-in-dir, fsync, rename, fsync-parent.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	return writeFileAtomicImpl(path, data, perm)
}

// writeFileAtomicImpl is the shared implementation for atomic writes.
func writeFileAtomicImpl(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
	}()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	closed = true

	if err := os.Rename(tmpName, path); err != nil {
		return err
	}

	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() {
		_ = dirFile.Close()
	}()

	if err := dirFile.Sync(); err != nil {
		if runtime.GOOS == "windows" {
			return nil
		}
		return err
	}
	return nil
}

// CleanupAtomicTempArtifactsOlderThan removes temp files created by
// WriteFileAtomic only after they are old enough to be considered abandoned.
func CleanupAtomicTempArtifactsOlderThan(root string, staleAfter time.Duration, now time.Time) ([]string, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	deleted := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasPrefix(d.Name(), ".tmp-") {
			return nil
		}
		if staleAfter > 0 {
			info, err := d.Info()
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return nil
				}
				return err
			}
			if now.Sub(info.ModTime()) < staleAfter {
				return nil
			}
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		deleted = append(deleted, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(deleted)
	return deleted, nil
}
