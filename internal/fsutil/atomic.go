package fsutil

import (
	"os"
	"path/filepath"
	"slices"
)

// WriteFileAtomic writes the file using temp-in-dir, fsync, rename, fsync-parent.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
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
	defer dirFile.Close()

	return dirFile.Sync()
}

// CleanupAtomicTempArtifacts removes stale temp files created by WriteFileAtomic.
func CleanupAtomicTempArtifacts(root string) ([]string, error) {
	deleted := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if len(d.Name()) < len(".tmp-") || d.Name()[:len(".tmp-")] != ".tmp-" {
			return nil
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
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
