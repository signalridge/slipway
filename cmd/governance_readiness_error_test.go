package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsDoesNotMislabelMalformedVerificationAsExecutionSummaryFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "stats should classify readiness failures correctly")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	verificationPath := state.VerificationFilePath(root, slug, "plan-audit")
	require.NoError(t, os.MkdirAll(filepath.Dir(verificationPath), 0o755))
	require.NoError(t, os.WriteFile(verificationPath, []byte("verdict: ["), 0o644))

	view, err := buildStatsView(root, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, view.IntegrityIssues, 1)
	assert.Contains(t, view.IntegrityIssues[0], slug)
	assert.Contains(t, view.IntegrityIssues[0], "verification_load_failed")
	assert.NotContains(t, view.IntegrityIssues[0], "execution_summary_load_failed")
	assert.Contains(t, view.IntegrityIssues[0], "evaluate stats readiness")
	assert.Contains(t, view.IntegrityIssues[0], "remediation: Run `slipway repair` to inspect authoritative verification files")
}

func TestNextReadinessFailureUsesGovernanceReadinessEnvelope(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "next should classify readiness failures correctly")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		verificationPath := state.VerificationFilePath(root, slug, "plan-audit")
		require.NoError(t, os.MkdirAll(filepath.Dir(verificationPath), 0o755))
		require.NoError(t, os.WriteFile(verificationPath, []byte("verdict: ["), 0o644))

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)

		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "verification_load_failed", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
		assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
		assert.Equal(t, "evaluate next skill evidence", cliErr.Details["operation"])
		assert.Equal(t, verificationReadPathForTest(root, slug, "plan-audit"), cliErr.Details["path"])
	})
}

func TestNextReadinessFailureEnvelopeJSON(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "next JSON envelope should classify readiness failures correctly")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		verificationPath := state.VerificationFilePath(root, slug, "plan-audit")
		require.NoError(t, os.MkdirAll(filepath.Dir(verificationPath), 0o755))
		require.NoError(t, os.WriteFile(verificationPath, []byte("verdict: ["), 0o644))

		_, stderr, err := runRootCommand([]string{"next", "--json"})
		require.Error(t, err)

		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryStateIntegrity, payload.Category)
		assert.Equal(t, exitCodeStateIntegrity, payload.ExitCode)
		assert.Equal(t, "verification_load_failed", payload.ErrorCode)
		assert.Equal(t, "evaluate next skill evidence", payload.Details["operation"])
		assert.Equal(t, verificationReadPathForTest(root, slug, "plan-audit"), payload.Details["path"])
	})
}

func TestStatusReadinessFailureUsesGovernanceReadinessEnvelope(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "status should classify readiness failures correctly")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeMalformedVerificationFile(t, root, slug, "plan-audit")

		var out bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetArgs([]string{"--change", slug})
		cmd.SetOut(&out)

		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "verification_load_failed", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
		assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
		assert.Equal(t, "build status view", cliErr.Details["operation"])
		assert.Equal(t, verificationReadPathForTest(root, slug, "plan-audit"), cliErr.Details["path"])
	})
}

func TestValidateReadinessFailureUsesGovernanceReadinessEnvelope(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "validate should classify readiness failures correctly")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writeMalformedVerificationFile(t, root, slug, "plan-audit")

		var out bytes.Buffer
		cmd := makeValidateCmd()
		cmd.SetArgs([]string{"--change", slug})
		cmd.SetOut(&out)

		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "verification_load_failed", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
		assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
		assert.Equal(t, "validate readiness", cliErr.Details["operation"])
		assert.Equal(t, verificationReadPathForTest(root, slug, "plan-audit"), cliErr.Details["path"])
	})
}

func TestReviewReadinessFailureUsesGovernanceReadinessEnvelope(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "review should classify readiness failures correctly")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		change.PlanSubStep = model.PlanSubStepNone
		change.WorkflowPreset = model.WorkflowPresetStandard
		require.NoError(t, state.SaveChange(root, change))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		materializeWaveExecutionForSummary(t, root, slug)

		writeMalformedVerificationFile(t, root, slug, "plan-audit")

		var out bytes.Buffer
		cmd := makeReviewCmd()
		cmd.SetArgs([]string{"--change", slug})
		cmd.SetOut(&out)

		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "verification_load_failed", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
		assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
		assert.Equal(t, "evaluate review prerequisites", cliErr.Details["operation"])
		assert.Equal(t, verificationReadPathForTest(root, slug, "plan-audit"), cliErr.Details["path"])
	})
}

func TestNextDiagnosticsProjectionFailureUsesGovernanceReadinessEnvelope(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "next should classify projection failures consistently")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.SuggestedWorkflowPreset = ""
		require.NoError(t, state.SaveChange(root, change))

		intentPath := filepath.Join(root, "artifacts", "changes", slug, "intent.md")
		require.NoError(t, os.RemoveAll(intentPath))
		require.NoError(t, os.MkdirAll(intentPath, 0o755))

		var out bytes.Buffer
		cmd := makeNextCmd()
		cmd.SetArgs([]string{"--json", "--diagnostics", "--change", slug})
		cmd.SetOut(&out)

		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "governance_readiness_failed", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
		assert.Equal(t, exitCodeStateIntegrity, cliErr.ExitCode)
		assert.Equal(t, "evaluate next skill evidence", cliErr.Details["operation"])
	})
}

func TestRunReadinessFailurePreservesInterruptedExecutionMarker(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelNonDiscovery, "run should preserve interrupted marker on readiness failure")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.SuggestedWorkflowPreset = ""
		interruptedAt := time.Now().UTC().Add(-time.Minute)
		change.InterruptedExecutionAt = interruptedAt
		require.NoError(t, state.SaveChange(root, change))

		intentPath := filepath.Join(root, "artifacts", "changes", slug, "intent.md")
		require.NoError(t, os.RemoveAll(intentPath))
		require.NoError(t, os.MkdirAll(intentPath, 0o755))

		var out bytes.Buffer
		cmd := makeRunCmd()
		cmd.SetArgs([]string{"--change", slug})
		cmd.SetOut(&out)

		err = cmd.Execute()
		require.Error(t, err)

		after, loadErr := state.LoadChange(root, slug)
		require.NoError(t, loadErr)
		assert.True(t, after.InterruptedExecutionAt.Equal(interruptedAt), "failed run must not clear interrupted execution state")
	})
}

func writeMalformedVerificationFile(t *testing.T, root, slug, skillName string) {
	t.Helper()

	verificationPath := state.VerificationFilePath(root, slug, skillName)
	require.NoError(t, os.MkdirAll(filepath.Dir(verificationPath), 0o755))
	require.NoError(t, os.WriteFile(verificationPath, []byte("verdict: ["), 0o644))
}
