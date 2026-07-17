package cmd

import "github.com/signalridge/slipway/internal/recoverycmd"

// recoveryCommand renders argv as a command the user can execute directly.
// This seam carries no build tag on purpose: the per-operating-system rendering
// lives in recoverycmd, so splitting this file again would duplicate one body
// behind opposing tags and let the two copies drift apart unnoticed.
func recoveryCommand(arguments ...string) string {
	return recoverycmd.Command(arguments...)
}
