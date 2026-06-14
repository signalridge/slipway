package progression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/wave"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTasksAndMaterializeWavePlan(t *testing.T, root string, change model.Change, tasks string) string {
	t.Helper()

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(tasks), 0o644))

	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)
	return tasksPath
}

func expectedTaskFreshnessInputsForWavePlan(
	t *testing.T,
	root string,
	change model.Change,
	runSummaryVersion int,
	taskID string,
) model.ExecutionTaskFreshnessInputs {
	t.Helper()
	wavePlan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)
	return state.ExpectedExecutionTaskFreshnessInputs(change, runSummaryVersion, taskID, wavePlan.TasksPlanHash)
}

func TestCollectNonPassTaskBlockers_AllPass(t *testing.T) {
	t.Parallel()
	runs := map[string]model.TaskRun{
		"t1": {TaskID: "t1", Verdict: model.TaskVerdictPass},
	}
	blockers := CollectNonPassTaskBlockers(runs)
	if len(blockers) != 0 {
		t.Fatalf("expected no blockers, got %v", blockers)
	}
}

func TestCollectNonPassTaskBlockers_WithFail(t *testing.T) {
	t.Parallel()
	runs := map[string]model.TaskRun{
		"t1": {TaskID: "t1", Verdict: model.TaskVerdictFail},
	}
	blockers := CollectNonPassTaskBlockers(runs)
	if len(blockers) == 0 {
		t.Fatal("expected blockers for non-pass task")
	}
}

func TestCollectNonPassTaskBlockers_Empty(t *testing.T) {
	t.Parallel()
	blockers := CollectNonPassTaskBlockers(nil)
	if blockers != nil {
		t.Fatalf("expected nil, got %v", blockers)
	}
}

func incompleteTestWavePlan() model.WavePlan {
	return model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{
			{WaveIndex: 1, Tasks: []model.WavePlanTask{{TaskID: "t1"}, {TaskID: "t2"}}},
			{WaveIndex: 2, Tasks: []model.WavePlanTask{{TaskID: "t3"}}},
		},
	}
}

func TestIncompleteExecutionTaskBlockers_MissingPlannedTaskBlocks(t *testing.T) {
	t.Parallel()
	// Evidence recorded only for the early tasks; t3 was planned but never run.
	runs := map[string]model.TaskRun{
		"t1": {TaskID: "t1", Verdict: model.TaskVerdictPass},
		"t2": {TaskID: "t2", Verdict: model.TaskVerdictPass},
	}
	blockers := IncompleteExecutionTaskBlockers(incompleteTestWavePlan(), runs)
	require.Len(t, blockers, 1)
	assert.Equal(t, "incomplete_execution_task", blockers[0].Code)
	assert.Equal(t, "t3", blockers[0].Detail)
}

func TestIncompleteExecutionTaskBlockers_AllRecordedNoBlock(t *testing.T) {
	t.Parallel()
	// Presence in runs is what matters here; a recorded-but-failing task is
	// reported by CollectNonPassTaskBlockers, not this check.
	runs := map[string]model.TaskRun{
		"t1": {TaskID: "t1", Verdict: model.TaskVerdictPass},
		"t2": {TaskID: "t2", Verdict: model.TaskVerdictPass},
		"t3": {TaskID: "t3", Verdict: model.TaskVerdictPass},
	}
	assert.Empty(t, IncompleteExecutionTaskBlockers(incompleteTestWavePlan(), runs))
}

func TestIncompleteExecutionTaskBlockers_MultipleMissingSorted(t *testing.T) {
	t.Parallel()
	blockers := IncompleteExecutionTaskBlockers(incompleteTestWavePlan(), map[string]model.TaskRun{
		"t2": {TaskID: "t2", Verdict: model.TaskVerdictPass},
	})
	require.Len(t, blockers, 2)
	assert.Equal(t, "t1", blockers[0].Detail)
	assert.Equal(t, "t3", blockers[1].Detail)
}

func TestIncompleteExecutionTaskBlockers_EmptyPlan(t *testing.T) {
	t.Parallel()
	assert.Nil(t, IncompleteExecutionTaskBlockers(model.WavePlan{}, map[string]model.TaskRun{}))
}

func TestBuildExecutionSummarySyncsDerivedFieldsAndWaveBlockers(t *testing.T) {
	t.Parallel()

	capturedAt := time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC)
	summary := BuildExecutionSummary(
		3,
		[]model.ExecutionTaskSummary{
			{
				TaskID:       "task-a",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"cmd/next.go"},
			},
			{
				TaskID:      "task-b",
				Verdict:     model.TaskVerdictFail,
				TaskKind:    model.TaskKindCode,
				Blockers:    []model.ReasonCode{model.NewReasonCode("required_skill_missing", "plan-audit")},
				CapturedAt:  capturedAt.Add(time.Second),
				EvidenceRef: "artifacts/runtime/task-b.json",
			},
		},
		capturedAt,
		&model.VerificationRecord{Blockers: []model.ReasonCode{model.NewReasonCode("wave_execution_blocked", "review")}},
	)

	assert.Equal(t, model.ExecutionVerdictFail, summary.OverallVerdict)
	assert.Equal(t, []string{"task-a"}, summary.CompletedTasks)
	assert.Equal(t, []string{"task-b"}, summary.NonPassTasks)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "task", "task-b:required_skill_missing:plan-audit"))
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "wave_execution_blocked", "review"))
}

func TestBuildExecutionSummaryPreservesDistinctTaskBlockerDetails(t *testing.T) {
	t.Parallel()

	summary := BuildExecutionSummary(
		1,
		[]model.ExecutionTaskSummary{
			{
				TaskID:   "task-a",
				Verdict:  model.TaskVerdictFail,
				TaskKind: model.TaskKindCode,
				Blockers: []model.ReasonCode{
					model.NewReasonCode("required_skill_missing", "spec-compliance-review"),
					model.NewReasonCode("required_skill_missing", "code-quality-review"),
				},
			},
		},
		time.Date(2026, 4, 8, 12, 0, 0, 0, time.UTC),
		nil,
	)

	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "task", "task-a:required_skill_missing:spec-compliance-review"))
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "task", "task-a:required_skill_missing:code-quality-review"))
	assert.Len(t, summary.OpenBlockers, 2)
}

func TestTasksPlanChangedSinceTaskEvidenceBlockersTracksAllTasksWhenHashChanges(t *testing.T) {
	t.Parallel()

	tasksPlanUpdatedAt := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	blockers := tasksPlanChangedSinceTaskEvidenceBlockers(
		"previous-hash",
		[]model.ExecutionTaskSummary{
			{
				TaskID:     "fresh-task",
				CapturedAt: tasksPlanUpdatedAt.Add(time.Second),
			},
			{
				TaskID:     "missing-capture",
				CapturedAt: time.Time{},
			},
			{
				TaskID:     "stale-task",
				CapturedAt: tasksPlanUpdatedAt.Add(-time.Second),
			},
		},
		"current-hash",
	)

	assert.Equal(t, []string{
		"tasks_plan_changed_since_task_evidence:fresh-task",
		"tasks_plan_changed_since_task_evidence:missing-capture",
		"tasks_plan_changed_since_task_evidence:stale-task",
	}, blockers)
}

func TestTasksPlanChangedSinceTaskEvidenceBlockersAppliesOnFirstExecution(t *testing.T) {
	t.Parallel()

	tasksPlanUpdatedAt := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	blockers := tasksPlanChangedSinceTaskEvidenceBlockers(
		"",
		[]model.ExecutionTaskSummary{
			{
				TaskID:     "fresh-task",
				CapturedAt: tasksPlanUpdatedAt.Add(time.Second),
			},
			{
				TaskID:     "stale-task",
				CapturedAt: tasksPlanUpdatedAt.Add(-time.Second),
			},
		},
		"current-hash",
	)

	assert.Empty(t, blockers)
}

