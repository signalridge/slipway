package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Execute runs Slipway's user-controlled soft-autopilot CLI.
func Execute() error {
	root := newRootCmd()
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	return executeRootCommand(root)
}

func executeRootCommand(root *cobra.Command) error {
	if err := root.Execute(); err != nil {
		cliErr := asCLIError(err)
		if emitErr := writeJSON(root.ErrOrStderr(), cliErr); emitErr != nil {
			return emitErr
		}
		return cliErr
	}
	return nil
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "slipway",
		Short:         "User-controlled soft autopilot for AI coding",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetVersionTemplate(fmt.Sprintf("slipway %s\n  commit: %s\n  built:  %s\n", version, commit, date))
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(
		makeInstallCmd(),
		makeUninstallCmd(),
		makeListCmd(),
		makeDoctorCmd(),
		makeRunCmd(),
		makeStatusCmd(),
		makeStopCmd(),
		makeMachineCmd(),
	)
	root.SetHelpCommand(&cobra.Command{
		Use:    "_help [command]",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}
			target, _, err := root.Find(args)
			if err != nil {
				return newUsageError("unknown_help_topic", err.Error(), defaultErrorNext())
			}
			return target.Help()
		},
	})
	return root
}
