package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/spf13/cobra"
)

func makeProtocolCmd() *cobra.Command {
	var root string
	command := &cobra.Command{
		Use:   "protocol",
		Short: "Machine protocol operations that generated adapters call to drive a Run",
		// Public on purpose: these operations are the published machine-protocol
		// contract, not an implementation detail, so the CLI must not present
		// them as one. They stay a distinct group because their caller is a
		// generated adapter rather than a person: each needs a Run and Action it
		// can only learn from the Action it was handed, and every response
		// already carries the exact next command to run.
		Args: usageNoArgs,
		RunE: func(*cobra.Command, []string) error {
			return newUsageError("protocol_operation_required", "a protocol operation is required", defaultErrorNext())
		},
	}
	command.PersistentFlags().StringVar(&root, "root", "", "workspace root (default: current Git worktree)")
	command.AddCommand(
		makeRunSubmitCmd(&root),
		makeRunAnswerCmd(&root),
		makeRunSkipCmd(&root),
		makeRunResumeCmd(&root),
		makeProtocolMaterialCmd(&root),
	)
	return command
}

func makeProtocolMaterialCmd(root *string) *cobra.Command {
	var runID string
	var actionID string
	var section string
	command := &cobra.Command{
		Use:   "material",
		Short: "Read one locally pinned Action source chapter",
		Args:  usageNoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if runID == "" {
				return newUsageError("run_id_required", "run cannot be empty", statusInspectionNextForRawRoot(*root, ""))
			}
			if validationErr := validateRunIDArgument(*root, runID); validationErr != nil {
				return validationErr
			}
			recoveryNext := statusInspectionNextForRawRoot(*root, runID)
			if actionID == "" {
				return newUsageError("action_id_required", "action cannot be empty", recoveryNext)
			}
			if section == "" {
				return newUsageError("material_section_required", "section cannot be empty", recoveryNext)
			}
			service, err := openAutopilotReadOnly(*root)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			material, err := service.ReadActionMaterial(runID, actionID, section)
			if err != nil {
				return withCLIErrorContext(err, service.RepositoryRoot(), runID)
			}
			if err := writeJSON(command.OutOrStdout(), material); err != nil {
				return withCLIErrorContext(
					errors.New("write action material: "+err.Error()),
					service.RepositoryRoot(),
					runID,
				)
			}
			return nil
		},
	}
	command.Flags().StringVar(&runID, "run", "", "Run ID")
	command.Flags().StringVar(&actionID, "action", "", "Action ID")
	command.Flags().StringVar(&section, "section", "", "source section key")
	return command
}

func makeRunSubmitCmd(root *string) *cobra.Command {
	var runID, actionID, outcomeFile string
	var outcomeStdin bool
	command := &cobra.Command{
		Use:   "submit",
		Short: "Report one Action Outcome and receive the next Action",
		Args:  usageNoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			fileSet := command.Flags().Changed("outcome-file")
			stdinSet := command.Flags().Changed("outcome-stdin") && outcomeStdin
			if runID == "" {
				return newUsageError("run_id_required", "run cannot be empty", defaultErrorNext())
			}
			if validationErr := validateRunIDArgument(*root, runID); validationErr != nil {
				return validationErr
			}
			recoveryNext := statusInspectionNextForRawRoot(*root, runID)
			if actionID == "" {
				return newUsageError("action_id_required", "action cannot be empty", recoveryNext)
			}
			if fileSet == stdinSet {
				return newUsageError("outcome_mode_required", "exactly one of outcome-file or outcome-stdin is required", recoveryNext)
			}
			if fileSet && strings.TrimSpace(outcomeFile) == "" {
				return newUsageError("outcome_file_required", "outcome-file cannot be empty", recoveryNext)
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
				return withCLIErrorContext(err, workspace, runID)
			}
			return writeCommittedProtocolResult(command, run)
		},
	}
	command.Flags().StringVar(&runID, "run", "", "run id")
	command.Flags().StringVar(&actionID, "action", "", "current action id")
	command.Flags().StringVar(&outcomeFile, "outcome-file", "", "Outcome JSON file")
	command.Flags().BoolVar(&outcomeStdin, "outcome-stdin", false, "read one Outcome JSON value from stdin")
	return command
}

