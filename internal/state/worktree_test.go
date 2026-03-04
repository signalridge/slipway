package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistScopeWorktreeMetadata(t *testing.T) {
	change := model.NewChangeState(mustRequestID(t), "slug")
	require.NoError(t, PersistScopeWorktreeMetadata(&change, "/tmp/repo", "main"))
	assert.Equal(t, "/tmp/repo", change.WorktreePath)
	assert.Equal(t, "main", change.WorktreeBranch)
}

func TestValidateWorktreeAuthenticity(t *testing.T) {
	repoRoot, worktreePath := setupRepoWithWorktree(t)

	err := ValidateWorktreeAuthenticity(repoRoot, worktreePath, "feature")
	require.NoError(t, err)
}

func TestValidateWorktreeAuthenticityMissingPath(t *testing.T) {
	repoRoot, _ := setupRepoWithWorktree(t)

	err := ValidateWorktreeAuthenticity(repoRoot, filepath.Join(repoRoot, "missing"), "feature")
	require.Error(t, err)
}

func TestValidateWorktreeAuthenticityNonWorktreePath(t *testing.T) {
	repoRoot, _ := setupRepoWithWorktree(t)
	other := t.TempDir()

	err := ValidateWorktreeAuthenticity(repoRoot, other, "feature")
	require.Error(t, err)
}

func TestValidateWorktreeAuthenticityBranchMismatch(t *testing.T) {
	repoRoot, worktreePath := setupRepoWithWorktree(t)

	err := ValidateWorktreeAuthenticity(repoRoot, worktreePath, "main")
	require.Error(t, err)
}

func setupRepoWithWorktree(t *testing.T) (repoRoot string, worktreePath string) {
	t.Helper()
	repoRoot = t.TempDir()
	worktreePath = filepath.Join(t.TempDir(), "feature-wt")

	runGit(t, repoRoot, "init", "--initial-branch=main")
	runGit(t, repoRoot, "config", "user.email", "test@example.com")
	runGit(t, repoRoot, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello"), 0o644))
	runGit(t, repoRoot, "add", ".")
	runGit(t, repoRoot, "commit", "-m", "init")
	runGit(t, repoRoot, "branch", "feature")
	runGit(t, repoRoot, "worktree", "add", worktreePath, "feature")

	return repoRoot, worktreePath
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v failed: %s", args, string(out))
}