func TestParseTaskEvidenceRejectsCompatibilityFallbacks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		payload string
		wantErr string
	}{
		{
			name:    "nested-task-run",
			payload: `{"task_run":{"task_id":"task-a","run_summary_version":1,"task_kind":"code","verdict":"pass","evidence_ref":"test"},"captured_at":"2026-04-06T10:00:00Z"}`,
			wantErr: "task_run is not supported",
		},
		{
			name:    "missing-task-id",
			payload: `{"run_summary_version":1,"task_kind":"code","verdict":"pass","evidence_ref":"test","captured_at":"2026-04-06T10:00:00Z"}`,
			wantErr: "task_id is required",
		},
		{
			name:    "missing-run-summary-version",
			payload: `{"task_id":"task-a","task_kind":"code","verdict":"pass","evidence_ref":"test","captured_at":"2026-04-06T10:00:00Z"}`,
			wantErr: "run_summary_version is required",
		},
		{
			name:    "missing-task-kind",
			payload: `{"task_id":"task-a","run_summary_version":1,"verdict":"pass","evidence_ref":"test","captured_at":"2026-04-06T10:00:00Z"}`,
			wantErr: "task_kind is required",
		},
		{
			name:    "missing-verdict",
			payload: `{"task_id":"task-a","run_summary_version":1,"task_kind":"code","evidence_ref":"test","captured_at":"2026-04-06T10:00:00Z"}`,
			wantErr: "verdict is required",
		},
		{
			name:    "missing-evidence-ref",
			payload: `{"task_id":"task-a","run_summary_version":1,"task_kind":"code","verdict":"pass","captured_at":"2026-04-06T10:00:00Z"}`,
			wantErr: "evidence_ref is required",
		},
		{
			name:    "missing-captured-at",
			payload: `{"task_id":"task-a","run_summary_version":1,"task_kind":"code","verdict":"pass","evidence_ref":"test"}`,
			wantErr: "captured_at is required",
		},
		{
			name:    "invalid-captured-at",
			payload: `{"task_id":"task-a","run_summary_version":1,"task_kind":"code","verdict":"pass","evidence_ref":"test","captured_at":"yesterday"}`,
			wantErr: "captured_at must be RFC3339Nano",
		},
		{
			name:    "legacy-input-hash",
			payload: `{"task_id":"task-a","run_summary_version":1,"task_kind":"code","verdict":"pass","evidence_ref":"test","captured_at":"2026-04-06T10:00:00Z","input_hash":"legacy"}`,
			wantErr: "input_hash is not supported",
		},
		{
			name:    "missing-freshness-inputs",
			payload: `{"task_id":"task-a","run_summary_version":1,"task_kind":"code","verdict":"pass","evidence_ref":"test","captured_at":"2026-04-06T10:00:00Z"}`,
			wantErr: "freshness_inputs is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			path := filepath.Join(root, tc.name+".json")
			require.NoError(t, os.WriteFile(path, []byte(tc.payload), 0o644))
			_, _, _, err := ParseTaskEvidence(root, path, 1)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestLoadExecutionTasksFromEvidenceRejectsFreshnessInputMismatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("freshness-mismatch")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "external_api_contracts"
	require.NoError(t, state.SaveChange(root, change))

	inputs := state.ExpectedExecutionTaskFreshnessInputs(change, 1, "task-a")
	inputs.GuardrailDomain = "auth_authz"
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"evidence_ref":        "test:task-a",
		"captured_at":         time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		"freshness_inputs":    inputs,
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, change.Slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	tasks, parseIssues, err := LoadExecutionTasksFromEvidence(root, change.Slug, 1)
	require.NoError(t, err)
	assert.Empty(t, tasks)
	require.Len(t, parseIssues, 1)
	assert.Contains(t, parseIssues[0], "freshness_inputs mismatch")
	assert.Contains(t, parseIssues[0], "guardrail_domain")
}

func TestLoadExecutionTasksFromEvidenceIgnoresPreviousRunVersionEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("ignore-previous-run-evidence")
	require.NoError(t, state.SaveChange(root, change))

	writeTaskEvidence := func(taskID string, runVersion int) {
		t.Helper()
		payload := map[string]any{
			"task_id":             taskID,
			"run_summary_version": runVersion,
			"task_kind":           "code",
			"verdict":             "pass",
			"evidence_ref":        "test:" + taskID,
			"captured_at":         time.Now().UTC().Format(time.RFC3339Nano),
			"freshness_inputs":    state.ExpectedExecutionTaskFreshnessInputs(change, runVersion, taskID),
		}
		raw, err := json.Marshal(payload)
		require.NoError(t, err)
		path := filepath.Join(state.EvidenceTasksDir(root, change.Slug), taskID+".json")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, raw, 0o644))
	}
	writeTaskEvidence("task-a", 1)
	writeTaskEvidence("task-b", 2)

	tasks, parseIssues, err := LoadExecutionTasksFromEvidence(root, change.Slug, 2)
	require.NoError(t, err)
	assert.Empty(t, parseIssues)
	require.Len(t, tasks, 1)
	assert.Equal(t, "task-b", tasks[0].TaskID)
}

func TestBuildResumeCompletedTasks(t *testing.T) {
	t.Parallel()
	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		Tasks: []model.ExecutionTaskSummary{
			{TaskID: "t1", Verdict: model.TaskVerdictPass, TaskKind: model.TaskKindCode},
			{TaskID: "t2", Verdict: model.TaskVerdictFail, TaskKind: model.TaskKindCode},
			{TaskID: "t3", Verdict: model.TaskVerdictPass, TaskKind: model.TaskKindDoc, Blockers: []model.ReasonCode{model.NewReasonCode("needs_retry", "")}},
		},
	}
	completed := BuildResumeCompletedTasks(summary)
	if !completed["t1"] {
		t.Error("expected t1 to be completed")
	}
	if completed["t2"] {
		t.Error("expected t2 to not be completed")
	}
	if completed["t3"] {
		t.Error("expected t3 to remain incomplete because blockers are still open")
	}
}

func TestSyncGovernedWaveExecution_PersistsExecutionSummaryAndRuntimeSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "wave-sync"
	recordedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [x] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)

	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"blockers":            []string{},
		"evidence_ref":        "test:task-a",
		"captured_at":         recordedAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	if err != nil {
		t.Fatalf("marshal task evidence: %v", err)
	}
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	if err := os.MkdirAll(filepath.Dir(taskPath), 0o755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	if err := os.WriteFile(taskPath, raw, 0o644); err != nil {
		t.Fatalf("write task evidence: %v", err)
	}

	result, err := SyncGovernedWaveExecution(root, change)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Updated {
		t.Fatalf("expected sync result to report updated state, got %+v", result)
	}
	summary, err := state.LoadExecutionSummary(root, slug)
	if err != nil {
		t.Fatalf("load execution summary: %v", err)
	}
	if summary.RunSummaryVersion != 1 {
		t.Fatalf("expected execution summary version 1, got %d", summary.RunSummaryVersion)
	}
	if len(summary.Tasks) != 1 || summary.Tasks[0].TaskID != "task-a" {
		t.Fatalf("expected persisted execution task summary, got %+v", summary.Tasks)
	}
}

func TestSyncGovernedWaveExecutionRecordsDegradedDispatchMode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-sync-degraded-dispatch"
	recordedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
		References: []string{
			"dispatch_mode:wave=1:degraded_sequential",
		},
	})
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [x] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code

- [x] `+"`task-b`"+` Implement task B
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)

	writeTaskEvidence := func(taskID string, changedFile string) {
		t.Helper()
		taskEvidence := map[string]any{
			"task_id":             taskID,
			"run_summary_version": 1,
			"task_kind":           "code",
			"verdict":             "pass",
			"changed_files":       []string{changedFile},
			"blockers":            []string{},
			"evidence_ref":        "test:" + taskID,
			"captured_at":         recordedAt.Format(time.RFC3339Nano),
			"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, taskID),
		}
		raw, err := json.Marshal(taskEvidence)
		require.NoError(t, err)
		taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
		require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
		require.NoError(t, os.WriteFile(taskPath, raw, 0o644))
	}
	writeTaskEvidence("task-a", "cmd/next.go")
	writeTaskEvidence("task-b", "cmd/run.go")

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, result.Updated)

	runs, err := state.LoadWaveRuns(root, slug, 1)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, model.WaveDispatchDegradedSequential, runs[0].DispatchMode)
}