func makeRunAnswerCmd(root *string) *cobra.Command {
	var runID, actionID, text, textFile, scopeSHA256 string
	var textStdin bool
	var confirmDestructive bool
	command := &cobra.Command{
		Use:   "answer",
		Short: "Answer a Clarify Action, or confirm an authorized destructive scope",
		Args:  usageNoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if runID == "" {
				return newUsageError("run_id_required", "run cannot be empty", defaultErrorNext())
			}
			if validationErr := validateRunIDArgument(*root, runID); validationErr != nil {
				return validationErr
			}
			recoveryNext := statusInspectionNextForRawRoot(*root, runID)
			if actionID == "" {
				return newUsageError("action_id_required", "action cannot be empty", recoveryNext)
			}
			textSet := command.Flags().Changed("text")
			textFileSet := command.Flags().Changed("text-file")
			textStdinSet := command.Flags().Changed("text-stdin") && textStdin
			textModeCount := 0
			if textSet {
				textModeCount++
			}
			if textFileSet {
				textModeCount++
			}
			if textStdinSet {
				textModeCount++
			}
			if textModeCount > 1 {
				return newUsageError(
					"answer_mode_conflict",
					"text, text-file, and text-stdin are mutually exclusive",
					recoveryNext,
				)
			}
			if textFileSet && strings.TrimSpace(textFile) == "" {
				return newUsageError("answer_file_required", "text-file cannot be empty", recoveryNext)
			}
			if textFileSet || textStdinSet {
				reader, closeReader, readErr := textInputReader(command, textFile, textStdinSet, "answer")
				if readErr != nil {
					return newUsageError("answer_unavailable", readErr.Error(), recoveryNext)
				}
				if closeReader != nil {
					defer func() { _ = closeReader() }()
				}
				text, readErr = readBoundedTextInput(reader)
				if readErr != nil {
					code := "invalid_answer"
					var limitErr *textInputLimitError
					if errors.As(readErr, &limitErr) {
						code = "answer_too_large"
					}
					return newUsageError(code, readErr.Error(), recoveryNext)
				}
			}
			if err := autopilot.ValidateAnswerText(text); err != nil {
				code := "invalid_answer"
				var limitErr *autopilot.AnswerLimitError
				if errors.As(err, &limitErr) {
					code = "answer_too_large"
				}
				return newUsageError(code, err.Error(), recoveryNext)
			}
			if confirmDestructive != (strings.TrimSpace(scopeSHA256) != "") {
				return newUsageError(
					"destructive_confirmation_pair_required",
					"confirm-destructive and a non-empty scope-sha256 must be provided together",
					recoveryNext,
				)
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
				return withCLIErrorContext(err, service.RepositoryRoot(), runID)
			}
			return writeCommittedProtocolResult(command, run)
		},
	}
	command.Flags().StringVar(&runID, "run", "", "run id")
	command.Flags().StringVar(&actionID, "action", "", "waiting action id")
	command.Flags().StringVar(&text, "text", "", "user answer, decline, or optional confirmation note")
	command.Flags().StringVar(&textFile, "text-file", "", "read the exact answer or optional confirmation note from a regular non-symlink file")
	command.Flags().BoolVar(&textStdin, "text-stdin", false, "read the exact answer or optional confirmation note from stdin")
	command.Flags().BoolVar(&confirmDestructive, "confirm-destructive", false, "attest current user confirmation of the exact destructive scope")
	command.Flags().StringVar(&scopeSHA256, "scope-sha256", "", "exact current destructive scope digest")
	return command
}

func makeRunSkipCmd(root *string) *cobra.Command {
	var runID, actionID string
	command := &cobra.Command{
		Use:   "skip",
		Short: "Skip the current Action and receive the next one",
		Args:  usageNoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if runID == "" {
				return newUsageError("run_id_required", "run cannot be empty", defaultErrorNext())
			}
			if validationErr := validateRunIDArgument(*root, runID); validationErr != nil {
				return validationErr
			}
			recoveryNext := statusInspectionNextForRawRoot(*root, runID)
			if actionID == "" {
				return newUsageError("action_id_required", "action cannot be empty", recoveryNext)
			}
			service, err := openAutopilot(*root)
			if err != nil {
				return err
			}
			defer func() { _ = service.Close() }()
			run, err := service.Skip(runID, actionID)
			if err != nil {
				return withCLIErrorContext(err, service.RepositoryRoot(), runID)
			}
			return writeCommittedProtocolResult(command, run)
		},
	}
	command.Flags().StringVar(&runID, "run", "", "run id")
	command.Flags().StringVar(&actionID, "action", "", "current action id")
	return command
}

