package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/speclane/internal/bootstrap"
	"github.com/signalridge/speclane/internal/model"
	"github.com/signalridge/speclane/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusDiagnosticsWhenNoActiveRequest(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		var out bytes.Buffer
		cmd := newStatusCmd()
		cmd.SetOut(&out)
		cmd.SetArgs(nil)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, "diagnostics", payload["lane_mode"])
		assert.Equal(t, "unknown", payload["evidence_freshness"])
	})
}

func TestStatusAdmissionOnlyView(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		var out bytes.Buffer
		cmd := newStatusCmd()
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, "admission_only", payload["lane_mode"])
		assert.Equal(t, "S6_RUN_WAVES", payload["current_state"])
	})
}

func TestContextDiagnosticsAndGovernedJSON(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		var diagOut bytes.Buffer
		diag := newContextCmd()
		diag.SetOut(&diagOut)
		diag.SetArgs([]string{"--format", "json"})
		require.NoError(t, diag.Execute())
		var diagPayload map[string]any
		require.NoError(t, json.Unmarshal(diagOut.Bytes(), &diagPayload))
		assert.Equal(t, "diagnostics", diagPayload["lane_mode"])
		assert.Equal(t, "unknown", diagPayload["evidence_freshness"])

		create := newNewCmd()
		create.SetArgs([]string{"--level", "L2", "refactor service modules"})
		require.NoError(t, create.Execute())

		var out bytes.Buffer
		cmd := newContextCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--format", "json"})
		require.NoError(t, cmd.Execute())
		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, "governed", payload["lane_mode"])
		assert.Equal(t, "S4_SPEC_BUNDLE", payload["current_state"])
	})
}

func TestRepairNormalizesSameRequestDualActive(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		requestID := mustRequestID(t)

		admission := model.NewAdmissionState(requestID)
		admission.Level = model.LevelL2
		admission.LevelSource = model.LevelSourceUserSelected
		admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
		require.NoError(t, state.SaveAdmission(root, admission))

		change := model.NewChangeState(requestID, "slug")
		change.Level = model.LevelL2
		change.LevelSource = model.LevelSourceUserSelected
		change.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		repair := newRepairCmd()
		repair.SetOut(&out)
		require.NoError(t, repair.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		assert.Equal(t, true, payload["dual_active_normalized"])

		loaded, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.AdmissionStatusSealedHandoff, loaded.AdmissionStatus)
	})
}

func TestRepairReportsDifferentRequestAmbiguity(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		for i := 0; i < 2; i++ {
			admission := model.NewAdmissionState(mustRequestID(t))
			admission.Level = model.LevelL1
			admission.LevelSource = model.LevelSourceUserSelected
			admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
			require.NoError(t, state.SaveAdmission(root, admission))
		}

		var out bytes.Buffer
		repair := newRepairCmd()
		repair.SetOut(&out)
		require.NoError(t, repair.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		findings, ok := payload["non_repairable_findings"].([]any)
		require.True(t, ok)
		assert.NotEmpty(t, findings)
	})
}

func TestRepairForwardsOrphanedGovernedAdmission(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		requestID := mustRequestID(t)

		admission := model.NewAdmissionState(requestID)
		admission.Level = model.LevelL3
		admission.LevelSource = model.LevelSourceUserSelected
		admission.CurrentState = model.StateS2Discover
		admission.RouteSnapshot = model.RouteSnapshot{Scores: model.Scores{}}
		require.NoError(t, state.SaveAdmission(root, admission))

		var out bytes.Buffer
		repair := newRepairCmd()
		repair.SetOut(&out)
		require.NoError(t, repair.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		list, ok := payload["governed_create_repairs"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, list)

		sealed, err := state.LoadAdmission(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.AdmissionStatusSealedHandoff, sealed.AdmissionStatus)
		assert.Equal(t, model.StateS1Analyze, sealed.CurrentState)

		change, err := state.LoadChange(root, requestID)
		require.NoError(t, err)
		assert.Equal(t, model.LevelL3, change.Level)
		assert.Equal(t, model.StateS2Discover, change.CurrentState)
		_, err = os.Stat(filepath.Join(root, "aircraft", "changes", change.Slug, "change.yaml"))
		require.NoError(t, err)
	})
}

