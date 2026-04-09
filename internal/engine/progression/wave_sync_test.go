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
				Blockers:    []model.ReasonCode{model.NewReasonCode("needs_retry", "")},
				CapturedAt:  capturedAt.Add(time.Second),
				EvidenceRef: "artifacts/runtime/task-b.json",
			},
		},
		capturedAt,
		&model.VerificationRecord{Blockers: []model.ReasonCode{model.NewReasonCode("wave_blocker", "review")}},
	)

	assert.Equal(t, model.ExecutionVerdictFail, summary.OverallVerdict)
	assert.Equal(t, []string{"task-a"}, summary.CompletedTasks)
	assert.Equal(t, []string{"task-b"}, summary.NonPassTasks)
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "task", "task-b:needs_retry"))
	assert.True(t, hasWaveReasonCode(summary.OpenBlockers, "wave_blocker", "review"))
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

func TestTasksPlanChangedSinceTaskEvidenceBlockersTracksOnlyStaleTasks(t *testing.T) {
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
		tasksPlanUpdatedAt,
	)

	assert.Equal(t, []string{
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
		tasksPlanUpdatedAt,
	)

	assert.Equal(t, []string{
		"tasks_plan_changed_since_task_evidence:stale-task",
	}, blockers)
}

func Test_deriveTaskIDFromEvidenceFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input, want string
	}{
		{"t1.json", "t1"},
		{"t1--session123.json", "t1"},
		{"task-abc.json", "task-abc"},
	}
	for _, tt := range tests {
		got := deriveTaskIDFromEvidenceFilename(tt.input)
		if got != tt.want {
			t.Errorf("deriveTaskIDFromEvidenceFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
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

	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"blockers":            []string{},
		"captured_at":         time.Now().UTC().Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(taskEvidence)
	if err != nil {
		t.Fatalf("marshal task evidence: %v", err)
	}
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), "task-a.json")
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

	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"blockers":            []string{},
		"captured_at":         capturedAt.Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	tasks, parseIssues, err := LoadExecutionTasksFromEvidence(root, slug, 1)
	require.NoError(t, err)
	require.Empty(t, parseIssues)

	matching := BuildExecutionSummary(1, tasks, capturedAt, &record)
	require.NoError(t, state.SaveExecutionSummary(root, slug, matching))

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

	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"blockers":            []string{},
		"captured_at":         capturedAt.Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), "task-a.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskPath), 0o755))
	require.NoError(t, os.WriteFile(taskPath, raw, 0o644))

	tasks, parseIssues, err := LoadExecutionTasksFromEvidence(root, slug, 1)
	require.NoError(t, err)
	require.Empty(t, parseIssues)

	matching := BuildExecutionSummary(1, tasks, capturedAt, &record)
	require.NoError(t, state.SaveExecutionSummary(root, slug, matching))

	summaryPath := filepath.Join(state.VerificationDir(root, slug), state.ExecutionSummaryFileName)
	infoBefore, err := os.Stat(summaryPath)
	require.NoError(t, err)

	result, err := SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	assert.False(t, result.Updated)

	infoAfter, err := os.Stat(summaryPath)
	require.NoError(t, err)
	assert.Equal(t, infoBefore.ModTime(), infoAfter.ModTime())
}

func TestCurrentTasksPlanHashUsesSemanticTaskPlanHash(t *testing.T) {
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

	got, _, err := currentTasksPlanState(root, change)
	require.NoError(t, err)
	want, err := wave.TaskPlanSemanticHash(tasks)
	require.NoError(t, err)
	assert.Equal(t, want, got)
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
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	if err := os.WriteFile(tasksPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write tasks.md: %v", err)
	}
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
		"captured_at":         record.Timestamp.Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(taskEvidence)
	if err != nil {
		t.Fatalf("marshal task evidence: %v", err)
	}
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), "task-a.json")
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
		Timestamp:  time.Now().UTC(),
		RunVersion: 1,
	}
	writeVerificationForTest(t, root, slug, SkillWaveOrchestration, record)

	sharedSessionID := "session-shared"
	for _, taskID := range []string{"task-a", "task-b"} {
		taskEvidence := map[string]any{
			"task_id":             taskID,
			"run_summary_version": 1,
			"task_kind":           "code",
			"verdict":             "pass",
			"blockers":            []string{},
			"captured_at":         time.Now().UTC().Format(time.RFC3339Nano),
			"session_id":          sharedSessionID,
		}
		raw, err := json.Marshal(taskEvidence)
		if err != nil {
			t.Fatalf("marshal task evidence: %v", err)
		}
		taskPath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), taskID+".json")
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
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(initialTasks), 0o644))
	initialHash, err := wave.TaskPlanSemanticHash(initialTasks)
	require.NoError(t, err)

	evidenceAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"captured_at":         evidenceAt.Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), "task-a.json")
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
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(initialTasks), 0o644))
	initialHash, err := wave.TaskPlanSemanticHash(initialTasks)
	require.NoError(t, err)

	firstEvidenceAt := record.Timestamp
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"captured_at":         firstEvidenceAt.Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), "task-a.json")
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

	taskEvidence = map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 2,
		"task_kind":           "code",
		"verdict":             "pass",
		"captured_at":         secondEvidenceAt.Format(time.RFC3339Nano),
	}
	raw, err = json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath = filepath.Join(state.EvidenceTasksDir(root, slug, 2), "task-a.json")
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

	updatedHash, err := wave.TaskPlanSemanticHash(updatedTasks)
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
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(initialTasks), 0o644))

	evidenceAt := record.Timestamp
	taskEvidence := map[string]any{
		"task_id":             "task-a",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"captured_at":         evidenceAt.Format(time.RFC3339Nano),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskPath := filepath.Join(state.EvidenceTasksDir(root, slug, 1), "task-a.json")
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
	assert.Empty(t, summary.TasksPlanHash, "first sync must not bind stale evidence to the current tasks hash")
}

func hasWaveReasonCode(reasons []model.ReasonCode, code, detail string) bool {
	for _, reason := range reasons {
		if reason.Code == code && reason.Detail == detail {
			return true
		}
	}
	return false
}
