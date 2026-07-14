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
	var budget int
	var noReview bool
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "run <goal>",
		Short: "Start a user-controlled soft-autopilot run",
		Example: "  slipway run \"<goal>\" --budget 8 --json\n" +
			"  slipway run \"<bounded goal>\" --source-file FILE --budget 8 --json",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			goal := strings.TrimSpace(args[0])
			if goal == "" {
				return newUsageError("goal_required", "goal cannot be empty", defaultErrorNext())
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
			startNext := runStartNext(workspace, goal, budget, noReview, true)
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
				return err
			}
			if jsonOutput {
				return writeProtocolResult(command, run)
			}
			next, err := autopilot.DeriveNext(run)
			if err != nil {
				return fmt.Errorf("derive next protocol operation: %w", err)
			}
			writer := command.OutOrStdout()
			if err := writeHumanRunStart(writer, run); err != nil {
				return err
			}
			return writeHumanNext(writer, next)
		},
	}
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

func makeRunSubmitCmd(root *string) *cobra.Command {
	var runID, actionID, outcomeFile string
	var outcomeStdin bool
	command := &cobra.Command{
		Use:    "submit",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			fileSet := command.Flags().Changed("outcome-file")
			stdinSet := command.Flags().Changed("outcome-stdin") && outcomeStdin
			if runID == "" {
				return newUsageError("run_id_required", "run cannot be empty", defaultErrorNext())
			}
			if actionID == "" {
				return newUsageError("action_id_required", "action cannot be empty", defaultErrorNext())
			}
			if fileSet == stdinSet {
				return newUsageError("outcome_mode_required", "exactly one of outcome-file or outcome-stdin is required", defaultErrorNext())
			}
			if fileSet && strings.TrimSpace(outcomeFile) == "" {
				return newUsageError("outcome_file_required", "outcome-file cannot be empty", defaultErrorNext())
			}

			workspace, err := resolveRoot(*root)
			if err != nil {
				return err
			}
			retryNext := submitRetryNext(workspace, runID, actionID, stdinSet)
			reader, closeReader, err := outcomeReader(command, outcomeFile, stdinSet)
			if err != nil {
				return newUsageError("outcome_unavailable", err.Error(), retryNext)
			}
			if closeReader != nil {
				defer func() { _ = closeReader() }()
			}
			outcome, err := autopilot.DecodeOutcome(reader)
			if err != nil {
				var versionErr *autopilot.VersionError
				if errors.As(err, &versionErr) {
					return newRuntimeError(
						"contract_version_mismatch",
						err.Error(),
						inputlessCommandNext(workspace, "refresh-adapters", "slipway", "install", "--refresh", "--root", workspace),
						nil,
					)
				}
				return newUsageError("invalid_outcome", err.Error(), retryNext)
			}

			service, err := openAutopilotResolved(workspace)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			run, err := service.Submit(runID, actionID, outcome)
			if err != nil {
				return err
			}
			return writeProtocolResult(command, run)
		},
	}
	command.Flags().StringVar(&runID, "run", "", "run id")
	command.Flags().StringVar(&actionID, "action", "", "current action id")
	command.Flags().StringVar(&outcomeFile, "outcome-file", "", "Outcome JSON file")
	command.Flags().BoolVar(&outcomeStdin, "outcome-stdin", false, "read one Outcome JSON value from stdin")
	_ = command.MarkFlagRequired("run")
	_ = command.MarkFlagRequired("action")
	return command
}

func makeRunAnswerCmd(root *string) *cobra.Command {
	var runID, actionID, text, scopeSHA256 string
	var confirmDestructive bool
	command := &cobra.Command{
		Use:    "answer",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if runID == "" {
				return newUsageError("run_id_required", "run cannot be empty", defaultErrorNext())
			}
			if actionID == "" {
				return newUsageError("action_id_required", "action cannot be empty", defaultErrorNext())
			}
			service, err := openAutopilot(*root)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			run, err := service.Answer(runID, actionID, autopilot.AnswerOptions{
				Text:               text,
				ConfirmDestructive: confirmDestructive,
				ScopeSHA256:        scopeSHA256,
			})
			if err != nil {
				return err
			}
			return writeProtocolResult(command, run)
		},
	}
	command.Flags().StringVar(&runID, "run", "", "run id")
	command.Flags().StringVar(&actionID, "action", "", "waiting action id")
	command.Flags().StringVar(&text, "text", "", "user answer, decline, or optional confirmation note")
	command.Flags().BoolVar(&confirmDestructive, "confirm-destructive", false, "attest current user confirmation of the exact destructive scope")
	command.Flags().StringVar(&scopeSHA256, "scope-sha256", "", "exact current destructive scope digest")
	_ = command.MarkFlagRequired("run")
	_ = command.MarkFlagRequired("action")
	return command
}

func makeRunSkipCmd(root *string) *cobra.Command {
	var runID, actionID string
	command := &cobra.Command{
		Use:    "skip",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if runID == "" {
				return newUsageError("run_id_required", "run cannot be empty", defaultErrorNext())
			}
			if actionID == "" {
				return newUsageError("action_id_required", "action cannot be empty", defaultErrorNext())
			}
			service, err := openAutopilot(*root)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			run, err := service.Skip(runID, actionID)
			if err != nil {
				return err
			}
			return writeProtocolResult(command, run)
		},
	}
	command.Flags().StringVar(&runID, "run", "", "run id")
	command.Flags().StringVar(&actionID, "action", "", "current action id")
	_ = command.MarkFlagRequired("run")
	_ = command.MarkFlagRequired("action")
	return command
}

