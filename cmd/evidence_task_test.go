package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
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
			// Must stay within the fixture wave-plan's target_files
			// (cmd/lifecycle_commands_test.go) so the Scope Contract passes; an
			// out-of-scope changed_file now resolves through the scope-contract
			// repair gate rather than a backward lifecycle mutation.
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
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
		require.NotNil(t, view.InvocationRoute)
		assert.Equal(t, "unbound_active", view.InvocationRoute.Kind)
		assert.Equal(t, slug, view.InvocationRoute.ChangeSlug)
		assert.True(t, view.InvocationRoute.LocalLifecycleExecutionAllowed)
		assert.True(t, view.InvocationRoute.EffectiveLifecycleExecutionAllowed)
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
		assert.Equal(t, []string{"cmd/lifecycle_commands_test.go"}, task.ChangedFiles)
		assert.Equal(t, []string{"cmd/lifecycle_commands_test.go"}, task.TargetFiles)
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
		_, err = buildNextViewForCommand(root, changeRef{Slug: slug}, nextViewOptions{AutoSkipEvidence: true, Command: "run"})
		require.NoError(t, err)

		summary, err := state.LoadExecutionSummary(root, slug)
		require.NoError(t, err)
		require.Len(t, summary.Tasks, 1)
		assert.Equal(t, "t-01", summary.Tasks[0].TaskID)
		assert.Equal(t, model.TaskVerdictPass, summary.Tasks[0].Verdict)
		assert.True(t, summary.Tasks[0].FreshnessInputs.Equal(expectedFreshnessInputs))
	})
}

func TestEvidenceTaskImportsResultFileWithDerivedLedgerFields(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		resultPath := filepath.Join(root, "task-result.json")
		rawResult, err := json.Marshal(map[string]any{
			"task_id":       "t-01",
			"verdict":       "pass",
			"evidence_ref":  "test:result-file",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"blockers":      []string{},
			"session_id":    "executor-a",
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(resultPath, rawResult, 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--result-file", "task-result.json",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceTaskView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, "t-01", view.TaskID)
		assert.Equal(t, 1, view.RunSummaryVersion)
		assert.True(t, view.Recorded)
		require.NotNil(t, view.InvocationRoute)
		assert.Equal(t, "unbound_active", view.InvocationRoute.Kind)
		assert.Equal(t, slug, view.InvocationRoute.ChangeSlug)

		wavePlan, err := state.LoadWavePlanForChange(root, change)
		require.NoError(t, err)
		expectedFreshnessInputs := state.ExpectedExecutionTaskFreshnessInputs(change, 1, "t-01", wavePlan.TasksPlanHash)
		assert.True(t, view.FreshnessInputs.Equal(expectedFreshnessInputs))

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		task, capturedAt, sessionID, err := progression.ParseTaskEvidence(root, taskEvidencePath, 1)
		require.NoError(t, err)
		assert.Equal(t, model.TaskKindVerification, task.TaskKind)
		assert.Equal(t, []string{"cmd/lifecycle_commands_test.go"}, task.TargetFiles)
		assert.Equal(t, []string{"cmd/lifecycle_commands_test.go"}, task.ChangedFiles)
		assert.False(t, capturedAt.IsZero())
		assert.Equal(t, "executor-a", sessionID)
		assert.True(t, task.FreshnessInputs.Equal(expectedFreshnessInputs))
	})
}

func TestEvidenceTaskImportsMultipleResultFilesAtomically(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createMultiTaskEvidenceTaskFixture(t, root)
		writeTaskResultFile(t, filepath.Join(root, "task-result-t-01.json"), "t-01", "test:batch-t-01", "cmd/evidence.go")
		writeTaskResultFile(t, filepath.Join(root, "task-result-t-02.json"), "t-02", "test:batch-t-02", "cmd/evidence_task_test.go")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--result-file", "task-result-t-01.json",
			"--result-file", "task-result-t-02.json",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceTaskBatchView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, 1, view.RunSummaryVersion)
		assert.True(t, view.Recorded)
		require.NotNil(t, view.InvocationRoute)
		assert.Equal(t, "unbound_active", view.InvocationRoute.Kind)
		assert.Equal(t, slug, view.InvocationRoute.ChangeSlug)
		assert.Equal(t, 2, view.RecordedCount)
		require.Len(t, view.Tasks, 2)
		assert.Equal(t, "t-01", view.Tasks[0].TaskID)
		assert.Equal(t, "t-02", view.Tasks[1].TaskID)
		for _, task := range view.Tasks {
			require.NotNil(t, task.InvocationRoute)
			assert.Equal(t, view.InvocationRoute, task.InvocationRoute)
		}

		wavePlan, err := state.LoadWavePlanForChange(root, change)
		require.NoError(t, err)
		for _, expected := range []struct {
			taskID      string
			evidenceRef string
			changedFile string
		}{
			{taskID: "t-01", evidenceRef: "test:batch-t-01", changedFile: "cmd/evidence.go"},
			{taskID: "t-02", evidenceRef: "test:batch-t-02", changedFile: "cmd/evidence_task_test.go"},
		} {
			taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), expected.taskID+".json")
			task, _, _, err := progression.ParseTaskEvidence(root, taskEvidencePath, 1)
			require.NoError(t, err)
			assert.Equal(t, expected.evidenceRef, task.EvidenceRef)
			assert.Equal(t, []string{expected.changedFile}, task.ChangedFiles)
			assert.True(t, task.FreshnessInputs.Equal(state.ExpectedExecutionTaskFreshnessInputs(change, 1, expected.taskID, wavePlan.TasksPlanHash)))
		}
	})
}

