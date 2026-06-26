package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveActiveChangeRefReportsBoundElsewhereFromRoot(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		worktreePath := filepath.Join(t.TempDir(), "bound-worktree")
		runGit(t, root, "worktree", "add", worktreePath, "-b", "bound-worktree")
		change := model.NewChange("bound-change")
		change.WorktreePath = worktreePath
		require.NoError(t, state.SaveChange(root, change))
		normalizedWorktreePath, normalizeErr := state.NormalizePath(worktreePath)
		require.NoError(t, normalizeErr)

		_, err := resolveActiveChangeRef(root, "")
		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		assert.Equal(t, "change_bound_to_other_worktree", cliErr.ErrorCode)
		assert.Equal(t, categoryPrecondition, cliErr.Category)
		assert.Contains(t, fmt.Sprint(cliErr.Details["bound_changes"]), "bound-change")
		assert.Contains(t, fmt.Sprint(cliErr.Details["bound_changes"]), normalizedWorktreePath)
		assert.Contains(t, cliErr.Remediation, "--change bound-change")
		assert.Contains(t, cliErr.Remediation, normalizedWorktreePath)
	})
}

func TestStatusFromRootReportsBoundElsewhere(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		worktreePath := filepath.Join(t.TempDir(), "status-bound-worktree")
		runGit(t, root, "worktree", "add", worktreePath, "-b", "status-bound-worktree")
		change := model.NewChange("status-bound-change")
		change.WorktreePath = worktreePath
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeStatusCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		err := cmd.Execute()

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		normalizedWorktreePath, normalizeErr := state.NormalizePath(worktreePath)
		require.NoError(t, normalizeErr)
		assert.Equal(t, "change_bound_to_other_worktree", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "--change status-bound-change")
		assert.Contains(t, cliErr.Remediation, normalizedWorktreePath)
		assert.NotContains(t, out.String(), `"slug"`, "unscoped root status must not render a stale action view for a bound worktree")
	})
}

func TestNextChangeFlagFromRootTargetsBoundWorktree(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		worktreePath := filepath.Join(t.TempDir(), "next-bound-worktree")
		runGit(t, root, "worktree", "add", worktreePath, "-b", "next-bound-worktree")
		change := model.NewChange("next-bound-change")
		change.WorktreePath = worktreePath
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		require.NoError(t, state.SaveChange(root, change))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeNextCmd())
		cmd.SetArgs([]string{"--json", "--change", change.Slug})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view nextView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		normalizedWorktreePath, err := state.NormalizePath(worktreePath)
		require.NoError(t, err)
		assert.Equal(t, normalizedWorktreePath, view.InputContext.WorkspaceRoot)
		assert.Equal(t, "next-bound-change", view.Slug)
		require.NotNil(t, view.InvocationRoute)
		assert.Equal(t, "explicit_bound_change", view.InvocationRoute.Kind)
		assert.Equal(t, "next-bound-change", view.InvocationRoute.ChangeSlug)
		assert.False(t, view.InvocationRoute.LocalLifecycleExecutionAllowed)
		assert.True(t, view.InvocationRoute.EffectiveLifecycleExecutionAllowed)
		assert.Equal(t, "slipway next --change next-bound-change", view.InvocationRoute.NextCommand)
	})
}

