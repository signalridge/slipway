package cmd

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachineProtocolDecisionAnswerUsesStructuredNext(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "choose a channel", "--root", repository, "--budget", "8", "--json", "--no-review")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)

	waiting := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeNeedsInput, "channel decision required")
	waiting.Pause = &autopilot.Pause{Reason: autopilot.PauseDecisionRequired, Question: "Which channel?"}
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, waiting))
	require.NoError(t, err, stderr)
	var state mutationEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &state))
	assert.Equal(t, autopilot.RunPaused, state.State)
	assert.Equal(t, autopilot.NextOperationAnswer, state.Next.Operation)
	require.Len(t, state.Next.Variants, 2)
	variant := state.Next.Variants[0]
	assert.Equal(t, "answer-decision", variant.ID)
	assert.NotEmpty(t, state.Next.WorkspaceIdentity)
	assert.Equal(t, []autopilot.NextInput{{Name: "text", Type: autopilot.NextInputString, Flag: "--text", Required: true}}, variant.Inputs)
	assert.Equal(t, "skip-action", state.Next.Variants[1].ID)
	canonicalRepository, err := filepath.EvalSymlinks(repository)
	require.NoError(t, err)
	assert.Contains(t, variant.BaseArgv, canonicalRepository)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, state.Next.WorkspaceIdentity)
	assert.NotContains(t, stdout, "next_command")
	assert.NotContains(t, stdout, "<answer>")

	human, humanStderr, err := executeForTest(t, "status", action.RunID, "--root", repository)
	require.NoError(t, err, humanStderr)
	assert.Contains(t, human, "answer-decision: requires text (string via --text)")
	assert.NotContains(t, human, "<answer>")

	stdout, stderr, err = executeForTest(t, "_machine", "answer", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--text", "stable")
	require.NoError(t, err, stderr)
	reoriented := decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionOrient, reoriented.Kind)

	stdout, stderr, err = executeForTest(t, "_machine", "answer", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--text", "stable")
	require.NoError(t, err, stderr)
	retried := decodeMutationAction(t, stdout)
	assert.Equal(t, reoriented.ActionID, retried.ActionID)

	stdout, stderr, err = executeForTest(t, "_machine", "answer", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--text", "different")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"answer_conflict"`)
	assert.NotContains(t, stderr, "next_command")

	stdout, stderr, err = executeForTest(t, "_machine", "answer", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--text", "stable", "--select-suggestion", "1")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "unknown flag")
}

func TestMachineProtocolStructuredDestructiveConfirmationIssuesExactGrant(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "delete exact target", "--root", repository, "--budget", "10", "--json", "--no-review")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)
	runID := action.RunID

	orient := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts")
	orient.SuggestedActions = []autopilot.SuggestedAction{{Kind: autopilot.ActionImplement, Brief: "Delete only after exact confirmation."}}
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", runID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, orient))
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	originatingActionID := action.ActionID

	targets := []autopilot.DestructiveTarget{{Kind: autopilot.DestructiveTargetPath, Value: "/absolute/target with spaces"}}
	digest, err := autopilot.ComputeDestructiveScopeSHA256("33333333-3333-4333-8333-333333333333", targets, "delete the target permanently")
	require.NoError(t, err)
	request := &autopilot.DestructiveRequest{
		RequestID: "33333333-3333-4333-8333-333333333333", Targets: targets,
		Impact: "delete the target permanently", ScopeSHA256: digest,
	}
	pause := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeNeedsInput, "confirmation required")
	pause.Pause = &autopilot.Pause{
		Reason: autopilot.PauseDestructiveConfirm, Question: "Confirm exact target?", DestructiveRequest: request,
	}
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", runID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, pause))
	require.NoError(t, err, stderr)
	var state mutationEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &state))
	assert.Equal(t, autopilot.PauseDestructiveConfirm, state.PauseReason)
	assert.Equal(t, autopilot.NextOperationAnswer, state.Next.Operation)
	require.Len(t, state.Next.Variants, 3)
	confirm := state.Next.Variants[0]
	assert.Equal(t, "confirm-destructive", confirm.ID)
	assert.Contains(t, confirm.BaseArgv, "--confirm-destructive")
	assert.Contains(t, confirm.BaseArgv, digest)
	assert.Equal(t, []autopilot.NextInput{{Name: "text", Type: autopilot.NextInputString, Flag: "--text", Required: false}}, confirm.Inputs)
	assert.Equal(t, "decline-or-feedback", state.Next.Variants[1].ID)
	assert.Equal(t, "skip-action", state.Next.Variants[2].ID)
	assert.NotContains(t, stdout, "next_command")

	stdout, stderr, err = executeForTest(t, "_machine", "answer", "--root", repository, "--run", runID, "--action", originatingActionID, "--confirm-destructive", "--scope-sha256", digest, "--text", "confirmed")
	require.NoError(t, err, stderr)
	action = decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionImplement, action.Kind)
	require.NotNil(t, action.DestructiveAuthorization)
	authorization := action.DestructiveAuthorization
	assert.Equal(t, request.RequestID, authorization.RequestID)
	assert.Equal(t, originatingActionID, authorization.OriginatingActionID)
	assert.Equal(t, request.ScopeSHA256, authorization.ScopeSHA256)
	assert.Equal(t, request.Targets, authorization.Targets)
	assert.Equal(t, request.Impact, authorization.Impact)

	statusJSON, statusStderr, err := executeForTest(t, "status", runID, "--root", repository, "--json")
	require.NoError(t, err, statusStderr)
	var status map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(statusJSON), &status))
	assert.Contains(t, status, "next")
	assert.NotContains(t, status, "next_command")
	var next autopilot.Next
	require.NoError(t, json.Unmarshal(status["next"], &next))
	assert.Equal(t, autopilot.NextOperationAction, next.Operation)
	canonicalRepository, err := filepath.EvalSymlinks(repository)
	require.NoError(t, err)
	for _, variant := range next.Variants {
		assert.Contains(t, variant.BaseArgv, canonicalRepository)
		assert.NotContains(t, variant.BaseArgv, next.WorkspaceIdentity)
		assert.NotContains(t, strings.Join(variant.BaseArgv, "\x00"), "<file>")
	}
}

func TestStatusJSONListsDerivedNextForEveryRun(t *testing.T) {
	repository := newCLIRepository(t)
	for _, goal := range []string{"first", "second"} {
		_, stderr, err := executeForTest(t, "run", goal, "--root", repository, "--json")
		require.NoError(t, err, stderr)
	}
	stdout, stderr, err := executeForTest(t, "status", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var output statusListOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &output))
	assert.Equal(t, machineContractVersion, output.ContractVersion)
	require.Len(t, output.Runs, 2)
	for _, run := range output.Runs {
		require.NoError(t, run.Next.Validate())
		assert.Equal(t, autopilot.NextOperationAction, run.Next.Operation)
		assert.Equal(t, run.WorkspaceIdentity.ID, run.Next.WorkspaceIdentity)
		assert.NotEqual(t, run.Workspace, run.Next.WorkspaceIdentity)
		assert.NotEmpty(t, run.Next.Variants)
	}
}