func TestSyncGovernedWaveExecutionUsesEffectiveParallelForDispatchMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		slug              string
		config            string
		persistedParallel bool
		references        []string
		want              model.WaveDispatchMode
	}{
		{
			name:              "default forced parallel records the declared parallel token",
			slug:              "wave-effective-parallel-default",
			persistedParallel: false,
			// A parallel_subagents wave requires an executor handle per planned task,
			// so both are recorded; otherwise the executor-handle gate would fire.
			references: []string{
				"dispatch_mode:wave=1:parallel_subagents",
				"executor_agent:wave=1:task=task-a:agent-a",
				"executor_agent:wave=1:task=task-b:agent-b",
			},
			want: model.WaveDispatchParallel,
		},
		{
			name:              "parallelization off suppresses stale persisted true",
			slug:              "wave-effective-parallel-off",
			config:            "execution:\n  parallelization: off\n",
			persistedParallel: true,
			want:              "",
		},
		{
			name:              "stale degraded dispatch is ignored when parallelization is off",
			slug:              "wave-effective-parallel-off-stale-dispatch",
			config:            "execution:\n  parallelization: off\n",
			persistedParallel: true,
			references:        []string{"dispatch_mode:wave=1:degraded_sequential"},
			want:              "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			recordedAt := time.Date(2026, 6, 9, 2, 0, 0, 0, time.UTC)
			change := model.NewChange(tt.slug)
			change.CurrentState = model.StateS2Execute
			change.PlanSubStep = model.PlanSubStepNone
			require.NoError(t, state.SaveChange(root, change))
			if tt.config != "" {
				require.NoError(t, os.WriteFile(state.ConfigPath(root), []byte(tt.config), 0o644))
			}
			bundleDir, err := state.GovernedBundleDir(root, change)
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(bundleDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [x] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code

- [x] `+"`task-b`"+` Implement task B
  - target_files: ["cmd/run.go"]
  - task_kind: code
`), 0o644))
			writeVerificationForTest(t, root, tt.slug, SkillWaveOrchestration, model.VerificationRecord{
				Verdict:    model.VerificationVerdictPass,
				Blockers:   []model.ReasonCode{},
				Timestamp:  recordedAt,
				RunVersion: 1,
				References: tt.references,
			})
			require.NoError(t, state.SaveWavePlan(root, tt.slug, model.WavePlan{
				Version:     model.WavePlanVersion,
				GeneratedAt: recordedAt.Add(-time.Hour),
				TotalTasks:  2,
				Waves: []model.WavePlanWave{{
					WaveIndex: 1,
					Parallel:  tt.persistedParallel,
					Tasks: []model.WavePlanTask{
						{TaskID: "task-a", TargetFiles: []string{"cmd/next.go"}, TaskKind: model.TaskKindCode},
						{TaskID: "task-b", TargetFiles: []string{"cmd/run.go"}, TaskKind: model.TaskKindCode},
					},
				}},
			}))

			writeTaskEvidence := func(taskID string, changedFile string) {
				t.Helper()
				taskEvidence := map[string]any{
					"task_id":             taskID,
					"run_summary_version": 1,
					"task_kind":           "code",
					"verdict":             "pass",
					"changed_files":       []string{changedFile},
					// target_files mirror each task's planned scope so the changed file
					// stays within scope and the disjoint files never overlap — this
					// test asserts dispatch-mode resolution, not the safety-net gates.
					"target_files":     []string{changedFile},
					"blockers":         []string{},
					"evidence_ref":     "test:" + taskID,
					"captured_at":      recordedAt.Add(-time.Minute).Format(time.RFC3339Nano),
					"freshness_inputs": state.ExpectedExecutionTaskFreshnessInputs(change, 1, taskID),
				}
				raw, err := json.Marshal(taskEvidence)
				require.NoError(t, err)
				taskPath := filepath.Join(state.EvidenceTasksDir(root, tt.slug), taskID+".json")
				require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
				require.NoError(t, os.WriteFile(taskPath, raw, 0o644))
			}
			writeTaskEvidence("task-a", "cmd/next.go")
			writeTaskEvidence("task-b", "cmd/run.go")

			result, err := SyncGovernedWaveExecution(root, change)
			require.NoError(t, err)
			require.Empty(t, result.Blockers)

			runs, err := state.LoadWaveRuns(root, tt.slug, 1)
			require.NoError(t, err)
			require.Len(t, runs, 1)
			assert.Equal(t, tt.want, runs[0].DispatchMode)
		})
	}
}

func TestSyncGovernedWaveExecution_PersistsIncompleteExecutionBlockerInSummary(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "wave-sync-incomplete"
	recordedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
	})
	// Plan has three tasks; evidence is recorded for only the first two.
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [ ] `+"`task-a`"+` A
  - target_files: ["cmd/next.go"]
  - task_kind: code

- [ ] `+"`task-b`"+` B
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`task-c`"+` C
  - target_files: ["cmd/status.go"]
  - task_kind: code
`)

	writeTaskEvidence := func(taskID string) {
		t.Helper()
		ev := map[string]any{
			"task_id":             taskID,
			"run_summary_version": 1,
			"task_kind":           "code",
			"verdict":             "pass",
			"changed_files":       []string{"cmd/next.go"},
			"blockers":            []string{},
			"evidence_ref":        "test:" + taskID,
			"captured_at":         recordedAt.Format(time.RFC3339Nano),
			"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, taskID),
		}
		raw, err := json.Marshal(ev)
		require.NoError(t, err)
		taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
		require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
		require.NoError(t, os.WriteFile(taskPath, raw, 0o644))
	}
	writeTaskEvidence("task-a")
	writeTaskEvidence("task-b")

	hasIncomplete := func(blockers []model.ReasonCode) bool {
		for _, b := range blockers {
			if b.Code == "incomplete_execution_task" && b.Detail == "task-c" {
				return true
			}
		}
		return false
	}

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	// The mutating sync returns the incomplete blocker (gates the advance)...
	require.True(t, hasIncomplete(result.Blockers), "sync must return incomplete_execution_task:task-c, got %+v", result.Blockers)

	// ...and it must be DURABLE in the saved summary's OpenBlockers so read-only
	// readiness (which surfaces summary OpenBlockers via SummaryIssues) reports
	// it too even though the summary is otherwise "ready" (issue #95 REQ-001).
	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	require.True(t, hasIncomplete(summary.OpenBlockers),
		"incomplete_execution_task must persist in the saved summary OpenBlockers, got %+v", summary.OpenBlockers)
	assert.Equal(t, model.ExecutionVerdictFail, summary.OverallVerdict)

	// End-to-end: read-only readiness must surface the durable blocker too — this
	// is exactly the path that previously dropped it once the partial summary was
	// written (refineS2WaveExecutionSkillBlockers short-circuits the preview).
	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)
	assert.True(t, hasIncomplete(readiness.Blockers),
		"read-only readiness must surface incomplete_execution_task:task-c, got %+v", readiness.Blockers)
}

func TestSyncGovernedWaveExecutionRejectsWaveEvidenceOlderThanTaskEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-sync-stale-wave-record"
	recordedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [ ] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)

	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"blockers":            []string{},
		"evidence_ref":        "test:task-a",
		"captured_at":         recordedAt.Add(time.Hour).Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.False(t, result.Updated)
	assert.True(t, hasWaveReasonCode(result.Blockers, "wave_orchestration_stale_task_evidence", "task-a"))
	assert.Contains(t, waveReasonMessage(result.Blockers, "wave_orchestration_stale_task_evidence"),
		"rerun wave-orchestration", "stale task-evidence blocker must carry an actionable remediation")

	_, err = state.LoadExecutionSummary(root, slug)
	require.Error(t, err)
}

func TestSyncGovernedWaveExecutionSurfacesParseIssuesAlongsideStaleEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-sync-stale-and-invalid"
	recordedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [ ] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)

	// Valid evidence captured after the wave record -> staleness.
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"blockers":            []string{},
		"evidence_ref":        "test:task-a",
		"captured_at":         recordedAt.Add(time.Hour).Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	tasksDir := state.EvidenceTasksDir(root, slug)
	require.NoError(t, os.MkdirAll(tasksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "task-a.json"), raw, 0o644))

	// A second evidence file that fails to parse -> parse issue that must not be
	// masked by the staleness early return.
	require.NoError(t, os.WriteFile(filepath.Join(tasksDir, "task-b.json"), []byte("{not valid json"), 0o644))

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "wave_orchestration_stale_task_evidence", "task-a"),
		"stale blocker must still be reported")
	assert.True(t, hasReasonCodeWithCode(result.Blockers, "task_evidence_invalid"),
		"invalid task evidence must not be masked by the stale blocker")
}

func hasReasonCodeWithCode(reasons []model.ReasonCode, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func TestSyncGovernedWaveExecution_DoesNotRewriteMatchingExecutionSummary(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-sync-stable"
	capturedAt := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  capturedAt,
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [x] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)
	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"target_files":        []string{"cmd/next.go"},
		"blockers":            []string{},
		"evidence_ref":        "test:task-a",
		"captured_at":         capturedAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	tasks, parseIssues, err := LoadExecutionTasksFromEvidence(root, slug, 1)
	require.NoError(t, err)
	require.Empty(t, parseIssues)

	matching := BuildExecutionSummary(1, tasks, capturedAt, &record)
	matching.TasksPlanHash, err = state.CurrentTasksPlanStructuralState(root, change)
	require.NoError(t, err)
	require.NoError(t, state.SaveExecutionSummary(root, slug, matching))

	runs, err := state.BuildWaveRuns(plan, 1, tasks, nil)
	require.NoError(t, err)
	require.NoError(t, state.SaveWaveRuns(root, slug, 1, runs))

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.False(t, result.Updated)
}

func TestSyncGovernedWaveExecution_DoesNotRewriteMatchingExecutionSummaryWithMonotonicTime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-sync-stable-monotonic"
	capturedAt := time.Now().UTC()
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  capturedAt,
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [ ] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)
	plan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)

	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"target_files":        []string{"cmd/next.go"},
		"blockers":            []string{},
		"evidence_ref":        "test:task-a",
		"captured_at":         capturedAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	tasks, parseIssues, err := LoadExecutionTasksFromEvidence(root, slug, 1)
	require.NoError(t, err)
	require.Empty(t, parseIssues)

	matching := BuildExecutionSummary(1, tasks, capturedAt, &record)
	matching.TasksPlanHash, err = state.CurrentTasksPlanStructuralState(root, change)
	require.NoError(t, err)
	require.NoError(t, state.SaveExecutionSummary(root, slug, matching))

	runs, err := state.BuildWaveRuns(plan, 1, tasks, nil)
	require.NoError(t, err)
	require.NoError(t, state.SaveWaveRuns(root, slug, 1, runs))

	summaryPath := filepath.Join(state.VerificationDir(root, slug), state.ExecutionSummaryFileName)
	infoBefore, err := os.Stat(summaryPath)
	require.NoError(t, err)

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.Empty(t, result.Blockers)

	infoAfter, err := os.Stat(summaryPath)
	require.NoError(t, err)
	assert.Equal(t, infoBefore.ModTime(), infoAfter.ModTime())
}

func TestCurrentTasksPlanStructuralHashIgnoresTargetFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	change := model.NewChange("plan-hash")
	require.NoError(t, state.SaveChange(root, change))

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	tasks := `# Tasks

- [ ] ` + "`task-a`" + ` Implement A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(tasks), 0o644))

	got, err := state.CurrentTasksPlanStructuralState(root, change)
	require.NoError(t, err)
	want, err := wave.TaskPlanStructuralHash(tasks)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	scope, err := state.CurrentTasksPlanScopeState(root, change)
	require.NoError(t, err)
	scopeWant, err := wave.TaskPlanScopeHash(tasks)
	require.NoError(t, err)
	assert.Equal(t, scopeWant, scope)
}

