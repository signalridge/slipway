package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusProgressOmitsRunSummaryVersionWhenNoExecutionSummary asserts that an
// execution-state change with no recorded execution summary serializes its
// progress object WITHOUT a run_summary_version field. Zero is the "no summary
// yet" sentinel that `evidence task` rejects, so emitting run_summary_version=0
// misleads callers (issue #211). On the pre-change code the struct tag lacked
// `,omitempty`, so the marshaled progress included "run_summary_version":0 and
// this assertion was RED; the `,omitempty` tag makes it GREEN.
func TestStatusProgressOmitsRunSummaryVersionWhenNoExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("no-summary-run-version")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.Progress)
	require.Equal(t, 0, view.Progress.RunSummaryVersion, "fixture must have no recorded run summary")

	raw, err := json.Marshal(view.Progress)
	require.NoError(t, err)

	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &decoded))
	_, present := decoded["run_summary_version"]
	assert.False(t, present, "progress must omit run_summary_version when no execution summary is recorded; got %s", raw)
}

// TestStatusProgressReportsRunSummaryVersionWhenSummaryRecorded asserts that once
// a real execution summary at run_summary_version 1 exists, the progress object
// reports run_summary_version == 1 both on the view and in serialized JSON.
// This is the back-compat half of issue #211: omit-on-zero must not suppress a
// genuinely recorded version (any real version is >= 1).
func TestStatusProgressReportsRunSummaryVersionWhenSummaryRecorded(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	change := model.NewChange("recorded-run-version")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	require.NoError(t, state.SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "task-a",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"cmd/status.go"},
				CapturedAt:   time.Now().UTC(),
			},
		},
	}))

	view, err := buildStatusViewFromChange(root, change)
	require.NoError(t, err)
	require.NotNil(t, view.Progress)
	assert.Equal(t, 1, view.Progress.RunSummaryVersion)

	raw, err := json.Marshal(view.Progress)
	require.NoError(t, err)

	var decoded struct {
		RunSummaryVersion int `json:"run_summary_version"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, 1, decoded.RunSummaryVersion, "recorded run_summary_version must still serialize; got %s", raw)
}

// TestResumeCheckpointOmitsRunSummaryVersionWhenZero covers the `next --json`
// half of REQ-002 (issue #211): the resume checkpoint is the only
// run_summary_version emitter on the next surface, so it must omit the field when
// no execution summary has been recorded yet (the zero sentinel `evidence task`
// rejects) while still serializing a genuinely recorded version (>= 1).
func TestResumeCheckpointOmitsRunSummaryVersionWhenZero(t *testing.T) {
	t.Parallel()

	rawZero, err := json.Marshal(&resumeCheckpoint{
		CompletedTaskIDs: []string{"t-01"},
		Freshness:        "fresh",
	})
	require.NoError(t, err)
	var decodedZero map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rawZero, &decodedZero))
	_, present := decodedZero["run_summary_version"]
	assert.False(t, present, "resume checkpoint must omit run_summary_version when zero; got %s", rawZero)

	rawReal, err := json.Marshal(&resumeCheckpoint{
		RunSummaryVersion: 1,
		CompletedTaskIDs:  []string{"t-01"},
	})
	require.NoError(t, err)
	var decodedReal struct {
		RunSummaryVersion int `json:"run_summary_version"`
	}
	require.NoError(t, json.Unmarshal(rawReal, &decodedReal))
	assert.Equal(t, 1, decodedReal.RunSummaryVersion, "recorded run_summary_version must still serialize; got %s", rawReal)
}
