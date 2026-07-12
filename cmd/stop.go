package cmd

import (
	"fmt"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/spf13/cobra"
)

func makeStopCmd() *cobra.Command {
	var root string
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "stop [run-id]",
		Short: "Stop an autopilot run without deleting its journal",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			service, err := openAutopilot(root)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			runID := ""
			if len(args) == 1 {
				runID = args[0]
			} else {
				runs, err := service.List()
				if err != nil {
					return err
				}
				candidates := make([]autopilot.Run, 0, len(runs))
				for _, run := range runs {
					if run.State == autopilot.RunActive || run.State == autopilot.RunPaused {
						candidates = append(candidates, run)
					}
				}
				if len(candidates) != 1 {
					return newUsageError(
						"run_id_required",
						fmt.Sprintf("stop requires a run id when %d runs can be stopped", len(candidates)),
						inputlessCommandNext(service.RepositoryRoot(), "list-runs", "slipway", "status", "--root", service.RepositoryRoot()),
					)
				}
				runID = candidates[0].ID
			}
			run, err := service.Stop(runID)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeProtocolResult(command, run)
			}
			if _, err := fmt.Fprintf(command.OutOrStdout(), "Stopped run %s.\n", run.ID); err != nil {
				return err
			}
			next, err := autopilot.DeriveNext(run)
			if err != nil {
				return err
			}
			return writeHumanNext(command.OutOrStdout(), next)
		},
	}
	command.Flags().StringVar(&root, "root", "", "workspace root (default: current Git worktree)")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit JSON")
	return command
}