func TestEvidenceTaskResultFileBatchLifecycleEventRecordsMixedVerdict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createMultiTaskEvidenceTaskFixture(t, root)
		writeTaskResultFile(t, filepath.Join(root, "task-result-t-01.json"), "t-01", "test:batch-t-01", "cmd/evidence.go")
		writeTaskResultFileWithVerdict(
			t,
			filepath.Join(root, "task-result-t-02.json"),
			"t-02",
			"blocked",
			"test:batch-t-02",
			"cmd/evidence_task_test.go",
			[]string{"blocked:review-finding"},
		)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--result-file", "task-result-t-01.json",
			"--result-file", "task-result-t-02.json",
		})
		require.NoError(t, cmd.Execute())

		events, err := state.ReadLifecycleEvents(root, change)
		require.NoError(t, err)
		require.NotEmpty(t, events)
		event := events[len(events)-1]
		assert.Equal(t, "task_evidence.recorded", event.EventType)
		assert.Equal(t, "mixed", event.Result)
		assert.Contains(t, event.Diagnostics, "task_ids=t-01,t-02")
		assert.Contains(t, event.Diagnostics, "task_verdicts=t-01:pass,t-02:blocked")
		assert.Contains(t, event.Diagnostics, "run_summary_version=1")
		assertTaskEvidenceWritten(t, root, slug, "t-01")
		assertTaskEvidenceWritten(t, root, slug, "t-02")
	})
}

func TestEvidenceTaskResultFileBatchRejectsDuplicateTaskWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createMultiTaskEvidenceTaskFixture(t, root)
		writeTaskResultFile(t, filepath.Join(root, "task-result-a.json"), "t-01", "test:batch-a", "cmd/evidence.go")
		writeTaskResultFile(t, filepath.Join(root, "task-result-b.json"), "t-01", "test:batch-b", "cmd/evidence_task_test.go")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--result-file", "task-result-a.json",
			"--result-file", "task-result-b.json",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_result_file_duplicate_task", cliErr.ErrorCode)
		assertTaskEvidenceNotWritten(t, root, slug, "t-01")
	})
}

func TestEvidenceTaskResultFileBatchRejectsDuplicateSessionWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createMultiTaskEvidenceTaskFixture(t, root)
		writeTaskResultFileWithSessionID(t, filepath.Join(root, "task-result-t-01.json"), "t-01", "test:batch-t-01", "cmd/evidence.go", "executor-a")
		writeTaskResultFileWithSessionID(t, filepath.Join(root, "task-result-t-02.json"), "t-02", "test:batch-t-02", "cmd/evidence_task_test.go", "executor-a")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--result-file", "task-result-t-01.json",
			"--result-file", "task-result-t-02.json",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_result_file_duplicate_session", cliErr.ErrorCode)
		assert.Equal(t, "executor-a", cliErr.Details["session_id"])
		assertTaskEvidenceNotWritten(t, root, slug, "t-01")
		assertTaskEvidenceNotWritten(t, root, slug, "t-02")
	})
}

func TestEvidenceTaskResultFileRejectsDuplicateSessionFromExistingRunEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createMultiTaskEvidenceTaskFixture(t, root)
		writeTaskResultFileWithSessionID(t, filepath.Join(root, "task-result-t-01.json"), "t-01", "test:initial-t-01", "cmd/evidence.go", "executor-a")

		firstCmd := commandForRoot(t, root, makeEvidenceCmd())
		firstCmd.SetArgs([]string{"task", "--json", "--result-file", "task-result-t-01.json"})
		firstCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, firstCmd.Execute())

		writeTaskResultFileWithSessionID(t, filepath.Join(root, "task-result-t-02.json"), "t-02", "test:duplicate-session-t-02", "cmd/evidence_task_test.go", "executor-a")
		secondCmd := commandForRoot(t, root, makeEvidenceCmd())
		secondCmd.SetArgs([]string{"task", "--json", "--result-file", "task-result-t-02.json"})
		cliErr := asCLIError(secondCmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_result_file_duplicate_session", cliErr.ErrorCode)
		assert.Equal(t, "executor-a", cliErr.Details["session_id"])
		assert.Equal(t, "t-01", cliErr.Details["first_task_id"])
		assert.Equal(t, "t-02", cliErr.Details["duplicate_task_id"])
		assertTaskEvidenceWritten(t, root, slug, "t-01")
		assertTaskEvidenceNotWritten(t, root, slug, "t-02")
	})
}

func TestEvidenceTaskResultFileBatchInvalidMemberWritesNoEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createMultiTaskEvidenceTaskFixture(t, root)
		writeTaskResultFile(t, filepath.Join(root, "task-result-t-01.json"), "t-01", "test:batch-t-01", "cmd/evidence.go")
		require.NoError(t, os.WriteFile(filepath.Join(root, "task-result-t-02.json"), []byte(`{"task_id":"t-02","verdict":"pass","evidence_ref":"test:missing-changed-files","changed_files":[]}`), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--result-file", "task-result-t-01.json",
			"--result-file", "task-result-t-02.json",
		})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_changed_file_required", cliErr.ErrorCode)
		assertTaskEvidenceNotWritten(t, root, slug, "t-01")
		assertTaskEvidenceNotWritten(t, root, slug, "t-02")
	})
}

