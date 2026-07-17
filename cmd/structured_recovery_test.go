package cmd

import (
	"encoding/json"
	"os"
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
	stdout, stderr, err = executeForTest(t, "protocol", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, waiting))
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
	assert.Contains(t, human, "Pending question: Which channel?")
	assert.Contains(t, human, "answer-decision: requires text (string via --text)")
	assert.NotContains(t, human, "<answer>")

	stdout, stderr, err = executeForTest(t, "protocol", "answer", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--text", "stable")
	require.NoError(t, err, stderr)
	reoriented := decodeMutationAction(t, stdout)
	assert.Equal(t, autopilot.ActionOrient, reoriented.Kind)

	stdout, stderr, err = executeForTest(t, "protocol", "answer", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--text", "stable")
	require.NoError(t, err, stderr)
	retried := decodeMutationAction(t, stdout)
	assert.Equal(t, reoriented.ActionID, retried.ActionID)

	stdout, stderr, err = executeForTest(t, "protocol", "answer", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--text", "different")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"answer_conflict"`)
	assert.NotContains(t, stderr, "next_command")

	stdout, stderr, err = executeForTest(t, "protocol", "answer", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--text", "stable", "--select-suggestion", "1")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "unknown flag")
}

func TestPendingQuestionTextIsSingleLineAndBounded(t *testing.T) {
	action := autopilot.Action{ActionID: "action-1"}
	run := autopilot.Run{
		CurrentAction: &action,
		Actions: []autopilot.ActionRecord{{
			Action:  action,
			Outcome: &autopilot.Outcome{Pause: &autopilot.Pause{Question: "first line\n" + strings.Repeat("界", maxPendingQuestionRunes)}},
		}},
	}

	question := pendingQuestionText(run)
	assert.Len(t, []rune(question), maxPendingQuestionRunes)
	assert.NotContains(t, question, "\n")
	assert.True(t, strings.HasSuffix(question, "…"))
}

func TestMachineProtocolStructuredDestructiveConfirmationIssuesExactGrant(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "delete exact target", "--root", repository, "--budget", "10", "--json", "--no-review")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, stdout)
	runID := action.RunID

	orient := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts")
	orient.SuggestedActions = []autopilot.SuggestedAction{{Kind: autopilot.ActionImplement, Brief: "Delete only after exact confirmation."}}
	stdout, stderr, err = executeForTest(t, "protocol", "submit", "--root", repository, "--run", runID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, orient))
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
	stdout, stderr, err = executeForTest(t, "protocol", "submit", "--root", repository, "--run", runID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, pause))
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

	stdout, stderr, err = executeForTest(t, "protocol", "answer", "--root", repository, "--run", runID, "--action", originatingActionID, "--confirm-destructive", "--scope-sha256", digest, "--text", "confirmed")
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

func TestStatusListMarksForeignWorkspaceRunsWithoutReplayingThem(t *testing.T) {
	repository := newCLIRepository(t)
	foreignRoot := filepath.Join(t.TempDir(), "foreign worktree")
	runCLIGit(t, repository, "worktree", "add", "--detach", foreignRoot, "HEAD")

	canonicalForeignRoot, err := filepath.EvalSymlinks(foreignRoot)
	require.NoError(t, err)
	startedJSON, stderr, err := executeForTest(t, "run", "foreign visibility", "--root", foreignRoot, "--json")
	require.NoError(t, err, stderr)
	started := decodeMutationAction(t, startedJSON)

	stdout, stderr, err := executeForTest(t, "status", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	listed := exactJSONObject(t, stdout, "contract_version", "runs", "unavailable_runs")
	listedRuns := rawJSONArray(t, listed, "runs")
	require.Len(t, listedRuns, 1)
	exactRawJSONObject(
		t,
		listedRuns[0],
		"contract_version", "id", "goal", "workspace", "workspace_identity", "workspace_foreign", "state", "created_at", "next",
	)
	var output statusListOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &output))
	require.Len(t, output.Runs, 1)
	assert.Empty(t, output.UnavailableRuns)
	foreign := output.Runs[0]
	assert.Equal(t, started.RunID, foreign.ID)
	assert.True(t, foreign.WorkspaceForeign)
	assert.Equal(t, canonicalForeignRoot, foreign.Workspace)
	assert.Nil(t, foreign.CurrentAction)
	assert.Nil(t, foreign.Actions)
	assert.Zero(t, foreign.InitialBudget)
	assert.Equal(t, autopilot.NextOperationCommand, foreign.Next.Operation)
	require.Len(t, foreign.Next.Variants, 1)
	assert.Equal(t, "inspect-run-in-its-workspace", foreign.Next.Variants[0].ID)
	assert.Equal(t, []string{"slipway", "status", foreign.ID, "--root", canonicalForeignRoot}, foreign.Next.Variants[0].BaseArgv)

	human, stderr, err := executeForTest(t, "status", "--root", repository)
	require.NoError(t, err, stderr)
	assert.Contains(t, human, foreign.ID)
	assert.Contains(t, human, "foreign=true")
	assert.Contains(t, human, "workspace="+canonicalForeignRoot)
	assert.NotContains(t, human, "remaining=")
}

func TestStatusListKeepsForeignRunIdentityPinnedAfterItsWorktreeDisappears(t *testing.T) {
	repository := newCLIRepository(t)
	foreignRoot := filepath.Join(t.TempDir(), "foreign worktree")
	runCLIGit(t, repository, "worktree", "add", "--detach", foreignRoot, "HEAD")

	canonicalForeignRoot, err := filepath.EvalSymlinks(foreignRoot)
	require.NoError(t, err)
	startedJSON, stderr, err := executeForTest(t, "run", "foreign visibility", "--root", foreignRoot, "--json")
	require.NoError(t, err, stderr)
	started := decodeMutationAction(t, startedJSON)

	// The owning worktree goes away while its identity stays pinned in the
	// journal, so nothing can be rediscovered from the vanished path.
	require.NoError(t, os.RemoveAll(canonicalForeignRoot))

	stdout, stderr, err := executeForTest(t, "status", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var output statusListOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &output))
	require.Len(t, output.Runs, 1)
	foreign := output.Runs[0]
	assert.Equal(t, started.RunID, foreign.ID)
	assert.True(t, foreign.WorkspaceForeign)
	assert.Equal(
		t,
		foreign.WorkspaceIdentity.ID,
		foreign.Next.WorkspaceIdentity,
		"next must carry the pinned identity, never one synthesized from the vanished path",
	)
	require.Len(t, foreign.Next.Variants, 1)
	assert.Equal(t, []string{"slipway", "status", foreign.ID, "--root", canonicalForeignRoot}, foreign.Next.Variants[0].BaseArgv)
}
