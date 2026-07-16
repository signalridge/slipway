package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachineProtocolStartSubmitSkipStopResume(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "update README", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionOrient, action.Kind)
	assert.Equal(t, autopilot.ContractVersion, action.ContractVersion)

	orient := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts gathered")
	orient.SuggestedActions = []autopilot.SuggestedAction{{Kind: autopilot.ActionImplement, Brief: "Implement the requested update."}}
	outcomePath := writeOutcome(t, orient)
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-file", outcomePath)
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionImplement, action.Kind)

	stdout, stderr, err = executeForTest(t, "_machine", "skip", "--root", repository, "--run", action.RunID, "--action", action.ActionID)
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionSummarize, action.Kind)

	stdout, stderr, err = executeForTest(t, "stop", action.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var state mutationEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &state))
	assert.Equal(t, autopilot.RunStopped, state.State)
	assert.Equal(t, autopilot.NextOperationResume, state.Next.Operation)
	assert.Equal(t, "resume-ad-hoc", state.Next.Variants[0].ID)

	stdout, stderr, err = executeForTest(t, "_machine", "resume", action.RunID, "--root", repository)
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionOrient, action.Kind)
}

func TestRunHumanOutputIsSummaryOnly(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "summarize human output", "--root", repository)
	require.NoError(t, err, stderr)
	assert.Regexp(t, `(?m)^Run .+ started\.$`, stdout)
	assert.Contains(t, stdout, "State: active\n")
	assert.Contains(t, stdout, "Goal: summarize human output\n")
	assert.Regexp(t, `(?m)^Budget remaining: [0-9]+$`, stdout)
	assert.Regexp(t, `(?m)^Current action: orient \(.+\)$`, stdout)
	assert.Contains(t, stdout, "Next choices:\n")
	assert.NotContains(t, stdout, `"contract_version"`)
	assert.NotContains(t, stdout, "{")
}

func TestMachineProtocolReadsOutcomeFromStdinAndReturnsPreciseVersionRecovery(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "inspect", "--root", repository, "--json", "--no-review")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)

	outcome := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts")
	outcome.SuggestedActions = []autopilot.SuggestedAction{{Kind: autopilot.ActionImplement, Brief: "Implement the inspected change."}}
	encoded, err := json.Marshal(outcome)
	require.NoError(t, err)
	stdout, stderr, err = executeForTestWithInput(t, string(encoded), "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-stdin")
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionImplement, action.Kind)

	bad := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "edited")
	bad.ContractVersion = 999
	encoded, err = json.Marshal(bad)
	require.NoError(t, err)
	stdout, stderr, err = executeForTestWithInput(t, string(encoded), "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-stdin")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"contract_version_mismatch"`)
	assert.NotContains(t, stderr, `"next_command"`)
	assert.Contains(t, stderr, `"id":"refresh-adapters"`)
}

func TestStatusListsMultipleRunsAndStopRequiresAnUnambiguousID(t *testing.T) {
	repository := newCLIRepository(t)
	for _, goal := range []string{"first", "second"} {
		_, stderr, err := executeForTest(t, "run", goal, "--root", repository, "--json")
		require.NoError(t, err, stderr)
	}
	stdout, stderr, err := executeForTest(t, "status", "--root", repository)
	require.NoError(t, err, stderr)
	assert.Contains(t, stdout, "first")
	assert.Contains(t, stdout, "second")

	stdout, stderr, err = executeForTest(t, "stop", "--root", repository)
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"run_id_required"`)
	assert.NotContains(t, stderr, `"next_command"`)
	assert.Contains(t, stderr, `"id":"list-runs"`)
}

func TestSubmitRejectsInvalidOutcomeShapeBeforeWritingJournal(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "inspect", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)

	valid, err := json.Marshal(machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts"))
	require.NoError(t, err)
	missingKind := strings.Replace(string(valid), `"action_kind":"orient",`, "", 1)
	stdout, stderr, err = executeForTestWithInput(t, missingKind, "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-stdin")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `required field \"action_kind\" is missing`)

	mismatchedKind := strings.Replace(string(valid), `"action_kind":"orient"`, `"action_kind":"clarify"`, 1)
	stdout, stderr, err = executeForTestWithInput(t, mismatchedKind, "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-stdin")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "does not match current action kind")

	bad := strings.Replace(string(valid), `"review":null`, `"review":null,"approved":true`, 1)
	stdout, stderr, err = executeForTestWithInput(t, bad, "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-stdin")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "unknown field")

	stdout, stderr, err = executeForTest(t, "status", action.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var run runStatusOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &run))
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, action.ActionID, run.CurrentAction.ActionID)
	assert.Nil(t, run.Actions[0].Outcome)
}

