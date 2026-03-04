package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/signalridge/speclane/internal/bootstrap"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoRequiresActiveRequest(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		cmd := newDoCmd()
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no active request")
	})
}

func TestDoL1RunsAutoChecksAndBecomesDoneReady(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		requestID := singleRequestID(t, ".spln/runtime/admissions")

		var out bytes.Buffer
		doCmd := newDoCmd()
		doCmd.SetOut(&out)
		require.NoError(t, doCmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, true, payload["done_ready"])
		assert.Equal(t, "S8_VERIFY", payload["current_state"])

		admission, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.StateS8Verify, admission.CurrentState)
		assert.Equal(t, 1, admission.LatestFrozenRunSummaryVersion)
		assert.NotEmpty(t, admission.TaskRuns)
	})
}

func TestDoCheckpointRequiresResumeResponse(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		requestID, err := model.NewRequestID()
		require.NoError(t, err)
		admission := model.NewAdmissionState(requestID)
		admission.Level = model.LevelL1
		admission.LevelSource = model.LevelSourceUserSelected
		admission.CurrentState = model.StateS6RunWaves
		admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
		admission.LatestFrozenRunSummaryVersion = 1
		admission.TaskRuns = map[string]model.TaskRun{
			"task-1__rv1": {
				TaskID:            "task-1",
				RunSummaryVersion: 1,
				Verdict:           model.TaskVerdictFail,
				Blockers:          []string{"need-human-decision"},
			},
		}
		require.NoError(t, state.SaveAdmission(root, admission))

		cmd := newDoCmd()
		err = cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--resume-response")
	})
}

func TestDoCheckpointResumePersistsPayload(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		requestID, err := model.NewRequestID()
		require.NoError(t, err)
		admission := model.NewAdmissionState(requestID)
		admission.Level = model.LevelL1
		admission.LevelSource = model.LevelSourceUserSelected
		admission.CurrentState = model.StateS6RunWaves
		admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
		admission.LatestFrozenRunSummaryVersion = 1
		admission.TaskRuns = map[string]model.TaskRun{
			"task-1__rv1": {
				TaskID:            "task-1",
				RunSummaryVersion: 1,
				Verdict:           model.TaskVerdictBlocked,
				Blockers:          []string{"await approval"},
			},
		}
		require.NoError(t, state.SaveAdmission(root, admission))

		doCmd := newDoCmd()
		doCmd.SetArgs([]string{"--resume-response", "retry"})
		require.NoError(t, doCmd.Execute())

		updated, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		require.NotEmpty(t, updated.ActionHistory)

		last := updated.ActionHistory[len(updated.ActionHistory)-1]
		assert.Equal(t, "do", last.Action)
		assert.WithinDuration(t, time.Now().UTC(), last.Timestamp, 5*time.Second)
		assert.Equal(t, "retry", last.Details["user_response_payload"])
	})
}