func TestSyncGovernedWaveExecution_ChecksOffPassingTasksInTasksChecklist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "wave-sync-checklist"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))
	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)

	bundleDir, err := state.GovernedBundleDir(root, change)
	if err != nil {
		t.Fatalf("resolve bundle dir: %v", err)
	}
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("mkdir bundle dir: %v", err)
	}
	content := `# Tasks

- [ ] ` + "`task-a`" + ` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code

- [ ] ` + "`task-b`" + ` Implement task B
  - target_files: ["cmd/status.go"]
  - task_kind: doc
`
	tasksPath := writeTasksAndMaterializeWavePlan(t, root, change, content)
	tasksPlanAt := record.Timestamp.Add(-1 * time.Minute)
	if err := os.Chtimes(tasksPath, tasksPlanAt, tasksPlanAt); err != nil {
		t.Fatalf("chtime tasks.md: %v", err)
	}

	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"blockers":            []string{},
		"evidence_ref":        "test:task-a",
		"captured_at":         record.Timestamp.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	if err != nil {
		t.Fatalf("marshal task evidence: %v", err)
	}
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	if err := os.MkdirAll(filepath.Dir(taskPath), 0o755); err != nil {
		t.Fatalf("mkdir task dir: %v", err)
	}
	if err := os.WriteFile(taskPath, raw, 0o644); err != nil {
		t.Fatalf("write task evidence: %v", err)
	}

	result, err := SyncGovernedWaveExecution(root, change)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Updated {
		t.Fatalf("expected sync result to report updated state, got %+v", result)
	}

	updatedContent, err := os.ReadFile(filepath.Join(bundleDir, "tasks.md"))
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	if !containsLine(string(updatedContent), "- [x] `task-a` Implement task A") {
		t.Fatalf("expected task-a to be checked off, got:\n%s", updatedContent)
	}
	if !containsLine(string(updatedContent), "- [ ] `task-b` Implement task B") {
		t.Fatalf("expected task-b to remain unchecked, got:\n%s", updatedContent)
	}
}

func containsLine(content, expected string) bool {
	for _, line := range strings.Split(content, "\n") {
		if line == expected {
			return true
		}
	}
	return false
}

func TestSyncGovernedWaveExecution_SharedSessionProducesBlocker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	slug := "wave-sync-shared-session"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
		RunVersion: 1,
		// A degraded_sequential token keeps the started parallel wave from tripping
		// the dispatch-evidence gate and requires no executor handles, so the only
		// blocker here is the session-isolation warning this test exercises.
		References: []string{"dispatch_mode:wave=1:degraded_sequential"},
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)
	tasksPath := writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [ ] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code

- [ ] `+"`task-b`"+` Implement task B
  - target_files: ["cmd/status.go"]
  - task_kind: code
`)
	tasksAt := record.Timestamp.Add(-time.Minute)
	require.NoError(t, os.Chtimes(tasksPath, tasksAt, tasksAt))

	sharedSessionID := "session-shared"
	for _, taskID := range []string{"task-a", "task-b"} {
		taskEvidence := map[string]any{
			"task_id":             taskID,
			"run_summary_version": 1,
			"task_kind":           "code",
			"verdict":             "pass",
			"blockers":            []string{},
			"evidence_ref":        "test:" + taskID,
			"captured_at":         record.Timestamp.Format(time.RFC3339Nano),
			"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, taskID),
			"session_id":          sharedSessionID,
		}
		raw, err := json.Marshal(taskEvidence)
		if err != nil {
			t.Fatalf("marshal task evidence: %v", err)
		}
		taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
		if err := os.MkdirAll(filepath.Dir(taskPath), 0o755); err != nil {
			t.Fatalf("mkdir task dir: %v", err)
		}
		if err := os.WriteFile(taskPath, raw, 0o644); err != nil {
			t.Fatalf("write task evidence: %v", err)
		}
	}

	result, err := SyncGovernedWaveExecution(root, change)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Blockers) != 1 {
		t.Fatalf("expected one session isolation blocker, got %+v", result)
	}
	if result.Blockers[0].Code != "session_isolation_warning" || result.Blockers[0].Detail != "session_id="+sharedSessionID+":shared_by=task-a,task-b" {
		t.Fatalf("unexpected blocker: %v", result.Blockers)
	}

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, result.Blockers[0].Code, result.Blockers[0].Detail))
}

func TestSyncGovernedWaveExecutionBlocksWhenTasksPlanChangedSinceEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-sync-plan-drift"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	initialTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Initial objective
  - target_files: ["cmd/next.go"]
  - task_kind: code
`
	tasksPath := writeTasksAndMaterializeWavePlan(t, root, change, initialTasks)
	initialHash, err := wave.TaskPlanStructuralHash(initialTasks)
	require.NoError(t, err)

	evidenceAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"evidence_ref":        "test:task-a",
		"captured_at":         evidenceAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        evidenceAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		TasksPlanHash:     initialHash,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-a",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: evidenceAt,
		}},
	}))

	updatedTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Updated objective
  - target_files: ["cmd/status.go"]
  - task_kind: code
