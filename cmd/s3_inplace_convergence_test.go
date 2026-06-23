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
// Today the wave plan only materializes on the S1->S2 entry, so a task added at
// S3 is stranded (never folded), and this fails on the TotalTasks assertion.
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