func TestSubmitRejectsSymlinkOutcomeFileBeforeWritingJournal(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "inspect", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)

	target := writeOutcome(t, machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts"))
	link := filepath.Join(t.TempDir(), "outcome.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-file", link)
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"outcome_unavailable"`)
	assert.Contains(t, stderr, "outcome file must be a regular non-symlink file")

	stdout, stderr, err = executeForTest(t, "status", action.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var run runStatusOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &run))
	require.NotNil(t, run.CurrentAction)
	assert.Equal(t, action.ActionID, run.CurrentAction.ActionID)
	assert.Nil(t, run.Actions[0].Outcome)
}

func TestRunRejectsExplicitZeroBudget(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "inspect", "--root", repository, "--budget", "0", "--json")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"invalid_budget"`)
	assert.Contains(t, stderr, `"exit_code":2`)
}

func TestRunResumeRejectsExplicitZeroBudget(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "inspect", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)

	stdout, stderr, err = executeForTest(t, "_machine", "resume", action.RunID, "--root", repository, "--budget", "0")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"invalid_budget"`)
	assert.Contains(t, stderr, `"exit_code":2`)
}

func TestMachineUsageValidationPrecedesRunstoreOpen(t *testing.T) {
	tests := []struct {
		name string
		code string
		args []string
	}{
		{name: "run budget", code: "invalid_budget", args: []string{"run", "inspect", "--budget", "1001", "--json"}},
		{name: "run source file", code: "source_file_required", args: []string{"run", "inspect", "--source-file", "", "--json"}},
		{name: "submit mode", code: "outcome_mode_required", args: []string{"_machine", "submit", "--run", "run-1", "--action", "action-1"}},
		{name: "answer run", code: "run_id_required", args: []string{"_machine", "answer", "--run", "", "--action", "action-1"}},
		{name: "skip action", code: "action_id_required", args: []string{"_machine", "skip", "--run", "run-1", "--action", ""}},
		{name: "resume source pair", code: "source_choice_requires_candidate", args: []string{"_machine", "resume", "run-1", "--source-choice", "adopt"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newCLIRepository(t)
			args := append(append([]string(nil), test.args...), "--root", repository)
			stdout, stderr, err := executeForTest(t, args...)
			require.Error(t, err)
			assert.Empty(t, stdout)
			assert.Contains(t, stderr, `"code":"`+test.code+`"`)
			assert.Contains(t, stderr, `"exit_code":2`)
			_, statErr := os.Stat(filepath.Join(repository, ".git", "slipway"))
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

func TestRunStartRecoveryPreservesOptionsAndDashLeadingGoal(t *testing.T) {
	repository := newCLIRepository(t)
	canonicalRepository, err := resolveRoot(repository)
	require.NoError(t, err)
	invalidSource := filepath.Join(t.TempDir(), "invalid-source.json")
	require.NoError(t, os.WriteFile(invalidSource, []byte("{}\n"), 0o600))

	stdout, stderr, err := executeForTest(
		t,
		"run", "--budget", "9", "--no-review", "--source-file", invalidSource,
		"--root", repository, "--json", "--", "-leading goal",
	)
	require.Error(t, err)
	assert.Empty(t, stdout)
	var cliErr CLIError
	require.NoError(t, json.Unmarshal([]byte(stderr), &cliErr))
	assert.Equal(t, "invalid_source", cliErr.Code)
	assert.Equal(t, autopilot.NextOperationStart, cliErr.Next.Operation)
	require.Len(t, cliErr.Next.Variants, 1)
	assert.Equal(t, []string{
		"slipway", "run", "--budget", "9", "--json", "--root", canonicalRepository,
		"--no-review", "--", "-leading goal",
	}, cliErr.Next.Variants[0].BaseArgv)
	resolved, resolveErr := cliErr.Next.Resolve("start-with-source", map[string]autopilot.NextInputValue{
		"source_file": {Type: autopilot.NextInputPath, Value: "/tmp/retry-source.json"},
	})
	require.NoError(t, resolveErr)
	assert.Equal(t, []string{
		"slipway", "run", "--budget", "9", "--json", "--root", canonicalRepository,
		"--no-review", "--source-file", "/tmp/retry-source.json", "--", "-leading goal",
	}, resolved)
	_, statErr := os.Stat(filepath.Join(repository, ".git", "slipway"))
	require.ErrorIs(t, statErr, os.ErrNotExist)

	successRepository := newCLIRepository(t)
	stdout, stderr, err = executeForTest(t, "run", "--root", successRepository, "--json", "--", "-leading goal")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)
	assert.Equal(t, "-leading goal", action.Goal)

	rootFlagGoalRepository := newCLIRepository(t)
	stdout, stderr, err = executeForTest(t, "run", "--root", rootFlagGoalRepository, "--json", "--", "--root")
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	assert.Equal(t, "--root", action.Goal)

	separatorGoalRepository := newCLIRepository(t)
	stdout, stderr, err = executeForTest(t, "run", "--root", separatorGoalRepository, "--json", "--", "--")
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	assert.Equal(t, "--", action.Goal)
}

func TestResumeSourceRecoveryPreservesReplacementBudget(t *testing.T) {
	repository := newCLIRepository(t)
	canonicalRepository, err := resolveRoot(repository)
	require.NoError(t, err)
	invalidSource := filepath.Join(t.TempDir(), "invalid-source.json")
	require.NoError(t, os.WriteFile(invalidSource, []byte("{}\n"), 0o600))

	stdout, stderr, err := executeForTest(
		t,
		"_machine", "resume", "run-1", "--budget", "9", "--source-file", invalidSource,
		"--root", repository,
	)
	require.Error(t, err)
	assert.Empty(t, stdout)
	assertMachineSchemaOutput(t, "cliError", stderr)
	var cliErr CLIError
	require.NoError(t, json.Unmarshal([]byte(stderr), &cliErr))
	assert.Equal(t, "invalid_source_candidate", cliErr.Code)
	assert.Equal(t, exitCodeUsage, cliErr.ExitCode)
	assert.Equal(t, autopilot.NextOperationResume, cliErr.Next.Operation)
	require.Len(t, cliErr.Next.Variants, 1)
	assert.Equal(t, []string{
		"slipway", "_machine", "resume", "run-1", "--root", canonicalRepository, "--budget", "9",
	}, cliErr.Next.Variants[0].BaseArgv)
	resolved, resolveErr := cliErr.Next.Resolve("refresh-source", map[string]autopilot.NextInputValue{
		"source_file": {Type: autopilot.NextInputPath, Value: "/tmp/retry-source.json"},
	})
	require.NoError(t, resolveErr)
	assert.Equal(t, []string{
		"slipway", "_machine", "resume", "run-1", "--root", canonicalRepository, "--budget", "9",
		"--source-file", "/tmp/retry-source.json",
	}, resolved)
	_, statErr := os.Stat(filepath.Join(repository, ".git", "slipway"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestFileAndFlagPreflightFailuresDoNotOpenRunstore(t *testing.T) {
	tests := []struct {
		name    string
		content string
		args    func(repository, path string) []string
		code    string
	}{
		{
			name: "blank goal",
			args: func(repository, _ string) []string {
				return []string{"run", "   ", "--root", repository, "--json"}
			},
			code: "goal_required",
		},
		{
			name:    "invalid start source",
			content: "{}\n",
			args: func(repository, path string) []string {
				return []string{"run", "goal", "--source-file", path, "--root", repository, "--json"}
			},
			code: "invalid_source",
		},
		{
			name:    "invalid resume source candidate",
			content: "{}\n",
			args: func(repository, path string) []string {
				return []string{"_machine", "resume", "run-1", "--source-file", path, "--root", repository}
			},
			code: "invalid_source_candidate",
		},
		{
			name:    "invalid outcome",
			content: `{"contract_version":2}`,
			args: func(repository, path string) []string {
				return []string{"_machine", "submit", "--run", "run-1", "--action", "action-1", "--outcome-file", path, "--root", repository}
			},
			code: "invalid_outcome",
		},
		{
			name: "malformed budget flag",
			args: func(repository, _ string) []string {
				return []string{"run", "goal", "--budget", "not-an-integer", "--root", repository, "--json"}
			},
			code: "invalid_usage",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newCLIRepository(t)
			path := filepath.Join(t.TempDir(), "input.json")
			if test.content != "" {
				require.NoError(t, os.WriteFile(path, []byte(test.content), 0o600))
			}
			stdout, stderr, err := executeForTest(t, test.args(repository, path)...)
			require.Error(t, err)
			assert.Empty(t, stdout)
			assert.Contains(t, stderr, `"code":"`+test.code+`"`)
			assert.Contains(t, stderr, `"exit_code":2`)
			_, statErr := os.Stat(filepath.Join(repository, ".git", "slipway"))
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

func TestEarlyPreflightErrorsUseExplicitRootIdentityWithoutOpeningEitherRunstore(t *testing.T) {
	cwdRepository := newCLIRepository(t)
	t.Chdir(cwdRepository)

	tests := []struct {
		name       string
		args       func(string) []string
		code       string
		inspectRun bool
	}{
		{
			name: "blank goal",
			args: func(repository string) []string { return []string{"run", "   ", "--root", repository, "--json"} },
			code: "goal_required",
		},
		{
			name: "invalid start budget",
			args: func(repository string) []string {
				return []string{"run", "goal", "--budget", "1001", "--root", repository, "--json"}
			},
			code: "invalid_budget",
		},
		{
			name: "invalid resume budget",
			args: func(repository string) []string {
				return []string{"_machine", "resume", "run-1", "--budget", "1001", "--root", repository}
			},
			code:       "invalid_budget",
			inspectRun: true,
		},
		{
			name: "malformed budget before equals root",
			args: func(repository string) []string {
				return []string{"run", "goal", "--budget", "not-an-integer", "--root=" + repository, "--json"}
			},
			code: "invalid_usage",
		},
		{
			name: "invalid resume mode",
			args: func(repository string) []string {
				return []string{"_machine", "resume", "run-1", "--source-choice", "invalid", "--candidate", "candidate-1", "--root", repository}
			},
			code:       "invalid_source_choice",
			inspectRun: true,
		},
		{
			name: "cobra flag error before root",
			args: func(repository string) []string { return []string{"run", "goal", "--unknown", "--root", repository} },
			code: "invalid_usage",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			explicitRepository := newCLIRepository(t)
			canonicalRepository, resolveErr := resolveRoot(explicitRepository)
			require.NoError(t, resolveErr)
			stdout, stderr, err := executeForTest(t, test.args(explicitRepository)...)
			require.Error(t, err)
			assert.Empty(t, stdout)
			var cliErr CLIError
			require.NoError(t, json.Unmarshal([]byte(stderr), &cliErr))
			assert.Equal(t, test.code, cliErr.Code)
			expected := autopilot.NoneNext(canonicalRepository)
			assert.Equal(t, expected.WorkspaceIdentity, cliErr.Next.WorkspaceIdentity)
			if test.inspectRun {
				assert.Equal(t, autopilot.NextOperationCommand, cliErr.Next.Operation)
				require.Len(t, cliErr.Next.Variants, 1)
				assert.Equal(t, []string{"slipway", "status", "run-1", "--root", canonicalRepository}, cliErr.Next.Variants[0].BaseArgv)
			} else {
				assert.Equal(t, autopilot.NextOperationNone, cliErr.Next.Operation)
			}
			for _, repository := range []string{cwdRepository, explicitRepository} {
				_, statErr := os.Stat(filepath.Join(repository, ".git", "slipway"))
				require.ErrorIs(t, statErr, os.ErrNotExist)
			}
		})
	}
}

func TestAnswerDestructiveFlagsMustBePairedBeforeOpeningRunstore(t *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	tests := []struct {
		name string
		args []string
	}{
		{name: "confirmation without scope", args: []string{"--confirm-destructive"}},
		{name: "scope without confirmation", args: []string{"--scope-sha256", digest}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newCLIRepository(t)
			canonicalRepository, resolveErr := resolveRoot(repository)
			require.NoError(t, resolveErr)
			args := []string{"_machine", "answer", "--run", "run-1", "--action", "action-1", "--root", repository}
			args = append(args, test.args...)
			stdout, stderr, err := executeForTest(t, args...)
			require.Error(t, err)
			assert.Empty(t, stdout)
			var cliErr CLIError
			require.NoError(t, json.Unmarshal([]byte(stderr), &cliErr))
			assert.Equal(t, "destructive_confirmation_pair_required", cliErr.Code)
			assert.Equal(t, autopilot.NextOperationCommand, cliErr.Next.Operation)
			require.Len(t, cliErr.Next.Variants, 1)
			assert.Equal(t, []string{"slipway", "status", "run-1", "--root", canonicalRepository}, cliErr.Next.Variants[0].BaseArgv)
			_, statErr := os.Stat(filepath.Join(repository, ".git", "slipway"))
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

func TestMachineProtocolReviewFindingsRouteToSummaryWithoutRepair(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "change", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)
	runID := action.RunID

	orient := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts")
	orient.SuggestedActions = []autopilot.SuggestedAction{{Kind: autopilot.ActionImplement, Brief: "Implement the change."}}
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", runID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, orient))
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)

	require.NoError(t, os.WriteFile(filepath.Join(repository, "change.go"), []byte("package sample\n"), 0o600))
	implementation := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "changed")
	implementation.Implementation = &autopilot.Implementation{
		Result:        autopilot.ImplementationApplied,
		FilesChanged:  []string{"change.go"},
		Activities:    []autopilot.Activity{},
		Uncertainties: []string{},
		Attempts:      1,
	}
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", runID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, implementation))
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionReview, action.Kind)

	review := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "finding reported")
	review.Review = &autopilot.Review{
		Result: autopilot.ReviewFindings,
		Findings: []autopilot.Finding{{
			Location: "change.go:1",
			Summary:  "missing package detail",
			Detail:   "report only; do not repair automatically",
		}},
		Uncertainties: []string{},
	}
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", runID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, review))
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionSummarize, action.Kind)
}

func decodeMutationAction(t *testing.T, payload string) autopilot.Action {
	t.Helper()
	var envelope mutationEnvelope
	require.NoError(t, json.Unmarshal([]byte(payload), &envelope))
	require.Equal(t, autopilot.ContractVersion, envelope.ContractVersion)
	require.NotNil(t, envelope.Action)
	require.Equal(t, envelope.RunID, envelope.Action.RunID)
	return *envelope.Action
}

func machineOutcome(actionID string, actionKind autopilot.ActionKind, status autopilot.OutcomeStatus, summary string) autopilot.Outcome {
	return autopilot.Outcome{
		ContractVersion:  autopilot.ContractVersion,
		ActionID:         actionID,
		ActionKind:       actionKind,
		Status:           status,
		Summary:          summary,
		Observations:     []string{},
		KnownIssues:      []string{},
		SuggestedActions: []autopilot.SuggestedAction{},
	}
}

func executeForTestWithInput(t *testing.T, input string, args ...string) (string, string, error) {
	t.Helper()
	root := newRootCmd()
	var stdout, stderr strings.Builder
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(input))
	root.SetArgs(args)
	err := executeRootCommand(root, args...)
	return stdout.String(), stderr.String(), err
}

func newCLIRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runCLIGit(t, root, "init", "-q")
	runCLIGit(t, root, "config", "user.name", "Slipway Test")
	runCLIGit(t, root, "config", "user.email", "test@example.com")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("initial\n"), 0o600))
	runCLIGit(t, root, "add", ".")
	runCLIGit(t, root, "commit", "-qm", "initial")
	return root
}

func runCLIGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
}

func writeOutcome(t *testing.T, outcome autopilot.Outcome) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "outcome.json")
	encoded, err := json.Marshal(outcome)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, encoded, 0o600))
	return path
}

func TestIssueBoundCLIStartImportsOnceAndExposesSafeStatus(t *testing.T) {
	repository := newCLIRepository(t)
	envelope := cliSourceEnvelope()
	sourcePath := writeCLISource(t, envelope)

	stdout, stderr, err := executeForTest(t, "run", "implement accepted requirements", "--root", repository, "--source-file", sourcePath, "--json")
	require.NoError(t, err, stderr)
	require.NoError(t, os.Remove(sourcePath))
	action := decodeMutationAction(t, stdout)
	require.NotNil(t, action.Source)
	require.NotNil(t, action.Requirements)
	assert.Equal(t, envelope.CanonicalURL, action.Source.CanonicalURL)
	assert.Equal(t, envelope.IssueID, action.Source.IssueID)
	require.Len(t, action.Requirements.Sections, 5)
	assert.Equal(t, "requirements", action.Requirements.Sections[1].Key)
	assert.NotContains(t, stdout, "Keep the exact CLI contract.")

	materialStdout, materialStderr, materialErr := executeForTest(
		t,
		"_machine", "material",
		"--root", repository,
		"--run", action.RunID,
		"--action", action.ActionID,
		"--section", "requirements",
	)
	require.NoError(t, materialErr, materialStderr)
	var material autopilot.ActionMaterial
	require.NoError(t, json.Unmarshal([]byte(materialStdout), &material))
	assert.Equal(t, "action_material", material.MessageType)
	assert.Equal(t, "requirements", material.Section.Key)
	assert.Contains(t, material.Section.Markdown, "Keep the exact CLI contract.")

	stdout, stderr, err = executeForTest(t, "status", action.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var run runStatusOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &run))
	require.NotNil(t, run.PinnedSource)
	assert.Equal(t, envelope.CanonicalURL, run.PinnedSource.CanonicalURL)
	assert.NotContains(t, stdout, sourcePath)
	assert.NotContains(t, stdout, filepath.Base(sourcePath))
	assert.NotContains(t, stdout, "<!-- slipway-level: change/v2 -->")
	assert.NotContains(t, stdout, "Non-normative implementation notes")

	help, helpStderr, err := executeForTest(t, "run", "--help")
	require.NoError(t, err, helpStderr)
	assert.Contains(t, help, "--source-file string")
	assert.Contains(t, help, `slipway run "<bounded goal>" --source-file FILE --budget 8 --json`)

	invalidRepository := newCLIRepository(t)
	invalidEnvelope := cliSourceEnvelope()
	invalidEnvelope.Body = strings.Replace(invalidEnvelope.Body, "<!-- slipway-level: change/v2 -->", "<!-- slipway-level: objective/v1 -->", 1)
	invalidPath := writeCLISource(t, invalidEnvelope)
	stdout, stderr, err = executeForTest(t, "run", "must reject objective", "--root", invalidRepository, "--source-file", invalidPath, "--json")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"invalid_source"`)
	stdout, stderr, err = executeForTest(t, "status", "--root", invalidRepository, "--json")
	require.NoError(t, err, stderr)
	var statusList statusListOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &statusList))
	assert.Equal(t, machineContractVersion, statusList.ContractVersion)
	assert.Empty(t, statusList.Runs)
	assert.NotNil(t, statusList.Runs)

	missingRepository := newCLIRepository(t)
	stdout, stderr, err = executeForTest(t, "run", "missing source", "--root", missingRepository, "--source-file", sourcePath, "--json")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.NotContains(t, stderr, sourcePath)
	assert.NotContains(t, stderr, filepath.Base(sourcePath))
}

