package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/signalridge/slipway/internal/bootstrap"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckpointSetsActiveCheckpoint(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "test checkpoint")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeCheckpointCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"--json", "--task-id", "task-01", "--type", "human_verify"})
		require.NoError(t, cmd.Execute())

		var view checkpointView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.True(t, view.Set)
		assert.Equal(t, "task-01", view.PausedTaskID)
		assert.Equal(t, "human_verify", view.CheckpointType)

		// Verify checkpoint persisted on change state
		change, err = state.LoadChange(root, slug)
		require.NoError(t, err)
		require.NotNil(t, change.ActiveCheckpoint)
		assert.Equal(t, "task-01", change.ActiveCheckpoint.PausedTaskID)
		assert.Equal(t, "human_verify", change.ActiveCheckpoint.CheckpointType)
	})
}

func TestCheckpointDecisionWithAllowedResponses(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "test decision checkpoint")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeCheckpointCmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{
			"--json",
			"--task-id", "task-02",
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
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		_ = createGovernedRequest(t, root, "L2", "test wrong state")

		cmd := makeCheckpointCmd()
		cmd.SetArgs([]string{"--task-id", "task-01"})
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "S2_EXECUTE")
	})
}

func TestCheckpointRejectsDuplicate(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "test duplicate checkpoint")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
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
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "test missing task id")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
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
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		slug := createGovernedRequest(t, root, "L2", "test decision no responses")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)

		change.CurrentState = model.StateS2Execute
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		cmd := makeCheckpointCmd()
		cmd.SetArgs([]string{"--task-id", "task-01", "--type", "decision"})
		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allowed_responses")
	})
}
