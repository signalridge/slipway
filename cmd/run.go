package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/spf13/cobra"
)

type textInputLimitError struct {
	limit int
}

func (err *textInputLimitError) Error() string {
	if err == nil {
		return "text input exceeds its byte limit"
	}
	return fmt.Sprintf("text input uses more than %d bytes", err.limit)
}

// mutationEnvelope is the single versioned shape for every successful run mutation.
// Active runs carry a non-null action plus derived submit/skip next variants;
// other states retain action when a current Action remains and otherwise omit it.
// This keeps the public surface uniform so a host never has to guess what follows.
type mutationEnvelope struct {
	ContractVersion int                        `json:"contract_version"`
	RunID           string                     `json:"run_id"`
	State           autopilot.RunState         `json:"state"`
	PauseReason     autopilot.PauseReason      `json:"pause_reason,omitempty"`
	Summary         string                     `json:"summary,omitempty"`
	Action          *autopilot.Action          `json:"action,omitempty"`
	Next            autopilot.Next             `json:"next"`
	PinnedSource    *autopilot.PinnedSource    `json:"pinned_source,omitempty"`
	SourceCandidate *autopilot.SourceCandidate `json:"source_candidate,omitempty"`
	ResumeOperation string                     `json:"resume_operation,omitempty"`
	BudgetApplied   *bool                      `json:"budget_applied,omitempty"`
}

func makeRunCmd() *cobra.Command {
	var root string
	var sourceFile string
	var goalFile string
	var goalStdin bool
	var budget int
	var noReview bool
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "run [flags] [-- <goal>]",
		Short: "Start a user-controlled soft-autopilot run",
		Example: "  slipway run --budget 8 --json -- \"<goal>\"\n" +
			"  slipway run --goal-file GOAL --source-file SOURCE --budget 8 --json",
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			goalFileSet := command.Flags().Changed("goal-file")
			goalStdinSet := command.Flags().Changed("goal-stdin") && goalStdin
			goalModeCount := len(args)
			if goalFileSet {
				goalModeCount++
			}
			if goalStdinSet {
				goalModeCount++
			}
			if goalModeCount == 0 {
				return newUsageError(
					"goal_required",
					"exactly one positional goal, goal-file, or goal-stdin input is required",
					defaultErrorNext(),
				)
			}
			if goalModeCount > 1 {
				return newUsageError(
					"goal_mode_conflict",
					"positional goal, goal-file, and goal-stdin are mutually exclusive",
					defaultErrorNext(),
				)
			}
			if goalFileSet && strings.TrimSpace(goalFile) == "" {
				return newUsageError("goal_file_required", "goal-file cannot be empty", defaultErrorNext())
			}

			goal := ""
			if len(args) == 1 {
				goal = args[0]
			} else {
				reader, closeReader, readErr := textInputReader(command, goalFile, goalStdinSet, "goal")
				if readErr != nil {
					return newUsageError("goal_unavailable", readErr.Error(), defaultErrorNext())
				}
				if closeReader != nil {
					defer func() { _ = closeReader() }()
				}
				goal, readErr = readBoundedTextInput(reader)
				if readErr != nil {
					code := "invalid_goal"
					var limitErr *textInputLimitError
					if errors.As(readErr, &limitErr) {
						code = "action_too_large"
					}
					return newUsageError(code, readErr.Error(), defaultErrorNext())
				}
			}
			if strings.TrimSpace(goal) == "" {
				return newUsageError("goal_required", "goal cannot be empty", defaultErrorNext())
			}
			if err := autopilot.ValidateGoal(goal); err != nil {
				code := "invalid_goal"
				var limitErr *autopilot.GoalLimitError
				if errors.As(err, &limitErr) {
					code = "action_too_large"
				}
				return newUsageError(code, err.Error(), defaultErrorNext())
			}
			if err := autopilot.ValidateBudget(budget); err != nil {
				return newUsageError("invalid_budget", err.Error(), defaultErrorNext())
			}
			if command.Flags().Changed("source-file") && sourceFile == "" {
				return newUsageError("source_file_required", "source-file cannot be empty", defaultErrorNext())
			}
			workspace, err := resolveRoot(root)
			if err != nil {
				return err
			}
			startNext := runStartNext(workspace, budget, noReview, true)
			var pinnedSource *autopilot.PinnedSource
			if sourceFile != "" {
				imported, err := autopilot.ImportSourceFile(sourceFile)
				if err != nil {
					return newUsageError("invalid_source", sourceImportErrorMessage(err), startNext)
				}
				pinnedSource = &imported
			}
			service, err := openAutopilotResolved(workspace)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			run, err := service.Start(goal, autopilot.CreateOptions{
				Budget:        budget,
				ReviewEnabled: !noReview,
				PinnedSource:  pinnedSource,
			})
			if err != nil {
				return withCLIErrorContext(err, workspace, "")
			}
			if jsonOutput {
				return writeCommittedProtocolResult(command, run)
			}
			ignoreBrokenPipeSignal()
			next, err := autopilot.DeriveNext(run)
			if err != nil {
				return committedRunOutputError(run, fmt.Errorf("derive next protocol operation: %w", err))
			}
			writer := command.OutOrStdout()
			if err := writeHumanRunStart(writer, run); err != nil {
				return committedRunOutputError(run, err)
			}
			if err := writeHumanNext(writer, next); err != nil {
				return committedRunOutputError(run, err)
			}
			return nil
		},
	}
	command.Flags().StringVar(&goalFile, "goal-file", "", "read the exact Run goal from a regular non-symlink file")
	command.Flags().BoolVar(&goalStdin, "goal-stdin", false, "read the exact Run goal from stdin")
	command.Flags().StringVar(&sourceFile, "source-file", "", "raw GitHub Change source envelope")
	command.Flags().IntVar(&budget, "budget", autopilot.DefaultBudget, "maximum number of Actions before pausing")
	command.Flags().BoolVar(&noReview, "no-review", false, "omit the default advisory review")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit machine-protocol JSON")
	command.PersistentFlags().StringVar(&root, "root", "", "workspace root (default: current Git worktree)")
	return command
}

