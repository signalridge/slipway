package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunAtS3AbsorbsAddedTaskInPlace is the RED reproduction for the S3
// in-place convergence objective: when a change is at S3_REVIEW and the author
// discovers more work, editing tasks.md to add a task must be ABSORBED IN
// PLACE. The wave plan must re-materialize at the SAME run version to fold the
// new task, unchanged-task evidence must be preserved, the new task must be
// surfaced as still needing evidence, and the lifecycle must NOT regress to
// S1/S2 or wipe prior evidence (the `slipway fix --start-reexecution` hammer).
//
// This locks the current S3 convergence contract: additive review-discovered
// tasks are folded into the live wave projection without bumping the run.
func TestRunAtS3AbsorbsAddedTaskInPlace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		// Record passing evidence for the original single task t-01 and drive
		// the change to S3_REVIEW (mirrors the established S2->S3 setup).
		rawResult, err := json.Marshal(map[string]any{
			"task_id":       "t-01",
			"verdict":       "pass",
			"evidence_ref":  "test:original-t-01",
			"changed_files": []string{"cmd/evidence.go"},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, "task-result.json"), rawResult, 0o644))

		evidenceCmd := commandForRoot(t, root, makeEvidenceCmd())
		evidenceCmd.SetArgs([]string{"task", "--json", "--result-file", "task-result.json"})
		evidenceCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, evidenceCmd.Execute())

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		// Discover at review that a new task is needed: append t-02 to tasks.md.
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

		// The public advancing flow must absorb the plan delta in place. It may
		// block afterwards on the new task's missing evidence; that is expected
		// and not asserted here.
		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--change", slug})
		runCmd.SetOut(&bytes.Buffer{})
		_ = runCmd.Execute()

		// In-place convergence: state must stay at S3 (no backward re-walk).
		reopened, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3Review, reopened.CurrentState,
			"editing tasks.md at S3 must converge in place, not regress the lifecycle")

		// The wave plan must re-materialize in place and fold the added task,
		// WITHOUT bumping the run version (that is what makes the edit cheap).
		plan, err := state.LoadWavePlanForChange(root, reopened)
		require.NoError(t, err)
		assert.Equal(t, 2, plan.TotalTasks,
			"the task added at S3 must be folded into the wave plan in place")
		assert.Equal(t, 1, plan.RunSummaryVersion,
			"in-place absorption must NOT bump the run version or wipe the run")

		// Unchanged-task evidence is preserved; only the new task needs evidence.
		assertTaskEvidenceWritten(t, root, slug, "t-01")
		assertTaskEvidenceNotWritten(t, root, slug, "t-02")
	})
}

func TestRunAtS3AbsorbsTargetFilesOnlyEditInPlace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:original-target-files",
		})
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` exercise command fixture
  - depends_on: []
  - target_files: ["cmd/lifecycle_commands_test.go", "cmd/fix.go"]
  - task_kind: verification
  - covers: [REQ-001]
`)))

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--change", slug})
		runCmd.SetOut(&bytes.Buffer{})
		_ = runCmd.Execute()

		reopened, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3Review, reopened.CurrentState,
			"target_files-only S3 task-plan edits must converge in place")

		plan, err := state.LoadWavePlanForChange(root, reopened)
		require.NoError(t, err)
		require.Len(t, plan.Waves, 1)
		require.Len(t, plan.Waves[0].Tasks, 1)
		assert.ElementsMatch(t, []string{"cmd/lifecycle_commands_test.go", "cmd/fix.go"}, plan.Waves[0].Tasks[0].TargetFiles)
		assert.Equal(t, 1, plan.RunSummaryVersion,
			"scope-only absorption must NOT bump the run version or wipe task evidence")
		assertTaskEvidenceWritten(t, root, slug, "t-01")
	})
}

func TestRunAtS3AbsorbsEditedTaskInPlace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		rawResult, err := json.Marshal(map[string]any{
			"task_id":       "t-01",
			"verdict":       "pass",
			"evidence_ref":  "test:original-t-01",
			"changed_files": []string{"cmd/evidence.go"},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, "task-result.json"), rawResult, 0o644))

		evidenceCmd := commandForRoot(t, root, makeEvidenceCmd())
		evidenceCmd.SetArgs([]string{"task", "--json", "--result-file", "task-result.json"})
		evidenceCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, evidenceCmd.Execute())

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` harden result file loading after review
  - depends_on: []
  - target_files: ["cmd/fix.go"]
  - task_kind: code
  - covers: [REQ-001]
`)))

		runCmd := commandForRoot(t, root, makeRunCmd())
		runCmd.SetArgs([]string{"--json", "--change", slug})
		runCmd.SetOut(&bytes.Buffer{})
		_ = runCmd.Execute()

		reopened, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Equal(t, model.StateS3Review, reopened.CurrentState,
			"editing an existing task at S3 must converge in place, not regress the lifecycle")

		plan, err := state.LoadWavePlanForChange(root, reopened)
		require.NoError(t, err)
		assert.Equal(t, 1, plan.TotalTasks)
		assert.Equal(t, 1, plan.RunSummaryVersion,
			"in-place absorption must NOT bump the run version or wipe the run")
		require.Len(t, plan.Waves, 1)
		require.Len(t, plan.Waves[0].Tasks, 1)
		assert.Equal(t, "harden result file loading after review", plan.Waves[0].Tasks[0].Objective)
		assert.Equal(t, []string{"cmd/fix.go"}, plan.Waves[0].Tasks[0].TargetFiles)
		assertTaskEvidenceWritten(t, root, slug, "t-01")

		restampResult, err := json.Marshal(map[string]any{
			"task_id":       "t-01",
			"verdict":       "pass",
			"evidence_ref":  "test:restamp-t-01",
			"changed_files": []string{"cmd/fix.go"},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, "task-result-restamp.json"), restampResult, 0o644))

		restampCmd := commandForRoot(t, root, makeEvidenceCmd())
		restampCmd.SetArgs([]string{"task", "--json", "--result-file", "task-result-restamp.json"})
		restampCmd.SetOut(&bytes.Buffer{})
		err = restampCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_already_recorded_at_review", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "review-driven repairs or tests")
		assert.Contains(t, cliErr.Remediation, "record fresh proof for")
		assert.Contains(t, cliErr.Remediation, "evidence")
		assert.NotContains(t, cliErr.Remediation, "record or refresh evidence only for the amended tasks")
	})

}
