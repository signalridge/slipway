//go:build !windows

package cmd

import "github.com/signalridge/slipway/internal/recoverycmd"

func recoveryCommand(arguments ...string) string {
	return recoverycmd.Command(arguments...)
}
