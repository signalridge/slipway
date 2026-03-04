package main

import (
	"errors"
	"os"

	"github.com/signalridge/speclane/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		var cliErr *cmd.CLIError
		if errors.As(err, &cliErr) {
			os.Exit(cliErr.ExitCode)
		}
		os.Exit(1)
	}
}
