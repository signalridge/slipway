//go:build !darwin && !linux && !windows

package fsutil

import (
	"fmt"
	"os"
)

func openFileNoFollow(_ *os.Root, name string) (*os.File, error) {
	return nil, fmt.Errorf("no-follow file open is unsupported on this platform for %q", name)
}