func TestEvidenceTaskResultFileRejectsExecutorOwnedLedgerFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		value any
	}{
		{name: "run summary version", field: "run_summary_version", value: 1},
		{name: "task kind", field: "task_kind", value: "verification"},
		{name: "target files", field: "target_files", value: []string{"cmd/lifecycle_commands_test.go"}},
		{name: "captured at", field: "captured_at", value: time.Now().UTC().Format(time.RFC3339Nano)},
		{name: "freshness inputs", field: "freshness_inputs", value: map[string]any{"task_id": "t-01"}},
		{name: "input hash", field: "input_hash", value: "legacy"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			withCommandWorkspace(t, root, func() {
				initTestWorkspace(t, root)
				slug, _ := createEvidenceTaskFixture(t, root)

				payload := map[string]any{
					"task_id":       "t-01",
					"verdict":       "pass",
					"evidence_ref":  "test:forbidden-ledger-field",
					"changed_files": []string{"cmd/lifecycle_commands_test.go"},
				}
				payload[tt.field] = tt.value
				rawResult, err := json.Marshal(payload)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(root, "task-result.json"), rawResult, 0o644))

				cmd := commandForRoot(t, root, makeEvidenceCmd())
				cmd.SetArgs([]string{"task", "--json", "--result-file", "task-result.json"})
				cliErr := asCLIError(cmd.Execute())
				require.NotNil(t, cliErr)
				assert.Equal(t, "evidence_task_result_file_ledger_field", cliErr.ErrorCode)
				assert.Equal(t, tt.field, cliErr.Details["field"])

				taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
				_, statErr := os.Stat(taskEvidencePath)
				require.Error(t, statErr)
				assert.True(t, os.IsNotExist(statErr))
			})
		})
	}
}

func TestEvidenceTaskResultFileRejectsManualLedgerFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		flag string
	}{
		{
			name: "task id",
			args: []string{"--task-id", "t-01"},
			flag: "--task-id",
		},
		{
			name: "run summary version",
			args: []string{"--run-summary-version", "1"},
			flag: "--run-summary-version",
		},
		{
			name: "task kind",
			args: []string{"--task-kind", "verification"},
			flag: "--task-kind",
		},
		{
			name: "verdict",
			args: []string{"--verdict", "pass"},
			flag: "--verdict",
		},
		{
			name: "evidence ref",
			args: []string{"--evidence-ref", "test:manual"},
			flag: "--evidence-ref",
		},
		{
			name: "target file",
			args: []string{"--target-file", "cmd/lifecycle_commands_test.go"},
			flag: "--target-file",
		},
		{
			name: "blocker",
			args: []string{"--blocker", "blocked:test"},
			flag: "--blocker",
		},
		{
			name: "captured at",
			args: []string{"--captured-at", time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)},
			flag: "--captured-at",
		},
		{
			name: "changed file",
			args: []string{"--changed-file", "cmd/lifecycle_commands_test.go"},
			flag: "--changed-file",
		},
		{
			name: "session id",
			args: []string{"--session-id", "manual-session"},
			flag: "--session-id",
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

				rawResult, err := json.Marshal(map[string]any{
					"task_id":       "t-01",
					"verdict":       "pass",
					"evidence_ref":  "test:mixed-mode",
					"changed_files": []string{"cmd/lifecycle_commands_test.go"},
				})
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(root, "task-result.json"), rawResult, 0o644))

				args := []string{"task", "--json", "--result-file", "task-result.json"}
				args = append(args, tt.args...)
				cmd := commandForRoot(t, root, makeEvidenceCmd())
				cmd.SetArgs(args)

				cliErr := asCLIError(cmd.Execute())
				require.NotNil(t, cliErr)
				assert.Equal(t, "evidence_task_result_file_mixed_mode", cliErr.ErrorCode)
				assert.Equal(t, tt.flag, cliErr.Details["flag"])

				taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
				_, statErr := os.Stat(taskEvidencePath)
				require.Error(t, statErr)
				assert.True(t, os.IsNotExist(statErr))
			})
		})
	}
}

func TestEvidenceTaskResultFileRejectsUnsafeResultPathWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupPath func(t *testing.T, root string) string
		errorCode string
	}{
		{
			name: "absolute path",
			setupPath: func(t *testing.T, root string) string {
				t.Helper()
				resultPath := filepath.Join(root, "task-result.json")
				writeTaskResultFile(t, resultPath, "t-01", "test:absolute-result-file", "cmd/lifecycle_commands_test.go")
				return resultPath
			},
			errorCode: "evidence_task_result_file_path_invalid",
		},
		{
			name: "symlink escape",
			setupPath: func(t *testing.T, root string) string {
				t.Helper()
				outside := t.TempDir()
				outsidePath := filepath.Join(outside, "task-result.json")
				writeTaskResultFile(t, outsidePath, "t-01", "test:symlink-result-file", "cmd/lifecycle_commands_test.go")
				linkPath := filepath.Join(root, "task-result-link.json")
				if err := os.Symlink(outsidePath, linkPath); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
				return "task-result-link.json"
			},
			errorCode: "evidence_task_result_file_path_invalid",
		},
		{
			name: "oversized file",
			setupPath: func(t *testing.T, root string) string {
				t.Helper()
				resultPath := filepath.Join(root, "task-result.json")
				require.NoError(t, os.WriteFile(resultPath, bytes.Repeat([]byte("x"), int(maxEvidenceTaskResultFileBytes)+1), 0o644))
				return "task-result.json"
			},
			errorCode: "evidence_task_result_file_too_large",
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
				resultPath := tt.setupPath(t, root)

				cmd := commandForRoot(t, root, makeEvidenceCmd())
				cmd.SetArgs([]string{"task", "--json", "--result-file", resultPath})
				cliErr := asCLIError(cmd.Execute())
				require.NotNil(t, cliErr)
				assert.Equal(t, tt.errorCode, cliErr.ErrorCode)
				assertTaskEvidenceNotWritten(t, root, slug, "t-01")
			})
		})
	}
}

