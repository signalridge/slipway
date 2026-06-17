package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The public per-task-evidence + wave-orchestration flow must materialize
// execution-summary.yaml by itself, so a downstream validate no longer blocks on
// run_summary_missing until an undocumented `slipway repair` (issue #228).
func TestIssue228EvidenceSkillWaveOrchestrationMaterializesExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		// Before any evidence, the public flow has produced no run summary.
		summaryBefore, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		require.NoError(t, err)
		require.Nil(t, summaryBefore)

		// Step 1: record the per-task evidence through the public command.
		taskCmd := commandForRoot(t, root, makeEvidenceCmd())
		taskCmd.SetArgs([]string{
			"task",
			"--change", slug,
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:t-01",
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
		})
		require.NoError(t, taskCmd.Execute())

		// Recording task evidence alone does not write the summary; the
		// wave-orchestration skill step owns that.
		summaryMid, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		require.NoError(t, err)
		require.Nil(t, summaryMid, "task evidence alone must not materialize the run summary")

		// Step 2: record wave-orchestration skill evidence through the public
		// command. This is the owning public step that must now materialize the
		// summary.
		skillCmd := commandForRoot(t, root, makeEvidenceCmd())
		skillCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", model.VerificationVerdictPass,
			"--reference", "wave-orchestration:pass",
			"--notes", "Wave orchestration passed.",
		})
		var out bytes.Buffer
		skillCmd.SetOut(&out)
		require.NoError(t, skillCmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.True(t, view.Recorded)
		assert.Equal(t, 1, view.RunVersion)

		// The summary is now materialized by the same public command — no repair.
		summaryAfter, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		require.NoError(t, err)
		require.NotNil(t, summaryAfter, "wave-orchestration skill evidence must materialize execution-summary.yaml")
		assert.Equal(t, 1, summaryAfter.RunSummaryVersion)
		require.Len(t, summaryAfter.Tasks, 1)
		assert.Equal(t, "t-01", summaryAfter.Tasks[0].TaskID)
		assert.True(t, state.ExecutionSummaryReady(summaryAfter))
	})
}

// issue227SeedTwoWaveExecution seeds an S2_EXECUTE change with a two-wave plan
// (task-01 in wave 1, task-02 in wave 2) and no materialized run summary.
func issue227SeedTwoWaveExecution(t *testing.T, root, slug string) {
	t.Helper()
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, writeBundleArtifactFile(
		bundlePath,
		slug,
		"tasks.md",
		[]byte("\n"+
			"- [ ] `task-01` first wave task\n"+
			"  - depends_on: []\n"+
			"  - target_files: [\"cmd/checkpoint.go\"]\n"+
			"  - task_kind: code\n\n"+
			"- [ ] `task-02` second wave task\n"+
			"  - depends_on: [\"task-01\"]\n"+
			"  - target_files: [\"cmd/evidence.go\"]\n"+
			"  - task_kind: code\n"),
	))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
}

// Mid-wave, after only wave 1's task evidence is recorded (and BEFORE any
// wave-orchestration skill evidence, so no run summary exists yet), a checkpoint
// on a wave-2 task must be settable: the documented per-task-evidence flow has
// completed wave 1, so wave 2 is the current incomplete wave (issue #227a).
func TestIssue227CheckpointSettableAtWaveBoundaryBeforeSkillEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "issue227 checkpoint wave boundary")
		issue227SeedTwoWaveExecution(t, root, slug)

		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		// Record wave 1's task as passing through the public command. No
		// wave-orchestration skill evidence yet -> no materialized run summary.
		taskCmd := commandForRoot(t, root, makeEvidenceCmd())
		taskCmd.SetArgs([]string{
			"task",
			"--change", slug,
			"--task-id", "task-01",
			"--run-summary-version", "1",
			"--task-kind", "code",
			"--verdict", "pass",
			"--evidence-ref", "test:task-01",
			"--changed-file", "cmd/checkpoint.go",
			"--target-file", "cmd/checkpoint.go",
		})
		require.NoError(t, taskCmd.Execute())

		summary, err := state.LoadOptionalRelevantExecutionSummary(root, change)
		require.NoError(t, err)
		require.Nil(t, summary, "no skill evidence yet means no materialized run summary")

		// Checkpoint on the wave-2 task must now succeed.
		cmd := commandForRoot(t, root, makeCheckpointCmd())
		cmd.SetArgs([]string{"--json", "--change", slug, "--task-id", "task-02", "--type", "human_verify"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		require.NoError(t, cmd.Execute(), "checkpoint output: %s", buf.String())

		var view checkpointView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.True(t, view.Set)
		assert.Equal(t, "task-02", view.PausedTaskID)
		assert.Equal(t, 2, view.PausedWaveIndex, "wave 2 is the current incomplete wave once wave 1 passed")

		reloaded, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.NotNil(t, reloaded.ActiveCheckpoint)
		assert.Equal(t, "task-02", reloaded.ActiveCheckpoint.PausedTaskID)
		assert.Equal(t, 2, reloaded.ActiveCheckpoint.PausedWaveIndex)
	})
}

// Mid-wave with NO task evidence yet, a checkpoint on a wave-1 task is still
// settable (the wave-1 default holds), while a wave-2 task is correctly rejected
// as not in the current wave. This guards that issue #227a's evidence-derived
// path does not regress the no-evidence baseline.
func TestIssue227CheckpointWaveOneDefaultWithoutTaskEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, "L2", "issue227 checkpoint wave one default")
		issue227SeedTwoWaveExecution(t, root, slug)

		// wave-2 task rejected: wave 1 is still the current incomplete wave.
		rejectCmd := commandForRoot(t, root, makeCheckpointCmd())
		rejectCmd.SetArgs([]string{"--change", slug, "--task-id", "task-02"})
		err := rejectCmd.Execute()
		require.Error(t, err)
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "checkpoint_task_not_in_current_wave", cliErr.ErrorCode)

		// wave-1 task accepted.
		acceptCmd := commandForRoot(t, root, makeCheckpointCmd())
		acceptCmd.SetArgs([]string{"--json", "--change", slug, "--task-id", "task-01"})
		var buf bytes.Buffer
		acceptCmd.SetOut(&buf)
		acceptCmd.SetErr(&buf)
		require.NoError(t, acceptCmd.Execute(), "checkpoint output: %s", buf.String())

		var view checkpointView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.True(t, view.Set)
		assert.Equal(t, 1, view.PausedWaveIndex)
	})
}
