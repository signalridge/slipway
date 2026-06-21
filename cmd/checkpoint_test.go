package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckpointSetsActiveCheckpoint(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "test checkpoint")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		taskID := materializeWavePlanCheckpointTask(t, root, change)

		cmd := makeCheckpointCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--json", "--task-id", taskID, "--type", "human_verify"})
		require.NoError(t, cmd.Execute())

		var view checkpointView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.True(t, view.Set)
		assert.Equal(t, taskID, view.PausedTaskID)
		assert.Equal(t, "human_verify", view.CheckpointType)

		// Verify checkpoint persisted on change state
		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		require.NotNil(t, change.ActiveCheckpoint)
		assert.Equal(t, taskID, change.ActiveCheckpoint.PausedTaskID)
		assert.Equal(t, "human_verify", change.ActiveCheckpoint.CheckpointType)
	})
}

func TestCheckpointDecisionWithAllowedResponses(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "test decision checkpoint")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		taskID := materializeWavePlanCheckpointTask(t, root, change)

		cmd := makeCheckpointCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{
			"--json",
			"--task-id", taskID,
			"--type", "decision",
			"--allowed-responses", "approve,reject,defer",
		})
		require.NoError(t, cmd.Execute())

		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		require.NotNil(t, change.ActiveCheckpoint)
		assert.Equal(t, "decision", change.ActiveCheckpoint.CheckpointType)
		assert.Equal(t, []string{"approve", "reject", "defer"}, change.ActiveCheckpoint.AllowedResponses)
	})
}

func TestCheckpointRejectsWrongState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		_ = createGovernedRequest(t, root, levelNonDiscovery, "test wrong state")

		cmd := makeCheckpointCmd()
		cmd.SetArgs([]string{"--task-id", "task-01"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "S2_IMPLEMENT")
	})
}

func TestCheckpointRejectsDuplicate(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "test duplicate checkpoint")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedTaskID:   "task-existing",
			CheckpointType: "human_verify",
		}
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeCheckpointCmd()
		cmd.SetArgs([]string{"--task-id", "task-new"})
		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already active")
	})
}

func TestCheckpointRequiresTaskID(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "test missing task id")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeCheckpointCmd()
		cmd.SetArgs([]string{"--json"})
		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--task-id")
		var cliErr *CLIError
		require.ErrorAs(t, err, &cliErr)
		assert.Equal(t, categoryInvalidUsage, cliErr.Category)
		assert.Equal(t, exitCodeInvalidUsage, cliErr.ExitCode)
		assert.Equal(t, "checkpoint_task_id_required", cliErr.ErrorCode)
	})
}

func TestCheckpointDecisionRequiresAllowedResponses(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "test decision no responses")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))
		taskID := materializeWavePlanCheckpointTask(t, root, change)

		cmd := makeCheckpointCmd()
		cmd.SetArgs([]string{"--task-id", taskID, "--type", "decision"})
		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allowed_responses")
	})
}

func TestCheckpointRejectsTaskOutsideCurrentWave(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "test checkpoint current wave enforcement")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [ ] `+"`task-01`"+` first wave task
  - depends_on: []
  - target_files: ["cmd/checkpoint.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` second wave task
  - depends_on: ["task-01"]
  - target_files: ["cmd/checkpoint.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)

		cmd := makeCheckpointCmd()
		cmd.SetArgs([]string{"--task-id", "task-02"})
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "checkpoint_task_not_in_current_wave", cliErr.ErrorCode)
	})
}

func TestCheckpointRejectsWhenWaveRunsAreMissing(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "checkpoint should fail closed when wave runs are missing")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`
- [x] `+"`task-01`"+` completed first wave
  - depends_on: []
  - target_files: ["cmd/checkpoint.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` pending second wave
  - depends_on: ["task-01"]
  - target_files: ["cmd/checkpoint.go"]
  - task_kind: code
`)))
		_, err = state.MaterializeWavePlan(root, change)
		require.NoError(t, err)
		writePassingExecutionSummary(t, root, slug, 1, "task-01")

		cmd := makeCheckpointCmd()
		cmd.SetArgs([]string{"--task-id", "task-02"})
		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "wave_runs_missing", cliErr.ErrorCode)
		assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	})
}

func materializeWavePlanCheckpointTask(t *testing.T, root string, change model.Change) string {
	t.Helper()

	plan, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	require.NotEmpty(t, plan.Waves)
	require.NotEmpty(t, plan.Waves[0].Tasks)
	return plan.Waves[0].Tasks[0].TaskID
}

// seedTwoWaveExecution seeds an S2_IMPLEMENT change with a two-wave plan
// (task-01 in wave 1, task-02 in wave 2) and no materialized run summary.
func seedTwoWaveExecution(t *testing.T, root, slug string) {
	t.Helper()
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
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
// completed wave 1, so wave 2 is the current incomplete wave.
//
// issue #227
func TestCheckpointSettableAtWaveBoundaryBeforeSkillEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "checkpoint wave boundary")
		seedTwoWaveExecution(t, root, slug)

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
// as not in the current wave. This guards that the evidence-derived path does not
// regress the no-evidence baseline.
//
// issue #227
func TestCheckpointWaveOneDefaultWithoutTaskEvidence(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "checkpoint wave one default")
		seedTwoWaveExecution(t, root, slug)

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