func TestEvidenceTaskResultFileRejectsInvalidPayloadsWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		payload   string
		errorCode string
	}{
		{
			name:      "malformed json",
			payload:   `{"task_id":`,
			errorCode: "evidence_task_result_file_invalid",
		},
		{
			name:      "invalid verdict",
			payload:   `{"task_id":"t-01","verdict":"maybe","evidence_ref":"test:invalid-verdict","changed_files":["cmd/lifecycle_commands_test.go"]}`,
			errorCode: "evidence_task_verdict_invalid",
		},
		{
			name:      "unknown task id",
			payload:   `{"task_id":"t-missing","verdict":"pass","evidence_ref":"test:unknown-task","changed_files":["cmd/lifecycle_commands_test.go"]}`,
			errorCode: "evidence_task_unknown",
		},
		{
			name:      "empty changed files",
			payload:   `{"task_id":"t-01","verdict":"pass","evidence_ref":"test:empty-changed-files","changed_files":[]}`,
			errorCode: "evidence_task_changed_file_required",
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
				require.NoError(t, os.WriteFile(filepath.Join(root, "task-result.json"), []byte(tt.payload), 0o644))

				cmd := commandForRoot(t, root, makeEvidenceCmd())
				cmd.SetArgs([]string{"task", "--json", "--result-file", "task-result.json"})
				cliErr := asCLIError(cmd.Execute())
				require.NotNil(t, cliErr)
				assert.Equal(t, tt.errorCode, cliErr.ErrorCode)
				assertTaskEvidenceNotWritten(t, root, slug, "t-01")
				assertTaskEvidenceNotWritten(t, root, slug, "t-missing")
			})
		})
	}
}

func TestEvidenceTaskResultFileRejectsMixedExistingRunVersions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		dir := state.EvidenceTasksDir(root, slug)
		require.NoError(t, os.MkdirAll(dir, 0o755))

		v1 := map[string]any{
			"task_id":             "t-01",
			"run_summary_version": 1,
			"task_kind":           "verification",
			"verdict":             "pass",
			"changed_files":       []string{"cmd/lifecycle_commands_test.go"},
			"target_files":        []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":        "test:v1",
			"captured_at":         time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
			"freshness_inputs":    map[string]any{"change_id": slug, "run_summary_version": 1, "task_id": "t-01"},
		}
		rawV1, err := json.Marshal(v1)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "t-01.json"), rawV1, 0o644))

		v2 := map[string]any{
			"task_id":             "t-02",
			"run_summary_version": 2,
			"task_kind":           "verification",
			"verdict":             "pass",
			"changed_files":       []string{"cmd/lifecycle_commands_test.go"},
			"target_files":        []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":        "test:v2",
			"captured_at":         time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
			"freshness_inputs":    map[string]any{"change_id": slug, "run_summary_version": 2, "task_id": "t-02"},
		}
		rawV2, err := json.Marshal(v2)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "t-02.json"), rawV2, 0o644))

		rawResult, err := json.Marshal(map[string]any{
			"task_id":       "t-01",
			"verdict":       "pass",
			"evidence_ref":  "test:mixed-version",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, "task-result.json"), rawResult, 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{"task", "--json", "--result-file", "task-result.json"})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_run_summary_version_ambiguous", cliErr.ErrorCode)
	})
}

func TestEvidenceTaskResultFileRejectsOlderAndActiveExistingRunVersionsWhenActiveRunIsTwo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		_, err := state.MaterializeWavePlanAtRunSummaryVersion(root, change, time.Now().UTC(), 2)
		require.NoError(t, err)

		dir := state.EvidenceTasksDir(root, slug)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		for _, existing := range []struct {
			taskID string
			run    int
		}{
			{taskID: "t-01", run: 1},
			{taskID: "t-02", run: 2},
		} {
			raw, err := json.Marshal(map[string]any{
				"task_id":             existing.taskID,
				"run_summary_version": existing.run,
				"task_kind":           "verification",
				"verdict":             "pass",
				"changed_files":       []string{"cmd/lifecycle_commands_test.go"},
				"target_files":        []string{"cmd/lifecycle_commands_test.go"},
				"evidence_ref":        "test:mixed-active-run-two",
				"captured_at":         time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
				"freshness_inputs":    map[string]any{"change_id": slug, "run_summary_version": existing.run, "task_id": existing.taskID},
			})
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(dir, existing.taskID+".json"), raw, 0o644))
		}

		rawResult, err := json.Marshal(map[string]any{
			"task_id":       "t-01",
			"verdict":       "pass",
			"evidence_ref":  "test:mixed-active-run-two",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, "task-result.json"), rawResult, 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{"task", "--json", "--result-file", "task-result.json"})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_run_summary_version_ambiguous", cliErr.ErrorCode)
	})
}

