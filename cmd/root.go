package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/spf13/cobra"
)

// desc returns the registry description for a command, ensuring the CLI help
// and the adapter surface stay consistent from a single source of truth.
func desc(id string) string { return toolgen.CommandDescription(id) }

type groupedCommand struct {
	Name        string
	Description string
}

type commandGroup struct {
	Title       string
	Description string
	Commands    []groupedCommand
}

var helpGroups = []commandGroup{
	{
		Title:       "Core lifecycle",
		Description: "new -> [next -> (AI executes skill)]* -> done",
		Commands: []groupedCommand{
			{Name: "new", Description: desc("new")},
			{Name: "preset", Description: desc("preset")},
			{Name: "next", Description: desc("next")},
			{Name: "done", Description: desc("done")},
			{Name: "status", Description: desc("status")},
		},
	},
	{
		Title:       "Discovery",
		Description: "Codebase discovery and mapping.",
		Commands: []groupedCommand{
			{Name: "codebase-map", Description: desc("codebase-map")},
		},
	},
	{
		Title:       "Situational",
		Description: "Commands used when workflow decisions are needed.",
		Commands: []groupedCommand{
			{Name: "review", Description: desc("review")},
			{Name: "validate", Description: desc("validate")},
			{Name: "sync", Description: desc("sync")},
			{Name: "pivot", Description: desc("pivot")},
			{Name: "cancel", Description: desc("cancel")},
			{Name: "repair", Description: desc("repair")},
			{Name: "checkpoint", Description: desc("checkpoint")},
		},
	},
	{
		Title:       "Diagnostics",
		Description: "Repo-local observability and integrity checks.",
		Commands: []groupedCommand{
			{Name: "stats", Description: desc("stats")},
			{Name: "health", Description: desc("health")},
		},
	},
	{
		Title:       "Setup",
		Description: "Workspace and environment setup commands.",
		Commands: []groupedCommand{
			{Name: "init", Description: desc("init")},
		},
	},
}

var rootCmd = &cobra.Command{
	Use:           "slipway",
	Short:         "Slipway change-governance workflow CLI",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	// Capture Cobra's built-in helpFunc before overriding, so subcommands can
	// use it directly without walking up the parent chain (which would recurse).
	defaultHelpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if c == rootCmd {
			if err := writeRootHelp(c.OutOrStdout()); err != nil {
				c.PrintErrln(err)
			}
			return
		}
		defaultHelpFunc(c, args)
	})

	err := rootCmd.Execute()
	if err == nil {
		return nil
	}
	cliErr := asCLIError(err)
	if emitErr := emitCLIError(rootCmd.ErrOrStderr(), cliErr); emitErr != nil {
		return emitErr
	}
	return cliErr
}

func writeRootHelp(w io.Writer) error {
	writer := newFormatWriter(w)
	writer.Writef("Slipway governance-first workflow CLI\n")
	writer.Writef("\n")
	writer.Writef("Usage:\n")
	writer.Writef("  slipway [command]\n")
	writer.Writef("\n")
	writer.Writef("Command Groups:\n")

	for _, group := range helpGroups {
		writer.Writef("  %s\n", group.Title)
		writer.Writef("    %s\n", group.Description)
		for _, cmd := range group.Commands {
			writer.Writef("    %-12s %s\n", cmd.Name, cmd.Description)
		}
		writer.Writef("\n")
	}

	writer.Writef("Use \"slipway [command] --help\" for details on a specific command.\n")
	return writer.Err()
}

func init() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.AddCommand(makeInitCmd())
	rootCmd.AddCommand(makeCodebaseMapCmd())
	rootCmd.AddCommand(makeNewCmd())
	rootCmd.AddCommand(makePresetCmd())
	rootCmd.AddCommand(makeNextCmd())
	rootCmd.AddCommand(makeStatusCmd())
	rootCmd.AddCommand(makeStatsCmd())
	rootCmd.AddCommand(makeHealthCmd())
	rootCmd.AddCommand(makeRootPathCmd())
	rootCmd.AddCommand(makeDoneCmd())
	rootCmd.AddCommand(makeCancelCmd())
	rootCmd.AddCommand(makeReviewCmd())
	rootCmd.AddCommand(makeValidateCmd())
	rootCmd.AddCommand(makeSyncCmd())
	rootCmd.AddCommand(makePivotCmd())
	rootCmd.AddCommand(makeRepairCmd())
	rootCmd.AddCommand(makeCheckpointCmd())
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Show help for a command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return writeRootHelp(cmd.OutOrStdout())
			}

			target, _, err := rootCmd.Find(args)
			if err != nil {
				return err
			}

			if target == nil || strings.EqualFold(target.Name(), "help") {
				return newInvalidUsageError(
					"unknown_help_topic",
					fmt.Sprintf("unknown help topic %q", strings.Join(args, " ")),
					"Run `slipway help` to see available commands.",
					nil,
				)
			}

			return target.Help()
		},
	})
}
