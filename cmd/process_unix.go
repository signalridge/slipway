//go:build unix

package cmd

import (
	"os"
	"syscall"
	"time"

	"github.com/signalridge/slipway/internal/writeutil"
)

// isPIDAlive checks if a process with the given PID is still running.
func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

// signalProcess sends a signal to the given PID. Returns nil if the process
// has already exited (ESRCH).
func signalProcess(pid int, sig syscall.Signal) error {
	err := syscall.Kill(pid, sig)
	if err == syscall.ESRCH {
		return nil // process already gone
	}
	return err
}

// preemptInFlightTasks sends SIGINT to active task PIDs, waits for the grace
// period, then SIGKILL any survivors. Returns (all pids, force-killed pids, error).
func preemptInFlightTasks(root, slug string, grace time.Duration) ([]int, []int, error) {
	pids, err := loadActiveTaskPIDs(root, slug)
	if err != nil || len(pids) == 0 {
		return nil, nil, err
	}
	for _, pid := range pids {
		if err := signalProcess(pid, syscall.SIGINT); err != nil {
			writeutil.BestEffortf(os.Stderr, "slipway: WARN: failed to send SIGINT to PID %d: %v\n", pid, err)
		}
	}
	if grace < 0 {
		grace = 0
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if len(filterAlivePIDs(pids)) == 0 {
			_ = clearActiveTaskPIDs(root, slug)
			return pids, nil, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	stillAlive := filterAlivePIDs(pids)
	for _, pid := range stillAlive {
		if err := signalProcess(pid, syscall.SIGKILL); err != nil {
			writeutil.BestEffortf(os.Stderr, "slipway: WARN: failed to send SIGKILL to PID %d: %v\n", pid, err)
		}
	}
	_ = clearActiveTaskPIDs(root, slug)
	return pids, stillAlive, nil
}