func TestEvidenceTaskResultFileRejectsExistingRunVersionNewerThanActiveRun(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		_, err := state.MaterializeWavePlanAtRunSummaryVersion(root, change, time.Now().UTC(), 2)
		require.NoError(t, err)

		writeTaskEvidenceFile(t, root, slug, 3, "t-02", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:newer-than-active",
		})
		writeTaskResultFile(t, filepath.Join(root, "task-result.json"), "t-01", "test:newer-existing-version", "cmd/lifecycle_commands_test.go")

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{"task", "--json", "--result-file", "task-result.json"})
		cliErr := asCLIError(cmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_task_run_summary_version_ambiguous", cliErr.ErrorCode)
	})
}

func TestEvidenceTaskResultFileAfterReexecutionUsesNewWavePlanRunVersion(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)

		rawResult, err := json.Marshal(map[string]any{
			"task_id":       "t-01",
			"verdict":       "pass",
			"evidence_ref":  "test:initial-result-file",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, "task-result.json"), rawResult, 0o644))

		initialEvidenceCmd := commandForRoot(t, root, makeEvidenceCmd())
		initialEvidenceCmd.SetArgs([]string{"task", "--json", "--result-file", "task-result.json"})
		initialEvidenceCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, initialEvidenceCmd.Execute())

		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		fixCmd := commandForRoot(t, root, makeFixCmd())
		fixCmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution"})
		fixCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, fixCmd.Execute())

		reopened, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.Equal(t, model.StateS2Implement, reopened.CurrentState)
		plan, err := state.LoadWavePlanForChange(root, reopened)
		require.NoError(t, err)
		require.Equal(t, 2, plan.RunSummaryVersion)

		rawResult, err = json.Marshal(map[string]any{
			"task_id":       "t-01",
			"verdict":       "pass",
			"evidence_ref":  "test:reexecution-result-file",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(root, "task-result.json"), rawResult, 0o644))

		reexecutionEvidenceCmd := commandForRoot(t, root, makeEvidenceCmd())
		reexecutionEvidenceCmd.SetArgs([]string{"task", "--json", "--result-file", "task-result.json"})
		var out bytes.Buffer
		reexecutionEvidenceCmd.SetOut(&out)
		require.NoError(t, reexecutionEvidenceCmd.Execute())

		var view evidenceTaskView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, 2, view.RunSummaryVersion)

		taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "t-01.json")
		task, _, _, err := progression.ParseTaskEvidence(root, taskEvidencePath, 2)
		require.NoError(t, err)
		assert.Equal(t, "test:reexecution-result-file", task.EvidenceRef)

		waveCmd := commandForRoot(t, root, makeEvidenceCmd())
		waveCmd.SetArgs([]string{
			"skill",
			"--json",
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes", "reexecution wave evidence",
		})
		out.Reset()
		waveCmd.SetOut(&out)
		require.NoError(t, waveCmd.Execute())

		var waveView evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &waveView))
		assert.Equal(t, 2, waveView.RunVersion)
	})
}

func TestEvidenceTaskResultFileAfterMultiTaskReexecutionAllowsSequentialReimport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createMultiTaskEvidenceTaskFixture(t, root)

		writeTaskResultFile(t, filepath.Join(root, "task-result-t-01.json"), "t-01", "test:initial-t-01", "cmd/evidence.go")
		writeTaskResultFile(t, filepath.Join(root, "task-result-t-02.json"), "t-02", "test:initial-t-02", "cmd/evidence_task_test.go")
		for _, resultFile := range []string{"task-result-t-01.json", "task-result-t-02.json"} {
			initialEvidenceCmd := commandForRoot(t, root, makeEvidenceCmd())
			initialEvidenceCmd.SetArgs([]string{"task", "--json", "--result-file", resultFile})
			initialEvidenceCmd.SetOut(&bytes.Buffer{})
			require.NoError(t, initialEvidenceCmd.Execute())
		}

		writePassingExecutionSummary(t, root, slug, 1, "t-01", "t-02")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		fixCmd := commandForRoot(t, root, makeFixCmd())
		fixCmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution"})
		fixCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, fixCmd.Execute())

		reopened, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		require.Equal(t, model.StateS2Implement, reopened.CurrentState)
		plan, err := state.LoadWavePlanForChange(root, reopened)
		require.NoError(t, err)
		require.Equal(t, 2, plan.RunSummaryVersion)
		require.Equal(t, 2, plan.TotalTasks)

		writeTaskResultFile(t, filepath.Join(root, "task-result-t-01.json"), "t-01", "test:reexecution-t-01", "cmd/evidence.go")
		firstCmd := commandForRoot(t, root, makeEvidenceCmd())
		firstCmd.SetArgs([]string{"task", "--json", "--result-file", "task-result-t-01.json"})
		firstCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, firstCmd.Execute())

		waveCmd := commandForRoot(t, root, makeEvidenceCmd())
		waveCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes", "must reject partial multi-task reexecution",
		})
		cliErr := asCLIError(waveCmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_task_evidence_incomplete", cliErr.ErrorCode)

		writeTaskResultFile(t, filepath.Join(root, "task-result-t-02.json"), "t-02", "test:reexecution-t-02", "cmd/evidence_task_test.go")
		secondCmd := commandForRoot(t, root, makeEvidenceCmd())
		secondCmd.SetArgs([]string{"task", "--json", "--result-file", "task-result-t-02.json"})
		secondCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, secondCmd.Execute())

		for _, expected := range []struct {
			taskID      string
			evidenceRef string
		}{
			{taskID: "t-01", evidenceRef: "test:reexecution-t-01"},
			{taskID: "t-02", evidenceRef: "test:reexecution-t-02"},
		} {
			taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), expected.taskID+".json")
			task, _, _, err := progression.ParseTaskEvidence(root, taskEvidencePath, 2)
			require.NoError(t, err)
			assert.Equal(t, expected.evidenceRef, task.EvidenceRef)
		}

		waveCmd = commandForRoot(t, root, makeEvidenceCmd())
		waveCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--reference", "dispatch_mode:wave=1:degraded_sequential",
			"--reference", "degraded_dispatch_justification:wave=1:tool_unavailable=single-threaded test fixture",
			"--notes", "multi-task reexecution complete",
		})
		var out bytes.Buffer
		waveCmd.SetOut(&out)
		require.NoError(t, waveCmd.Execute())

		var waveView evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &waveView))
		assert.Equal(t, 2, waveView.RunVersion)
	})
}

