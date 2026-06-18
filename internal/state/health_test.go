package state

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnotateActiveChangeImpactScopesSlugSpecificErrors(t *testing.T) {
	findings := []HealthFinding{
		{
			Severity: model.ReasonSeverityError,
			Category: "execution_summary",
			Slug:     "other-change",
			Message:  "other change is broken",
		},
		{
			Severity: model.ReasonSeverityError,
			Category: "execution_summary",
			Slug:     "active-change",
			Message:  "active change is broken",
		},
		{
			Severity: model.ReasonSeverityError,
			Category: "config",
			Message:  "workspace config is broken",
		},
	}

	annotateActiveChangeImpact(findings, "active-change")

	assert.False(t, findings[0].ActiveChangeBlocking)
	assert.Equal(t, "non_blocking_for_active_change", findings[0].ActiveChangeImpact)
	assert.True(t, findings[1].ActiveChangeBlocking)
	assert.Equal(t, "blocking_for_active_change", findings[1].ActiveChangeImpact)
	assert.True(t, findings[2].ActiveChangeBlocking)
	assert.Equal(t, "blocking_for_active_change", findings[2].ActiveChangeImpact)
}

func TestCollectHealthReportFindsBrokenConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(ConfigPath(root), []byte("defaults: ["), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)
	require.NotEmpty(t, report.Findings)

	var categories []string
	for _, finding := range report.Findings {
		categories = append(categories, finding.Category)
	}
	assert.Contains(t, categories, "config")
}

func TestCollectHealthReportReportsUnreadableChangeAuthority(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("corrupt-change")
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.WriteFile(BundleChangeFilePath(root, change.Slug), []byte("slug: corrupt-change\ncurrent_state: [\n"), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "bundle_integrity" || finding.Slug != change.Slug {
			continue
		}
		found = true
		assert.True(t, healthFindingHasReasonCode(finding, "change_bundle_unreadable"))
		assert.False(t, finding.Repairable)
	}
	assert.True(t, found, "expected unreadable authority finding")
}

func TestCollectHealthReportReportsUnreadableHiddenWorktreeAuthority(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	branch := "health-hidden-unreadable-branch"
	worktreeRoot := addGitWorktree(t, root, branch)

	change := model.NewChange("hidden-unreadable-change")
	require.NoError(t, PersistScopeWorktreeMetadata(&change, worktreeRoot, branch))
	require.NoError(t, SaveChange(root, change))

	require.NoError(t, os.Remove(ConfigPath(worktreeRoot)))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))
	require.NoError(t, os.WriteFile(BundleChangeFilePath(worktreeRoot, change.Slug), []byte("slug: hidden-unreadable-change\ncurrent_state: [\n"), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "bundle_integrity" || finding.Slug != change.Slug {
			continue
		}
		found = true
		assert.True(t, healthFindingHasReasonCode(finding, "change_bundle_unreadable"))
		assert.False(t, finding.Repairable)
	}
	assert.True(t, found, "expected hidden unreadable authority finding")
}

func TestCollectHealthReportReportsUnreadableExecutionSummary(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("corrupt-execution-summary")
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	summaryPath := filepath.Join(VerificationDir(root, change.Slug), ExecutionSummaryFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(summaryPath), 0o755))
	require.NoError(t, os.WriteFile(summaryPath, []byte("version: ["), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "execution_summary" || finding.Slug != change.Slug {
			continue
		}
		found = true
		assert.True(t, healthFindingHasReasonCode(finding, "execution_summary_unreadable"))
		assert.False(t, finding.Repairable)
	}
	assert.True(t, found, "expected execution summary integrity finding")
}

func TestCollectHealthReportDoesNotRequirePersistedWavePlanDuringS2(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("missing-wave-plan")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))
	bundleDir := filepath.Dir(BundleChangeFilePath(root, change.Slug))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` recover wave plan
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`), 0o644))
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "t-01",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: time.Now().UTC(),
		}},
	}))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "wave_execution" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "wave_plan_missing" {
				found = true
			}
		}
	}
	assert.False(t, found, "S2 health must live-derive from tasks.md instead of requiring persisted wave-plan.yaml")
}