`
	require.NoError(t, os.WriteFile(tasksPath, []byte(updatedTasks), 0o644))
	planChangedAt := evidenceAt.Add(2 * time.Minute)
	require.NoError(t, os.Chtimes(tasksPath, planChangedAt, planChangedAt))

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "tasks_plan_changed_since_task_evidence", "task-a"))
	currentTasksRaw, err := os.ReadFile(tasksPath)
	require.NoError(t, err)
	assert.Equal(t, updatedTasks, string(currentTasksRaw), "tasks.md must remain unchanged when task evidence is stale against the current plan")

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "tasks_plan_changed_since_task_evidence", "task-a"))
	assert.Equal(t, initialHash, summary.TasksPlanHash)

	planChangedAgainAt := planChangedAt.Add(time.Minute)
	require.NoError(t, os.Chtimes(tasksPath, planChangedAgainAt, planChangedAgainAt))

	result, err = SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "tasks_plan_changed_since_task_evidence", "task-a"))
	currentTasksRaw, err = os.ReadFile(tasksPath)
	require.NoError(t, err)
	assert.Equal(t, updatedTasks, string(currentTasksRaw), "repeated syncs must not rewrite tasks.md while stale-plan blockers remain")

	summary, err = state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "tasks_plan_changed_since_task_evidence", "task-a"))
	assert.Equal(t, initialHash, summary.TasksPlanHash)
}

func TestSyncGovernedWaveExecutionRematerializesScopeOnlyTaskPlanChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-sync-scope-only"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)

	initialTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`
	tasksPath := writeTasksAndMaterializeWavePlan(t, root, change, initialTasks)
	initialStructuralHash, err := wave.TaskPlanStructuralHash(initialTasks)
	require.NoError(t, err)
	initialScopeHash, err := wave.TaskPlanScopeHash(initialTasks)
	require.NoError(t, err)

	evidenceAt := record.Timestamp
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"target_files":        []string{"cmd/next.go"},
		"evidence_ref":        "test:task-a",
		"captured_at":         evidenceAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	// Scope-only change: a different (directory) target that still covers the
	// recorded changed file cmd/next.go, so this stays a pure rematerialization
	// test and the planned-scope escape audit (which now uses plan target_files)
	// does not legitimately fire on stranded evidence.
	scopeOnlyTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Implement task A
  - target_files: ["cmd"]
  - task_kind: code
`
	require.NoError(t, os.WriteFile(tasksPath, []byte(scopeOnlyTasks), 0o644))
	scopeOnlyAt := evidenceAt.Add(2 * time.Minute)
	require.NoError(t, os.Chtimes(tasksPath, scopeOnlyAt, scopeOnlyAt))
	updatedStructuralHash, err := wave.TaskPlanStructuralHash(scopeOnlyTasks)
	require.NoError(t, err)
	updatedScopeHash, err := wave.TaskPlanScopeHash(scopeOnlyTasks)
	require.NoError(t, err)
	require.Equal(t, initialStructuralHash, updatedStructuralHash)
	require.NotEqual(t, initialScopeHash, updatedScopeHash)

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.False(t, hasWaveReasonCode(result.Blockers, "tasks_plan_changed_since_task_evidence", "task-a"))

	wavePlan, err := state.LoadWavePlanForChange(root, change)
	require.NoError(t, err)
	assert.Equal(t, updatedStructuralHash, wavePlan.TasksPlanHash)
	assert.Equal(t, updatedStructuralHash, wavePlan.TasksPlanStructuralHash)
	assert.Equal(t, updatedScopeHash, wavePlan.TasksPlanScopeHash)
	require.Len(t, wavePlan.Waves, 1)
	require.Len(t, wavePlan.Waves[0].Tasks, 1)
	assert.Equal(t, []string{"cmd"}, wavePlan.Waves[0].Tasks[0].TargetFiles)

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.Equal(t, updatedStructuralHash, summary.TasksPlanHash)
	assert.Empty(t, summary.OpenBlockers)

	currentTasksRaw, err := os.ReadFile(tasksPath)
	require.NoError(t, err)
	assert.Contains(t, string(currentTasksRaw), "- [x] `task-a` Implement task A")
	assert.Contains(t, string(currentTasksRaw), `target_files: ["cmd"]`)
}

func TestSyncGovernedWaveExecutionClearsPlanDriftAfterFreshEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-sync-plan-drift-recovery"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	initialTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Initial objective
  - target_files: ["cmd/next.go"]
  - task_kind: code
`
	tasksPath := writeTasksAndMaterializeWavePlan(t, root, change, initialTasks)
	initialHash, err := wave.TaskPlanStructuralHash(initialTasks)
	require.NoError(t, err)

	firstEvidenceAt := record.Timestamp
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"evidence_ref":        "test:task-a",
		"captured_at":         firstEvidenceAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        firstEvidenceAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		TasksPlanHash:     initialHash,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-a",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: firstEvidenceAt,
		}},
	}))

	updatedTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Updated objective
  - target_files: ["cmd/status.go"]
  - task_kind: code
`
	require.NoError(t, os.WriteFile(tasksPath, []byte(updatedTasks), 0o644))
	planChangedAt := firstEvidenceAt.Add(2 * time.Minute)
	require.NoError(t, os.Chtimes(tasksPath, planChangedAt, planChangedAt))

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "tasks_plan_changed_since_task_evidence", "task-a"))

	secondEvidenceAt := planChangedAt.Add(2 * time.Minute)
	record = model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  secondEvidenceAt,
		RunVersion: 2,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	taskEvidence = map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 2,
		"task_kind":           "code",
		"verdict":             "pass",
		"evidence_ref":        "test:task-a",
		"captured_at":         secondEvidenceAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 2, "task-a"),
	}
	raw, err = json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath = filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	result, err = SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.False(t, hasWaveReasonCode(result.Blockers, "tasks_plan_changed_since_task_evidence", "task-a"))

	recoveredTasks := `# Tasks

- [x] ` + "`task-a`" + ` Updated objective
  - target_files: ["cmd/status.go"]
  - task_kind: code
`
	currentTasksRaw, err := os.ReadFile(tasksPath)
	require.NoError(t, err)
	assert.Equal(t, recoveredTasks, string(currentTasksRaw))

	updatedHash, err := wave.TaskPlanStructuralHash(updatedTasks)
	require.NoError(t, err)
	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.Empty(t, summary.OpenBlockers)
	assert.Equal(t, updatedHash, summary.TasksPlanHash)
	assert.Equal(t, 2, summary.RunSummaryVersion)
}

func TestSyncGovernedWaveExecutionBlocksFirstSummaryWhenTasksChangedAfterEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "wave-sync-first-summary-plan-drift"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)

	bundleDir, err := state.GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	initialTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Initial objective
  - target_files: ["cmd/next.go"]
  - task_kind: code
`
	tasksPath := writeTasksAndMaterializeWavePlan(t, root, change, initialTasks)

	evidenceAt := record.Timestamp
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"evidence_ref":        "test:task-a",
		"captured_at":         evidenceAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	updatedTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Updated objective
  - target_files: ["cmd/status.go"]
  - task_kind: code