func TestEvidenceSkillWaveOrchestrationRejectsMixedTaskEvidenceDespiteStaleExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))
		writeSkillVerification(t, root, slug, progression.SkillSpecComplianceReview, model.VerificationRecord{
			Verdict:    model.VerificationVerdictFail,
			Blockers:   []model.ReasonCode{model.NewReasonCode("review_layer_failed", "R1")},
			Timestamp:  time.Now().UTC(),
			RunVersion: 1,
		})

		fixCmd := commandForRoot(t, root, makeFixCmd())
		fixCmd.SetArgs([]string{"--json", "--change", slug, "--start-reexecution"})
		fixCmd.SetOut(&bytes.Buffer{})
		require.NoError(t, fixCmd.Execute())

		writeTaskEvidenceFile(t, root, slug, 1, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:stale-run-one",
		})
		writeTaskEvidenceFile(t, root, slug, 2, "t-02", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:active-run-two",
		})

		waveCmd := commandForRoot(t, root, makeEvidenceCmd())
		waveCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes", "must not stamp stale run one",
		})
		cliErr := asCLIError(waveCmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_task_evidence_run_summary_ambiguous", cliErr.ErrorCode)

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.NoError(t, err)
		assert.Equal(t, 1, rec.RunVersion)
		assert.NotContains(t, rec.Notes, "must not stamp stale run one")
	})
}

func TestEvidenceSkillWaveOrchestrationRejectsInvalidTaskEvidenceDespiteStaleExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writeTaskEvidenceFile(t, root, slug, 2, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:invalid-freshness",
			"freshness_inputs": map[string]any{
				"change_id":           slug,
				"run_summary_version": 2,
				"task_id":             "wrong-task",
			},
		})

		waveCmd := commandForRoot(t, root, makeEvidenceCmd())
		waveCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes", "must not hide invalid task evidence behind stale summary",
		})
		cliErr := asCLIError(waveCmd.Execute())
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_task_evidence_invalid", cliErr.ErrorCode)
		assert.Equal(t, 2, cliErr.Details["run_summary_version"])

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.Error(t, err)
		assert.True(t, errors.Is(err, os.ErrNotExist))
		assert.Equal(t, model.VerificationRecord{}, rec)
	})
}

func TestEvidenceSkillWaveOrchestrationUsesNewTaskEvidenceInsteadOfStaleExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 2, "t-01")
		writeTaskEvidenceFile(t, root, slug, 3, "t-01", map[string]any{
			"task_kind":     "verification",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"target_files":  []string{"cmd/lifecycle_commands_test.go"},
			"evidence_ref":  "test:active-run-three",
		})

		waveCmd := commandForRoot(t, root, makeEvidenceCmd())
		waveCmd.SetArgs([]string{
			"skill",
			"--json",
			"--change", slug,
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes", "stamp active run three",
		})
		var out bytes.Buffer
		waveCmd.SetOut(&out)
		require.NoError(t, waveCmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, 3, view.RunVersion)

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.NoError(t, err)
		assert.Equal(t, 3, rec.RunVersion)
		assert.Equal(t, "stamp active run three", rec.Notes)
	})
}

func TestEvidenceSkillRecordsCLIStampedVerificationAndDigest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 2, "t-01")
		writePassingWaveEvidence(t, root, slug, 2)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		notesPath := filepath.Join(root, "review-notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("review notes from disk\n"), 0o644))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--json",
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", "pass",
			"--reference", "layer:R0=pass",
			"--reference", "scope_contract:pass",
			"--notes-file", "review-notes.md",
		})
		var out bytes.Buffer
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillSpecComplianceReview, view.Skill)
		assert.Equal(t, 2, view.RunVersion)
		assert.True(t, view.Recorded)
		assert.True(t, view.Stamped)
		assert.Contains(t, view.Path, "verification/"+progression.SkillSpecComplianceReview+".yaml")

		rec, err := state.LoadVerification(root, slug, progression.SkillSpecComplianceReview)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Equal(t, 2, rec.RunVersion)
		assert.False(t, rec.Timestamp.IsZero())
		assert.Equal(t, "review notes from disk", rec.Notes)
		assert.Equal(t, []string{"layer:R0=pass", "scope_contract:pass"}, rec.References)

		digests, err := state.LoadOptionalEvidenceDigestsForChange(root, change)
		require.NoError(t, err)
		require.NotNil(t, digests)
		assert.Contains(t, digests.Skills, progression.SkillSpecComplianceReview)
	})
}

