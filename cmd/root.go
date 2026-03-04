package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

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
		Title:       "Daily",
		Description: "Common day-to-day workflow commands.",
		Commands: []groupedCommand{
			{Name: "new", Description: "Create and route a new executable request"},
			{Name: "do", Description: "Execute one next action for the active request"},
			{Name: "done", Description: "Finalize a done-ready request and archive it"},
			{Name: "status", Description: "Show lifecycle status and blockers"},
		},
	},
	{
		Title:       "Situational",
		Description: "Commands used when workflow decisions are needed.",
		Commands: []groupedCommand{
			{Name: "analyze", Description: "Refresh intake analysis for the active request"},
			{Name: "review", Description: "Run review flow for current execution artifacts"},
			{Name: "pivot", Description: "Reroute or rescope an active request"},
			{Name: "cancel", Description: "Cancel an active request and archive terminal state"},
			{Name: "context", Description: "Show compact execution context"},
			{Name: "repair", Description: "Run bounded local integrity repairs"},
		},
	},
	{
		Title:       "Expert",
		Description: "Workspace and environment setup commands.",
		Commands: []groupedCommand{
			{Name: "init", Description: "Initialize .spln runtime directories and defaults"},
		},
	},
}

var rootCmd = &cobra.Command{
	Use:           "spln",
	Short:         "SpecLane governance-first workflow CLI",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() error {
	rootCmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		writeRootHelp(c.OutOrStdout())
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

func writeRootHelp(w io.Writer) {
	fmt.Fprintln(w, "SpecLane governance-first workflow CLI")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  spln [command]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Command Groups:")

	for _, group := range helpGroups {
		fmt.Fprintf(w, "  %s\n", group.Title)
		fmt.Fprintf(w, "    %s\n", group.Description)
		for _, cmd := range group.Commands {
			fmt.Fprintf(w, "    %-12s %s\n", cmd.Name, cmd.Description)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "Use \"spln [command] --help\" for details on a specific command.")
}

func init() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newNewCmd())
	rootCmd.AddCommand(newDoCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newContextCmd())
	rootCmd.AddCommand(newDoneCmd())
	rootCmd.AddCommand(newCancelCmd())
	rootCmd.AddCommand(newPivotCmd())
	rootCmd.AddCommand(newRepairCmd())
	rootCmd.AddCommand(newAnalyzeCmd())
	rootCmd.AddCommand(newReviewCmd())
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:   "help [command]",
		Short: "Show help for a command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				writeRootHelp(cmd.OutOrStdout())
				return nil
			}

			target, _, err := rootCmd.Find(args)
			if err != nil {
				return err
			}

			if target == nil || strings.EqualFold(target.Name(), "help") {
				return fmt.Errorf("unknown help topic %q", strings.Join(args, " "))
			}

			return target.Help()
		},
	})
}
