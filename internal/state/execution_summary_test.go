package state

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	ctxpack "github.com/signalridge/slipway/internal/engine/context"
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
				EvidenceRef:       ".git/slipway/runtime/changes/demo/evidence/tasks/task-a.json",
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

func TestSaveExecutionSummaryWritesStructuralFreshnessInputs(t *testing.T) {
	t.Parallel()

	root := createRuntimeLayout(t)
	change := saveActiveChangeForTest(t, root, "structural-freshness")
	change.CurrentState = model.StateS2Execute
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "external_api_contracts"
	require.NoError(t, SaveChange(root, change))

	now := time.Date(2026, 4, 8, 1, 2, 3, 0, time.UTC)
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 7,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "task-a",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: now,
		}},
	}))

	loaded, err := LoadExecutionSummary(root, change.Slug)
	require.NoError(t, err)
	require.Len(t, loaded.Tasks, 1)
	assert.Equal(t, model.ExecutionTaskFreshnessInputs{
		ChangeID:          change.Slug,
		RunSummaryVersion: 7,
		TaskID:            "task-a",
		GuardrailDomain:   "external_api_contracts",
	}, loaded.Tasks[0].FreshnessInputs)
	assert.Empty(t, loaded.Tasks[0].EvidenceInputHash)
}

func TestHashOnlyExecutionSummaryIsStaleWithFieldDiagnostics(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	change := saveActiveChangeForTest(t, root, "legacy-hash-only")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "external_api_contracts"
	require.NoError(t, SaveChange(root, change))

	path := testExecutionSummaryPath(root, change.Slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
run_summary_version: 2
captured_at: 2026-04-08T00:00:00Z
overall_verdict: pass
completed_tasks:
  - task-a
tasks:
  - task_id: task-a
    verdict: pass
    task_kind: code
    evidence_input_hash: wrong-content-hash
    captured_at: 2026-04-08T00:00:00Z
`), 0o644))

	ctx, err := LoadRelevantExecutionSummaryContext(root, change)
	require.NoError(t, err)
	assert.Contains(t, ctx.Issues, StaleExecutionEvidenceBlockerToken)
	assert.Equal(t, "stale", ctx.Diagnostics.Status)
	require.NotEmpty(t, ctx.Diagnostics.TaskInputDiffs)
	assert.Equal(t, "change_id", ctx.Diagnostics.TaskInputDiffs[0].Field)
	assert.Contains(t, ctx.Diagnostics.TaskInputDiffs[0].Current, "legacy evidence_input_hash=wrong-content-hash")
	require.NotNil(t, ctx.Diagnostics.PathAuthority)
	assert.Contains(t, ctx.Diagnostics.PathAuthority.RuntimeEvidencePath, ".git/slipway/runtime/changes/"+change.Slug)
	assert.Contains(t, ctx.Diagnostics.NextAction, "regenerate")
}

func TestExecutionSummaryFreshnessDiagnosticsDetectsManualTaskTimestampDrift(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	change := saveActiveChangeForTest(t, root, "task-timestamp-drift")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	runtimeCapturedAt := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	summaryCapturedAt := runtimeCapturedAt.Add(time.Hour)
	taskEvidenceDir := EvidenceTasksDir(root, change.Slug)
	require.NoError(t, os.MkdirAll(taskEvidenceDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(taskEvidenceDir, "task-a.json"),
		[]byte(`{"run_summary_version":2,"captured_at":"`+runtimeCapturedAt.Format(time.RFC3339Nano)+`"}`),
		0o644,
	))
	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 2,
		CapturedAt:        summaryCapturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:          "task-a",
			Verdict:         model.TaskVerdictPass,
			TaskKind:        model.TaskKindCode,
			CapturedAt:      summaryCapturedAt,
			FreshnessInputs: ExpectedExecutionTaskFreshnessInputs(change, 2, "task-a"),
		}},
	}

	diagnostics := ExecutionSummaryFreshnessDiagnostics(root, change, summary)

	assert.Equal(t, "stale", diagnostics.Status)
	require.NotEmpty(t, diagnostics.TaskInputDiffs)
	diff := diagnostics.TaskInputDiffs[0]
	assert.Equal(t, "task-a", diff.TaskID)
	assert.Equal(t, "captured_at", diff.Field)
	assert.Equal(t, runtimeCapturedAt.Format(time.RFC3339Nano), diff.Expected)
	assert.Equal(t, summaryCapturedAt.Format(time.RFC3339Nano), diff.Current)
	assert.Contains(t, diff.EvidencePath, ".git/slipway/runtime/changes/"+change.Slug+"/evidence/tasks/task-a.json")
	assert.Contains(t, diff.NextAction, "do not edit timestamps by hand")
}

func TestTaskEvidenceCapturedAtIgnoresMismatchedRunVersion(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	change := saveActiveChangeForTest(t, root, "task-evidence-version-guard")

	taskEvidenceDir := EvidenceTasksDir(root, change.Slug)
	require.NoError(t, os.MkdirAll(taskEvidenceDir, 0o755))
	capturedAt := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	require.NoError(t, os.WriteFile(
		filepath.Join(taskEvidenceDir, "task-a.json"),
		[]byte(`{"run_summary_version":1,"captured_at":"`+capturedAt.Format(time.RFC3339Nano)+`"}`),
		0o644,
	))

	// Evidence on disk belongs to run version 1. A diagnostic comparing it
	// against a version-2 summary must treat the file as absent rather than
	// misattributing a cross-version captured_at drift.
	_, ok, err := taskEvidenceCapturedAt(root, change.Slug, 2, "task-a")
	require.NoError(t, err)
	assert.False(t, ok, "mismatched run-version evidence must be treated as absent")

	// The same evidence is authoritative for its own run version.
	got, ok, err := taskEvidenceCapturedAt(root, change.Slug, 1, "task-a")
	require.NoError(t, err)
	require.True(t, ok, "matching run-version evidence must be read")
	assert.True(t, capturedAt.Equal(got))
}

func TestExecutionSummaryFreshnessDiagnosticsPrefersPlanningSourceAsFirstCause(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	change := saveActiveChangeForTest(t, root, "planning-source-first")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	change.GuardrailDomain = "external_api_contracts"
	require.NoError(t, SaveChange(root, change))

	bundleDir, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte("# Tasks\n\n- [ ] `task-a` rerun diagnostics\n"), 0o644))
	updatedAt := time.Date(2026, 4, 8, 1, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(tasksPath, updatedAt, updatedAt))

	capturedAt := updatedAt.Add(-time.Hour)
	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 2,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:            "task-a",
			Verdict:           model.TaskVerdictPass,
			TaskKind:          model.TaskKindCode,
			EvidenceInputHash: "legacy-input-hash",
			CapturedAt:        capturedAt,
		}},
	}

	diagnostics := ExecutionSummaryFreshnessDiagnostics(root, change, summary)
	require.Equal(t, "stale", diagnostics.Status)
	require.NotNil(t, diagnostics.FirstStaleCause)
	assert.Equal(t, StalePlanningEvidenceBlockerToken, diagnostics.FirstStaleCause.Reason)
	assert.Contains(t, diagnostics.FirstStaleCause.SourceArtifact, "tasks.md")

	foundStructuralDiff := false
	for _, pair := range diagnostics.DownstreamEvidenceChain {
		if pair.Reason == StaleExecutionEvidenceBlockerToken && pair.SourceArtifact == "" {
			foundStructuralDiff = true
			break
		}
	}
	assert.True(t, foundStructuralDiff, "structural task-input drift should remain in downstream diagnostics")
}

func TestExecutionSummaryFreshnessTreatsTasksPlanHashMismatchAsStale(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	change := saveActiveChangeForTest(t, root, "tasks-plan-hash-mismatch")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(`
- [x] `+"`t-01`"+` complete implementation
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/a.go"]
  - task_kind: code
`), 0o644))
	sourceUpdatedAt := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(tasksPath, sourceUpdatedAt, sourceUpdatedAt))

	capturedAt := sourceUpdatedAt.Add(time.Hour)
	require.NoError(t, SaveExecutionSummary(root, change.Slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		TasksPlanHash:     "operator-edited-wrong-hash",
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:     "t-01",
			Verdict:    model.TaskVerdictPass,
			TaskKind:   model.TaskKindCode,
			CapturedAt: capturedAt,
		}},
	}))
	loaded, err := LoadExecutionSummary(root, change.Slug)
	require.NoError(t, err)

	assert.Equal(t, ctxpack.EvidenceFreshnessStale, ExecutionSummaryFreshness(root, change, &loaded))
	diagnostics := ExecutionSummaryFreshnessDiagnostics(root, change, &loaded)
	assert.Equal(t, "stale", diagnostics.Status)
	require.NotNil(t, diagnostics.FirstStaleCause)
	assert.Equal(t, StalePlanningEvidenceBlockerToken, diagnostics.FirstStaleCause.Reason)
	assert.Contains(t, diagnostics.FirstStaleCause.SourceArtifact, "tasks.md")
	assert.Equal(t, sourceUpdatedAt.Format(time.RFC3339Nano), diagnostics.FirstStaleCause.SourceUpdatedAt)
	assert.Equal(t, capturedAt.Format(time.RFC3339Nano), diagnostics.FirstStaleCause.EvidenceCapturedAt)

	ctx, err := LoadRelevantExecutionSummaryContext(root, change)
	require.NoError(t, err)
	assert.Contains(t, ctx.Issues, StalePlanningEvidenceBlockerToken)
}

func TestExecutionSummaryFreshnessDiagnosticsIncludesPlanningEvidenceChain(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	change := saveActiveChangeForTest(t, root, "planning-evidence-chain")
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, SaveChange(root, change))

	bundleDir, err := GovernedBundleDir(root, change)
	require.NoError(t, err)
	verifyDir := filepath.Join(bundleDir, "verification")
	require.NoError(t, os.MkdirAll(verifyDir, 0o755))

	tasksPath := filepath.Join(bundleDir, "tasks.md")
	require.NoError(t, os.WriteFile(tasksPath, []byte(`
- [x] `+"`task-a`"+` update diagnostics chain
  - wave: 1
  - target_files: ["cmd/next.go"]
  - task_kind: code
`), 0o644))
	sourceUpdatedAt := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	require.NoError(t, os.Chtimes(tasksPath, sourceUpdatedAt, sourceUpdatedAt))

	planAuditPath := filepath.Join(verifyDir, planAuditFileName)
	wavePlanPath := filepath.Join(verifyDir, WavePlanFileName)
	require.NoError(t, os.WriteFile(planAuditPath, []byte("verdict: pass\n"), 0o644))
	require.NoError(t, os.WriteFile(wavePlanPath, []byte("waves: []\n"), 0o644))
	planAuditAt := sourceUpdatedAt.Add(-20 * time.Minute)
	wavePlanAt := sourceUpdatedAt.Add(-10 * time.Minute)
	require.NoError(t, os.Chtimes(planAuditPath, planAuditAt, planAuditAt))
	require.NoError(t, os.Chtimes(wavePlanPath, wavePlanAt, wavePlanAt))

	capturedAt := sourceUpdatedAt.Add(-30 * time.Minute)
	summary := &model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"task-a"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:          "task-a",
			Verdict:         model.TaskVerdictPass,
			TaskKind:        model.TaskKindCode,
			CapturedAt:      capturedAt,
			FreshnessInputs: ExpectedExecutionTaskFreshnessInputs(change, 1, "task-a"),
		}},
	}

	diagnostics := ExecutionSummaryFreshnessDiagnostics(root, change, summary)

	assert.Equal(t, "stale", diagnostics.Status)
	require.NotNil(t, diagnostics.FirstStaleCause)
	assert.Contains(t, diagnostics.FirstStaleCause.SourceArtifact, "tasks.md")
	assert.Contains(t, diagnostics.FirstStaleCause.EvidenceArtifact, planAuditFileName)
	assert.Contains(t, diagnostics.FirstStaleCause.NextAction, "plan-audit")
	assert.True(t, containsFreshnessChainEdge(diagnostics.DownstreamEvidenceChain, planAuditFileName, WavePlanFileName))
	assert.True(t, containsFreshnessChainEdge(diagnostics.DownstreamEvidenceChain, WavePlanFileName, ExecutionSummaryFileName))
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

func containsFreshnessChainEdge(pairs []ExecutionFreshnessPair, sourceName, evidenceName string) bool {
	for _, pair := range pairs {
		if strings.Contains(pair.SourceArtifact, sourceName) && strings.Contains(pair.EvidenceArtifact, evidenceName) {
			return true
		}
	}
	return false
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
				EvidenceRef:  filepath.ToSlash(filepath.Join(ChangeDir(root, change.Slug), "evidence", "tasks", "task-a.json")),
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