func TestIssueBoundCLIResumeCandidateBudgetAndIdempotency(t *testing.T) {
	repository := newCLIRepository(t)
	original := cliSourceEnvelope()
	startPath := writeCLISource(t, original)
	stdout, stderr, err := executeForTest(t, "run", "implement accepted requirements", "--root", repository, "--source-file", startPath, "--budget", "8", "--json")
	require.NoError(t, err, stderr)
	require.NoError(t, os.Remove(startPath))
	initial := decodeMutationAction(t, stdout)

	amended := cliSourceEnvelope()
	setCLISourceSection(&amended, "requirements", "\n# Requirements\n\nKeep the amended CLI contract.\n")
	setCLISourceParentRevision(&amended, initial.Source.RequirementsRevision)
	candidatePath := writeCLISource(t, amended)
	stdout, stderr, err = executeForTest(t, "_machine", "resume", initial.RunID, "--root", repository, "--source-file", candidatePath, "--budget", "20")
	require.NoError(t, err, stderr)
	require.NoError(t, os.Remove(candidatePath))
	var paused mutationEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &paused))
	assert.Equal(t, autopilot.RunPaused, paused.State)
	assert.Equal(t, autopilot.PauseDecisionRequired, paused.PauseReason)
	require.NotNil(t, paused.SourceCandidate)
	assert.True(t, paused.SourceCandidate.Valid)
	require.NotNil(t, paused.BudgetApplied)
	assert.False(t, *paused.BudgetApplied)
	assert.Equal(t, autopilot.ResumeOperationSourceCandidate, paused.ResumeOperation)
	candidateID := paused.SourceCandidate.CandidateID

	human, humanStderr, err := executeForTest(t, "status", initial.RunID, "--root", repository)
	require.NoError(t, err, humanStderr)
	assert.Contains(t, human, "Current source candidate: "+candidateID)
	assert.NotContains(t, human, candidatePath)

	candidateStatusJSON, candidateStatusStderr, err := executeForTest(t, "status", initial.RunID, "--root", repository, "--json")
	require.NoError(t, err, candidateStatusStderr)
	var candidateStatus runStatusOutput
	require.NoError(t, json.Unmarshal([]byte(candidateStatusJSON), &candidateStatus))
	require.Len(t, candidateStatus.Next.Variants, 2)
	assert.Equal(t, "keep-pinned", candidateStatus.Next.Variants[0].ID)
	assert.Equal(t, []string{
		"slipway", "_machine", "resume", initial.RunID, "--root", candidateStatus.Workspace,
		"--source-choice", "pinned", "--candidate", candidateID,
	}, candidateStatus.Next.Variants[0].BaseArgv)
	assert.Empty(t, candidateStatus.Next.Variants[0].Inputs)
	assert.Equal(t, "adopt", candidateStatus.Next.Variants[1].ID)
	assert.Equal(t, []string{
		"slipway", "_machine", "resume", initial.RunID, "--root", candidateStatus.Workspace,
		"--source-choice", "adopt", "--candidate", candidateID,
	}, candidateStatus.Next.Variants[1].BaseArgv)
	assert.Empty(t, candidateStatus.Next.Variants[1].Inputs)

	stdout, stderr, err = executeForTest(t, "_machine", "resume", initial.RunID, "--root", repository, "--source-choice", "adopt", "--candidate", candidateID, "--budget", "5")
	require.NoError(t, err, stderr)
	adopted := decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionOrient, adopted.Kind)
	assert.Equal(t, 4, adopted.RemainingBudget)
	require.NotNil(t, adopted.Source)
	assert.Equal(t, paused.SourceCandidate.RequirementsRevision, adopted.Source.RequirementsRevision)

	retry, retryStderr, err := executeForTest(t, "_machine", "resume", initial.RunID, "--root", repository, "--source-choice", "adopt", "--candidate", candidateID, "--budget", "999")
	require.NoError(t, err, retryStderr)
	assert.JSONEq(t, stdout, retry)

	conflictStdout, conflictStderr, err := executeForTest(t, "_machine", "resume", initial.RunID, "--root", repository, "--source-choice", "pinned", "--candidate", candidateID)
	require.Error(t, err)
	assert.Empty(t, conflictStdout)
	assert.Contains(t, conflictStderr, `"code":"source_choice_conflict"`)

	statusJSON, statusStderr, err := executeForTest(t, "status", initial.RunID, "--root", repository, "--json")
	require.NoError(t, err, statusStderr)
	var run runStatusOutput
	require.NoError(t, json.Unmarshal([]byte(statusJSON), &run))
	assert.Nil(t, run.SourceCandidate)
	require.NotNil(t, run.LastSourceChoice)
	assert.Equal(t, candidateID, run.LastSourceChoice.CandidateID)
	require.NotNil(t, run.LastResumeResult)
	assert.True(t, run.LastResumeResult.BudgetApplied)
	assert.Equal(t, 4, run.RemainingBudget)
}

