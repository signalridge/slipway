package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupArchivedChangeWorktree builds a repo with a Slipway-convention dedicated
// worktree (`.worktrees/<slug>` on `feat/<slug>`) for slug, saves a change bound
// to it, then archives it as done. It returns the repo root and the worktree
// path so callers can assert how the archived change resolves from each.
func setupArchivedChangeWorktree(t *testing.T, slug string) (repoRoot, worktreePath string) {
	t.Helper()
	repoRoot = t.TempDir()
	runGit(t, repoRoot, "init", "--initial-branch=main")
	runGit(t, repoRoot, "config", "user.email", "test@example.com")
	runGit(t, repoRoot, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, ".slipway.yaml"), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))
	runGit(t, repoRoot, "add", ".")
	runGit(t, repoRoot, "commit", "-m", "init")

	branch := DefaultWorktreeBranch(slug)
	worktreePath = DefaultWorktreePath(repoRoot, slug)
	runGit(t, repoRoot, "branch", branch)
	runGit(t, repoRoot, "worktree", "add", worktreePath, branch)

	change := model.NewChange(slug)
	change.CurrentState = model.StateS2Implement
	change.PlanSubStep = model.PlanSubStepNone
	change.WorktreePath = worktreePath
	change.WorktreeBranch = branch
	require.NoError(t, SaveChange(repoRoot, change))

	_, err := ArchiveChange(repoRoot, change, model.ChangeStatusDone)
	require.NoError(t, err)
	return repoRoot, worktreePath
}

// TestFindArchivedChangeForWorktreePrefersLocalArchivedChange proves the #283
// fix: from inside an archived change's dedicated worktree, the local archived
// change is recovered even though the git-local binding was removed at archive
// time. This is what lets unscoped status/validate report the local archived
// change instead of an unrelated active change bound to a different worktree.
func TestFindArchivedChangeForWorktreePrefersLocalArchivedChange(t *testing.T) {
	t.Parallel()
	slug := "archived-review-worktree"
	root, worktreePath := setupArchivedChangeWorktree(t, slug)

	// Archive removed the runtime binding; resolution must reconstruct it.
	_, statErr := os.Stat(WorktreeBindingPath(root, slug))
	require.True(t, os.IsNotExist(statErr), "archive should have removed the runtime worktree binding")

	resolved, ok, err := FindArchivedChangeForWorktree(root, worktreePath)
	require.NoError(t, err)
	require.True(t, ok, "archived change owning the invocation worktree must be found")
	assert.Equal(t, slug, resolved.Slug)
	assert.Equal(t, model.ChangeStatusDone, resolved.Status)
}

// TestFindArchivedChangeForWorktreeIgnoresProjectRoot proves the project root is
// never treated as a dedicated change worktree, so unscoped resolution invoked
// from the repo root still falls through to active-change resolution (status
// from the repo root keeps reporting the global active change unchanged).
func TestFindArchivedChangeForWorktreeIgnoresProjectRoot(t *testing.T) {
	t.Parallel()
	root, _ := setupArchivedChangeWorktree(t, "archived-not-at-root")

	_, ok, err := FindArchivedChangeForWorktree(root, root)
	require.NoError(t, err)
	assert.False(t, ok, "the project root must not match any archived change's dedicated worktree")
}
