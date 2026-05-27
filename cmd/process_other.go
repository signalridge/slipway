//go:build !unix

package cmd

import (
	"fmt"
	"time"
)

// isPIDAlive is not supported on non-Unix platforms.
func isPIDAlive(_ int) bool {
	return false
}

// preemptInFlightTasks is not supported on non-Unix platforms once there are
// active task PIDs to signal. With no recorded task PIDs, it is a no-op so
// ordinary cancel/abort flows still work on Windows.
func preemptInFlightTasks(root, slug string, _ time.Duration) ([]int, []int, error) {
	pids, err := loadActiveTaskPIDs(root, slug)
	if err != nil || len(pids) == 0 {
		return nil, nil, err
	}
	return nil, nil, fmt.Errorf("process signaling is not supported on this platform")
}
