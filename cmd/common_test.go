package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/engine/artifact"
	"github.com/signalridge/slipway/internal/engine/progression"
	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testProcessCWDMu sync.Mutex
	stableTestWD     = initialTestWorkingDirectory()
)

func initialTestWorkingDirectory() string {
	wd, err := os.Getwd()
	if err == nil {
		return wd
	}
	return os.TempDir()
}

func withProcessWorkingDirectory(t *testing.T, dir string, fn func()) {
	t.Helper()

	testProcessCWDMu.Lock()
	defer testProcessCWDMu.Unlock()

	previousWD, err := os.Getwd()
	if err != nil {
		previousWD = stableTestWD
	}
	require.NoError(t, os.Chdir(dir))
	defer restoreProcessWorkingDirectory(t, previousWD)

	fn()
}

func restoreProcessWorkingDirectory(t *testing.T, preferred string) {
	t.Helper()

	for _, dir := range []string{preferred, stableTestWD, os.TempDir()} {
		if dir == "" {
			continue
		}
		if err := os.Chdir(dir); err == nil {
			return
		}
	}
	t.Fatalf("failed to restore process working directory")
}

func setCommandProjectRoot(cmd *cobra.Command, root string) {
	if cmd == nil {
		return
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, projectRootContextKey{}, root))
}

func TestResolveExplicitChangeRejectsInactiveSlug(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "inactive explicit request")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.Status = model.ChangeStatusCancelled
	require.NoError(t, state.SaveChange(root, change))

	_, err = resolveExplicitChange(root, slug)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "not_active", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
	assert.Equal(t, slug, cliErr.Slug)
	assert.Equal(t, string(model.ChangeStatusCancelled), cliErr.Details["status"])
}

func TestProjectRootFromWDRejectsUninitializedGitRepo(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		_, err := projectRootFromWD()
		require.Error(t, err)
		assert.ErrorIs(t, err, fsutil.ErrProjectRootNotFound)
		assert.Contains(t, err.Error(), "run `slipway init`")
	})
}

func withCommandWorkspace(t *testing.T, root string, fn func()) {
	t.Helper()
	ensureTestGitRepo(t, root)
	fn()
}

func commandForRoot(t *testing.T, root string, cmd *cobra.Command) *cobra.Command {
	t.Helper()
	setCommandProjectRoot(cmd, root)
	return cmd
}

func TestResolveExplicitChangeRejectsUnknownSlug(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	_, err := resolveExplicitChange(root, "slug-missing")
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "change_not_found", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
	assert.Equal(t, "slug-missing", cliErr.Slug)
}

func TestResolveExplicitChangeRejectsInvalidSlugBeforeStateLookup(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	tests := []struct {
		name string
		slug string
	}{
		{name: "parent traversal", slug: "../x"},
		{name: "slash", slug: "bad/slug"},
		{name: "backslash", slug: `bad\slug`},
		{name: "dot", slug: "."},
		{name: "dot dot", slug: ".."},
		{name: "uppercase", slug: "Bad-Slug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveExplicitChange(root, tt.slug)
			cliErr := asCLIError(err)
			require.NotNil(t, cliErr)
			assert.Equal(t, "invalid_change_slug", cliErr.ErrorCode)
			assert.Equal(t, categoryInvalidUsage, cliErr.Category)
			assert.Equal(t, tt.slug, cliErr.Details["slug"])
		})
	}
}

func TestResolveExplicitChangeRejectsArchivedSlugWithConcreteDiagnostic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "archived explicit request")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	_, err = state.ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	_, err = resolveExplicitChange(root, slug)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "archived_change_not_validatable", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
	assert.Equal(t, slug, cliErr.Slug)
	assert.Equal(t, string(model.ChangeStatusDone), cliErr.Details["status"])
	assert.Equal(t, true, cliErr.Details["archived"])
	assert.Contains(t, fmt.Sprint(cliErr.Details["archive_path"]), filepath.ToSlash(filepath.Join("artifacts", "changes", "archived", slug, "change.yaml")))
	assert.Contains(t, cliErr.Remediation, "archived")
}

