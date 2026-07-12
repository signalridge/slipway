//go:build linux

package fsutil

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func renameNoReplaceAt(oldDirectory *os.File, oldName string, newDirectory *os.File, newName string) error {
	err := unix.Renameat2(int(oldDirectory.Fd()), oldName, int(newDirectory.Fd()), newName, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EOPNOTSUPP) {
		return fmt.Errorf("%w: rename %s to %s", ErrFileTransactionNoReplaceUnsupported, oldName, newName)
	}
	if err != nil {
		return fmt.Errorf("rename %s to %s without replacement: %w", oldName, newName, err)
	}
	return nil
}
