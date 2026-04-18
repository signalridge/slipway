package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAbortRequiresExecuteState(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		_ = createGovernedRequest(t, root, "L2", "abort wrong state")

		cmd := makeAbortCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		err := cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "abort_state_invalid", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "slipway cancel")
	})
}

func TestAbortRejectsUnexpectedArgs(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		_ = createGovernedRequest(t, root, "L2", "abort rejects unexpected args")

		cmd := makeAbortCmd()
		cmd.SetArgs([]string{"unexpected"})

		err := cmd.Execute()
		require.Error(t, err)
		assertUnexpectedArgError(t, err)
	})
}

func TestAbortOutsideExecuteDoesNotPreemptTrackedProcesses(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "abort should not preempt outside execute")

		cfg := model.DefaultConfig()
		cfg.Execution.CancelGracePeriodSeconds = 0
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		proc := exec.Command("sh", "-c", "trap '' INT; sleep 30")
		require.NoError(t, proc.Start())
		t.Cleanup(func() {
			if proc.ProcessState == nil {
				_ = proc.Process.Kill()
				_, _ = proc.Process.Wait()
			}
		})

		raw, err := json.Marshal([]int{proc.Process.Pid})
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(state.TaskPIDFilePath(root, slug)), 0o755))
		require.NoError(t, os.WriteFile(state.TaskPIDFilePath(root, slug), raw, 0o644))

		cmd := makeAbortCmd()
		cmd.SetArgs([]string{"--json", "--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		err = cmd.Execute()
		require.Error(t, err)

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "abort_state_invalid", cliErr.ErrorCode)
		assert.True(t, isPIDAlive(proc.Process.Pid), "abort outside execute must not signal tracked task processes")

		_, statErr := os.Stat(filepath.Join(state.ChangeDir(root, slug), "evidence", "tasks", "cancel"))
		assert.True(t, os.IsNotExist(statErr), "abort outside execute must not emit preemption evidence")
	})
}

func TestAbortClearsCheckpointAndPreservesActiveChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "abort preserve change")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		change.ActiveCheckpoint = &model.ActiveCheckpoint{
			PausedWaveIndex: 0,
			PausedTaskID:    "task-01",
			CheckpointType:  "human_verify",
		}
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeAbortCmd()
		cmd.SetArgs([]string{"--json"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		require.NoError(t, cmd.Execute())

		var view abortView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, governedExecutionMode, view.ExecutionMode)
		assert.Equal(t, string(model.ChangeStatusActive), view.Status)
		assert.Equal(t, string(model.StateS2Execute), view.CurrentState)

		active, err := state.FindActiveChange(root)
		require.NoError(t, err)
		assert.Equal(t, slug, active.Slug)

		after, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		assert.Nil(t, after.ActiveCheckpoint)
		assert.Equal(t, model.ChangeStatusActive, after.Status)
		assert.Equal(t, model.StateS2Execute, after.CurrentState)
		assert.False(t, after.InterruptedExecutionAt.IsZero())
	})
}

func TestAbortTextUsesRunWhenNoResumableWaveStateExists(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "abort text should suggest fresh run")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeAbortCmd()
		cmd.SetArgs([]string{"--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		require.NoError(t, cmd.Execute())

		assert.Contains(t, buf.String(), "`slipway run`")
		assert.NotContains(t, buf.String(), "`slipway run --resume`")
	})
}

func TestAbortTextUsesRunResumeWhenResumableWaveStateExists(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, "L2", "abort text should suggest resume")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		writePassingExecutionSummary(t, root, slug, 1, "task-01")
		bundlePath := filepath.Join(root, "artifacts", "changes", slug)
		require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`
- [x] `+"`task-01`"+` preserve completed wave
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-02`"+` continue incomplete wave
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/run.go"]
  - task_kind: code
`), 0o644))
		materializeWaveExecutionForSummary(t, root, slug)

		cmd := makeAbortCmd()
		cmd.SetArgs([]string{"--change", slug})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		require.NoError(t, cmd.Execute())

		assert.Contains(t, buf.String(), "`slipway run --resume`")
	})
}

func TestWritePreemptionEvidenceUsesActionSpecificPathAndPayload(t *testing.T) {
	root := t.TempDir()

	abortPath, err := writePreemptionEvidence(root, "change-abort", "abort", []int{101}, []int{202})
	require.NoError(t, err)
	assert.Contains(t, abortPath, filepath.Join("evidence", "tasks", "abort"))

	var abortPayload map[string]any
	rawAbort, err := os.ReadFile(abortPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(rawAbort, &abortPayload))
	assert.Equal(t, "abort", abortPayload["action"])
	assert.Equal(t, "abort", abortPayload["outcome"])

	cancelPath, err := writePreemptionEvidence(root, "change-cancel", "cancel", []int{303}, nil)
	require.NoError(t, err)
	assert.Contains(t, cancelPath, filepath.Join("evidence", "tasks", "cancel"))

	var cancelPayload map[string]any
	rawCancel, err := os.ReadFile(cancelPath)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(rawCancel, &cancelPayload))
	assert.Equal(t, "cancel", cancelPayload["action"])
	assert.Equal(t, "cancel", cancelPayload["outcome"])
}
