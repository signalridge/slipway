package cmd

import (
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	commandLockWaitUnit = 10 * time.Millisecond
	commandCancelGraceUnit = 10 * time.Millisecond
	processPreemptionPollInterval = time.Millisecond
	os.Exit(m.Run())
}
