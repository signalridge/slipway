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

func TestEvidenceTaskWrongStateInS3RoutesToGoalVerificationAndFinalCloseout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		_, change := createEvidenceTaskFixture(t, root)
		change.CurrentState = model.StateS3Review
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
		assert.Contains(t, cliErr.Remediation, "goal-verification")
		assert.Contains(t, cliErr.Remediation, "final-closeout")
	})
}

func TestEvidenceTaskWrongStateInS3RoutesToReviewAndVerificationEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		_, change := createEvidenceTaskFixture(t, root)
		change.CurrentState = model.StateS3Review
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
		assert.Contains(t, cliErr.Remediation, "spec-compliance-review")
		assert.Contains(t, cliErr.Remediation, "code-quality-review")
		assert.Contains(t, cliErr.Remediation, "goal-verification")
		assert.Contains(t, cliErr.Remediation, "final-closeout")
	})
}
