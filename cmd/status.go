package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/spf13/cobra"
)

type runStatusOutput struct {
	autopilot.Run
	Next autopilot.Next `json:"next"`
}

func (output runStatusOutput) MarshalJSON() ([]byte, error) {
	if !output.WorkspaceForeign {
		type localRunStatusOutput runStatusOutput
		return json.Marshal(localRunStatusOutput(output))
	}
	return json.Marshal(map[string]any{
		"contract_version":   output.ContractVersion,
		"id":                 output.ID,
		"goal":               output.Goal,
		"workspace":          output.Workspace,
		"workspace_identity": output.WorkspaceIdentity,
		"workspace_foreign":  true,
		"state":              output.State,
		"created_at":         output.CreatedAt,
		"next":               output.Next,
	})
}

type statusListOutput struct {
	ContractVersion int                        `json:"contract_version"`
	Runs            []runStatusOutput          `json:"runs"`
	UnavailableRuns []autopilot.UnavailableRun `json:"unavailable_runs"`
}

func makeStatusCmd() *cobra.Command {
	var root string
	var jsonOutput bool
	var section string
	command := &cobra.Command{
		Use:   "status [run-id] [--section KEY]",
		Short: "Show soft-autopilot run journals",
		Example: "  slipway status\n" +
			"  slipway status RUN --json\n" +
			"  slipway status RUN --section requirements",
		Args: usageMaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if len(args) == 1 {
				if validationErr := validateRunIDArgument(root, args[0]); validationErr != nil {
					return validationErr
				}
			}
			if command.Flags().Changed("section") {
				if len(args) != 1 {
					return newUsageError(
						"run_id_required",
						"reading a pinned section requires one run id",
						statusInspectionNextForRawRoot(root, ""),
					)
				}
				if section == "" {
					return newUsageError(
						"material_section_required",
						"section cannot be empty",
						statusInspectionNextForRawRoot(root, args[0]),
					)
				}
			}
			service, err := openAutopilotReadOnly(root)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			if section != "" {
				material, err := service.ReadPinnedMaterial(args[0], section)
				if err != nil {
					return targetedStatusLoadError(service.RepositoryRoot(), args[0], err)
				}
				if jsonOutput {
					return writeJSON(command.OutOrStdout(), material)
				}
				return writeHumanPinnedMaterial(command.OutOrStdout(), material)
			}
			if len(args) == 1 {
				run, err := service.Load(args[0])
				if err != nil {
					return targetedStatusLoadError(service.RepositoryRoot(), args[0], err)
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
			runs, unavailable, err := service.ListRecovery()
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
					UnavailableRuns: unavailable,
				})
			}
			if len(runs) == 0 && len(unavailable) == 0 {
				_, err := fmt.Fprintln(command.OutOrStdout(), "No run journals were found.")
				return err
			}
			for _, run := range runs {
				if run.WorkspaceForeign {
					if _, err := fmt.Fprintf(command.OutOrStdout(), "%s  %-7s  foreign=true  workspace=%s  %s\n", run.ID, run.State, run.Workspace, run.Goal); err != nil {
						return err
					}
					continue
				}
				if _, err := fmt.Fprintf(command.OutOrStdout(), "%s  %-7s  remaining=%d  %s\n", run.ID, run.State, run.RemainingBudget, run.Goal); err != nil {
					return err
				}
			}
			for _, entry := range unavailable {
				if _, err := fmt.Fprintf(command.OutOrStdout(), "%s  unavailable  code=%s  %s\n", entry.ID, entry.Code, entry.Detail); err != nil {
					return err
				}
			}
			return nil
		},
	}
	command.Flags().StringVar(&root, "root", "", "workspace root (default: current Git worktree)")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit JSON")
	command.Flags().StringVar(&section, "section", "", "read one pinned source chapter of the given Run")
	return command
}

// writeHumanPinnedMaterial prints the chapter and says plainly what it is. A
// reader has to be able to tell an inspection of pinned text from an Action
// that authorizes work.
func writeHumanPinnedMaterial(writer io.Writer, material autopilot.PinnedMaterial) error {
	_, err := fmt.Fprintf(
		writer,
		"Run %s (%s) pinned chapter %q as %s.\nSection revision: %s\nRequirements revision: %s\nThis is the pinned text only; it authorizes no work.\n\n# %s\n\n%s",
		material.RunID,
		material.RunState,
		material.Section.Key,
		material.Section.Role,
		material.Section.SectionRevision,
		material.RequirementsRevision,
		material.Section.Title,
		material.Section.Markdown,
	)
	if err != nil {
		return err
	}
	if !strings.HasSuffix(material.Section.Markdown, "\n") {
		_, err = fmt.Fprintln(writer)
	}
	return err
}

func targetedStatusLoadError(repositoryRoot, runID string, err error) error {
	var protocolErr *autopilot.ProtocolError
	if errors.As(err, &protocolErr) {
		if protocolErr.Code == "invalid_run_id" {
			return newUsageError(protocolErr.Code, protocolErr.Message, protocolErr.Next)
		}
		return err
	}
	if errors.Is(err, autopilot.ErrRunBusy) {
		return newRuntimeError(
			"run_busy",
			err.Error(),
			statusInspectionNext(repositoryRoot, runID),
			nil,
		)
	}
	return newRuntimeError(
		"run_journal_invalid",
		err.Error(),
		inputlessCommandNext(repositoryRoot, "list-runs", "slipway", "status", "--root", repositoryRoot),
		nil,
	)
}

func projectStatusRun(run autopilot.Run) autopilot.Run {
	if run.State == autopilot.RunStopped || run.State == autopilot.RunEnded {
		run.CurrentAction = nil
	}
	return run
}

func makeRunStatusOutput(run autopilot.Run) (runStatusOutput, error) {
	run = projectStatusRun(run)
	if run.WorkspaceForeign {
		// Journal replay already rejects a Run whose pinned identity is
		// unusable, so the owning worktree is addressed from that identity
		// rather than from whatever currently sits at its path.
		next, err := autopilot.NewPinnedWorkspaceCommandNext(
			run.WorkspaceIdentity,
			"inspect-run-in-its-workspace",
			[]string{"slipway", "status", run.ID, "--root", run.WorkspaceIdentity.WorktreeRoot},
			[]autopilot.NextInput{},
		)
		if err != nil {
			return runStatusOutput{}, fmt.Errorf("derive foreign run next: %w", err)
		}
		return runStatusOutput{Run: run, Next: next}, nil
	}
	next, err := autopilot.DeriveNext(run)
	if err != nil {
		return runStatusOutput{}, fmt.Errorf("derive run next: %w", err)
	}
	return runStatusOutput{Run: run, Next: next}, nil
}

const maxPendingQuestionRunes = 200

func pendingQuestionText(run autopilot.Run) string {
	if run.CurrentAction == nil {
		return ""
	}
	for index := len(run.Actions) - 1; index >= 0; index-- {
		record := run.Actions[index]
		if record.Action.ActionID != run.CurrentAction.ActionID || record.Outcome == nil || record.Outcome.Pause == nil {
			continue
		}
		question := strings.Join(strings.Fields(record.Outcome.Pause.Question), " ")
		if runes := []rune(question); len(runes) > maxPendingQuestionRunes {
			question = string(runes[:maxPendingQuestionRunes-1]) + "…"
		}
		return question
	}
	return ""
}

func writeRunStatus(command *cobra.Command, run autopilot.Run) error {
	run = projectStatusRun(run)
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
	if question := pendingQuestionText(run); question != "" {
		if _, err := fmt.Fprintf(writer, "Pending question: %s\n", question); err != nil {
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