func TestEvidenceSkillRejectsUnknownSkillWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--skill", "../escape",
			"--verdict", "pass",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_invalid", cliErr.ErrorCode)

		_, statErr := os.Stat(state.VerificationFilePath(root, slug, "../escape"))
		require.Error(t, statErr)
	})
}

func TestEvidenceSkillRejectsRunSummaryBoundSkillWithoutExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug := createGovernedRequest(t, root, levelNonDiscovery, "skill evidence command")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS3Review
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", "pass",
		})
		err = cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_run_summary_missing", cliErr.ErrorCode)
	})
}

func TestEvidenceSkillRecordsWaveOrchestrationBeforeExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		taskCmd := commandForRoot(t, root, makeEvidenceCmd())
		taskCmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wave-bootstrap-task",
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
		})
		require.NoError(t, taskCmd.Execute())

		summary, err := state.LoadOptionalExecutionSummary(root, slug)
		require.NoError(t, err)
		require.Nil(t, summary)

		notesPath := filepath.Join(root, "wave-notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("wave evidence from task ledger\n"), 0o644))
		skillCmd := commandForRoot(t, root, makeEvidenceCmd())
		skillCmd.SetArgs([]string{
			"skill",
			"--json",
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes-file", "wave-notes.md",
		})
		var out bytes.Buffer
		skillCmd.SetOut(&out)
		require.NoError(t, skillCmd.Execute())

		var view evidenceSkillView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
		assert.Equal(t, progression.SkillWaveOrchestration, view.Skill)
		assert.Equal(t, 1, view.RunVersion)
		assert.True(t, view.Recorded)
		assert.True(t, view.Stamped)

		rec, err := state.LoadVerification(root, slug, progression.SkillWaveOrchestration)
		require.NoError(t, err)
		assert.Equal(t, model.VerificationVerdictPass, rec.Verdict)
		assert.Equal(t, 1, rec.RunVersion)
		assert.Equal(t, "wave evidence from task ledger", rec.Notes)

		digests, err := state.LoadOptionalEvidenceDigestsForChange(root, model.NewChange(slug))
		require.NoError(t, err)
		require.NotNil(t, digests)
		assert.Contains(t, digests.Skills, progression.SkillWaveOrchestration)
	})
}

func TestEvidenceSkillRejectsWrongWorkflowStateWithoutWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, change := createEvidenceTaskFixture(t, root)
		writePassingExecutionSummary(t, root, slug, 1, "t-01")
		writePassingWaveEvidence(t, root, slug, 1)
		change.CurrentState = model.StateS2Implement
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"skill",
			"--skill", progression.SkillSpecComplianceReview,
			"--verdict", "pass",
		})
		err := cmd.Execute()
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "evidence_skill_wrong_state", cliErr.ErrorCode)

		_, statErr := os.Stat(state.VerificationFilePath(root, slug, progression.SkillSpecComplianceReview))
		require.Error(t, statErr)
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

func TestNormalizeEvidencePathsUsesPublicSlashPaths(t *testing.T) {
	t.Parallel()

	got, err := normalizeEvidencePaths([]string{`cmd\run.go`, "cmd/run.go"})
	require.NoError(t, err)
	assert.Equal(t, []string{"cmd/run.go"}, got)
}

func TestNormalizeEvidencePathsRejectsWindowsAbsolutePath(t *testing.T) {
	t.Parallel()

	_, err := normalizeEvidencePaths([]string{`C:\tmp\file.go`})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace-relative")
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

func writeTaskResultFile(t *testing.T, path, taskID, evidenceRef, changedFile string) {
	t.Helper()

	writeTaskResultFileWithVerdict(t, path, taskID, "pass", evidenceRef, changedFile, nil)
}

func writeTaskResultFileWithSessionID(t *testing.T, path, taskID, evidenceRef, changedFile, sessionID string) {
	t.Helper()

	writeTaskResultFilePayload(t, path, map[string]any{
		"task_id":       taskID,
		"verdict":       "pass",
		"evidence_ref":  evidenceRef,
		"changed_files": []string{changedFile},
		"blockers":      []string{},
		"session_id":    sessionID,
	})
}

func writeTaskResultFileWithVerdict(
	t *testing.T,
	path string,
	taskID string,
	verdict string,
	evidenceRef string,
	changedFile string,
	blockers []string,
) {
	t.Helper()

	writeTaskResultFilePayload(t, path, map[string]any{
		"task_id":       taskID,
		"verdict":       verdict,
		"evidence_ref":  evidenceRef,
		"changed_files": []string{changedFile},
		"blockers":      blockers,
	})
}

func writeTaskResultFilePayload(t *testing.T, path string, payload map[string]any) {
	t.Helper()

	rawResult, err := json.Marshal(payload)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, rawResult, 0o644))
}

func assertTaskEvidenceWritten(t *testing.T, root, slug, taskID string) {
	t.Helper()

	taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
	_, statErr := os.Stat(taskEvidencePath)
	require.NoError(t, statErr)
}

func assertTaskEvidenceNotWritten(t *testing.T, root, slug, taskID string) {
	t.Helper()

	taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
	_, statErr := os.Stat(taskEvidencePath)
	require.Error(t, statErr)
	assert.True(t, os.IsNotExist(statErr))
}