func TestBoundWorktreeCommandsExposeConsistentLocalInvocationRoute(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		slug := "local-bound-route"
		branch := state.DefaultWorktreeBranch(slug)
		worktreePath := filepath.Join(t.TempDir(), slug)
		runGit(t, root, "worktree", "add", worktreePath, "-b", branch)

		change := model.NewChange(slug)
		change.Description = "exercise local bound invocation route"
		change.WorkflowPreset = model.WorkflowPresetStandard
		change.QualityMode = model.QualityModeStandard
		change.ComplexityLevel = "simple"
		change.CurrentState = model.StateS1Plan
		change.PlanSubStep = model.PlanSubStepBundle
		change.WorktreePath = worktreePath
		change.WorktreeBranch = branch
		require.NoError(t, state.SaveChange(root, change))
		writeMinimalGovernedBundle(t, worktreePath, change)

		withNestedWorkingDirectory(t, worktreePath, func() {
			var statusOut bytes.Buffer
			statusCmd := makeStatusCmd()
			statusCmd.SetArgs([]string{"--json"})
			statusCmd.SetOut(&statusOut)
			require.NoError(t, statusCmd.Execute())
			var status statusView
			require.NoError(t, json.Unmarshal(statusOut.Bytes(), &status))

			var nextOut bytes.Buffer
			nextCmd := makeNextCmd()
			nextCmd.SetArgs([]string{"--json"})
			nextCmd.SetOut(&nextOut)
			require.NoError(t, nextCmd.Execute())
			var next nextHandoffView
			require.NoError(t, json.Unmarshal(nextOut.Bytes(), &next))

			var validateOut bytes.Buffer
			validateCmd := makeValidateCmd()
			validateCmd.SetArgs([]string{"--json"})
			validateCmd.SetOut(&validateOut)
			require.NoError(t, validateCmd.Execute())
			var validate validateView
			require.NoError(t, json.Unmarshal(validateOut.Bytes(), &validate))

			routes := map[string]*invocationRouteView{
				"status":   status.InvocationRoute,
				"next":     next.InvocationRoute,
				"validate": validate.InvocationRoute,
			}
			for surface, route := range routes {
				assertLocalInvocationRoute(t, surface, route, slug)
			}
			assert.Equal(t, status.InvocationRoute, next.InvocationRoute)
			assert.Equal(t, status.InvocationRoute, validate.InvocationRoute)
		})
	})
}

func TestInvocationRouteWithoutNextCommandUsesInspectOnlyRemediation(t *testing.T) {
	root := t.TempDir()
	worktreePath := filepath.Join(root, ".worktrees", "archived-local-route")
	change := model.NewChange("archived-local-route")
	change.Status = model.ChangeStatusDone
	change.CurrentState = model.StateDone
	change.WorktreePath = worktreePath

	route := buildInvocationRouteView(root, change, root, false)
	require.NotNil(t, route)
	assert.Equal(t, "archived", route.Kind)
	assert.False(t, route.LocalLifecycleExecutionAllowed)
	assert.False(t, route.EffectiveLifecycleExecutionAllowed)
	assert.Empty(t, route.NextCommand)
	assert.NotEmpty(t, route.Remediation)
	assert.Contains(t, route.Remediation, "to inspect the change")
	assert.NotContains(t, route.Remediation, "or run")
	assert.NotContains(t, route.Remediation, "``")
}

func assertLocalInvocationRoute(t *testing.T, surface string, route *invocationRouteView, slug string) {
	t.Helper()

	require.NotNil(t, route, "%s invocation_route", surface)
	assert.Equal(t, "local_active", route.Kind, surface)
	assert.Equal(t, slug, route.ChangeSlug, surface)
	assert.True(t, route.LocalLifecycleExecutionAllowed, surface)
	assert.True(t, route.EffectiveLifecycleExecutionAllowed, surface)
	assert.NotEmpty(t, route.InvocationWorkspacePath, surface)
	assert.NotEmpty(t, route.BoundWorkspacePath, surface)
	assert.NotEmpty(t, route.ChangeAuthorityPath, surface)
	assert.Equal(t, "slipway next", route.NextCommand, surface)
	assert.Empty(t, route.Remediation, surface)
}

