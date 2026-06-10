package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
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
