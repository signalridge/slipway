//go:build windows

package cmd

import (
	"os"
	"testing"
)

func TestIsPIDAliveNonPositive(t *testing.T) {
	if isPIDAlive(0) {
		t.Error("isPIDAlive(0) = true, want false")
	}
	if isPIDAlive(-1) {
		t.Error("isPIDAlive(-1) = true, want false")
	}
}

func TestIsPIDAliveSelf(t *testing.T) {
	if !isPIDAlive(os.Getpid()) {
		t.Errorf("isPIDAlive(os.Getpid()=%d) = false, want true", os.Getpid())
	}
}

func TestIsPIDAliveDeadPID(t *testing.T) {
	// A PID that is extremely unlikely to be in use. OpenProcess should fail
	// (or GetExitCodeProcess should report a non-stillActive exit code),
	// yielding false.
	const unlikelyPID = 0x7FFFFFFE
	if isPIDAlive(unlikelyPID) {
		t.Errorf("isPIDAlive(%d) = true, want false", unlikelyPID)
	}
}