func TestValidateChangeFlagRejectsMissingSlugWithoutWritingState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ensureTestGitRepo(t, root)
	initTestWorkspace(t, root)

	cmd := commandForRoot(t, root, makeValidateCmd())
	cmd.SetArgs([]string{"--json", "--change", "missing-explicit-change"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()

	cliErr := asCLIError(err)
	require.NotNil(t, cliErr)
	assert.Equal(t, "change_not_found", cliErr.ErrorCode)
	assert.Equal(t, "missing-explicit-change", cliErr.Slug)
	assert.NotContains(t, out.String(), "no active change or ambiguous")
	_, statErr := os.Stat(state.BundleChangeFilePath(root, "missing-explicit-change"))
	assert.True(t, os.IsNotExist(statErr), "validate must not create state for a missing explicit change")
}

func TestRunFromRootReportsBoundElsewhere(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		worktreePath := filepath.Join(t.TempDir(), "run-bound-worktree")
		runGit(t, root, "worktree", "add", worktreePath, "-b", "run-bound-worktree")
		change := model.NewChange("run-bound-change")
		change.WorktreePath = worktreePath
		require.NoError(t, state.SaveChange(root, change))

		cmd := commandForRoot(t, root, makeRunCmd())
		cmd.SetArgs([]string{"--json"})
		err := cmd.Execute()

		cliErr := asCLIError(err)
		require.NotNil(t, cliErr)
		normalizedWorktreePath, normalizeErr := state.NormalizePath(worktreePath)
		require.NoError(t, normalizeErr)
		assert.Equal(t, "change_bound_to_other_worktree", cliErr.ErrorCode)
		assert.Contains(t, cliErr.Remediation, "--change run-bound-change")
		assert.Contains(t, cliErr.Remediation, normalizedWorktreePath)
	})
}

func TestResolveActiveChangeRefPrefersArchivedWorktreeOverUnboundActive(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		archivedSlug := "archived-review-worktree"
		worktreePath := setupArchivedWorktreeForResolution(t, root, archivedSlug)

		unbound := model.NewChange("unbound-active-change")
		unbound.CurrentState = model.StateS2Implement
		unbound.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, unbound))

		withNestedWorkingDirectory(t, worktreePath, func() {
			ref, err := resolveActiveChangeRef(root, "")
			assert.Empty(t, ref.Slug)
			cliErr := asCLIError(err)
			require.NotNil(t, cliErr)
			assert.Equal(t, "archived_change_not_validatable", cliErr.ErrorCode)
			assert.Equal(t, archivedSlug, cliErr.Slug)
			assert.Equal(t, true, cliErr.Details["archived"])
		})
	})
}

func TestResolveActiveChangeRefKeepsRootActiveOnArchivedBranch(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		archivedSlug := "archived-root-branch-resolution"
		worktreePath := setupArchivedWorktreeForResolution(t, root, archivedSlug)
		runGit(t, worktreePath, "checkout", "--detach")
		runGit(t, root, "checkout", state.DefaultWorktreeBranch(archivedSlug))

		active := model.NewChange("root-active-change")
		active.CurrentState = model.StateS2Implement
		active.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, active))

		withNestedWorkingDirectory(t, root, func() {
			ref, err := resolveActiveChangeRef(root, "")
			require.NoError(t, err)
			assert.Equal(t, active.Slug, ref.Slug)
		})
	})
}

func TestStatusKeepsRootActiveOnArchivedBranch(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		archivedSlug := "archived-root-branch-status"
		worktreePath := setupArchivedWorktreeForResolution(t, root, archivedSlug)
		runGit(t, worktreePath, "checkout", "--detach")
		runGit(t, root, "checkout", state.DefaultWorktreeBranch(archivedSlug))

		active := model.NewChange("root-active-status")
		active.CurrentState = model.StateS2Implement
		active.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, active))

		var out bytes.Buffer
		cmd := commandForRoot(t, root, makeStatusCmd())
		cmd.SetArgs([]string{"--json"})
		cmd.SetOut(&out)
		require.NoError(t, cmd.Execute())

		var view statusView
		require.NoError(t, json.Unmarshal(out.Bytes(), &view))
		assert.Equal(t, active.Slug, view.Slug)
		assert.False(t, view.Archived)
		assert.NotEqual(t, "archived", view.ExecutionMode)
	})
}