func makeRunResumeCmd(root *string) *cobra.Command {
	var budget int
	var sourceFile string
	var usePinnedSource bool
	var sourceChoice string
	var candidateID string
	command := &cobra.Command{
		Use:    "resume <run-id>",
		Hidden: true,
		Example: "  slipway _machine resume RUN [--budget N]\n" +
			"  slipway _machine resume RUN --source-file FILE [--budget N]\n" +
			"  slipway _machine resume RUN --use-pinned-source [--budget N]\n" +
			"  slipway _machine resume RUN --source-choice pinned|adopt --candidate CANDIDATE [--budget N]",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			budgetSet := command.Flags().Changed("budget")
			sourceFileSet := command.Flags().Changed("source-file")
			choiceSet := command.Flags().Changed("source-choice")
			candidateSet := command.Flags().Changed("candidate")
			if args[0] == "" {
				return newUsageError("run_id_required", "run cannot be empty", defaultErrorNext())
			}
			if budgetSet {
				if err := autopilot.ValidateBudget(budget); err != nil {
					return newUsageError("invalid_budget", err.Error(), defaultErrorNext())
				}
			}
			var replacementBudget *int
			if budgetSet {
				replacement := budget
				replacementBudget = &replacement
			}
			if sourceFileSet && sourceFile == "" {
				return newUsageError("source_file_required", "source-file cannot be empty", defaultErrorNext())
			}
			if choiceSet != candidateSet || (candidateSet && candidateID == "") {
				return newUsageError("source_choice_requires_candidate", "source-choice and candidate must be provided together", defaultErrorNext())
			}
			if choiceSet && sourceChoice != string(autopilot.SourceChoicePinned) && sourceChoice != string(autopilot.SourceChoiceAdopt) {
				return newUsageError("invalid_source_choice", "source-choice must be pinned or adopt", defaultErrorNext())
			}
			modeCount := 0
			if sourceFileSet {
				modeCount++
			}
			if usePinnedSource {
				modeCount++
			}
			if choiceSet {
				modeCount++
			}
			if modeCount > 1 {
				return newUsageError("source_mode_conflict", "source-file, use-pinned-source, and source-choice are mutually exclusive", defaultErrorNext())
			}

			workspace, err := resolveRoot(*root)
			if err != nil {
				return err
			}
			var refreshedSource *autopilot.SourceCandidateInput
			if sourceFileSet {
				imported, err := autopilot.ImportSourceCandidateFile(sourceFile)
				if err != nil {
					return newUsageError("invalid_source_candidate", sourceImportErrorMessage(err), resumeSourceNext(workspace, args[0], replacementBudget))
				}
				refreshedSource = &imported
			}

			service, err := openAutopilotResolved(workspace)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			run, err := service.Resume(args[0], autopilot.ResumeOptions{
				Budget:          replacementBudget,
				RefreshedSource: refreshedSource,
				UsePinnedSource: usePinnedSource,
				SourceChoice:    autopilot.SourceChoice(sourceChoice),
				CandidateID:     candidateID,
			})
			if err != nil {
				return err
			}
			return writeProtocolResult(command, run)
		},
	}
	command.Flags().IntVar(&budget, "budget", 0, "replace remaining Action budget (default: preserve or replenish)")
	command.Flags().StringVar(&sourceFile, "source-file", "", "refreshed raw GitHub Change source envelope")
	command.Flags().BoolVar(&usePinnedSource, "use-pinned-source", false, "continue explicitly with the pinned source snapshot")
	command.Flags().StringVar(&sourceChoice, "source-choice", "", "resolve current source candidate: pinned or adopt")
	command.Flags().StringVar(&candidateID, "candidate", "", "current source candidate ID")
	return command
}

func openAutopilot(root string) (*autopilot.Service, error) {
	resolved, err := resolveRoot(root)
	if err != nil {
		return nil, err
	}
	return openAutopilotResolved(resolved)
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
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("outcome file must be a regular non-symlink file")
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
		return nil, nil, fmt.Errorf("outcome file changed while opening")
	}
	return file, file.Close, nil
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

func runStartNext(workspace, goal string, budget int, noReview, sourceRequired bool) autopilot.Next {
	base := []string{"slipway", "run", "--budget", fmt.Sprint(budget), "--json", "--root", workspace}
	if noReview {
		base = append(base, "--no-review")
	}
	base = append(base, "--", goal)
	inputs := []autopilot.NextInput{}
	variantID := "retry-run"
	if sourceRequired {
		variantID = "start-with-source"
		inputs = []autopilot.NextInput{{Name: "source_file", Type: autopilot.NextInputPath, Flag: "--source-file", Required: true}}
	}
	return commandNext(workspace, autopilot.NextOperationStart, variantID, base, inputs)
}

func submitRetryNext(workspace, runID, actionID string, stdin bool) autopilot.Next {
	base := []string{"slipway", "_machine", "submit", "--run", runID, "--action", actionID, "--root", workspace}
	if stdin {
		return commandNext(
			workspace,
			autopilot.NextOperationAction,
			"submit-outcome-stdin",
			append(base, "--outcome-stdin"),
			[]autopilot.NextInput{},
		)
	}
	return commandNext(
		workspace,
		autopilot.NextOperationAction,
		"submit-outcome-file",
		base,
		[]autopilot.NextInput{{Name: "outcome_file", Type: autopilot.NextInputPath, Flag: "--outcome-file", Required: true}},
	)
}

func resumeSourceNext(workspace, runID string, budget *int) autopilot.Next {
	base := []string{"slipway", "_machine", "resume", runID, "--root", workspace}
	if budget != nil {
		base = append(base, "--budget", fmt.Sprint(*budget))
	}
	return commandNext(
		workspace,
		autopilot.NextOperationResume,
		"refresh-source",
		base,
		[]autopilot.NextInput{{Name: "source_file", Type: autopilot.NextInputPath, Flag: "--source-file", Required: true}},
	)
}