func TestCLIResumeEnforcesSourceModeCombinations(t *testing.T) {
	resumeHelp, resumeHelpStderr, err := executeForTest(t, "_machine", "resume", "--help")
	require.NoError(t, err, resumeHelpStderr)
	assert.Contains(t, resumeHelp, "--source-file string")
	assert.Contains(t, resumeHelp, "--use-pinned-source")
	assert.Contains(t, resumeHelp, "--source-choice string")
	assert.Contains(t, resumeHelp, "--candidate string")
	assert.Contains(t, resumeHelp, "slipway _machine resume RUN --source-choice pinned|adopt --candidate CANDIDATE [--budget N]")

	issueRepository := newCLIRepository(t)
	sourcePath := writeCLISource(t, cliSourceEnvelope())
	stdout, stderr, err := executeForTest(t, "run", "issue-bound", "--root", issueRepository, "--source-file", sourcePath, "--json")
	require.NoError(t, err, stderr)
	issueAction := decodeMutationAction(t, stdout)

	stdout, stderr, err = executeForTest(t, "_machine", "resume", issueAction.RunID, "--root", issueRepository)
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"source_mode_required"`)

	stdout, stderr, err = executeForTest(t, "_machine", "resume", issueAction.RunID, "--root", issueRepository, "--use-pinned-source")
	require.NoError(t, err, stderr)
	refreshed := decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionOrient, refreshed.Kind)
	require.NotNil(t, refreshed.Source)

	adHocRepository := newCLIRepository(t)
	stdout, stderr, err = executeForTest(t, "run", "ad-hoc", "--root", adHocRepository, "--json")
	require.NoError(t, err, stderr)
	adHocAction := decodeMutationAction(t, stdout)
	stdout, stderr, err = executeForTest(t, "_machine", "resume", adHocAction.RunID, "--root", adHocRepository, "--use-pinned-source")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"source_mode_not_allowed"`)

	stdout, stderr, err = executeForTest(t, "_machine", "resume", issueAction.RunID, "--root", issueRepository, "--source-choice", "adopt")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"source_choice_requires_candidate"`)
	stdout, stderr, err = executeForTest(t, "_machine", "resume", issueAction.RunID, "--root", issueRepository, "--source-file", sourcePath, "--use-pinned-source")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"source_mode_conflict"`)
}

