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
	_ = os.Setenv("SLIPWAY_HOST_CAPABILITIES", "subagent")
	_ = os.Setenv("SLIPWAY_HOST_CAPABILITY_FALLBACKS", "")
	os.Exit(m.Run())
}
