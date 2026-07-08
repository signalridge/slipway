package cmd

import (
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvidenceTaskRunSummaryVersionHelpAdvertisesDerivedDefault(t *testing.T) {
	t.Parallel()

	flag := makeEvidenceTaskCmd().Flags().Lookup("run-summary-version")
	require.NotNil(t, flag, "evidence task must expose a --run-summary-version flag")
	assert.Contains(t, flag.Usage, "omit to let Slipway derive it",
		"--run-summary-version help must describe the derived default, got %q", flag.Usage)
}

func TestEvidenceTaskFlagHelpDocumentsHostOwnedSurface(t *testing.T) {
	t.Parallel()

	flags := makeEvidenceTaskCmd().Flags()
	for _, flagName := range []string{"task-id", "verdict", "evidence-ref", "changed-file", "blocker", "session-id", "no-op-justification"} {
		flag := flags.Lookup(flagName)
		require.NotNil(t, flag, "evidence task must expose --%s", flagName)
		assert.NotContains(t, flag.Usage, "Manual flag mode only", "--%s help should be part of the public host-owned task surface", flagName)
		assert.NotContains(t, flag.Usage, "(required)", "--%s help must not imply unconditional requiredness", flagName)
	}
	for _, flagName := range []string{"run-summary-version", "task-kind", "target-file", "captured-at"} {
		flag := flags.Lookup(flagName)
		require.NotNil(t, flag, "evidence task must expose --%s", flagName)
		assert.Contains(t, flag.Usage, "Optional", "--%s help should mark ledger/debug assertions as optional", flagName)
		assert.NotContains(t, flag.Usage, "(required)", "--%s help must not imply unconditional requiredness", flagName)
	}
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
		cmd.SetArgs([]string{"restamp"})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr, "retired `evidence restamp` must fail closed as an unknown subcommand")
		assert.Equal(t, "evidence_unknown_subcommand", cliErr.ErrorCode)
	})
}

// TestEvidenceSuiteResultSubcommandFailsClosed pins the retired `suite-result`
// keystone to a fail-closed error. The parent `evidence` command is nested under
// the root, where Cobra's native unknown-subcommand rejection does not fire, so
// without the explicit Args validator a stale `evidence suite-result` call would
// no-op into help with exit 0 and falsely read as "suite proof recorded".
func TestEvidenceSuiteResultSubcommandFailsClosed(t *testing.T) {
	t.Parallel()

	cmd := commandForRoot(t, t.TempDir(), makeEvidenceCmd())
	cmd.SetArgs([]string{"suite-result"})
	cliErr := asCLIError(cmd.Execute())
	require.NotNil(t, cliErr, "retired `evidence suite-result` must fail closed, not no-op into help")
	assert.Equal(t, "evidence_suite_result_retired", cliErr.ErrorCode)
	assert.Contains(t, cliErr.Remediation, "ship-verification")
}

// TestEvidenceUnknownSubcommandFailsClosed asserts any unregistered evidence token
// fails closed rather than silently printing help with exit 0.
func TestEvidenceUnknownSubcommandFailsClosed(t *testing.T) {
	t.Parallel()

	cmd := commandForRoot(t, t.TempDir(), makeEvidenceCmd())
	cmd.SetArgs([]string{"definitely-not-a-subcommand"})
	cliErr := asCLIError(cmd.Execute())
	require.NotNil(t, cliErr, "unknown evidence subcommand must fail closed")
	assert.Equal(t, "evidence_unknown_subcommand", cliErr.ErrorCode)
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

func TestEvidenceTaskActiveChangeReloadsAfterCachedRefResolution(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		readCtx := newStateReadContext(root)
		ref, err := resolveActiveChangeRefWithReadContext(readCtx, "")
		require.NoError(t, err)
		require.Equal(t, slug, ref.Slug)

		change.Status = model.ChangeStatusDone
		change.CurrentState = model.StateDone
		require.NoError(t, state.SaveChange(root, change))

		_, err = loadActiveChangeWithReadContext(
			readCtx,
			ref.Slug,
			"cannot record task evidence for governed status %q",
			"Task evidence can only be recorded for an active governed change.",
		)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "not_active", cliErr.ErrorCode)
		assert.Equal(t, string(model.ChangeStatusDone), cliErr.Details["status"])
		assert.Equal(t, model.ChangeStatusDone, readCtx.changes[slug].Status)
	})
}