func makeRunResumeCmd(root *string) *cobra.Command {
	var budget int
	var sourceFile string
	var usePinnedSource bool
	var sourceChoice string
	var candidateID string
	command := &cobra.Command{
		Use:   "resume <run-id>",
		Short: "Resume a stopped Run or a resumable pause",
		Example: "  slipway protocol resume RUN [--budget N]\n" +
			"  slipway protocol resume RUN --source-file FILE [--budget N]\n" +
			"  slipway protocol resume RUN --use-pinned-source [--budget N]\n" +
			"  slipway protocol resume RUN --source-choice pinned|adopt --candidate CANDIDATE [--budget N]",
		Args: usageExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			budgetSet := command.Flags().Changed("budget")
			sourceFileSet := command.Flags().Changed("source-file")
			choiceSet := command.Flags().Changed("source-choice")
			candidateSet := command.Flags().Changed("candidate")
			if args[0] == "" {
				return newUsageError("run_id_required", "run cannot be empty", defaultErrorNext())
			}
			if validationErr := validateRunIDArgument(*root, args[0]); validationErr != nil {
				return validationErr
			}
			recoveryNext := statusInspectionNextForRawRoot(*root, args[0])
			if budgetSet {
				if err := autopilot.ValidateBudget(budget); err != nil {
					return newUsageError("invalid_budget", err.Error(), recoveryNext)
				}
			}
			var replacementBudget *int
			if budgetSet {
				replacement := budget
				replacementBudget = &replacement
			}
			if sourceFileSet && sourceFile == "" {
				return newUsageError("source_file_required", "source-file cannot be empty", recoveryNext)
			}
			if choiceSet != candidateSet || (candidateSet && candidateID == "") {
				return newUsageError("source_choice_requires_candidate", "source-choice and candidate must be provided together", recoveryNext)
			}
			if choiceSet && sourceChoice != string(autopilot.SourceChoicePinned) && sourceChoice != string(autopilot.SourceChoiceAdopt) {
				return newUsageError("invalid_source_choice", "source-choice must be pinned or adopt", recoveryNext)
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
				return newUsageError("source_mode_conflict", "source-file, use-pinned-source, and source-choice are mutually exclusive", recoveryNext)
			}

			workspace, err := resolveRoot(*root)
			if err != nil {
				return err
			}
			var refreshedSource *autopilot.SourceCandidateInput
			if sourceFileSet {
				imported, importErr := autopilot.ImportSourceCandidateFile(sourceFile)
				if importErr != nil {
					return newUsageError(
						"invalid_source_candidate",
						sourceImportErrorMessage(importErr),
						resumeSourceRecoveryNext(workspace, args[0], replacementBudget),
					)
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
				return withCLIErrorContext(err, workspace, args[0])
			}
			return writeCommittedProtocolResult(command, run)
		},
	}
	command.Flags().IntVar(&budget, "budget", 0, "replace remaining Action budget (default: preserve or replenish)")
	command.Flags().StringVar(&sourceFile, "source-file", "", "refreshed raw GitHub Change source envelope")
	command.Flags().BoolVar(&usePinnedSource, "use-pinned-source", false, "continue explicitly with the pinned source snapshot")
	command.Flags().StringVar(&sourceChoice, "source-choice", "", "resolve current source candidate: pinned or adopt")
	command.Flags().StringVar(&candidateID, "candidate", "", "current source candidate ID")
	return command
}

func submitRetryNext(workspace, runID, actionID string, stdin bool) autopilot.Next {
	base := []string{"slipway", "protocol", "submit", "--run", runID, "--action", actionID, "--root", workspace}
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

// resumeSourceRecoveryNext derives the retry command from the Run itself so an
// ad-hoc Run — which never accepts a refreshed source — is not handed a
// source-refresh variant it can only reject again. The Run is read without
// creating or repairing the run namespace, so a rejected envelope still leaves
// no durable state behind; the static source route remains the fallback for a
// Run that cannot be read at all.
func resumeSourceRecoveryNext(workspace, runID string, budget *int) autopilot.Next {
	service, err := autopilot.OpenServiceReadOnly(workspace)
	if err != nil {
		return resumeSourceNext(workspace, runID, budget)
	}
	defer func() { _ = service.Close() }()
	run, err := service.Load(runID)
	if err != nil {
		return resumeSourceNext(workspace, runID, budget)
	}
	next, err := autopilot.DeriveResumeNext(run)
	if err != nil {
		return resumeSourceNext(workspace, runID, budget)
	}
	return next
}

func resumeSourceNext(workspace, runID string, budget *int) autopilot.Next {
	base := []string{"slipway", "protocol", "resume", runID, "--root", workspace}
	inputs := []autopilot.NextInput{{
		Name: "source_file", Type: autopilot.NextInputPath, Flag: "--source-file", Required: true,
	}}
	if budget != nil {
		base = append(base, "--budget", fmt.Sprint(*budget))
	} else {
		inputs = append(inputs, autopilot.NextInput{
			Name: "budget", Type: autopilot.NextInputString, Flag: "--budget", Required: false,
		})
	}
	return commandNext(
		workspace,
		autopilot.NextOperationResume,
		"refresh-source",
		base,
		inputs,
	)
}