func TestValidateChangeFlagRejectsArchivedSlugWithConcreteDiagnostic(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "validate archived explicit request")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	_, err = state.ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)

	cmd := commandForRoot(t, root, makeValidateCmd())
	cmd.SetArgs([]string{"--json", "--change", slug})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err = cmd.Execute()

	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "archived_change_not_validatable", cliErr.ErrorCode)
	assert.Equal(t, categoryPrecondition, cliErr.Category)
	assert.Equal(t, slug, cliErr.Slug)
	assert.Equal(t, string(model.ChangeStatusDone), cliErr.Details["status"])
	assert.NotContains(t, out.String(), "no active change or ambiguous")
}

func TestExplicitChangeCommandsUseFastPathWhenOtherBundleIsOrphaned(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "explicit fast path target")
	orphanDir := filepath.Join(root, "artifacts", "changes", "orphaned-active-bundle")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(orphanDir, "notes.md"), []byte("orphaned\n"), 0o644))

	t.Run("status", func(t *testing.T) {
		cmd := commandForRoot(t, root, makeStatusCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		var out bytes.Buffer
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var view statusView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
	})

	t.Run("next", func(t *testing.T) {
		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		var out bytes.Buffer
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
	})

	t.Run("validate", func(t *testing.T) {
		cmd := commandForRoot(t, root, makeValidateCmd())
		cmd.SetArgs([]string{"--json", "--change", slug})
		var out bytes.Buffer
		cmd.SetOut(&out)

		require.NoError(t, cmd.Execute())

		var view validateView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, slug, view.Slug)
	})
}

func TestResolveExplicitChangeSurfacesCorruptState(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "corrupt explicit governed request")
	require.NoError(t, os.WriteFile(
		state.BundleChangeFilePath(root, slug),
		[]byte("slug: ["),
		0o644,
	))

	_, err := resolveExplicitChange(root, slug)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	assert.Equal(t, slug, cliErr.Slug)
}

func TestLoadActiveChangeRejectsInactiveStatus(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "inactive change helper")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.Status = model.ChangeStatusDone
	require.NoError(t, state.SaveChange(root, change))

	_, err = loadActiveChange(
		root,
		slug,
		"cannot operate on governed status %q",
		"Only active governed changes are supported.",
	)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "not_active", cliErr.ErrorCode)
	assert.Equal(t, slug, cliErr.Slug)
	assert.Equal(t, string(model.ChangeStatusDone), cliErr.Details["status"])
}

func TestLoadActiveChangeSurfacesCorruptStateIntegrity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "corrupt change helper")
	require.NoError(t, os.WriteFile(
		state.BundleChangeFilePath(root, slug),
		[]byte("slug: ["),
		0o644,
	))

	_, err := loadActiveChange(
		root,
		slug,
		"cannot operate on governed status %q",
		"Only active governed changes are supported.",
	)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	assert.Equal(t, slug, cliErr.Slug)
}

func TestLoadExecutionContextWrapsCorruptExecutionSummaryIntegrity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "corrupt execution summary helper")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)

	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	summaryPath := executionSummaryPathForTest(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(summaryPath), 0o755))
	require.NoError(t, os.WriteFile(summaryPath, []byte("version: ["), 0o644))

	_, err = loadExecutionContext(root, change)
	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "execution_summary_load_failed", cliErr.ErrorCode)
	assert.Equal(t, categoryStateIntegrity, cliErr.Category)
	assert.Equal(t, slug, cliErr.Slug)
	assert.Contains(t, cliErr.Remediation, "slipway repair")
	assert.Equal(t, summaryPath, cliErr.Details["path"])
}

func TestGovernanceReadinessErrorCodeDoesNotGuessFromMessageText(t *testing.T) {
	err := fmt.Errorf("parse verification plan-audit while validating execution summary linkage")
	assert.Equal(t, "governance_readiness_failed", governanceReadinessErrorCode(err))
}

func TestGovernanceReadinessErrorCodeRecognizesVerificationLoadError(t *testing.T) {
	err := &state.VerificationLoadError{
		Path: "/tmp/plan-audit.yaml",
		Err:  fmt.Errorf("parse verification plan-audit: broken"),
	}
	assert.Equal(t, "verification_load_failed", governanceReadinessErrorCode(err))
}