`
	require.NoError(t, os.WriteFile(tasksPath, []byte(updatedTasks), 0o644))
	planChangedAt := evidenceAt.Add(2 * time.Minute)
	require.NoError(t, os.Chtimes(tasksPath, planChangedAt, planChangedAt))

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "tasks_plan_changed_since_task_evidence", "task-a"))

	currentTasksRaw, err := os.ReadFile(tasksPath)
	require.NoError(t, err)
	assert.Equal(t, updatedTasks, string(currentTasksRaw), "first sync must not mark tasks complete when the plan changed after evidence capture")

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "tasks_plan_changed_since_task_evidence", "task-a"))
	currentHash, err := wave.TaskPlanStructuralHash(updatedTasks)
	require.NoError(t, err)
	assert.NotEqual(t, currentHash, summary.TasksPlanHash, "first sync must not bind stale evidence to the current tasks hash")
}

func scopeEscapePlan(taskID string, targets ...string) model.WavePlan {
	return model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Tasks:     []model.WavePlanTask{{TaskID: taskID, TargetFiles: targets}},
		}},
	}
}

func TestTaskChangedFileScopeEscapeBlockers_OutOfScopeBlocks(t *testing.T) {
	t.Parallel()
	// Coverage is judged against the PLAN's target_files. The evidence widens its
	// own target_files to cover internal/b.go, but the plan only grants
	// internal/a.go, so the audit must still fire (REQ-002 anti-bypass).
	plan := scopeEscapePlan("t-01", "internal/a.go")
	tasks := []model.ExecutionTaskSummary{
		{
			TaskID:       "t-01",
			TargetFiles:  []string{"internal/a.go", "internal/b.go"},
			ChangedFiles: []string{"internal/a.go", "internal/b.go"},
		},
	}
	blockers := TaskChangedFileScopeEscapeBlockers(plan, tasks)
	require.Len(t, blockers, 1)
	assert.Equal(t, "task_changed_file_scope_escape", blockers[0].Code)
	assert.Equal(t, "t-01:internal/b.go", blockers[0].Detail)
}

func TestTaskChangedFileScopeEscapeBlockers_DirectoryTargetCovers(t *testing.T) {
	t.Parallel()
	// A directory target covers a changed file nested beneath it (REQ-002 glob/dir
	// coverage), so no scope-escape blocker is produced.
	plan := scopeEscapePlan("t-01", "internal/engine/")
	tasks := []model.ExecutionTaskSummary{
		{
			TaskID:       "t-01",
			ChangedFiles: []string{"internal/engine/wave/wave.go"},
		},
	}
	assert.Empty(t, TaskChangedFileScopeEscapeBlockers(plan, tasks))
}

func TestTaskChangedFileScopeEscapeBlockers_MalformedGlobTargetFailsClosed(t *testing.T) {
	t.Parallel()
	// Invalid glob syntax is broad for planner conflict detection, but it must not
	// grant coverage authority in the post-run scope audit. Treating it as a
	// match-all here would let malformed target_files suppress scope escapes.
	plan := scopeEscapePlan("t-01", "internal/[")
	tasks := []model.ExecutionTaskSummary{
		{
			TaskID:       "t-01",
			ChangedFiles: []string{"cmd/run.go"},
		},
	}
	blockers := TaskChangedFileScopeEscapeBlockers(plan, tasks)
	require.Len(t, blockers, 1)
	assert.Equal(t, "task_changed_file_scope_escape", blockers[0].Code)
	assert.Equal(t, "t-01:cmd/run.go", blockers[0].Detail)
}

func TestTaskChangedFileScopeEscapeBlockers_EmptyPlanTargetsFailClosed(t *testing.T) {
	t.Parallel()
	// The shared-worktree safety model is declared target_files plus exhaustive
	// changed_files. A planned task with no target_files has no declared write
	// scope, so any recorded changed_file must fail closed as a scope escape.
	plan := scopeEscapePlan("t-01")
	tasks := []model.ExecutionTaskSummary{
		{
			TaskID:       "t-01",
			ChangedFiles: []string{"internal/a.go"},
		},
	}
	blockers := TaskChangedFileScopeEscapeBlockers(plan, tasks)
	require.Len(t, blockers, 1)
	assert.Equal(t, "task_changed_file_scope_escape", blockers[0].Code)
	assert.Equal(t, "t-01:internal/a.go", blockers[0].Detail)
}

func TestTaskChangedFileScopeEscapeBlockers_AllWithinTargetNoBlock(t *testing.T) {
	t.Parallel()
	plan := scopeEscapePlan("t-01", "internal/a.go", "internal/b.go")
	tasks := []model.ExecutionTaskSummary{
		{
			TaskID:       "t-01",
			ChangedFiles: []string{"internal/a.go", "internal/b.go"},
		},
	}
	assert.Empty(t, TaskChangedFileScopeEscapeBlockers(plan, tasks))
}

func TestTaskChangedFileScopeEscapeBlockers_OrphanEvidenceSkipped(t *testing.T) {
	t.Parallel()
	// Evidence for a task absent from the plan is an orphan owned by a dedicated
	// gate; the scope-escape audit only adjudicates planned tasks.
	plan := scopeEscapePlan("t-01", "internal/a.go")
	tasks := []model.ExecutionTaskSummary{
		{TaskID: "t-99", ChangedFiles: []string{"internal/z.go"}},
	}
	assert.Empty(t, TaskChangedFileScopeEscapeBlockers(plan, tasks))
}

func TestParallelWaveChangedFileOverlapBlockers_ParallelOverlapBlocks(t *testing.T) {
	t.Parallel()
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  true,
			Tasks:     []model.WavePlanTask{{TaskID: "task-a"}, {TaskID: "task-b"}},
		}},
	}
	tasks := []model.ExecutionTaskSummary{
		{TaskID: "task-a", ChangedFiles: []string{"internal/x.go"}},
		{TaskID: "task-b", ChangedFiles: []string{"internal/x.go"}},
	}
	blockers := ParallelWaveChangedFileOverlapBlockers(plan, tasks)
	require.Len(t, blockers, 1)
	assert.Equal(t, "parallel_wave_changed_file_overlap", blockers[0].Code)
	assert.Equal(t, "1:internal/x.go:task-a,task-b", blockers[0].Detail)
}

func TestParallelWaveChangedFileOverlapBlockers_SequentialOverlapAllowed(t *testing.T) {
	t.Parallel()
	// Two non-parallel (sequential) waves sharing a changed file must not block:
	// they cannot run concurrently, so they cannot clobber each other.
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{
			{WaveIndex: 1, Parallel: false, Tasks: []model.WavePlanTask{{TaskID: "task-a"}}},
			{WaveIndex: 2, Parallel: false, Tasks: []model.WavePlanTask{{TaskID: "task-b"}}},
		},
	}
	tasks := []model.ExecutionTaskSummary{
		{TaskID: "task-a", ChangedFiles: []string{"internal/x.go"}},
		{TaskID: "task-b", ChangedFiles: []string{"internal/x.go"}},
	}
	assert.Empty(t, ParallelWaveChangedFileOverlapBlockers(plan, tasks))
}

func TestParallelWaveChangedFileOverlapBlockers_SameWaveDistinctFilesNoBlock(t *testing.T) {
	t.Parallel()
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  true,
			Tasks:     []model.WavePlanTask{{TaskID: "task-a"}, {TaskID: "task-b"}},
		}},
	}
	tasks := []model.ExecutionTaskSummary{
		{TaskID: "task-a", ChangedFiles: []string{"internal/a.go"}},
		{TaskID: "task-b", ChangedFiles: []string{"internal/b.go"}},
	}
	assert.Empty(t, ParallelWaveChangedFileOverlapBlockers(plan, tasks))
}

func TestDispatchEvidenceBlockers_StartedParallelWaveMissingTokenBlocks(t *testing.T) {
	t.Parallel()
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  true,
			Tasks:     []model.WavePlanTask{{TaskID: "task-a"}, {TaskID: "task-b"}},
		}},
	}
	// Both planned tasks have recorded evidence, so the wave is started; with no
	// dispatch token the engine fails closed instead of inferring parallel (REQ-004).
	tasks := []model.ExecutionTaskSummary{
		{TaskID: "task-a"},
		{TaskID: "task-b"},
	}
	blockers := DispatchEvidenceBlockers(plan, tasks, nil)
	require.Len(t, blockers, 1)
	assert.Equal(t, "dispatch_mode_absent_on_started_parallel_wave", blockers[0].Code)
	assert.Equal(t, "1", blockers[0].Detail)
}

func TestDispatchEvidenceBlockers_ValidTokensDoNotBlock(t *testing.T) {
	t.Parallel()
	// A parallel_subagents token and a degraded_sequential token are both explicit
	// dispatch evidence, so neither started parallel wave is blocked.
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{
			{WaveIndex: 1, Parallel: true, Tasks: []model.WavePlanTask{{TaskID: "task-a"}}},
			{WaveIndex: 2, Parallel: true, Tasks: []model.WavePlanTask{{TaskID: "task-b"}}},
		},
	}
	tasks := []model.ExecutionTaskSummary{
		{TaskID: "task-a"},
		{TaskID: "task-b"},
	}
	dispatchModes := map[int]model.WaveDispatchMode{
		1: model.WaveDispatchParallel,
		2: model.WaveDispatchDegradedSequential,
	}
	assert.Empty(t, DispatchEvidenceBlockers(plan, tasks, dispatchModes))
}

func TestDispatchEvidenceBlockers_NonParallelAndPendingWavesDoNotBlock(t *testing.T) {
	t.Parallel()
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{
			// Non-parallel waves never require dispatch evidence.
			{WaveIndex: 1, Parallel: false, Tasks: []model.WavePlanTask{{TaskID: "task-a"}}},
			// Parallel but not started (no recorded task evidence), so no blocker yet.
			{WaveIndex: 2, Parallel: true, Tasks: []model.WavePlanTask{{TaskID: "task-b"}}},
		},
	}
	tasks := []model.ExecutionTaskSummary{
		{TaskID: "task-a"},
	}
	assert.Empty(t, DispatchEvidenceBlockers(plan, tasks, nil))
}

func TestExecutorAgentBlockers_ParallelSubagentsMissingHandleBlocks(t *testing.T) {
	t.Parallel()
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  true,
			Tasks:     []model.WavePlanTask{{TaskID: "task-a"}, {TaskID: "task-b"}},
		}},
	}
	dispatchModes := map[int]model.WaveDispatchMode{1: model.WaveDispatchParallel}
	// Only task-a has a recorded handle, so task-b is blocked (REQ-005).
	handles := map[int]map[string]string{1: {"task-a": "agent-a"}}
	tasks := []model.ExecutionTaskSummary{{TaskID: "task-a"}, {TaskID: "task-b"}}
	blockers := ExecutorAgentBlockers(plan, tasks, dispatchModes, handles)
	require.Len(t, blockers, 1)
	assert.Equal(t, "executor_agent_missing", blockers[0].Code)
	assert.Equal(t, "1:task-b", blockers[0].Detail)
}

func TestExecutorAgentBlockers_AllHandlesPresentNoBlock(t *testing.T) {
	t.Parallel()
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  true,
			Tasks:     []model.WavePlanTask{{TaskID: "task-a"}, {TaskID: "task-b"}},
		}},
	}
	dispatchModes := map[int]model.WaveDispatchMode{1: model.WaveDispatchParallel}
	handles := map[int]map[string]string{1: {"task-a": "agent-a", "task-b": "agent-b"}}
	tasks := []model.ExecutionTaskSummary{{TaskID: "task-a"}, {TaskID: "task-b"}}
	assert.Empty(t, ExecutorAgentBlockers(plan, tasks, dispatchModes, handles))
}

func TestExecutorAgentBlockers_DegradedAndNonParallelWavesRequireNoHandles(t *testing.T) {
	t.Parallel()
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{
			// Degraded-sequential dispatch requires no executor handles.
			{WaveIndex: 1, Parallel: true, Tasks: []model.WavePlanTask{{TaskID: "task-a"}}},
			// A non-parallel wave never requires handles regardless of dispatch map.
			{WaveIndex: 2, Parallel: false, Tasks: []model.WavePlanTask{{TaskID: "task-b"}}},
		},
	}
	dispatchModes := map[int]model.WaveDispatchMode{1: model.WaveDispatchDegradedSequential}
	tasks := []model.ExecutionTaskSummary{{TaskID: "task-a"}, {TaskID: "task-b"}}
	assert.Empty(t, ExecutorAgentBlockers(plan, tasks, dispatchModes, nil))
}

func TestExecutorAgentBlockers_NonParallelWaveWithStaleParallelTokenRequiresNoHandles(t *testing.T) {
	t.Parallel()
	// A non-parallel wave can still carry a stale parallel_subagents dispatch
	// token (recorded while parallel, then parallelization turned off so
	// ApplyEffectiveParallel cleared the flag). REQ-005 says non-parallel waves
	// MUST NOT require handles, so no executor_agent_missing blocker is emitted
	// even though the dispatch map still names parallel_subagents for the wave.
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  false,
			Tasks:     []model.WavePlanTask{{TaskID: "task-a"}, {TaskID: "task-b"}},
		}},
	}
	dispatchModes := map[int]model.WaveDispatchMode{1: model.WaveDispatchParallel}
	tasks := []model.ExecutionTaskSummary{{TaskID: "task-a"}, {TaskID: "task-b"}}
	assert.Empty(t, ExecutorAgentBlockers(plan, tasks, dispatchModes, nil))
}

func TestExecutorAgentBlockers_PendingParallelSubagentsWaveRequiresNoHandles(t *testing.T) {
	t.Parallel()
	// A pending wave may already have a future dispatch token in the verification
	// references, but no planned task has recorded execution evidence yet. Match
	// BuildWaveRuns/DispatchEvidenceBlockers: handle completeness is evaluated
	// only after the wave has started.
	plan := model.WavePlan{
		Version: model.WavePlanVersion,
		Waves: []model.WavePlanWave{{
			WaveIndex: 1,
			Parallel:  true,
			Tasks:     []model.WavePlanTask{{TaskID: "task-a"}, {TaskID: "task-b"}},
		}},
	}
	dispatchModes := map[int]model.WaveDispatchMode{1: model.WaveDispatchParallel}
	assert.Empty(t, ExecutorAgentBlockers(plan, nil, dispatchModes, nil))
}

// TestSyncGovernedWaveExecutionSurfacesScopeEscapeBlocker proves the new
// scope-escape gate folds into the single wave-execution assembly point: the
// blocker is both returned from the sync and made durable in the saved
// summary's OpenBlockers (issue #95 durability), so read-only readiness reports
// it too.
func TestSyncGovernedWaveExecutionSurfacesScopeEscapeBlocker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "wave-sync-scope-escape"
	recordedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
	})
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [x] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)

	// changed_files escapes the planned target_files (cmd/run.go is not covered).
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go", "cmd/run.go"},
		"target_files":        []string{"cmd/next.go"},
		"blockers":            []string{},
		"evidence_ref":        "test:task-a",
		"captured_at":         recordedAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "task_changed_file_scope_escape", "task-a:cmd/run.go"),
		"sync must return task_changed_file_scope_escape:task-a:cmd/run.go, got %+v", result.Blockers)

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "task_changed_file_scope_escape", "task-a:cmd/run.go"),
		"scope-escape blocker must persist in saved summary OpenBlockers, got %+v", summary.OpenBlockers)

	readiness, err := EvaluateGovernanceReadiness(root, change, GovernanceReadinessOptions{})
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(readiness.Blockers, "task_changed_file_scope_escape", "task-a:cmd/run.go"),
		"read-only readiness must surface the scope-escape blocker, got %+v", readiness.Blockers)
}

// TestSyncGovernedWaveExecutionSurfacesParallelOverlapBlocker proves the
// parallel-overlap gate folds into the same assembly point. Two file-disjoint,
// dependency-free tasks land in one wave that the default forced-parallel mode
// marks Parallel; both record the same changed file, which is the clobber risk
// REQ-003 blocks.
func TestSyncGovernedWaveExecutionSurfacesParallelOverlapBlocker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "wave-sync-parallel-overlap"
	recordedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
		References: []string{"dispatch_mode:wave=1:parallel_subagents"},
	})
	// Disjoint target_files keep both tasks in the same (parallel) wave.
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [x] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code

- [x] `+"`task-b`"+` Implement task B
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)

	writeTaskEvidence := func(taskID, targetFile string) {
		t.Helper()
		taskEvidence := map[string]any{
			"task_id":             taskID,
			"run_summary_version": 1,
			"task_kind":           "code",
			"verdict":             "pass",
			// Both tasks record the SAME changed file (cmd/shared.go), which is the
			// same-wave overlap REQ-003 blocks. Because the plan target_files do not
			// grant cmd/shared.go to either task, scope-escape may also fire; this test
			// asserts the overlap blocker is still returned and persisted.
			"changed_files":    []string{targetFile, "cmd/shared.go"},
			"target_files":     []string{targetFile, "cmd/shared.go"},
			"blockers":         []string{},
			"evidence_ref":     "test:" + taskID,
			"captured_at":      recordedAt.Format(time.RFC3339Nano),
			"freshness_inputs": expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, taskID),
		}
		raw, err := json.Marshal(taskEvidence)
		require.NoError(t, err)
		taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
		require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
		require.NoError(t, os.WriteFile(taskPath, raw, 0o644))
	}
	writeTaskEvidence("task-a", "cmd/next.go")
	writeTaskEvidence("task-b", "cmd/run.go")

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "parallel_wave_changed_file_overlap", "1:cmd/shared.go:task-a,task-b"),
		"sync must return parallel_wave_changed_file_overlap:1:cmd/shared.go:task-a,task-b, got %+v", result.Blockers)

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "parallel_wave_changed_file_overlap", "1:cmd/shared.go:task-a,task-b"),
		"overlap blocker must persist in saved summary OpenBlockers, got %+v", summary.OpenBlockers)
}

// TestSyncGovernedWaveExecutionBlocksStartedParallelWaveMissingDispatchEvidence
// proves the dispatch-evidence gate folds into the same assembly point. Two
// file-disjoint, dependency-free tasks land in one wave that default forced
// parallel marks Parallel; with no dispatch_mode reference recorded, the engine
// must not infer parallel dispatch and instead fails closed with a blocker that
// is durable in the saved summary's OpenBlockers (REQ-004). The blocked wave also
// records no dispatch mode.
func TestSyncGovernedWaveExecutionBlocksStartedParallelWaveMissingDispatchEvidence(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "wave-sync-missing-dispatch"
	recordedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	// No dispatch_mode reference: the started parallel wave has no dispatch evidence.
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
	})
	// Disjoint target_files keep both tasks in the same (parallel) wave.
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [x] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code

- [x] `+"`task-b`"+` Implement task B
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)

	writeTaskEvidence := func(taskID, targetFile string) {
		t.Helper()
		taskEvidence := map[string]any{
			"task_id":             taskID,
			"run_summary_version": 1,
			"task_kind":           "code",
			"verdict":             "pass",
			// changed_files stay within each task's own target so only the
			// dispatch-evidence gate fires here, not scope-escape or overlap.
			"changed_files":    []string{targetFile},
			"target_files":     []string{targetFile},
			"blockers":         []string{},
			"evidence_ref":     "test:" + taskID,
			"captured_at":      recordedAt.Format(time.RFC3339Nano),
			"freshness_inputs": expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, taskID),
		}
		raw, err := json.Marshal(taskEvidence)
		require.NoError(t, err)
		taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
		require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
		require.NoError(t, os.WriteFile(taskPath, raw, 0o644))
	}
	writeTaskEvidence("task-a", "cmd/next.go")
	writeTaskEvidence("task-b", "cmd/run.go")

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "dispatch_mode_absent_on_started_parallel_wave", "1"),
		"sync must return dispatch_mode_absent_on_started_parallel_wave:1, got %+v", result.Blockers)

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "dispatch_mode_absent_on_started_parallel_wave", "1"),
		"dispatch-evidence blocker must persist in saved summary OpenBlockers, got %+v", summary.OpenBlockers)

	runs, err := state.LoadWaveRuns(root, slug, 1)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Empty(t, runs[0].DispatchMode, "a wave blocked for missing dispatch evidence must not record an inferred mode")
}

// TestSyncGovernedWaveExecutionBlocksParallelSubagentsWaveMissingExecutorHandle
// proves the executor-handle gate folds into the same assembly point. The wave is
// dispatched parallel_subagents with a handle for task-a but none for task-b, so
// the engine fails closed with a per-task blocker that is durable in the saved
// summary's OpenBlockers (REQ-005).
func TestSyncGovernedWaveExecutionBlocksParallelSubagentsWaveMissingExecutorHandle(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "wave-sync-missing-executor-handle"
	recordedAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	// parallel_subagents dispatch with a handle for task-a only; task-b is missing.
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  recordedAt,
		RunVersion: 1,
		References: []string{
			"dispatch_mode:wave=1:parallel_subagents",
			"executor_agent:wave=1:task=task-a:agent-a",
		},
	})
	// Disjoint target_files keep both tasks in the same (parallel) wave.
	writeTasksAndMaterializeWavePlan(t, root, change, `# Tasks