func TestCollectHealthReportReportsMalformedTaskEvidenceWithoutFailing(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("malformed-task-evidence")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))
	bundleDir := filepath.Dir(BundleChangeFilePath(root, change.Slug))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` report malformed task evidence
  - depends_on: []
  - target_files: ["cmd/health.go"]
  - task_kind: code
`), 0o644))
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "t-01",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: time.Now().UTC(),
		}},
	}))
	_, err := MaterializeWavePlan(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(EvidenceTasksDir(root, change.Slug), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(EvidenceTasksDir(root, change.Slug), "broken.json"), []byte("{"), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "execution_evidence" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "task_evidence_unreadable" && strings.Contains(reason.Detail, "broken.json") {
				found = true
				assert.False(t, finding.Repairable)
			}
		}
	}
	assert.True(t, found, "expected malformed task evidence finding")
}

func TestCollectHealthReportIgnoresMissingPersistedWavePlanWhenCurrentTasksDrifted(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("missing-wave-plan-drifted")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Dir(BundleChangeFilePath(root, change.Slug))
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(`# Tasks

- [ ] `+"`t-01`"+` historical task
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`), 0o644))

	capturedAt := time.Now().UTC()
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "t-01",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: capturedAt,
		}},
	}))

	updatedAt := capturedAt.Add(2 * time.Second)
	require.NoError(t, os.WriteFile(tasksPath, []byte(`# Tasks

- [ ] `+"`t-02`"+` replacement task after drift
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`), 0o644))
	require.NoError(t, os.Chtimes(tasksPath, updatedAt, updatedAt))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "wave_execution" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "wave_plan_missing" {
				found = true
			}
		}
	}
	assert.False(t, found, "missing persisted wave-plan cache is not an S2 health issue")
}

