package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunAtS3AcceptsFoldedTaskEvidenceAndRejectsRestamp is the forward-exit
// companion to TestRunAtS3AbsorbsAddedTaskInPlace. Folding a task in at S3 must
// also be EXITABLE through the public surface: the folded task's evidence is
// recordable at S3 via `slipway evidence task`, and the next `slipway run`
// absorbs it so the `incomplete_execution_task` blocker clears in place — no
// backward rescope re-walk. Without a forward path the convergence is a deadlock.
func TestRunAtS3AcceptsFoldedTaskEvidenceAndRejectsRestamp(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		recordTaskResult(t, root, "t-01", "test:original-t-01", "cmd/evidence.go")
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		// Discover a task at review.
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` harden result file loading
  - depends_on: []
  - target_files: ["cmd/evidence.go"]
  - task_kind: code
  - covers: [REQ-001]

- [ ] `+"`t-02`"+` cover the newly discovered gap
  - depends_on: []
  - target_files: ["cmd/evidence_task_test.go"]
  - task_kind: test
  - covers: [REQ-001]
`)))

		// First run: absorb the plan delta in place; t-02 surfaces as incomplete.
		runOnce(t, root, slug)
		reopened, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.Equal(t, model.StateS3Review, reopened.CurrentState)

		// Forward exit: the folded task's evidence is recordable AT S3.
		err = recordTaskResultErr(t, root, "t-02", "test:new-t-02", "cmd/evidence_task_test.go")
		require.NoError(t, err, "S3 must accept the folded task's evidence as the public forward exit")

		// No restamp: re-recording an already-evidenced task at S3 is rejected.
		err = recordTaskResultErr(t, root, "t-01", "test:restamp-t-01", "cmd/evidence.go")
		require.Error(t, err, "already-evidenced tasks stay frozen at S3 (no restamp)")
		assert.Contains(t, err.Error(), "already has recorded evidence")

		// The folded task's evidence is now written to the ledger — the public
		// forward exit that was previously a hard deadlock.
		assertTaskEvidenceWritten(t, root, slug, "t-02")
	})
}

// TestRunAtS3ConvergesFoldedTaskThroughWaveReRecord exercises the COMPLETE forward
// exit, symmetric to the S2 flow: at S3 the folded task's evidence is recorded
// (Door 1), then wave-orchestration evidence is re-recorded (Door 2) so the fresh
// wave record post-dates that task evidence. Re-recording the wave run rebuilds the
// execution summary in place, clearing the incomplete_execution_task blocker — no
// backward rescope re-walk. Without Door 2 the wave record stays older than the
// folded task's evidence and the convergence deadlocks.
func TestRunAtS3ConvergesFoldedTaskThroughWaveReRecord(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		recordTaskResult(t, root, "t-01", "test:original-t-01", "cmd/evidence.go")
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` harden result file loading
  - depends_on: []
  - target_files: ["cmd/evidence.go"]
  - task_kind: code
  - covers: [REQ-001]

- [ ] `+"`t-02`"+` cover the newly discovered gap
  - depends_on: []
  - target_files: ["cmd/evidence_task_test.go"]
  - task_kind: test
  - covers: [REQ-001]
`)))

		// First run folds the plan delta in place; t-02 surfaces as incomplete and the
		// blocker is durable in the persisted summary.
		runOnce(t, root, slug)
		require.True(t, summaryHasIncompleteTask(t, root, slug), "fold must surface incomplete_execution_task")

		// Door 1: record the folded task's evidence.
		require.NoError(t, recordTaskResultErr(t, root, "t-02", "test:new-t-02", "cmd/evidence_task_test.go"))

		// Door 2: re-attest the wave run at S3. The fresh wave record post-dates t-02's
		// evidence, and recording it rebuilds the summary in place.
		require.NoError(t, recordWaveEvidenceErr(t, root, slug), "S3 must accept the wave re-record while convergence is pending")

		// The incomplete blocker is cleared by the in-place rebuild.
		assert.False(t, summaryHasIncompleteTask(t, root, slug), "wave re-record must clear incomplete_execution_task in place")

		// State never regressed to S1/S2 through the convergence.
		reopened, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3Review, reopened.CurrentState)

		// Once converged, the wave re-record door closes again (no incomplete signal).
		require.Error(t, recordWaveEvidenceErr(t, root, slug), "settled review must not keep the wave re-record door open")
	})
}