func writeHumanRunStart(writer io.Writer, run autopilot.Run) error {
	currentAction := "none"
	if run.CurrentAction != nil {
		currentAction = fmt.Sprintf("%s (%s)", run.CurrentAction.Kind, run.CurrentAction.ActionID)
	}
	_, err := fmt.Fprintf(
		writer,
		"Run %s started.\nState: %s\nGoal: %s\nBudget remaining: %d\nCurrent action: %s\n",
		run.ID,
		run.State,
		run.Goal,
		run.RemainingBudget,
		currentAction,
	)
	return err
}

func openAutopilot(root string) (*autopilot.Service, error) {
	resolved, err := resolveRoot(root)
	if err != nil {
		return nil, err
	}
	return openAutopilotResolved(resolved)
}

func openAutopilotReadOnly(root string) (*autopilot.Service, error) {
	resolved, err := resolveRoot(root)
	if err != nil {
		return nil, err
	}
	service, err := autopilot.OpenServiceReadOnly(resolved)
	if err != nil {
		return nil, newRuntimeError(
			"runstore_unavailable",
			err.Error(),
			inputlessCommandNext(resolved, "run-doctor", "slipway", "doctor", "--root", resolved),
			nil,
		)
	}
	return service, nil
}

func openAutopilotResolved(resolved string) (*autopilot.Service, error) {
	service, err := autopilot.OpenService(resolved)
	if err != nil {
		return nil, newRuntimeError(
			"runstore_unavailable",
			err.Error(),
			inputlessCommandNext(resolved, "run-doctor", "slipway", "doctor", "--root", resolved),
			nil,
		)
	}
	return service, nil
}

