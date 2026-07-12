//go:build !darwin && !linux && !windows

package fsutil

import (
	"fmt"
	"os"
)

func renameNoReplaceAt(_ *os.File, oldName string, _ *os.File, newName string) error {
	return fmt.Errorf("%w on this platform: rename %s to %s", ErrFileTransactionNoReplaceUnsupported, oldName, newName)
}
