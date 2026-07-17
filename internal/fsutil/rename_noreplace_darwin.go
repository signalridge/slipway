//go:build darwin

package fsutil

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func renameNoReplaceAt(oldDirectory *os.File, oldName string, newDirectory *os.File, newName string) error {
	err := unix.RenameatxNp(int(oldDirectory.Fd()), oldName, int(newDirectory.Fd()), newName, unix.RENAME_EXCL)
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOTSUP) {
		return fmt.Errorf("%w: rename %s to %s", ErrFileTransactionNoReplaceUnsupported, oldName, newName)
	}
	if err != nil {
		return fmt.Errorf("rename %s to %s without replacement: %w", oldName, newName, err)
	}
	return nil
}