- [x] `+"`task-a`"+` Implement task A
  - target_files: ["cmd/next.go"]
  - task_kind: code

- [x] `+"`task-b`"+` Implement task B
  - target_files: ["cmd/run.go"]
  - task_kind: code
`)

	writeTaskEvidence := func(taskID, targetFile string) {
		t.Helper()
		taskEvidence := map[string]any{
			"task_id":             taskID,
			"run_summary_version": 1,
			"task_kind":           "code",
			"verdict":             "pass",
			"changed_files":       []string{targetFile},
			"target_files":        []string{targetFile},
			"blockers":            []string{},
			"evidence_ref":        "test:" + taskID,
			"captured_at":         recordedAt.Format(time.RFC3339Nano),
			"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, taskID),
		}
		raw, err := json.Marshal(taskEvidence)
		require.NoError(t, err)
		taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), taskID+".json")
		require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
		require.NoError(t, os.WriteFile(taskPath, raw, 0o644))
	}
	writeTaskEvidence("task-a", "cmd/next.go")
	writeTaskEvidence("task-b", "cmd/run.go")

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "executor_agent_missing", "1:task-b"),
		"sync must return executor_agent_missing:1:task-b, got %+v", result.Blockers)
	assert.False(t, hasWaveReasonCode(result.Blockers, "executor_agent_missing", "1:task-a"),
		"task-a has a recorded handle and must not be blocked, got %+v", result.Blockers)

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "executor_agent_missing", "1:task-b"),
		"executor-handle blocker must persist in saved summary OpenBlockers, got %+v", summary.OpenBlockers)
}

// TestSyncGovernedWaveExecutionSuppressesSafetyNetsUnderPlanDrift proves the
// safety-net gate honors the same len(planDriftBlockers)==0 suppression guard
// that incomplete-execution blockers use: when plan drift is present (which owns
// its own remediation), the scope-escape blocker is not surfaced.
func TestSyncGovernedWaveExecutionSuppressesSafetyNetsUnderPlanDrift(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	slug := "wave-sync-safetynet-suppressed"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	record := model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)

	initialTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Initial objective
  - target_files: ["cmd/next.go"]
  - task_kind: code
`
	tasksPath := writeTasksAndMaterializeWavePlan(t, root, change, initialTasks)

	evidenceAt := record.Timestamp
	// changed_files escapes target_files (would normally fire scope-escape)...
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/run.go"},
		"target_files":        []string{"cmd/next.go"},
		"evidence_ref":        "test:task-a",
		"captured_at":         evidenceAt.Format(time.RFC3339Nano),
		"freshness_inputs":    expectedTaskFreshnessInputsForWavePlan(t, root, change, 1, "task-a"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	// ...but the tasks plan changed after evidence capture, raising plan drift,
	// which must suppress the safety-net blockers.
	updatedTasks := `# Tasks

- [ ] ` + "`task-a`" + ` Updated objective
  - target_files: ["cmd/status.go"]
  - task_kind: code
`
	require.NoError(t, os.WriteFile(tasksPath, []byte(updatedTasks), 0o644))
	planChangedAt := evidenceAt.Add(2 * time.Minute)
	require.NoError(t, os.Chtimes(tasksPath, planChangedAt, planChangedAt))

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.True(t, hasWaveReasonCode(result.Blockers, "tasks_plan_changed_since_task_evidence", "task-a"),
		"plan-drift blocker must be present, got %+v", result.Blockers)
	assert.False(t, hasReasonCodeWithCode(result.Blockers, "task_changed_file_scope_escape"),
		"safety-net blockers must be suppressed under plan drift, got %+v", result.Blockers)
}