func TestStatusWaveExecutionIssuesPreserveCanonicalCLIErrorCodes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code string
	}{
		{name: "wave plan load failed", code: "wave_plan_load_failed"},
		{name: "wave runs load failed", code: "wave_runs_load_failed"},
		{name: "wave runs invalid count", code: "wave_runs_invalid_count"},
		{name: "wave run version mismatch", code: "wave_run_version_mismatch"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reasons, diagnostics := statusWaveExecutionIssues(newStateIntegrityError(
				tt.code,
				"wave execution artifact error",
				"Run `slipway repair`.",
				"status-wave",
				nil,
			))

			require.Len(t, reasons, 1)
			assert.Equal(t, tt.code, reasons[0].Code)
			assert.NotEqual(t, "unknown_reason_code", reasons[0].Code)
			assert.NotEmpty(t, diagnostics)
		})
	}
}

func TestStatusWaveExecutionIssuesWrapNonReasonCLIErrorCodes(t *testing.T) {
	t.Parallel()

	reasons, diagnostics := statusWaveExecutionIssues(newPreconditionError(
		"change_bound_to_other_worktree",
		"active change is bound to another worktree",
		"Use --change.",
		"",
		nil,
	))

	require.Len(t, reasons, 1)
	assert.Equal(t, "wave_execution_unavailable", reasons[0].Code)
	assert.Contains(t, reasons[0].Detail, "change_bound_to_other_worktree")
	assert.NotEqual(t, "unknown_reason_code", reasons[0].Code)
	assert.NotEmpty(t, diagnostics)
}

func TestLoadExecutionContextUsesAuthoritativeWorktreeSummaryForHiddenBoundChange(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelDiscovery, "hidden worktree summary path should stay authoritative")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, change))

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		now := time.Now().UTC()
		require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
			Version:           model.ExecutionSummaryVersion,
			RunSummaryVersion: 1,
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
		require.NoError(t, os.Remove(state.WorkspaceScopeMarkerPath(worktreeRoot)))

		execCtx, err := loadExecutionContext(root, bound)
		require.NoError(t, err)
		require.True(t, execCtx.Ready)
		assert.Equal(t, 1, execCtx.LatestRunVersion)
	})
}