func outcomeReader(command *cobra.Command, path string, stdin bool) (io.Reader, func() error, error) {
	if stdin {
		return command.InOrStdin(), nil, nil
	}
	return regularFileReader(path, "outcome")
}

func textInputReader(command *cobra.Command, path string, stdin bool, label string) (io.Reader, func() error, error) {
	if stdin {
		return command.InOrStdin(), nil, nil
	}
	return regularFileReader(path, label)
}

func regularFileReader(path, label string) (io.Reader, func() error, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%s file must be a regular non-symlink file", label)
	}
	file, err := os.Open(path) // #nosec G304 -- user-selected file is Lstat-checked and its opened identity is verified below.
	if err != nil {
		return nil, nil, err
	}
	opened, statErr := file.Stat()
	current, lstatErr := os.Lstat(path)
	if statErr != nil || lstatErr != nil || current.Mode()&os.ModeSymlink != 0 || !current.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(before, current) {
		_ = file.Close()
		if statErr != nil {
			return nil, nil, statErr
		}
		if lstatErr != nil {
			return nil, nil, lstatErr
		}
		return nil, nil, fmt.Errorf("%s file changed while opening", label)
	}
	return file, file.Close, nil
}

func readBoundedTextInput(reader io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, int64(autopilot.MaxTextInputBytes)+1))
	if err != nil {
		return "", fmt.Errorf("read text input: %w", err)
	}
	if len(data) > autopilot.MaxTextInputBytes {
		return "", &textInputLimitError{limit: autopilot.MaxTextInputBytes}
	}
	return string(data), nil
}

func sourceImportErrorMessage(err error) string {
	if strings.HasPrefix(err.Error(), "read source file:") {
		return "source file could not be read safely"
	}
	return "source file could not be imported: " + err.Error()
}

func writeProtocolResult(command *cobra.Command, run autopilot.Run) error {
	if run.State == autopilot.RunActive && run.CurrentAction == nil {
		return errors.New("active protocol result requires a current action")
	}
	next, err := autopilot.DeriveNext(run)
	if err != nil {
		return fmt.Errorf("derive next protocol operation: %w", err)
	}
	output := mutationEnvelope{
		ContractVersion: autopilot.ContractVersion,
		RunID:           run.ID,
		State:           run.State,
		PauseReason:     run.PauseReason,
		Summary:         run.Summary,
		Action:          run.CurrentAction,
		Next:            next,
		PinnedSource:    run.PinnedSource,
		SourceCandidate: run.SourceCandidate,
	}
	if command.Name() == "resume" && run.LastResumeResult != nil {
		budgetApplied := run.LastResumeResult.BudgetApplied
		output.ResumeOperation = run.LastResumeResult.Operation
		output.BudgetApplied = &budgetApplied
	}
	return writeJSON(command.OutOrStdout(), output)
}

func writeCommittedProtocolResult(command *cobra.Command, run autopilot.Run) error {
	ignoreBrokenPipeSignal()
	if err := writeProtocolResult(command, run); err != nil {
		return committedRunOutputError(run, err)
	}
	return nil
}

func committedRunOutputError(run autopilot.Run, err error) error {
	if err == nil {
		return nil
	}
	return withCLIErrorContext(&committedOutputError{err: err}, run.Workspace, run.ID)
}

func runStartNext(workspace string, budget int, noReview, sourceRequired bool) autopilot.Next {
	base := []string{"slipway", "run", "--budget", fmt.Sprint(budget), "--json", "--root", workspace}
	if noReview {
		base = append(base, "--no-review")
	}
	inputs := []autopilot.NextInput{{
		Name: "goal_file", Type: autopilot.NextInputPath, Flag: "--goal-file", Required: true,
	}}
	variantID := "retry-run"
	if sourceRequired {
		variantID = "start-with-source"
		inputs = append(inputs, autopilot.NextInput{
			Name: "source_file", Type: autopilot.NextInputPath, Flag: "--source-file", Required: true,
		})
	}
	return commandNext(workspace, autopilot.NextOperationStart, variantID, base, inputs)
}
