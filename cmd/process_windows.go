//go:build windows

package cmd

import (
	"fmt"
	"syscall"
	"time"
)

// stillActive mirrors the Windows STILL_ACTIVE / STATUS_PENDING value returned
// by GetExitCodeProcess for a process that has not yet exited.
const stillActive uint32 = 259

// processQueryLimitedInformation is PROCESS_QUERY_LIMITED_INFORMATION (0x1000).
// The stdlib syscall package exposes PROCESS_QUERY_INFORMATION, but not this
// lower-privilege access mask.
const processQueryLimitedInformation uint32 = 0x1000

// isPIDAlive checks if a process with the given PID is still running.
//
// Edge case: a process whose real exit code is exactly 259 (stillActive) would
// be misreported as alive after it has exited. This is rare and acceptable for
// stale-lock cleanup purposes.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
}

// preemptInFlightTasks is not supported on Windows once there are active task
// PIDs to signal. With no recorded task PIDs, it is a no-op so ordinary
// cancel/abort flows still work on Windows.
func preemptInFlightTasks(root, slug string, _ time.Duration) ([]int, []int, error) {
	pids, err := loadActiveTaskPIDs(root, slug)
	if err != nil || len(pids) == 0 {
		return nil, nil, err
	}
	return nil, nil, fmt.Errorf("process signaling is not supported on this platform")
}
