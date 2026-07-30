//go:build !windows

package cmd

import (
	"os/signal"
	"syscall"
)

func ignoreBrokenPipeSignal() {
	signal.Ignore(syscall.SIGPIPE)
}
