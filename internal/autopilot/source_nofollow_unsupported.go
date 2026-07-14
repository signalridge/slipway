//go:build !darwin && !linux && !windows

package autopilot

import (
	"fmt"
	"os"
)

func openSourceFileNoFollow(_ *os.Root, name string) (*os.File, error) {
	return nil, fmt.Errorf("no-follow source open is unsupported on this platform for %q", name)
}