func mustRequestID(t *testing.T) string {
	t.Helper()
	id, err := model.NewRequestID()
	require.NoError(t, err)
	return id
}

func TestContextTextFormat(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		create := newNewCmd()
		create.SetArgs([]string{"--level", "L1", "fix login timeout"})
		require.NoError(t, create.Execute())

		var out bytes.Buffer
		cmd := newContextCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--format", "text"})
		require.NoError(t, cmd.Execute())

		content := out.String()
		assert.Contains(t, content, "lane_mode:")
		assert.Contains(t, content, "current_state:")
	})
}

func TestContextIncludesWaveEnvelopeAtS6(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		requestID := mustRequestID(t)

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
				TaskKind:          model.TaskKindCode,
				Verdict:           model.TaskVerdictPass,
				TargetFiles:       []string{"a.go"},
				EvidenceRef:       "e1",
			},
		}
		require.NoError(t, state.SaveAdmission(root, admission))

		var out bytes.Buffer
		cmd := newContextCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--format", "json"})
		require.NoError(t, cmd.Execute())

		var payload map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &payload))
		envelope, ok := payload["wave_envelope"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "task-1", envelope["task_id"])
		assert.Equal(t, "rv1", envelope["wave_id"])
		_, hasDependsOn := envelope["depends_on"]
		assert.True(t, hasDependsOn)
		_, hasMustHaves := envelope["must_haves"]
		assert.True(t, hasMustHaves)
	})
}

func TestContextIncludesCheckpointResumeBundle(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		requestID := mustRequestID(t)

		continuationPath := filepath.Join(root, ".spln", "evidence", "tasks", requestID, "continuation", "resume.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(continuationPath), 0o755))
		payload := map[string]any{
			"resume_signal":         "checkpoint_response_required",
			"user_response_payload": "approved",
			"blockers":              []string{"checkpoint_waiting_for_operator"},
		}
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(continuationPath, raw, 0o644))

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
				TaskKind:          model.TaskKindCode,
				Verdict:           model.TaskVerdictBlocked,
				TargetFiles:       []string{"a.go"},
				EvidenceRef:       filepath.Join(root, ".spln", "evidence", "tasks", requestID, "rv1", "task-1.json"),
				Blockers:          []string{"checkpoint_waiting_for_operator"},
			},
		}
		admission.ActionHistory = append(admission.ActionHistory, model.ActionEvent{
			Action:    "do",
			State:     model.StateS6RunWaves,
			Timestamp: time.Now().UTC(),
			Details: map[string]string{
				"continuation_evidence_ref": continuationPath,
				"resume_signal":             "checkpoint_response_required",
				"user_response_payload":     "approved",
			},
		})
		require.NoError(t, state.SaveAdmission(root, admission))

		var out bytes.Buffer
		cmd := newContextCmd()
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--format", "json"})
		require.NoError(t, cmd.Execute())

		var response map[string]any
		require.NoError(t, json.Unmarshal(out.Bytes(), &response))
		checkpoint, ok := response["checkpoint_resume"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "rv1", checkpoint["prior_run_id"])
		assert.Equal(t, "task-1", checkpoint["paused_task_id"])
		assert.Equal(t, "checkpoint_response_required", checkpoint["checkpoint_type"])
		assert.Equal(t, "approved", checkpoint["user_response_payload"])
	})
}

func TestRepairCollectRequestIDsDeterministic(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".spln", "runtime", "admissions"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".spln", "runtime", "changes"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".spln", "runtime", "admissions", "b.yaml"), []byte("x"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".spln", "runtime", "changes", "a.yaml"), []byte("x"), 0o644))
	assert.Equal(t, []string{"a", "b"}, collectRequestIDs(root))
}