func hasWaveReasonCode(reasons []model.ReasonCode, code, detail string) bool {
	for _, reason := range reasons {
		if reason.Code == code && reason.Detail == detail {
			return true
		}
	}
	return false
}

func waveReasonMessage(reasons []model.ReasonCode, code string) string {
	for _, reason := range reasons {
		if reason.Code == code {
			return reason.Message
		}
	}
	return ""
}

func TestWaveRunsEqualDetectsDispatchModeChange(t *testing.T) {
	t.Parallel()

	base := []model.WaveRun{{
		WaveIndex:         1,
		RunSummaryVersion: 1,
		Verdict:           model.WaveVerdictPass,
		DispatchMode:      model.WaveDispatchParallel,
	}}
	// Identical except DispatchMode. waveRunsEqual gates whether wave-run evidence
	// is re-persisted, so a dispatch-mode flip (e.g. parallelization off->forced)
	// must register as a difference or the stale mode is never written to disk.
	flipped := []model.WaveRun{{
		WaveIndex:         1,
		RunSummaryVersion: 1,
		Verdict:           model.WaveVerdictPass,
		DispatchMode:      "",
	}}

	assert.True(t, waveRunsEqual(base, base), "identical runs are equal")
	assert.False(t, waveRunsEqual(base, flipped), "a dispatch_mode change must not be treated as equal")
}