func TestProjectFreshnessIgnoresDerivedTaskCheckboxSync(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "freshness should ignore derived task checkbox sync")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	oldTS := time.Now().UTC().Add(-3 * time.Second)
	capturedAt := oldTS.Add(time.Second)
	writeSkillVerification(t, root, slug, progression.SkillWaveOrchestration, model.VerificationRecord{
		Verdict:    model.VerificationVerdictPass,
		Blockers:   []model.ReasonCode{},
		Timestamp:  capturedAt,
		RunVersion: 1,
	})

	taskEvidence := map[string]any{
		"task_id":             "task-01",
		"run_summary_version": 1,
		"task_kind":           "code",
		"verdict":             "pass",
		"changed_files":       []string{"cmd/next.go"},
		"blockers":            []string{},
		"evidence_ref":        "test:task-01",
		"captured_at":         capturedAt.Format(time.RFC3339Nano),
		"freshness_inputs":    state.ExpectedExecutionTaskFreshnessInputs(change, 1, "task-01"),
	}
	raw, err := json.Marshal(taskEvidence)
	require.NoError(t, err)
	taskEvidencePath := filepath.Join(state.EvidenceTasksDir(root, slug), "task-01.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(taskEvidencePath), 0o755))
	require.NoError(t, os.WriteFile(taskEvidencePath, raw, 0o644))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-01`"+` preserve freshness across derived checkbox sync
  - target_files: ["cmd/next.go"]
  - task_kind: code
`)))
	for _, name := range []string{"change.yaml", "intent.md", "requirements.md", "research.md", "decision.md", "tasks.md", "assurance.md"} {
		path := filepath.Join(bundlePath, name)
		if _, err := os.Stat(path); err == nil {
			require.NoError(t, os.Chtimes(path, oldTS, oldTS))
		}
	}
	_, err = state.MaterializeWavePlan(root, change)
	require.NoError(t, err)

	result, err := progression.SyncGovernedWaveExecution(root, change)
	require.NoError(t, err)
	require.True(t, result.Updated)

	summary, err := state.LoadExecutionSummary(root, slug)
	require.NoError(t, err)

	assert.Equal(t, "fresh", projectFreshnessForExecMode(root, change, &summary, nil))
}

func TestProjectFreshnessTracksTasksPlanHash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "freshness tracks tasks plan hash")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-01`"+` keep evidence fresh
  - target_files: ["cmd/placeholder.go"]
  - task_kind: code
`)))
	hash, err := state.CurrentTasksPlanStructuralState(root, change)
	require.NoError(t, err)

	oldTS := time.Now().UTC().Add(-5 * time.Second)
	capturedAt := oldTS.Add(time.Second)
	for _, name := range []string{"change.yaml", "intent.md", "requirements.md", "research.md", "decision.md", "tasks.md", "assurance.md"} {
		path := filepath.Join(bundlePath, name)
		if _, err := os.Stat(path); err == nil {
			require.NoError(t, os.Chtimes(path, oldTS, oldTS))
		}
	}

	summary := model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        capturedAt,
		OverallVerdict:    model.ExecutionVerdictPass,
		TasksPlanHash:     hash,
		CompletedTasks:    []string{"task-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:       "task-01",
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindCode,
			ChangedFiles: []string{"cmd/placeholder.go"},
			TargetFiles:  []string{"cmd/placeholder.go"},
			CapturedAt:   capturedAt,
		}},
	}
	state.ApplyExecutionSummaryFreshnessInputs(&summary, change)
	summary.SyncDerivedFields()
	writeExecutionSummary(t, root, slug, summary)

	assert.Equal(t, "fresh", projectFreshnessForExecMode(root, change, &summary, nil))

	require.NoError(t, writeBundleArtifactFile(bundlePath, slug, "tasks.md", []byte(`# Tasks

- [ ] `+"`task-01`"+` changed evidence plan
  - target_files: ["cmd/placeholder.go"]
  - task_kind: code
`)))

	assert.Equal(t, "stale", projectFreshnessForExecMode(root, change, &summary, nil))
}

func TestProjectFreshnessIgnoresNonFreshnessBlockers(t *testing.T) {
	root := t.TempDir()
	change := model.NewChange("freshness-scope")
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
	state.ApplyExecutionSummaryFreshnessInputs(&summary, change)
	summary.SyncDerivedFields()

	assert.Equal(t, "fresh", projectFreshnessForExecMode(root, change, &summary, []model.ReasonCode{model.NewReasonCode("required_skill_missing", "")}))
	assert.Equal(t, "stale", projectFreshnessForExecMode(root, change, &summary, []model.ReasonCode{model.NewReasonCode(state.StaleExecutionEvidenceBlockerToken, "")}))
}

func TestProjectFreshnessFailsClosedWhenFreshnessArtifactIsUnreadable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "freshness fails closed on unreadable artifact")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	deniedDir := filepath.Join(bundlePath, "denied")
	targetPath := filepath.Join(deniedDir, "secret.md")
	require.NoError(t, os.MkdirAll(deniedDir, 0o755))
	require.NoError(t, os.WriteFile(targetPath, []byte("secret\n"), 0o644))
	require.NoError(t, os.RemoveAll(filepath.Join(bundlePath, "tasks.md")))
	require.NoError(t, os.Symlink(targetPath, filepath.Join(bundlePath, "tasks.md")))
	require.NoError(t, os.Chmod(deniedDir, 0o000))
	t.Cleanup(func() {
		_ = os.Chmod(deniedDir, 0o755)
	})
	if _, err := os.ReadFile(filepath.Join(bundlePath, "tasks.md")); err == nil {
		t.Skip("permission-denied freshness scenario is not reproducible for the current user")
	}

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

	assert.Equal(t, "stale", projectFreshnessForExecMode(root, change, &summary, nil))
}

func TestCurrentWorktreeRootPropagatesGitErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake git shell wrapper lookup is not reliable on Windows")
	}

	root := t.TempDir()
	realGit, err := exec.LookPath("git")
	require.NoError(t, err)

	fakeBin := filepath.Join(root, "fake-bin")
	require.NoError(t, os.MkdirAll(fakeBin, 0o755))
	fakeGit := filepath.Join(fakeBin, "git")
	require.NoError(t, os.WriteFile(fakeGit, []byte(fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [ "$#" -ge 2 ] && [ "$1" = "rev-parse" ] && [ "$2" = "--show-toplevel" ]; then
  printf 'permission denied\n' >&2
  exit 1
fi
exec %q "$@"
`, realGit)), 0o755))

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = currentWorktreeRoot()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git rev-parse --show-toplevel")
}

