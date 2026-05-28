package state

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testReason(code, detail string) model.ReasonCode {
	return model.NewReasonCode(code, detail)
}

func testExecutionSummaryPath(root, slug string) string {
	return filepath.Join(VerificationDir(root, slug), ExecutionSummaryFileName)
}

func TestExecutionSummaryFileLivesInVerificationDir(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	assert.Equal(
		t,
		filepath.Join(root, "artifacts", "changes", "demo", "verification", ExecutionSummaryFileName),
		testExecutionSummaryPath(root, "demo"),
	)
}

func TestExecutionSummaryPathForReadUsesArchivedVerificationDir(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("archived-summary-path")
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))
	_, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	assert.Equal(
		t,
		filepath.Join(root, "artifacts", "changes", "archived", change.Slug, "verification", ExecutionSummaryFileName),
		ExecutionSummaryPathForRead(root, change.Slug),
	)
}

func TestExecutionSummaryPathForReadPrefersHiddenSiblingWorktreeBundle(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "hidden-worktree-summary-path"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, PersistScopeWorktreeMetadata(&change, worktreeRoot, "feature"))
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	assert.Equal(
		t,
		filepath.Join(change.WorktreePath, "artifacts", "changes", slug, "verification", ExecutionSummaryFileName),
		ExecutionSummaryPathForRead(root, slug),
	)
}

func TestSaveLoadExecutionSummaryRoundTrip(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	saveActiveChangeForTest(t, root, "demo")
	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 2,
		CapturedAt:        time.Date(2026, 4, 4, 1, 2, 3, 0, time.UTC),
		OverallVerdict:    model.ExecutionVerdictFail,
		CompletedTasks:    []string{"task-a"},
		NonPassTasks:      []string{"task-b"},
		OpenBlockers:      []model.ReasonCode{testReason("task_blocker", "task-b:lint_failed")},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:            "task-a",
				Verdict:           model.TaskVerdictPass,
				TaskKind:          model.TaskKindCode,
				ChangedFiles:      []string{"cmd/status.go"},
				EvidenceRef:       ".git/slipway/runtime/changes/demo/evidence/tasks/rv2/task-a.json",
				EvidenceInputHash: "abc123",
				CapturedAt:        time.Date(2026, 4, 4, 1, 0, 0, 0, time.UTC),
			},
			{
				TaskID:       "task-b",
				Verdict:      model.TaskVerdictFail,
				TaskKind:     model.TaskKindVerification,
				ChangedFiles: []string{"cmd/review.go"},
				Blockers:     []model.ReasonCode{testReason("lint_failed", "")},
				CapturedAt:   time.Date(2026, 4, 4, 1, 1, 0, 0, time.UTC),
			},
		},
	}

	require.NoError(t, SaveExecutionSummary(root, "demo", summary))

	loaded, err := LoadExecutionSummary(root, "demo")
	require.NoError(t, err)
	assert.Equal(t, summary.RunSummaryVersion, loaded.RunSummaryVersion)
	assert.Equal(t, summary.OverallVerdict, loaded.OverallVerdict)
	assert.Len(t, loaded.Tasks, 2)
	assert.Equal(t, "task-a", loaded.Tasks[0].TaskID)
	assert.Equal(t, "abc123", loaded.Tasks[0].EvidenceInputHash)
	assert.Equal(t, []model.ReasonCode{testReason("lint_failed", "")}, loaded.Tasks[1].Blockers)
}

func TestLoadExecutionSummaryReturnsErrorOnInvalidSummary(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	saveActiveChangeForTest(t, root, "demo")
	path := testExecutionSummaryPath(root, "demo")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("version: ["), 0o644))

	_, err := LoadExecutionSummary(root, "demo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse execution summary")
}

func TestLoadExecutionSummaryRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	saveActiveChangeForTest(t, root, "demo")
	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-a",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: time.Now().UTC(),
		}},
	}
	require.NoError(t, SaveExecutionSummary(root, "demo", summary))

	path := testExecutionSummaryPath(root, "demo")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	raw = append(raw, []byte("\nunexpected_field: true\n")...)
	require.NoError(t, os.WriteFile(path, raw, 0o644))

	_, err = LoadExecutionSummary(root, "demo")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse execution summary")
}

