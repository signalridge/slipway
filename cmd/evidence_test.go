package cmd

import (
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvidenceTaskRunSummaryVersionHelpAdvertisesFirstVersionIsOne(t *testing.T) {
	t.Parallel()

	flag := makeEvidenceTaskCmd().Flags().Lookup("run-summary-version")
	require.NotNil(t, flag, "evidence task must expose a --run-summary-version flag")
	assert.Contains(t, flag.Usage, ">= 1",
		"--run-summary-version help must keep advertising the >= 1 rule, got %q", flag.Usage)
	assert.Contains(t, strings.ToLower(flag.Usage), "first task-evidence run version is 1",
		"--run-summary-version help must tell users the first task-evidence run version is 1, got %q", flag.Usage)
}

func TestEvidenceTaskManualFlagHelpIsScopedToManualMode(t *testing.T) {
	t.Parallel()

	flags := makeEvidenceTaskCmd().Flags()
	for _, flagName := range []string{"task-id", "run-summary-version", "task-kind", "verdict", "evidence-ref", "changed-file", "target-file", "blocker", "captured-at", "session-id"} {
		flag := flags.Lookup(flagName)
		require.NotNil(t, flag, "evidence task must expose --%s", flagName)
		assert.Contains(t, flag.Usage, "Manual flag mode only", "--%s help should be scoped to manual mode", flagName)
		assert.NotContains(t, flag.Usage, "(required)", "--%s help must not be unconditionally required next to --result-file", flagName)
	}
}

func TestEvidenceTaskResultFileHelpAdvertisesImportPath(t *testing.T) {
	t.Parallel()

	flag := makeEvidenceTaskCmd().Flags().Lookup("result-file")
	require.NotNil(t, flag, "evidence task must expose a --result-file flag")
	assert.Contains(t, flag.Usage, "executor result JSON")
	assert.Contains(t, flag.Usage, "task_id")
	assert.Contains(t, flag.Usage, "changed_files")
	assert.Contains(t, flag.Usage, "may be repeated")
	assert.Contains(t, flag.Usage, "atomic batch import")
}

func TestEvidenceTaskRunSummaryVersionZeroIsRejected(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "0",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:zero-version",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_run_summary_version_invalid", cliErr.ErrorCode)
	})
}

func TestEvidenceRestampCommandIsNotRegistered(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{"restamp", "--json"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown command")
	})
}

// TestEvidenceTaskWrongStateBeforeImplement asserts task evidence is rejected from
// states that own neither wave execution nor its review convergence. S2_IMPLEMENT
// (normal wave execution) and S3_REVIEW (in-place convergence for a folded-in task)
// are the only recordable states; recording from S1_PLAN fails closed with a
// remediation that names both valid states.
func TestEvidenceTaskWrongStateBeforeImplement(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		_, change := createEvidenceTaskFixture(t, root)
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepAudit
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wrong-state",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_wrong_state", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "S2_IMPLEMENT")
		assert.Contains(t, cliErr.Remediation, "S3_REVIEW")
	})
}
