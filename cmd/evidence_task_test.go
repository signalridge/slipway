package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvidenceTaskRecordsRuntimeEvidenceAndBuildsExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		capturedAt := time.Now().UTC()
		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:evidence-task",
			"--changed-file", "cmd/evidence.go",
			"--target-file", "cmd/evidence.go",
			"--captured-at", capturedAt.Format(time.RFC3339Nano),
			"--session-id", "session-a",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceTaskView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		wavePlan, err := state.LoadWavePlanForChange(root, change)
		require.NoError(t, err)
		expectedFreshnessInputs := state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01", wavePlan.TasksPlanHash)
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, "t-01", view.TaskID)
		assert.True(t, view.Recorded)
		assert.True(t, view.FreshnessInputs.Equal(expectedFreshnessInputs))

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		raw, err := os.ReadFile(taskEvidencePath)
		require.NoError(t, err)
		var payload map[string]any
		require.NoError(t, json.Unmarshal(raw, &payload))
		assert.NotContains(t, payload, "input_hash")
		assert.Contains(t, view.Path, ".git/slipway/runtime/changes/"+slug+"/evidence/tasks/t-01.json")

		task, parsedAt, sessionID, err := progression.ParseTaskEvidence(root, taskEvidencePath, 1)
		require.NoError(t, err)
		assert.Equal(t, "t-01", task.TaskID)
		assert.Equal(t, model.TaskVerdictPass, task.Verdict)
		assert.Equal(t, model.TaskKindVerification, task.TaskKind)
		assert.Equal(t, []string{"cmd/evidence.go"}, task.ChangedFiles)
		assert.Equal(t, []string{"cmd/evidence.go"}, task.TargetFiles)
		assert.True(t, capturedAt.Equal(parsedAt))
		assert.Equal(t, "session-a", sessionID)
		assert.True(t, task.FreshnessInputs.Equal(expectedFreshnessInputs))

		writeSkillVerification(t, root, slug, progression.SkillWaveOrchestration, model.VerificationRecord{
			Verdict:    model.VerificationVerdictPass,
			Blockers:   []model.ReasonCode{},
			Timestamp:  capturedAt.Add(time.Second),
			RunVersion: 1,
			References: []string{"task:evidence:t-01"},
		})
		_, err = buildNextView(root, changeRef{Slug: slug}, "", false, true, false)
		require.NoError(t, err)

		summary, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		require.Len(t, summary.Tasks, 1)
		assert.Equal(t, "t-01", summary.Tasks[0].TaskID)
		assert.Equal(t, model.TaskVerdictPass, summary.Tasks[0].Verdict)
		assert.True(t, summary.Tasks[0].FreshnessInputs.Equal(expectedFreshnessInputs))
	})
}

func TestEvidenceTaskRejectsUnsafeTaskID(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "../escape",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:unsafe",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_id_invalid", cliErr.ErrorCode)
	})
}

func TestEvidenceTaskRejectsInvalidVerdictWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "maybe",
			"--evidence-ref", "test:invalid-verdict",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_verdict_invalid", cliErr.ErrorCode)

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, statErr := os.Stat(taskEvidencePath)
		require.Error(t, statErr)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestEvidenceTaskRejectsFutureCapturedAtWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:future-captured-at",
			"--captured-at", time.Now().UTC().Add(30 * time.Second).Format(time.RFC3339Nano),
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_captured_at_invalid", cliErr.ErrorCode)

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, statErr := os.Stat(taskEvidencePath)
		require.Error(t, statErr)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestEvidenceTaskRejectsRunVersionMismatchWhenWaveEvidenceExists(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		writePassingWaveEvidence(t, root, slug, 2)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wrong-run-version",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_run_summary_version_mismatch", cliErr.ErrorCode)
		assert.Equal(t, 2, cliErr.Details["expected"])
		assert.Equal(t, 1, cliErr.Details["got"])

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		_, statErr := os.Stat(taskEvidencePath)
		require.Error(t, statErr)
		assert.True(t, os.IsNotExist(statErr))
	})
}

func TestEvidenceTaskRejectsNonWorkspaceRelativePathsWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		flag      string
		value     string
		errorCode string
	}{
		{
			name:      "changed file parent traversal",
			flag:      "--changed-file",
			value:     "../escape.go",
			errorCode: "evidence_task_changed_file_invalid",
		},
		{
			name:      "target file absolute path",
			flag:      "--target-file",
			value:     "/tmp/escape.go",
			errorCode: "evidence_task_target_file_invalid",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			withCommandWorkspace(t, root, func() {
				initTestWorkspace(t, root)
				slug, _ := createEvidenceTaskFixture(t, root)

				cmd := commandForRoot(t, root, makeEvidenceCmd())
				cmd.SetArgs([]string{
					"task",
					"--task-id", "t-01",
					"--run-summary-version", "1",
					"--task-kind", "verification",
					"--verdict", "pass",
					"--evidence-ref", "test:invalid-path",
					tt.flag, tt.value,
				})
				err := cmd.Execute()
				cliErr := asCLIError(err)
				require.NotNil(t, cliErr)
				assert.Equal(t, tt.errorCode, cliErr.ErrorCode)

				taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
				_, statErr := os.Stat(taskEvidencePath)
				require.Error(t, statErr)
				assert.True(t, os.IsNotExist(statErr))
			})
		})
	}
}

func createEvidenceTaskFixture(t *testing.T, root string) (string, model.Change) {
	t.Helper()

	slug := createGovernedRequest(t, root, "L2", "evidence task command")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	return slug, change
}