// corruptWavePlanCache overwrites the engine-owned wave-plan.yaml cache with
// view-only fields (wave_count/advisories) that the persisted schema rejects
// under KnownFields(true), so loadCurrentWavePlanForCommand fails closed with
// state.ErrWavePlanCacheUnreadable.
func corruptWavePlanCache(t *testing.T, root, slug string) {
	t.Helper()
	cachePath := state.WavePlanPathForRead(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o755))
	require.NoError(t, os.WriteFile(cachePath, []byte("wave_count: 1\nadvisories: [\"narrow\"]\nwaves: []\n"), 0o644))
}

// assertWavePlanCacheUnreadableError asserts an evidence command translated a
// corrupt engine-owned cache into the canonical wave_plan_unreadable recovery
// story instead of misdirecting the user to edit tasks.md.
func assertWavePlanCacheUnreadableError(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "wave_plan_unreadable", cliErr.ErrorCode,
		"a corrupt engine-owned cache must surface as wave_plan_unreadable, not a tasks.md-derivation failure")
	assert.Contains(t, cliErr.Remediation, "wave-plan.yaml")
	assert.Contains(t, cliErr.Remediation, "slipway repair")
	assert.Contains(t, cliErr.Remediation, "must not be hand-edited",
		"cache-unreadable remediation must describe the cache as engine-owned / not hand-editable")
	assert.NotContains(t, cliErr.Remediation, "Fix tasks.md",
		"cache-unreadable remediation must not tell the user to edit tasks.md")
}

// TestEvidenceTaskInteractiveCacheUnreadableNamesCacheNotTasks covers the
// `slipway evidence task --task-id` surface: a corrupt engine-owned cache must
// route to the cache + `slipway repair` recovery, not the tasks.md remediation.
func TestEvidenceTaskInteractiveCacheUnreadableNamesCacheNotTasks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)
		corruptWavePlanCache(t, root, slug)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:evidence-task",
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
		})
		assertWavePlanCacheUnreadableError(t, cmd.Execute())
	})
}

// TestEvidenceTaskResultFileCacheUnreadableNamesCacheNotTasks covers the
// `slipway evidence task --result-file` import surface.
func TestEvidenceTaskResultFileCacheUnreadableNamesCacheNotTasks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		resultPath := filepath.Join(root, "task-result.json")
		rawResult, err := json.Marshal(map[string]any{
			"task_id":       "t-01",
			"verdict":       "pass",
			"evidence_ref":  "test:result-file",
			"changed_files": []string{"cmd/lifecycle_commands_test.go"},
			"blockers":      []string{},
			"session_id":    "executor-a",
		})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(resultPath, rawResult, 0o644))

		corruptWavePlanCache(t, root, slug)

		cmd := commandForRoot(t, root, makeEvidenceCmd())
		cmd.SetArgs([]string{
			"task",
			"--json",
			"--result-file", "task-result.json",
		})
		assertWavePlanCacheUnreadableError(t, cmd.Execute())
	})
}

// TestEvidenceSkillWaveOrchestrationCacheUnreadableNamesCacheNotTasks covers the
// `slipway evidence skill wave-orchestration` surface: task evidence is recorded
// while the cache is valid, then the cache is corrupted before recording
// wave-orchestration evidence.
func TestEvidenceSkillWaveOrchestrationCacheUnreadableNamesCacheNotTasks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	withCommandWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		slug, _ := createEvidenceTaskFixture(t, root)

		taskCmd := commandForRoot(t, root, makeEvidenceCmd())
		taskCmd.SetArgs([]string{
			"task",
			"--json",
			"--task-id", "t-01",
			"--run-summary-version", "1",
			"--task-kind", "verification",
			"--verdict", "pass",
			"--evidence-ref", "test:wave-bootstrap-task",
			"--changed-file", "cmd/lifecycle_commands_test.go",
			"--target-file", "cmd/lifecycle_commands_test.go",
		})
		require.NoError(t, taskCmd.Execute())

		// Corrupt the cache only after valid task evidence exists, so the
		// wave-orchestration run-version derivation reaches the wave-plan load.
		corruptWavePlanCache(t, root, slug)

		notesPath := filepath.Join(root, "wave-notes.md")
		require.NoError(t, os.WriteFile(notesPath, []byte("wave evidence from task ledger\n"), 0o644))
		skillCmd := commandForRoot(t, root, makeEvidenceCmd())
		skillCmd.SetArgs([]string{
			"skill",
			"--json",
			"--skill", progression.SkillWaveOrchestration,
			"--verdict", "pass",
			"--reference", "wave-orchestration:pass",
			"--notes-file", "wave-notes.md",
		})
		assertWavePlanCacheUnreadableError(t, skillCmd.Execute())
	})
}

func createEvidenceTaskFixture(t *testing.T, root string) (string, model.Change) {
	t.Helper()

	slug := createGovernedRequest(t, root, levelNonDiscovery, "evidence task command")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	return slug, change
}

func createMultiTaskEvidenceTaskFixture(t *testing.T, root string) (string, model.Change) {
	t.Helper()

	slug, change := createEvidenceTaskFixture(t, root)
	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`t-01`"+` harden result file loading
  - depends_on: []
  - target_files: ["cmd/evidence.go"]
  - task_kind: code
  - covers: [REQ-001]

- [ ] `+"`t-02`"+` cover result file reexecution
  - depends_on: []
  - target_files: ["cmd/evidence_task_test.go"]
  - task_kind: test
  - covers: [REQ-001]
`)))
	_, err := state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	return slug, change
}