func TestStatusCommandFromBoundWorktreeUsesBoundScopeConfigCopy(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelDiscovery, "bound worktree should read main-scope config")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		change.NeedsDiscovery = true
		change.WorkflowPreset = model.WorkflowPresetLight
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

		cfg, err := model.LoadConfig(state.ConfigPath(root))
		require.NoError(t, err)
		cfg.Governance.MinPreset = model.WorkflowPresetStrict
		require.NoError(t, model.SaveConfig(state.ConfigPath(root), cfg))

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")
		assert.Equal(t, branch, currentGitBranch(t, worktreeRoot))

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))
		writePassingExecutionSummary(t, root, slug, 1, "t-01")

		_, err = os.Stat(state.ConfigPath(worktreeRoot))
		require.NoError(t, err)

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(worktreeRoot))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		var buf bytes.Buffer
		cmd := makeStatusCmd()
		cmd.SetOut(&buf)
		cmd.SetArgs([]string{"--json", "--change", slug})
		require.NoError(t, cmd.Execute())

		var view statusView
		require.NoError(t, json.Unmarshal(buf.Bytes(), &view))
		assert.Equal(t, "strict", view.EffectiveWorkflowPreset)
		assert.NotContains(t, model.ReasonSpecs(view.Blockers), state.WorktreeReasonDedicatedRequired)
		expectedSource := state.DisplayPath(root, filepath.Join(worktreeRoot, "artifacts", "changes", slug, "change.yaml"))
		assert.Equal(t, expectedSource, view.SourceStateFile)
		require.NotNil(t, view.FreshnessDiagnostics)
		require.NotNil(t, view.FreshnessDiagnostics.PathAuthority)
		assert.Equal(t, state.DisplayPath(root, worktreeRoot), view.FreshnessDiagnostics.PathAuthority.InvocationWorkspacePath)
		assert.Equal(t, state.DisplayPath(root, worktreeRoot), view.FreshnessDiagnostics.PathAuthority.BoundWorkspacePath)
	})
}

func TestResolveActiveChangeRefFromNestedBoundWorktreeCWD(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644))
		runGit(t, root, "add", ".")
		runGit(t, root, "commit", "-m", "init")
		initTestWorkspace(t, root)

		slug := createGovernedRequest(t, root, levelDiscovery, "nested bound worktree active change resolution")
		change, err := state.LoadChange(root, slug)
		require.NoError(t, err)
		change.CurrentState = model.StateS2Implement
		change.PlanSubStep = model.PlanSubStepNone
		change.NeedsDiscovery = true
		require.NoError(t, state.SaveChange(root, change))
		require.NoError(t, artifact.ScaffoldGovernedBundleForChange(root, change, ""))

		worktreeRoot := filepath.Join(t.TempDir(), slug)
		branch := "feat/" + slug
		runGit(t, root, "worktree", "add", worktreeRoot, "-b", branch, "HEAD")

		bound := change
		require.NoError(t, state.PersistScopeWorktreeMetadata(&bound, worktreeRoot, branch))
		require.NoError(t, state.RelocateGovernedBundle(root, change, bound))
		require.NoError(t, state.SaveChange(root, bound))

		nested := filepath.Join(worktreeRoot, "pkg", "feature")
		require.NoError(t, os.MkdirAll(nested, 0o755))

		previousWD, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(nested))
		defer func() {
			_ = os.Chdir(previousWD)
		}()

		resolvedRoot, err := projectRootFromWD()
		require.NoError(t, err)
		ref, err := resolveActiveChangeRef(resolvedRoot, "")
		require.NoError(t, err)
		assert.Equal(t, slug, ref.Slug)
	})
}

