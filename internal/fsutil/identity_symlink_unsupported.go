//go:build !darwin && !linux && !windows

package fsutil

import (
	"fmt"
	"os"
)

func openSymlinkIdentity(_ *os.Root, name string) (*os.File, error) {
	return nil, fmt.Errorf("%w: pin symbolic-link identity %s", ErrFileTransactionNoReplaceUnsupported, name)
}

func readSymlinkIdentity(_ *os.Root, _ string, _ *os.File) (string, error) {
	return "", fmt.Errorf("%w: read pinned symbolic-link target", ErrFileTransactionNoReplaceUnsupported)
}

func validateSymlinkTransactionIdentity(_ os.FileInfo) error {
	return fmt.Errorf("%w on this platform", ErrFileTransactionSymlinkUnsupported)
}