func cliSourceEnvelope() autopilot.RawSourceEnvelope {
	issueURL := "https://github.com/signalridge/slipway/issues/434"
	definitions := []struct {
		key     string
		role    autopilot.SourceSectionRole
		title   string
		payload string
	}{
		{key: "outcome", role: autopilot.SourceSectionOutcome, title: "Outcome", payload: "\n# Outcome\n\nSafe issue-bound CLI behavior.\n"},
		{key: "requirements", role: autopilot.SourceSectionRequirements, title: "Requirements", payload: "\n# Requirements\n\nKeep the exact CLI contract.\n"},
		{key: "acceptance-examples", role: autopilot.SourceSectionAcceptanceExamples, title: "Acceptance examples", payload: "\n# Acceptance examples\n\nThe source file may be deleted after import.\n"},
		{key: "constraints", role: autopilot.SourceSectionConstraints, title: "Constraints", payload: "\n# Constraints\n\nNever persist the source path.\n"},
		{key: "non-goals", role: autopilot.SourceSectionNonGoals, title: "Non-goals", payload: "\n# Non-goals\n\nNo provider implementation.\n"},
	}
	comments := make([]autopilot.RawSourceComment, len(definitions))
	sections := make([]autopilot.SourceManifestSection, len(definitions))
	for index, definition := range definitions {
		databaseID := int64(index + 1001)
		body := "<!-- slipway-section:v1 key=" + definition.key + " -->" + definition.payload
		digest, err := autopilot.ComputeSourceCommentBodySHA256(body)
		if err != nil {
			panic(err)
		}
		comments[index] = autopilot.RawSourceComment{
			NodeID:     fmt.Sprintf("IC_cli_%s", definition.key),
			DatabaseID: databaseID,
			URL:        fmt.Sprintf("%s#issuecomment-%d", issueURL, databaseID),
			UpdatedAt:  "2026-07-12T09:00:00Z",
			AuthorID:   "U_cli_author",
			Body:       body,
		}
		sections[index] = autopilot.SourceManifestSection{
			Key:               definition.key,
			Role:              definition.role,
			Title:             definition.title,
			CommentNodeID:     comments[index].NodeID,
			CommentDatabaseID: databaseID,
			BodySHA256:        digest,
		}
	}
	manifest, err := json.MarshalIndent(autopilot.SourceManifest{
		ManifestVersion: autopilot.SourceManifestVersion,
		Profile:         autopilot.SourceProfileChangeV2,
		Sections:        sections,
	}, "", "  ")
	if err != nil {
		panic(err)
	}
	return autopilot.RawSourceEnvelope{
		SourceVersion: autopilot.SourceVersion,
		Provider:      "github",
		Host:          "github.com",
		RepositoryID:  "R_cliSourceRepository",
		IssueID:       "I_cliSourceIssue",
		IssueNumber:   434,
		CanonicalURL:  issueURL,
		UpdatedAt:     "2026-07-12T09:00:00Z",
		FetchedAt:     "2026-07-12T09:01:00Z",
		Title:         "[Change] CLI source lifecycle",
		Body:          "<!-- slipway-level: change/v2 -->\n\n```slipway-manifest\n" + string(manifest) + "\n```\n",
		Labels:        []string{"level:change", "kind:refactor"},
		Comments:      comments,
	}
}

