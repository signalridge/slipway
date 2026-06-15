package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"
)

const (
	// renameRetryAttempts bounds how many times the final rename is retried on
	// Windows when a concurrent reader holds the destination file open.
	renameRetryAttempts = 10
	// renameRetryBaseDelay is the per-attempt backoff; the total budget across
	// all attempts stays well under one second.
	renameRetryBaseDelay = 5 * time.Millisecond

	// errWindowsSharingViolation is ERROR_SHARING_VIOLATION (32): the file is in
	// use by another process and cannot be replaced.
	errWindowsSharingViolation = 32
	// errWindowsAccessDenied is ERROR_ACCESS_DENIED (5): access to the file is
	// denied, which Windows also reports for a held destination during rename.
	errWindowsAccessDenied = 5
)

// WriteFileAtomic writes the file using temp-in-dir, fsync, rename, fsync-parent.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	return writeFileAtomicImpl(path, data, perm)
}

// writeFileAtomicImpl is the shared implementation for atomic writes.
func writeFileAtomicImpl(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil { // #nosec G301 -- directory is a user-facing project or governance artifact location where executable/searchable mode is intentional.
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

	if err := renameWithRetry(tmpName, path); err != nil {
		return err
	}

	dirFile, err := os.Open(dir) // #nosec G304 -- path is resolved from repository or governed artifact authority before this read.
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

// renameWithRetry renames oldPath to newPath. On Windows, a concurrent reader
// holding the destination open can make the rename fail transiently with a
// sharing violation; in that case the rename is retried with a bounded backoff.
// On other platforms it behaves exactly like os.Rename.
func renameWithRetry(oldPath, newPath string) error {
	return renameWithRetryFunc(oldPath, newPath, runtime.GOOS, os.Rename, time.Sleep)
}

func renameWithRetryFunc(
	oldPath, newPath, goos string,
	rename func(string, string) error,
	sleep func(time.Duration),
) error {
	err := rename(oldPath, newPath)
	if err == nil || goos != "windows" {
		return err
	}

	for attempt := 0; attempt < renameRetryAttempts && isWindowsSharingViolation(err); attempt++ {
		sleep(renameRetryBaseDelay)
		err = rename(oldPath, newPath)
		if err == nil {
			return nil
		}
	}
	return err
}

// isWindowsSharingViolation reports whether err is a Windows error indicating
// the destination file is held open by another reader during a rename. The
// numeric errno comparison compiles on every platform, so no build tags are
// needed; on non-Windows platforms these codes simply never occur.
func isWindowsSharingViolation(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == errWindowsSharingViolation || errno == errWindowsAccessDenied
	}
	return false
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
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) { // #nosec G122 -- path is selected by WalkDir under the caller-owned temp root and limited to .tmp-* cleanup.
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