func TestCollectHealthReportIgnoresPersistedWavePlanDriftDuringS2(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("wave-plan-drift")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Dir(BundleChangeFilePath(root, change.Slug))
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(`# Tasks

- [ ] `+"`t-01`"+` preserve original task
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`), 0o644))
	_, err := MaterializeWavePlan(root, change)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(tasksPath, []byte(`# Tasks

- [ ] `+"`t-02`"+` replace task after drift
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "wave_execution" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "wave_plan_drift" {
				found = true
			}
		}
	}
	assert.False(t, found, "S2 health must ignore stale persisted wave-plan cache")
}

func TestCollectHealthReportIgnoresUnreadablePersistedWavePlanDuringS2(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("unreadable-wave-plan-drifted")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Dir(BundleChangeFilePath(root, change.Slug))
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(`# Tasks

- [ ] `+"`t-01`"+` historical task
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`), 0o644))

	capturedAt := time.Now().UTC()
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "t-01",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: capturedAt,
		}},
	}))
	_, err := MaterializeWavePlan(root, change)
	require.NoError(t, err)

	updatedAt := capturedAt.Add(2 * time.Second)
	require.NoError(t, os.WriteFile(tasksPath, []byte(`# Tasks

- [ ] `+"`t-02`"+` replacement task after drift
  - depends_on: []
  - target_files: ["cmd/repair.go"]
  - task_kind: code
`), 0o644))
	require.NoError(t, os.Chtimes(tasksPath, updatedAt, updatedAt))
	require.NoError(t, os.WriteFile(WavePlanPathForRead(root, change.Slug), []byte("version: [\n"), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "wave_execution" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "wave_plan_unreadable" {
				found = true
			}
		}
	}
	assert.False(t, found, "S2 health must ignore unreadable persisted wave-plan cache")
}

func TestCollectHealthReportReportsMissingWaveRuns(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("missing-wave-runs")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Dir(BundleChangeFilePath(root, change.Slug))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` recover wave evidence
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code
`), 0o644))
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:       "t-01",
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: []string{"cmd/run.go"},
			CapturedAt:   time.Now().UTC(),
		}},
	}))
	_, err := MaterializeWavePlan(root, change)
	require.NoError(t, err)

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "wave_execution" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "wave_runs_missing" {
				found = true
				assert.True(t, finding.Repairable)
			}
		}
	}
	assert.True(t, found, "expected missing wave-run health finding")
}

func TestCollectHealthReportReportsIncompleteWaveRuns(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("incomplete-wave-runs")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Dir(BundleChangeFilePath(root, change.Slug))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [x] `+"`t-01`"+` completed first wave
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`t-02`"+` pending second wave
  - depends_on: ["t-01"]
  - target_files: ["cmd/review.go"]
  - task_kind: code
`), 0o644))
	now := time.Now().UTC()
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:       "t-01",
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: []string{"cmd/run.go"},
			CapturedAt:   now,
		}},
	}))

	plan, err := MaterializeWavePlan(root, change)
	require.NoError(t, err)
	runs, err := BuildWaveRuns(plan, 1, []model.ExecutionTaskSummary{{
		TaskID:       "t-01",
		Verdict:      model.TaskVerdictPass,
		TaskKind:     model.TaskKindCode,
		ChangedFiles: []string{"cmd/run.go"},
		CapturedAt:   now,
	}}, nil)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	require.NoError(t, SaveWaveRuns(root, change.Slug, 1, runs[:1]))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "wave_execution" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "wave_runs_incomplete" {
				found = true
				assert.True(t, finding.Repairable)
				assert.Equal(t, model.ReasonSeverityError, finding.Severity)
			}
		}
	}
	assert.True(t, found, "expected incomplete wave-run health finding")
}

func TestCollectHealthReportReportsWaveTaskLinkageMismatch(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("wave-task-linkage-mismatch")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Dir(BundleChangeFilePath(root, change.Slug))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` build first wave
  - depends_on: []
  - target_files: ["cmd/run.go"]
  - task_kind: code

- [ ] `+"`t-02`"+` build second wave
  - depends_on: ["t-01"]
  - target_files: ["cmd/review.go"]
  - task_kind: code
`), 0o644))

	now := time.Now().UTC()
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01", "t-02"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "t-01",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"cmd/run.go"},
				CapturedAt:   now,
			},
			{
				TaskID:       "t-02",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"cmd/review.go"},
				CapturedAt:   now.Add(time.Second),
			},
		},
	}))

	_, err := MaterializeWavePlan(root, change)
	require.NoError(t, err)
	require.NoError(t, SaveWaveRuns(root, change.Slug, 1, []model.WaveRun{
		{
			WaveIndex:         1,
			RunSummaryVersion: 1,
			TaskRuns: []model.TaskRunRef{{
				TaskID:            "t-02",
				RunSummaryVersion: 1,
			}},
			Verdict: model.WaveVerdictPass,
		},
		{
			WaveIndex:         2,
			RunSummaryVersion: 1,
			TaskRuns: []model.TaskRunRef{{
				TaskID:            "t-01",
				RunSummaryVersion: 1,
			}},
			Verdict: model.WaveVerdictPass,
		},
	}))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "wave_execution" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "wave_task_linkage_mismatch" {
				found = true
				assert.True(t, finding.Repairable)
				assert.Contains(t, reason.Detail, "wave 1")
				assert.Contains(t, reason.Detail, "t-02")
			}
		}
	}
	assert.True(t, found, "expected wave/task linkage mismatch finding")
}

func TestCollectHealthReportReportsStaleCheckpoint(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	cfg := model.DefaultConfig()
	cfg.Execution.LockStaleAfterSeconds = 60
	require.NoError(t, model.SaveConfig(ConfigPath(root), cfg))

	change := model.NewChange("stale-checkpoint")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.ActiveCheckpoint = &model.ActiveCheckpoint{
		PausedTaskID:    "t-01",
		PausedWaveIndex: 1,
		PausedAt:        time.Now().UTC().Add(-10 * time.Minute),
		CheckpointType:  string(model.CheckpointHumanVerify),
	}
	require.NoError(t, SaveChange(root, change))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "execution_checkpoint" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "checkpoint_stale" {
				found = true
				assert.True(t, finding.Repairable)
			}
		}
	}
	assert.True(t, found, "expected stale checkpoint health finding")
}

func TestCollectHealthReportFindsOrphanBundleDirsAcrossWorktrees(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	worktreeRoot := addGitWorktree(t, root, "health-orphan-branch")
	require.NoError(t, EnsureWorkspaceScopeMarker(root, worktreeRoot))
	orphanDir := filepath.Join(worktreeRoot, "artifacts", "changes", "orphan-worktree-bundle")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(orphanDir, "intent.md"), []byte("# orphan\n"), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	var orphanReasons []string
	var orphanSlugs []string
	for _, finding := range report.Findings {
		for _, reason := range finding.Reasons {
			if reason.Code == "orphan_bundle_directory" {
				orphanReasons = append(orphanReasons, reason.Detail)
				orphanSlugs = append(orphanSlugs, finding.Slug)
			}
		}
	}
	assert.Contains(t, orphanReasons, "orphan-worktree-bundle")
	assert.Contains(t, orphanSlugs, "orphan-worktree-bundle")
}

func TestCollectHealthReportDeduplicatesMatchingOrphanBundleDirsAcrossWorktrees(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	for _, branch := range []string{"health-orphan-branch-a", "health-orphan-branch-b"} {
		worktreeRoot := addGitWorktree(t, root, branch)
		require.NoError(t, EnsureWorkspaceScopeMarker(root, worktreeRoot))
		orphanDir := filepath.Join(worktreeRoot, "artifacts", "changes", "shared-orphan")
		require.NoError(t, os.MkdirAll(orphanDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(orphanDir, "intent.md"), []byte("# orphan\n"), 0o644))
	}

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	count := 0
	for _, finding := range report.Findings {
		for _, reason := range finding.Reasons {
			if reason.Code == "orphan_bundle_directory" && reason.Detail == "shared-orphan" {
				count++
			}
		}
	}
	assert.Equal(t, 1, count)
}

func TestCollectHealthReportFindsHiddenWorktreeOrphanBundleDirs(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	worktreeRoot := addGitWorktree(t, root, "health-hidden-orphan-branch")
	require.NoError(t, EnsureWorkspaceScopeMarker(root, worktreeRoot))
	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	orphanDir := filepath.Join(worktreeRoot, "artifacts", "changes", "hidden-orphan-worktree-bundle")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(orphanDir, "intent.md"), []byte("# orphan\n"), 0o644))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	var orphanReasons []string
	for _, finding := range report.Findings {
		for _, reason := range finding.Reasons {
			if reason.Code == "orphan_bundle_directory" {
				orphanReasons = append(orphanReasons, reason.Detail)
			}
		}
	}
	assert.Contains(t, orphanReasons, "hidden-orphan-worktree-bundle")
}

func TestCollectHealthReportIgnoresEmptyOrphanBundleDirs(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "artifacts", "changes", "empty-residue", "verification"), 0o755))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	for _, finding := range report.Findings {
		for _, reason := range finding.Reasons {
			assert.NotEqual(t, "empty-residue", reason.Detail)
		}
	}
}

func TestCollectHealthReportMarksInvalidWorktreeBindingNonRepairable(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	change := model.NewChange("invalid-worktree-binding")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.NeedsDiscovery = true
	change.WorktreePath = root
	change.WorktreeBranch = "main"
	require.NoError(t, SaveChange(root, change))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "worktree" || finding.Slug != change.Slug {
			continue
		}
		found = true
		assert.False(t, finding.Repairable, "invalid bound worktree should require explicit operator action")
		assert.True(t, healthFindingHasReasonCode(finding, WorktreeReasonDedicatedRequired))
	}
	assert.True(t, found, "expected worktree integrity finding")
}

func TestCollectHealthReportReportsMissingBoundWorktreeScopeConfig(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	change := model.NewChange("missing-bound-worktree-config")
	change.Status = model.ChangeStatusActive
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.NeedsDiscovery = true
	change.WorktreePath = worktreeRoot
	change.WorktreeBranch = "feature"
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.Remove(filepath.Join(worktreeRoot, ".slipway.yaml")))

	report, err := CollectHealthReport(root)
	require.NoError(t, err)

	found := false
	for _, finding := range report.Findings {
		if finding.Category != "worktree" || finding.Slug != change.Slug {
			continue
		}
		for _, reason := range finding.Reasons {
			if reason.Code == "workspace_scope_config_missing" {
				found = true
				assert.True(t, finding.Repairable)
				assert.Contains(t, finding.RepairHint, "slipway repair")
			}
		}
	}
	assert.True(t, found, "expected missing bound-worktree scope config finding")
}

func healthFindingHasReasonCode(finding HealthFinding, code string) bool {
	for _, reason := range finding.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func TestCollectHealthReportFailsWhenWorkspaceDiscoveryFails(t *testing.T) {
	root := createRuntimeLayout(t)
	installFakeGitForStoreTests(t, root, true)

	_, err := CollectHealthReport(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list git worktrees")
}

func TestOrphanBundleSlugsReturnsNonNotExistReadErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows maps this file-as-directory setup to a not-exist error")
	}
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Dir(ActiveBundlesDir(root)), 0o755))
	require.NoError(t, os.WriteFile(ActiveBundlesDir(root), []byte("not a directory"), 0o644))

	_, err := OrphanBundleSlugs(root)
	require.Error(t, err)
}