func setCLISourceSection(envelope *autopilot.RawSourceEnvelope, key, payload string) {
	roles := map[string]autopilot.SourceSectionRole{
		"outcome":             autopilot.SourceSectionOutcome,
		"requirements":        autopilot.SourceSectionRequirements,
		"acceptance-examples": autopilot.SourceSectionAcceptanceExamples,
		"constraints":         autopilot.SourceSectionConstraints,
		"non-goals":           autopilot.SourceSectionNonGoals,
	}
	titles := map[string]string{
		"outcome":             "Outcome",
		"requirements":        "Requirements",
		"acceptance-examples": "Acceptance examples",
		"constraints":         "Constraints",
		"non-goals":           "Non-goals",
	}
	sections := make([]autopilot.SourceManifestSection, len(envelope.Comments))
	for index := range envelope.Comments {
		comment := &envelope.Comments[index]
		commentKey := cliSourceCommentKey(comment.Body)
		if commentKey == key {
			comment.NodeID += "_replacement"
			comment.DatabaseID += 100_000
			comment.URL = fmt.Sprintf("%s#issuecomment-%d", envelope.CanonicalURL, comment.DatabaseID)
			comment.Body = "<!-- slipway-section:v1 key=" + key + " -->" + payload
		}
		digest, err := autopilot.ComputeSourceCommentBodySHA256(comment.Body)
		if err != nil {
			panic(err)
		}
		sections[index] = autopilot.SourceManifestSection{
			Key:               commentKey,
			Role:              roles[commentKey],
			Title:             titles[commentKey],
			CommentNodeID:     comment.NodeID,
			CommentDatabaseID: comment.DatabaseID,
			BodySHA256:        digest,
		}
	}
	manifest, err := json.MarshalIndent(autopilot.SourceManifest{
		ManifestVersion: autopilot.SourceManifestVersion,
		Profile:         autopilot.SourceProfileChangeV2,
		Sections:        sections,
	}, "", "  ")
	if err != nil {
		panic(err)
	}
	envelope.Body = "<!-- slipway-level: change/v2 -->\n\n```slipway-manifest\n" + string(manifest) + "\n```\n"
}

func cliSourceCommentKey(body string) string {
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return strings.TrimSuffix(strings.TrimPrefix(line, "<!-- slipway-section:v1 key="), " -->")
	}
	return ""
}

func setCLISourceParentRevision(
	envelope *autopilot.RawSourceEnvelope,
	revision string,
) {
	start := strings.Index(envelope.Body, "{")
	end := strings.LastIndex(envelope.Body, "\n```")
	if start < 0 || end <= start {
		panic("source manifest not found")
	}
	var manifest autopilot.SourceManifest
	if err := json.Unmarshal([]byte(envelope.Body[start:end]), &manifest); err != nil {
		panic(err)
	}
	manifest.ParentRequirementsRevision = revision
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		panic(err)
	}
	envelope.Body = "<!-- slipway-level: change/v2 -->\n\n```slipway-manifest\n" + string(encoded) + "\n```\n"
}

func writeCLISource(t *testing.T, envelope autopilot.RawSourceEnvelope) string {
	t.Helper()
	raw, err := json.Marshal(envelope)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "source-envelope.json")
	require.NoError(t, os.WriteFile(path, raw, 0o400))
	return path
}
