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

// preemptInFlightTasks is not supported on non-Unix platforms.
func preemptInFlightTasks(_, _ string, _ time.Duration) ([]int, []int, error) {
	return nil, nil, fmt.Errorf("process signaling is not supported on this platform")
}