// TestLoadAuthoritativeWaveExecutionCacheUnreadableNamesCacheNotTasks asserts
// that when the engine-owned wave-plan.yaml cache carries unsupported/view-only
// fields, the surfaced error is wave_plan_unreadable and its remediation names
// the cache + regenerate path and never tells the user to edit tasks.md
// (REQ-001).
func TestLoadAuthoritativeWaveExecutionCacheUnreadableNamesCacheNotTasks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "cache unreadable points at the engine-owned wave plan cache")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS3Review
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [x] `+"`t-01`"+` solo wave
  - depends_on: []
  - target_files: ["cmd/common.go"]
  - task_kind: verification
  - covers: [REQ-001]
`), 0o644))

	now := time.Now().UTC()
	require.NoError(t, state.SaveExecutionSummary(root, slug, model.ExecutionSummary{
		Version:           model.ExecutionSummaryVersion,
		RunSummaryVersion: 1,
		CapturedAt:        now,
		OverallVerdict:    model.ExecutionVerdictPass,
		CompletedTasks:    []string{"t-01"},
		Tasks: []model.ExecutionTaskSummary{{
			TaskID:       "t-01",
			Verdict:      model.TaskVerdictPass,
			TaskKind:     model.TaskKindVerification,
			ChangedFiles: []string{"cmd/common.go"},
			CapturedAt:   now,
		}},
	}))

	change, err = state.LoadChange(root, slug)
	require.NoError(t, err)
	// Corrupt the engine-owned cache with view-only fields the persisted schema
	// rejects under KnownFields(true). A non-S2 state loads the cache directly.
	cachePath := state.WavePlanPathForRead(root, slug)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o755))
	require.NoError(t, os.WriteFile(cachePath, []byte("wave_count: 1\nadvisories: [\"narrow\"]\nwaves: []\n"), 0o644))

	_, err = loadAuthoritativeWaveExecution(root, change, 1, "status")
	require.Error(t, err)

	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "wave_plan_unreadable", cliErr.ErrorCode)
	assert.Contains(t, cliErr.Remediation, "wave-plan.yaml")
	assert.Contains(t, cliErr.Remediation, "slipway repair")
	assert.Contains(t, cliErr.Remediation, "must not be hand-edited",
		"cache-unreadable remediation must describe the cache as engine-owned / not hand-editable")
	// REQ-001: the remediation may cite tasks.md as the regenerate SOURCE
	// (`slipway repair` rebuilds the cache from tasks.md), but it must NOT
	// instruct the user to update/edit tasks.md themselves.
	assert.NotContains(t, cliErr.Remediation, "Update tasks.md",
		"cache-unreadable remediation must not tell the user to update tasks.md")
}

// TestLoadAuthoritativeWaveExecutionTasksDerivationFailureKeepsTasksGuidance
// asserts a genuine tasks.md-derivation failure (S2, unschedulable tasks.md)
// keeps the tasks.md-oriented remediation and stays wave_plan_load_failed
// (REQ-002).
func TestLoadAuthoritativeWaveExecutionTasksDerivationFailureKeepsTasksGuidance(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	slug := createGovernedRequest(t, root, levelNonDiscovery, "tasks derivation failure keeps tasks guidance")
	change, err := state.LoadChange(root, slug)
	require.NoError(t, err)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	require.NoError(t, state.SaveChange(root, change))

	bundlePath := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundlePath, 0o755))
	// Unschedulable: t-01 depends on a task that does not exist -> derivation
	// fails (not a cache parse failure).
	require.NoError(t, os.WriteFile(filepath.Join(bundlePath, "tasks.md"), []byte(`# Tasks

- [ ] `+"`t-01`"+` broken dependency
  - depends_on: ["t-missing"]
  - target_files: ["cmd/common.go"]
  - task_kind: code
  - covers: [REQ-002]
`), 0o644))

	change, err = state.LoadChange(root, slug)
	require.NoError(t, err)

	_, err = loadAuthoritativeWaveExecution(root, change, 1, "implement")
	require.Error(t, err)

	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "wave_plan_load_failed", cliErr.ErrorCode)
	assert.Contains(t, cliErr.Remediation, "tasks.md",
		"a genuine tasks.md-derivation failure must keep tasks.md-oriented guidance")
}