func TestLoadExecutionSummaryAcceptsStructuredReasonCodes(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	saveActiveChangeForTest(t, root, "demo")
	path := testExecutionSummaryPath(root, "demo")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
run_summary_version: 2
captured_at: 2026-04-08T00:00:00Z
overall_verdict: fail
non_pass_tasks:
  - task-b
open_blockers:
  - code: task_non_pass
    severity: error
    message: "Task did not pass: task-b"
    detail: task-b
tasks:
  - task_id: task-b
    verdict: fail
    blockers:
      - code: lint_failed
        severity: error
        message: "Lint failed"
`), 0o644))

	loaded, err := LoadExecutionSummary(root, "demo")
	require.NoError(t, err)
	require.Len(t, loaded.OpenBlockers, 1)
	assert.Equal(t, "task_non_pass", loaded.OpenBlockers[0].Code)
	require.Len(t, loaded.Tasks, 1)
	require.Len(t, loaded.Tasks[0].Blockers, 1)
	assert.Equal(t, "lint_failed", loaded.Tasks[0].Blockers[0].Code)
}

func TestExecutionSummaryReadyAllowsSummaryLevelBlockers(t *testing.T) {
	t.Parallel()

	summaryWithBlockers := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		OpenBlockers:      []model.ReasonCode{testReason("session_isolation_warning", "session_id=abc:shared_by=task-a,task-b")},
	}
	assert.True(t, ExecutionSummaryReady(&summaryWithBlockers))

	readyWithTasks := summaryWithBlockers
	readyWithTasks.OpenBlockers = nil
	readyWithTasks.OverallVerdict = model.ExecutionVerdictPass
	readyWithTasks.Tasks = []model.ExecutionTaskSummary{{
		TaskID:     "task-a",
		Verdict:    model.TaskVerdictPass,
		TaskKind:   model.TaskKindCode,
		CapturedAt: time.Now().UTC(),
	}}
	assert.True(t, ExecutionSummaryReady(&readyWithTasks))
}

func TestExecutionSummaryWithSummaryLevelBlockersIsReady(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	saveActiveChangeForTest(t, root, "demo")
	require.NoError(t, SaveExecutionSummary(root, "demo", model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 4,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictFail,
		OpenBlockers:      []model.ReasonCode{testReason("session_isolation_warning", "session_id=abc:shared_by=task-a,task-b")},
	}))

	summary, err := LoadExecutionSummary(root, "demo")
	require.NoError(t, err)
	assert.True(t, ExecutionSummaryReady(&summary))
	assert.Equal(t, 4, summary.RunSummaryVersion)
}

func TestExecutionSummaryWithNoReadySignalsIsNotReady(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	saveActiveChangeForTest(t, root, "demo")
	require.NoError(t, SaveExecutionSummary(root, "demo", model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 4,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		Tasks:             []model.ExecutionTaskSummary{},
	}))

	summary, err := LoadExecutionSummary(root, "demo")
	require.NoError(t, err)
	assert.False(t, ExecutionSummaryReady(&summary))
}

func TestSaveExecutionSummaryRejectsHiddenSiblingWorktreeBundle(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "hidden-worktree-summary"

	change := model.NewChange(slug)
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	err := SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-a",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: time.Now().UTC(),
		}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authoritative bundle")

	_, statErr := os.Stat(filepath.Join(root, "artifacts", "changes", slug, "verification", ExecutionSummaryFileName))
	assert.ErrorIs(t, statErr, os.ErrNotExist)

	_, statErr = os.Stat(filepath.Join(worktreeRoot, "artifacts", "changes", slug, "verification", ExecutionSummaryFileName))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestLoadExecutionSummaryRejectsHiddenSiblingWorktreeFallback(t *testing.T) {
	t.Parallel()

	root, worktreeRoot := setupRepoWithWorktree(t)
	slug := "hidden-worktree-summary-read"

	change := model.NewChange(slug)
	change.WorktreePath = worktreeRoot
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.Remove(WorkspaceScopeMarkerPath(worktreeRoot)))

	staleRootDir := filepath.Join(root, "artifacts", "changes", slug, "verification")
	require.NoError(t, os.MkdirAll(staleRootDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(staleRootDir, ExecutionSummaryFileName), []byte(`version: 1
run_summary_version: 1
captured_at: 2026-04-06T00:00:00Z
overall_verdict: pass
completed_tasks: ["task-a"]
tasks:
  - task_id: task-a
    verdict: pass
    task_kind: code
    captured_at: 2026-04-06T00:00:00Z
`), 0o644))

	_, err := LoadExecutionSummary(root, slug)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authoritative bundle")
}

func TestSaveExecutionSummaryRejectsMissingAuthorityFile(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	slug := "missing-authority-summary"
	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))
	require.NoError(t, os.Remove(BundleChangeFilePath(root, slug)))

	err := SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-a",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: time.Now().UTC(),
		}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "change bundle missing authority file")

	_, statErr := os.Stat(filepath.Join(root, "artifacts", "changes", slug, "verification", ExecutionSummaryFileName))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestLoadExecutionSummaryReadsArchivedBundleSummary(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("archived-demo")
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 3,
		CapturedAt:        time.Date(2026, 4, 5, 1, 2, 3, 0, time.UTC),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "task-a",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"cmd/status.go"},
				CapturedAt:   time.Date(2026, 4, 5, 1, 0, 0, 0, time.UTC),
			},
		},
	}
	require.NoError(t, SaveExecutionSummary(root, change.Slug, summary))

	writeVerificationForTest(t, root, change.Slug, "plan-audit", model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  time.Date(2026, 4, 5, 1, 1, 0, 0, time.UTC),
		RunVersion: 3,
	})

	_, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	loaded, err := LoadExecutionSummary(root, change.Slug)
	require.NoError(t, err)
	assert.Equal(t, 3, loaded.RunSummaryVersion)
	assert.Equal(t, model.ExecutionVerdictPass, loaded.OverallVerdict)

	records, err := ListVerifications(root, change.Slug)
	require.NoError(t, err)
	if assert.Contains(t, records, "plan-audit") {
		assert.Equal(t, model.VerificationVerdictPass, records["plan-audit"].Verdict)
	}
}

func TestArchiveChangeScrubsRuntimeEvidenceRefsFromArchivedExecutionSummary(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("archived-summary-scrub")
	change.CurrentState = model.StateDone
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 5,
		CapturedAt:        time.Date(2026, 4, 5, 1, 2, 3, 0, time.UTC),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{
			{
				TaskID:       "task-a",
				Verdict:      model.TaskVerdictPass,
				TaskKind:     model.TaskKindCode,
				ChangedFiles: []string{"cmd/status.go"},
				EvidenceRef:  filepath.ToSlash(filepath.Join(ChangeDir(root, change.Slug), "evidence", "tasks", "rv5", "task-a.json")),
				CapturedAt:   time.Date(2026, 4, 5, 1, 0, 0, 0, time.UTC),
			},
		},
	}
	require.NoError(t, SaveExecutionSummary(root, change.Slug, summary))

	_, err := ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	loaded, err := LoadExecutionSummary(root, change.Slug)
	require.NoError(t, err)
	require.Len(t, loaded.Tasks, 1)
	assert.Empty(t, loaded.Tasks[0].EvidenceRef, "archived execution summaries must not retain runtime-only evidence refs")
}

func TestExecutionSummaryIssuesFailClosedWhenFreshnessArtifactIsUnreadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX unreadable-directory semantics are required for this freshness failure")
	}
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("freshness-unreadable")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        time.Now().UTC(),
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-01",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: time.Now().UTC(),
		}},
	}
	require.NoError(t, SaveExecutionSummary(root, change.Slug, summary))

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	deniedDir := filepath.Join(bundleDir, "denied")
	targetPath := filepath.Join(deniedDir, "secret.md")
	require.NoError(t, os.MkdirAll(deniedDir, 0o755))
	require.NoError(t, os.WriteFile(targetPath, []byte("secret\n"), 0o644))
	require.NoError(t, os.RemoveAll(filepath.Join(bundleDir, "tasks.md")))
	require.NoError(t, os.Symlink(targetPath, filepath.Join(bundleDir, "tasks.md")))
	require.NoError(t, os.Chmod(deniedDir, 0o000))
	t.Cleanup(func() {
		_ = os.Chmod(deniedDir, 0o755)
	})

	ctx, err := LoadRelevantExecutionSummaryContext(root, change)
	require.NoError(t, err)
	assert.Contains(t, ctx.Issues, StaleExecutionEvidenceBlockerToken)
}

func TestExecutionSummaryIssuesClassifyPlanningArtifactDrift(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("planning-drift")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "intent.md"), []byte("# Intent\n"), 0o644))

	capturedAt := time.Now().UTC().Add(-time.Second)
	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-01",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: capturedAt,
		}},
	}
	require.NoError(t, SaveExecutionSummary(root, change.Slug, summary))
	require.NoError(t, os.Chtimes(filepath.Join(bundleDir, "intent.md"), time.Now().UTC(), time.Now().UTC()))

	ctx, err := LoadRelevantExecutionSummaryContext(root, change)
	require.NoError(t, err)
	assert.Contains(t, ctx.Issues, StalePlanningEvidenceBlockerToken)
	assert.NotContains(t, ctx.Issues, StaleExecutionEvidenceBlockerToken)
}

func TestExecutionSummaryIssuesIgnoreAssuranceOnlyEdits(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := model.NewChange("assurance-only")
	change.CurrentState = model.StateS4Verify
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir := filepath.Join(root, "artifacts", "changes", change.Slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "assurance.md"), []byte("# Assurance\n"), 0o644))

	capturedAt := time.Now().UTC().Add(-time.Second)
	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-01",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindVerification,
			CapturedAt: capturedAt,
		}},
	}
	require.NoError(t, SaveExecutionSummary(root, change.Slug, summary))
	require.NoError(t, os.Chtimes(filepath.Join(bundleDir, "assurance.md"), time.Now().UTC(), time.Now().UTC()))

	ctx, err := LoadRelevantExecutionSummaryContext(root, change)
	require.NoError(t, err)
	assert.NotContains(t, ctx.Issues, StalePlanningEvidenceBlockerToken)
	assert.NotContains(t, ctx.Issues, StaleExecutionEvidenceBlockerToken)
}
