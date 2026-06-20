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
	})
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
