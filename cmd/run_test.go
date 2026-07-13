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
	var action autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))
	assert.Equal(t, autopilot.ActionOrient, action.Kind)
	assert.Equal(t, autopilot.ContractVersion, action.ContractVersion)

	orient := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts gathered")
	orient.SuggestedActions = []autopilot.SuggestedAction{{Kind: autopilot.ActionImplement, Brief: "Implement the requested update."}}
	outcomePath := writeOutcome(t, orient)
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-file", outcomePath)
	require.NoError(t, err, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))
	assert.Equal(t, autopilot.ActionImplement, action.Kind)

	stdout, stderr, err = executeForTest(t, "_machine", "skip", "--root", repository, "--run", action.RunID, "--action", action.ActionID)
	require.NoError(t, err, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))
	assert.Equal(t, autopilot.ActionSummarize, action.Kind)

	stdout, stderr, err = executeForTest(t, "stop", action.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var state protocolStateOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &state))
	assert.Equal(t, autopilot.RunStopped, state.State)
	assert.Equal(t, autopilot.NextOperationResume, state.Next.Operation)
	assert.Equal(t, "resume-ad-hoc", state.Next.Variants[0].ID)

	stdout, stderr, err = executeForTest(t, "_machine", "resume", action.RunID, "--root", repository)
	require.NoError(t, err, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))
	assert.Equal(t, autopilot.ActionOrient, action.Kind)
}

func TestMachineProtocolReadsOutcomeFromStdinAndReturnsPreciseVersionRecovery(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "inspect", "--root", repository, "--json", "--no-review")
	require.NoError(t, err, stderr)
	var action autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))

	outcome := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts")
	outcome.SuggestedActions = []autopilot.SuggestedAction{{Kind: autopilot.ActionImplement, Brief: "Implement the inspected change."}}
	encoded, err := json.Marshal(outcome)
	require.NoError(t, err)
	stdout, stderr, err = executeForTestWithInput(t, string(encoded), "_machine", "submit", "--root", repository, "--run", action.RunID, "--action", action.ActionID, "--outcome-stdin")
	require.NoError(t, err, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))
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
	var action autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))

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
	var action autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))

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
}

func TestRunResumeRejectsExplicitZeroBudget(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "inspect", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var action autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))

	stdout, stderr, err = executeForTest(t, "_machine", "resume", action.RunID, "--root", repository, "--budget", "0")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"invalid_budget"`)
}

func TestMachineProtocolReviewFindingsRouteToSummaryWithoutRepair(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "change", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var action autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))
	runID := action.RunID

	orient := machineOutcome(action.ActionID, action.Kind, autopilot.OutcomeCompleted, "facts")
	orient.SuggestedActions = []autopilot.SuggestedAction{{Kind: autopilot.ActionImplement, Brief: "Implement the change."}}
	stdout, stderr, err = executeForTest(t, "_machine", "submit", "--root", repository, "--run", runID, "--action", action.ActionID, "--outcome-file", writeOutcome(t, orient))
	require.NoError(t, err, stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))

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
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))
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
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))
	assert.Equal(t, autopilot.ActionSummarize, action.Kind)
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
	err := executeRootCommand(root)
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
	var action autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &action))
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
	var initial autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &initial))

	amended := cliSourceEnvelope()
	setCLISourceSection(&amended, "requirements", "\n# Requirements\n\nKeep the amended CLI contract.\n")
	setCLISourceParentRevision(&amended, initial.Source.RequirementsRevision)
	candidatePath := writeCLISource(t, amended)
	stdout, stderr, err = executeForTest(t, "_machine", "resume", initial.RunID, "--root", repository, "--source-file", candidatePath, "--budget", "20")
	require.NoError(t, err, stderr)
	require.NoError(t, os.Remove(candidatePath))
	var paused protocolStateOutput
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
	var adopted autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &adopted))
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
	var issueAction autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &issueAction))

	stdout, stderr, err = executeForTest(t, "_machine", "resume", issueAction.RunID, "--root", issueRepository)
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"source_mode_required"`)

	stdout, stderr, err = executeForTest(t, "_machine", "resume", issueAction.RunID, "--root", issueRepository, "--use-pinned-source")
	require.NoError(t, err, stderr)
	var refreshed autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &refreshed))
	assert.Equal(t, autopilot.ActionOrient, refreshed.Kind)
	require.NotNil(t, refreshed.Source)

	adHocRepository := newCLIRepository(t)
	stdout, stderr, err = executeForTest(t, "run", "ad-hoc", "--root", adHocRepository, "--json")
	require.NoError(t, err, stderr)
	var adHocAction autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(stdout), &adHocAction))
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
