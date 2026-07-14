package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/autopilot"
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
	return executeRootCommand(root, os.Args[1:]...)
}

func executeRootCommand(root *cobra.Command, args ...string) error {
	root.SetArgs(args)
	errorRoot := rootForEarlyError(args)
	if err := root.Execute(); err != nil {
		cliErr := asCLIError(err)
		if errorRoot != "" && cliErr.Next.Operation == autopilot.NextOperationNone {
			cliErr.Next = autopilot.NoneNext(errorRoot)
		}
		if emitErr := writeJSON(root.ErrOrStderr(), cliErr); emitErr != nil {
			return emitErr
		}
		return cliErr
	}
	return nil
}

func rootForEarlyError(args []string) string {
	explicit, found := rawRootArgument(args)
	if !found {
		return ""
	}
	resolved, err := resolveRoot(explicit)
	if err != nil {
		return ""
	}
	return resolved
}

func rawRootArgument(args []string) (string, bool) {
	var explicit string
	var found bool
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			break
		}
		if strings.HasPrefix(argument, "--root=") {
			explicit = strings.TrimPrefix(argument, "--root=")
			found = true
			continue
		}
		if argument == "--root" && index+1 < len(args) {
			explicit = args[index+1]
			found = true
			index++
		}
	}
	return explicit, found
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "slipway",
		Short:         "User-controlled soft autopilot for AI coding",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return newUsageError("invalid_usage", err.Error(), defaultErrorNext())
	})
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
