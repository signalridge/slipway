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
		Description: "new -> intake -> plan -> implement -> review -> done; run is a shortcut driver.",
		Commands: []groupedCommand{
			{Name: "new", Description: desc("new")},
			{Name: "intake", Description: desc("intake")},
			{Name: "plan", Description: desc("plan")},
			{Name: "implement", Description: desc("implement")},
			{Name: "review", Description: desc("review")},
			{Name: "fix", Description: desc("fix")},
			{Name: "done", Description: desc("done")},
			{Name: "next", Description: desc("next")},
			{Name: "run", Description: desc("run")},
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
			{Name: "preset", Description: desc("preset")},
			{Name: "validate", Description: desc("validate")},
			{Name: "abort", Description: desc("abort")},
			{Name: "cancel", Description: desc("cancel")},
			{Name: "delete", Description: desc("delete")},
			{Name: "repair", Description: desc("repair")},
			{Name: "checkpoint", Description: desc("checkpoint")},
			{Name: "evidence", Description: desc("evidence")},
		},
	},
	{
		Title:       "Helpers",
		Description: "Helper tools used by generated skills; explicit backends and domain tools fail closed when unavailable.",
		Commands: []groupedCommand{
			{Name: "tool", Description: desc("tool")},
		},
	},
	{
		Title:       "Diagnostics",
		Description: "Repo-local observability and integrity checks.",
		Commands: []groupedCommand{
			{Name: "learn", Description: desc("learn")},
			{Name: "stats", Description: desc("stats")},
			{Name: "health", Description: desc("health")},
			{Name: "instructions", Description: desc("instructions")},
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

var rootCmd = newRootCmd()

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Execute runs the root command.
func Execute() error {
	return executeRootCommand(rootCmd)
}

func executeRootCommand(cmd *cobra.Command) error {
	// Capture Cobra's built-in helpFunc before overriding, so subcommands can
	// use it directly without walking up the parent chain (which would recurse).
	defaultHelpFunc := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if c == cmd {
			if err := writeRootHelp(c.OutOrStdout()); err != nil {
				c.PrintErrln(err)
			}
			return
		}
		defaultHelpFunc(c, args)
	})

	err := cmd.Execute()
	if err == nil {
		return nil
	}
	cliErr := asCLIError(err)
	if emitErr := emitCLIError(cmd.ErrOrStderr(), cliErr); emitErr != nil {
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

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "slipway",
		Short:         "Slipway change-governance workflow CLI",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetVersionTemplate(fmt.Sprintf("slipway %s\n  commit: %s\n  built:  %s\n", version, commit, date))
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	cmd.AddCommand(makeInitCmd())
	cmd.AddCommand(makeCodebaseMapCmd())
	cmd.AddCommand(makeNewCmd())
	cmd.AddCommand(makeIntakeCmd())
	cmd.AddCommand(makePlanCmd())
	cmd.AddCommand(makeImplementCmd())
	cmd.AddCommand(makePresetCmd())
	cmd.AddCommand(makeNextCmd())
	cmd.AddCommand(makeRunCmd())
	cmd.AddCommand(makeStatusCmd())
	cmd.AddCommand(makeLearnCmd())
	cmd.AddCommand(makeStatsCmd())
	cmd.AddCommand(makeHealthCmd())
	cmd.AddCommand(makeInstructionsCmd())
	cmd.AddCommand(makeRootPathCmd())
	cmd.AddCommand(makeDoneCmd())
	cmd.AddCommand(makeAbortCmd())
	cmd.AddCommand(makeCancelCmd())
	cmd.AddCommand(makeDeleteCmd())
	cmd.AddCommand(makeReviewCmd())
	cmd.AddCommand(makeFixCmd())
	cmd.AddCommand(makeValidateCmd())
	cmd.AddCommand(makeRepairCmd())
	cmd.AddCommand(makeCheckpointCmd())
	cmd.AddCommand(makeEvidenceCmd())
	cmd.AddCommand(makeHookCmd())
	cmd.AddCommand(makeToolCmd())
	cmd.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Show help for a command",
		RunE: func(helpCmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return writeRootHelp(helpCmd.OutOrStdout())
			}

			target, _, err := cmd.Find(args)
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
	return cmd
}