func TestResolveActiveChangeRefKeepsBoundActiveBeforeArchivedCandidate(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		archivedSlug := "archived-candidate-with-active-binding"
		worktreePath := setupArchivedWorktreeForResolution(t, root, archivedSlug)

		active := model.NewChange("active-bound-to-archived-worktree")
		active.CurrentState = model.StateS2Implement
		active.PlanSubStep = model.PlanSubStepNone
		active.WorktreePath = worktreePath
		active.WorktreeBranch = state.DefaultWorktreeBranch(archivedSlug)
		require.NoError(t, state.SaveChange(root, active))

		withNestedWorkingDirectory(t, worktreePath, func() {
			ref, err := resolveActiveChangeRef(root, "")
			require.NoError(t, err)
			assert.Equal(t, active.Slug, ref.Slug)
		})
	})
}

func TestResolveActiveChangeRefPreservesMultipleActiveMatchesBeforeArchivedFallback(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		archivedSlug := "archived-multiple-active-resolution"
		worktreePath := setupArchivedWorktreeForResolution(t, root, archivedSlug)

		activeA := model.NewChange("active-bound-duplicate-a")
		activeA.CurrentState = model.StateS2Implement
		activeA.PlanSubStep = model.PlanSubStepNone
		activeA.WorktreePath = worktreePath
		activeA.WorktreeBranch = state.DefaultWorktreeBranch(archivedSlug)
		require.NoError(t, state.SaveChange(root, activeA))

		activeB := model.NewChange("active-bound-duplicate-b")
		activeB.CurrentState = model.StateS2Implement
		activeB.PlanSubStep = model.PlanSubStepNone
		activeB.WorktreePath = worktreePath
		activeB.WorktreeBranch = state.DefaultWorktreeBranch(archivedSlug)
		require.NoError(t, state.SaveChange(root, activeB))

		withNestedWorkingDirectory(t, worktreePath, func() {
			ref, err := resolveActiveChangeRef(root, "")
			assert.Empty(t, ref.Slug)
			cliErr := asCLIError(err)
			require.NotNil(t, cliErr)
			assert.Equal(t, "active_context_ambiguous", cliErr.ErrorCode)
		})
	})
}

func TestResolveActiveChangeRefFailsClosedOnCorruptLocalArchive(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		initTestWorkspace(t, root)
		initGitRepoForWorktreeTests(t, root)

		archivedSlug := "archived-corrupt-resolution"
		worktreePath := setupArchivedWorktreeForResolution(t, root, archivedSlug)
		archivePath, err := state.ArchivedChangeFilePathForRead(root, archivedSlug)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(archivePath, []byte("slug: ["), 0o644))

		unbound := model.NewChange("unbound-active-corrupt-fallback")
		unbound.CurrentState = model.StateS2Implement
		unbound.PlanSubStep = model.PlanSubStepNone
		require.NoError(t, state.SaveChange(root, unbound))

		withNestedWorkingDirectory(t, worktreePath, func() {
			ref, err := resolveActiveChangeRef(root, "")
			assert.Empty(t, ref.Slug)
			cliErr := asCLIError(err)
			require.NotNil(t, cliErr)
			assert.Equal(t, categoryStateIntegrity, cliErr.Category)
			assert.Equal(t, "change_state_load_failed", cliErr.ErrorCode)
			assert.Equal(t, archivedSlug, cliErr.Slug)
			assert.Equal(t, filepath.Join("artifacts", "changes", "archived", archivedSlug, "change.yaml"), cliErr.Details["path"])
		})
	})
}

func setupArchivedWorktreeForResolution(t *testing.T, root, slug string) string {
	t.Helper()

	branch := state.DefaultWorktreeBranch(slug)
	worktreePath := state.DefaultWorktreePath(root, slug)
	runGit(t, root, "branch", branch)
	runGit(t, root, "worktree", "add", worktreePath, branch)

	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreePath
	change.WorktreeBranch = branch
	require.NoError(t, state.SaveChange(root, change))

	_, err := state.ArchiveChange(root, change, model.ChangeStatusDone)
	require.NoError(t, err)
	return worktreePath
}

func withNestedWorkingDirectory(t *testing.T, dir string, fn func()) {
	t.Helper()

	previousWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	defer func() {
		require.NoError(t, os.Chdir(previousWD))
	}()

	fn()
}