// TestRunAtS3RejectsWaveReRecordBeforeFoldedTaskEvidence locks the S3 ordering
// contract symmetric to S2: Door 2 (wave-orchestration re-record) must not write a
// passing record before Door 1 (the folded task's evidence) is in the ledger. The
// re-materialized wave plan surfaces the folded task as incomplete_execution_task,
// so attempting the wave re-record first must fail closed with
// evidence_skill_task_evidence_incomplete — the SAME completeness guard S2 enforces
// — rather than saving a misleading out-of-order passing wave record.
func TestRunAtS3RejectsWaveReRecordBeforeFoldedTaskEvidence(t *testing.T) {
	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		recordTaskResult(t, root, "t-01", "test:original-t-01", "cmd/evidence.go")
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` harden result file loading
  - depends_on: []
  - target_files: ["cmd/evidence.go"]
  - task_kind: code
  - covers: [REQ-001]

- [ ] `+"`t-02`"+` cover the newly discovered gap
  - depends_on: []
  - target_files: ["cmd/evidence_task_test.go"]
  - task_kind: test
  - covers: [REQ-001]
`)))

		// Fold the plan delta in place; t-02 surfaces as incomplete.
		runOnce(t, root, slug)
		require.True(t, summaryHasIncompleteTask(t, root, slug), "fold must surface incomplete_execution_task")

		// Door 2 before Door 1: re-record wave-orchestration WITHOUT first recording
		// the folded task's evidence. This must fail closed like S2, not save a passing
		// out-of-order wave record.
		err := recordWaveEvidenceErr(t, root, slug)
		require.Error(t, err, "wave re-record before folded task evidence must fail closed")
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_task_evidence_incomplete", cliErr.ErrorCode)
		require.NotNil(t, cliErr.Recovery)
		assert.Equal(t, "slipway evidence task --task-id t-02 --verdict <verdict> --evidence-ref <ref> [--changed-file <path>]...", cliErr.Recovery.PrimaryCommand)
		assert.Equal(t, "incomplete_execution_task", cliErr.Recovery.Steps[0].Code)

		// No passing wave-orchestration record may have been written out of order.
		assert.True(t, summaryHasIncompleteTask(t, root, slug), "rejected re-record must leave the incomplete blocker intact")
	})
}

func recordWaveEvidenceErr(t *testing.T, root, slug string) error {
	t.Helper()
	cmd := commandForRoot(t, root, makeEvidenceCmd())
	cmd.SetArgs([]string{
		"skill",
		"--change", slug,
		"--skill", "wave-orchestration",
		"--verdict", "pass",
		"--reference", "wave-orchestration:pass",
		"--notes", "Wave re-attested for folded task.",
	})
	cmd.SetOut(&bytes.Buffer{})
	return cmd.Execute()
}

func summaryHasIncompleteTask(t *testing.T, root, slug string) bool {
	t.Helper()
	summary, err := state.LoadOptionalExecutionSummary(root, slug)
	require.NoError(t, err)
	if summary == nil {
		return false
	}
	for _, blocker := range summary.OpenBlockers {
		if strings.TrimSpace(blocker.Code) == "incomplete_execution_task" {
			return true
		}
	}
	return false
}

func recordTaskResult(t *testing.T, root, taskID, ref, changedFile string) {
	t.Helper()
	require.NoError(t, recordTaskResultErr(t, root, taskID, ref, changedFile))
}

func recordTaskResultErr(t *testing.T, root, taskID, ref, changedFile string) error {
	t.Helper()
	cmd := commandForRoot(t, root, makeEvidenceCmd())
	cmd.SetArgs([]string{
		"task", "--json",
		"--task-id", taskID,
		"--verdict", "pass",
		"--evidence-ref", ref,
		"--changed-file", changedFile,
	})
	cmd.SetOut(&bytes.Buffer{})
	return cmd.Execute()
}

func runOnce(t *testing.T, root, slug string) {
	t.Helper()
	cmd := commandForRoot(t, root, makeRunCmd())
	cmd.SetArgs([]string{"--json", "--change", slug})
	cmd.SetOut(&bytes.Buffer{})
	_ = cmd.Execute()
}
