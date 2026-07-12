package cmd

import (
	"fmt"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/spf13/cobra"
)

type runStatusOutput struct {
	autopilot.Run
	Next autopilot.Next `json:"next"`
}

type statusListOutput struct {
	ContractVersion int               `json:"contract_version"`
	Runs            []runStatusOutput `json:"runs"`
}

func makeStatusCmd() *cobra.Command {
	var root string
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "status [run-id]",
		Short: "Show soft-autopilot run journals",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			service, err := openAutopilot(root)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			if len(args) == 1 {
				run, err := service.Load(args[0])
				if err != nil {
					return newRuntimeError(
						"run_not_found",
						err.Error(),
						inputlessCommandNext(service.RepositoryRoot(), "list-runs", "slipway", "status", "--root", service.RepositoryRoot()),
						nil,
					)
				}
				if jsonOutput {
					output, err := makeRunStatusOutput(run)
					if err != nil {
						return err
					}
					return writeJSON(command.OutOrStdout(), output)
				}
				return writeRunStatus(command, run)
			}
			runs, err := service.List()
			if err != nil {
				return err
			}
			if jsonOutput {
				outputs := make([]runStatusOutput, 0, len(runs))
				for _, run := range runs {
					output, err := makeRunStatusOutput(run)
					if err != nil {
						return err
					}
					outputs = append(outputs, output)
				}
				return writeJSON(command.OutOrStdout(), statusListOutput{
					ContractVersion: machineContractVersion,
					Runs:            outputs,
				})
			}
			if len(runs) == 0 {
				_, err := fmt.Fprintln(command.OutOrStdout(), "No run journals were found.")
				return err
			}
			for _, run := range runs {
				if _, err := fmt.Fprintf(command.OutOrStdout(), "%s  %-7s  remaining=%d  %s\n", run.ID, run.State, run.RemainingBudget, run.Goal); err != nil {
					return err
				}
			}
			return nil
		},
	}
	command.Flags().StringVar(&root, "root", "", "workspace root (default: current Git worktree)")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit JSON")
	return command
}

func makeRunStatusOutput(run autopilot.Run) (runStatusOutput, error) {
	next, err := autopilot.DeriveNext(run)
	if err != nil {
		return runStatusOutput{}, fmt.Errorf("derive run next: %w", err)
	}
	return runStatusOutput{Run: run, Next: next}, nil
}

func writeRunStatus(command *cobra.Command, run autopilot.Run) error {
	writer := command.OutOrStdout()
	if _, err := fmt.Fprintf(writer, "Run: %s\nState: %s\nGoal: %s\nBudget remaining: %d\n", run.ID, run.State, run.Goal, run.RemainingBudget); err != nil {
		return err
	}
	if run.PinnedSource != nil {
		if _, err := fmt.Fprintf(
			writer,
			"Pinned source: %s (%s)\nSource revision: %s\nRequirements revision: %s\n",
			run.PinnedSource.CanonicalURL,
			run.PinnedSource.IssueID,
			run.PinnedSource.SourceRevision,
			run.PinnedSource.RequirementsRevision,
		); err != nil {
			return err
		}
	}
	if run.PauseReason != "" {
		if _, err := fmt.Fprintf(writer, "Pause reason: %s\n", run.PauseReason); err != nil {
			return err
		}
	}
	if run.SourceCandidate != nil {
		if _, err := fmt.Fprintf(
			writer,
			"Current source candidate: %s (%s, %s)\n",
			run.SourceCandidate.CandidateID,
			run.SourceCandidate.Classification,
			run.SourceCandidate.ClassificationCode,
		); err != nil {
			return err
		}
		if run.SourceCandidate.ClassificationError != "" {
			if _, err := fmt.Fprintf(writer, "Candidate classification: %s\n", run.SourceCandidate.ClassificationError); err != nil {
				return err
			}
		}
	}
	if run.CurrentAction != nil {
		if _, err := fmt.Fprintf(writer, "Current action: %s (%s)\n", run.CurrentAction.ActionID, run.CurrentAction.Kind); err != nil {
			return err
		}
	}
	if run.Summary != "" {
		if _, err := fmt.Fprint(writer, run.Summary); err != nil {
			return err
		}
	}
	next, err := autopilot.DeriveNext(run)
	if err != nil {
		return fmt.Errorf("derive run next: %w", err)
	}
	return writeHumanNext(writer, next)
}
