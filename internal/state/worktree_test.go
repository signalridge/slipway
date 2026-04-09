package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func hasReasonCode(reasons []model.ReasonCode, code string) bool {
	for _, reason := range reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func TestPersistScopeWorktreeMetadata(t *testing.T) {
	t.Parallel()
	change := model.NewChange("slug")
	require.NoError(t, PersistScopeWorktreeMetadata(&change, "/tmp/repo", "main"))
	assert.Equal(t, "/tmp/repo", change.WorktreePath)
	assert.Equal(t, "main", change.WorktreeBranch)
}

func TestValidateWorktreeAuthenticity(t *testing.T) {
	t.Parallel()
	repoRoot, worktreePath := setupRepoWithWorktree(t)

	reasons, err := ValidateWorktreeAuthenticityReasons(repoRoot, worktreePath, "feature")
	require.NoError(t, err)
	assert.Empty(t, reasons)
}

func TestValidateWorktreeAuthenticityMissingPath(t *testing.T) {
	t.Parallel()
	repoRoot, _ := setupRepoWithWorktree(t)

	reasons, reasonErr := ValidateWorktreeAuthenticityReasons(repoRoot, filepath.Join(repoRoot, "missing"), "feature")
	require.NoError(t, reasonErr)
	assert.Contains(t, reasons, WorktreeReasonPathInvalid)
}

func TestValidateWorktreeAuthenticityNonWorktreePath(t *testing.T) {
	t.Parallel()
	repoRoot, _ := setupRepoWithWorktree(t)
	other := t.TempDir()

	reasons, err := ValidateWorktreeAuthenticityReasons(repoRoot, other, "feature")
	require.NoError(t, err)
	assert.Contains(t, reasons, WorktreeReasonPathInvalid)
}

func TestValidateWorktreeAuthenticityBranchMismatch(t *testing.T) {
	t.Parallel()
	repoRoot, worktreePath := setupRepoWithWorktree(t)

	reasons, reasonErr := ValidateWorktreeAuthenticityReasons(repoRoot, worktreePath, "main")
	require.NoError(t, reasonErr)
	assert.Contains(t, reasons, WorktreeReasonBranchMismatch)
}

func TestValidateWorktreeAuthenticityMetadataMissing(t *testing.T) {
	t.Parallel()
	repoRoot, _ := setupRepoWithWorktree(t)

	reasons, err := ValidateWorktreeAuthenticityReasons(repoRoot, "", "")
	require.NoError(t, err)
	assert.Equal(t, []string{WorktreeReasonMetadataRequired}, reasons)
}

func TestValidateDedicatedWorktreeAuthenticityReasonsRejectsMainWorktreeForNestedScope(t *testing.T) {
	t.Parallel()

	repoRoot := createRuntimeRepoLayout(t)
	scopeRoot := filepath.Join(repoRoot, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(scopeRoot, ".slipway.yaml"), []byte("{}"), 0o644))

	reasons, err := ValidateDedicatedWorktreeAuthenticityReasons(scopeRoot, repoRoot, "main")
	require.NoError(t, err)
	assert.True(t, hasReasonCode(reasons, WorktreeReasonDedicatedRequired))
}

func TestListGitWorktreesCachedWithListerReusesCacheUntilProbeChanges(t *testing.T) {
	repoRoot, _ := setupRepoWithWorktree(t)
	resetGitCommonDirCache()
	worktreesDir := filepath.Join(GitCommonDir(repoRoot), "worktrees")
	// Ensure worktrees dir exists (setupRepoWithWorktree already creates one).

	worktreeA := filepath.Join(repoRoot, "wt-a")
	worktreeB := filepath.Join(repoRoot, "wt-b")
	calls := 0
	current := map[string]struct{}{
		worktreeA: {},
	}
	lister := func(string) (map[string]struct{}, error) {
		calls++
		out := make(map[string]struct{}, len(current))
		for path := range current {
			out[path] = struct{}{}
		}
		return out, nil
	}

	first, err := listGitWorktreesCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.Contains(t, first, worktreeA)

	second, err := listGitWorktreesCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.Contains(t, second, worktreeA)
	assert.Equal(t, 1, calls, "unchanged worktree probe should reuse cached listing")

	current = map[string]struct{}{
		worktreeB: {},
	}
	later := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(worktreesDir, later, later))

	third, err := listGitWorktreesCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.Contains(t, third, worktreeB)
	assert.Equal(t, 2, calls, "worktree probe changes must invalidate the cache")
}

func TestListGitWorktreesCachedWithListerInvalidatesWhenEntryFingerprintChanges(t *testing.T) {
	repoRoot, _ := setupRepoWithWorktree(t)
	resetGitCommonDirCache()
	worktreesDir := filepath.Join(GitCommonDir(repoRoot), "worktrees")

	initialEntry := filepath.Join(worktreesDir, "entry-a")
	require.NoError(t, os.MkdirAll(initialEntry, 0o755))

	worktreeA := filepath.Join(repoRoot, "wt-a")
	worktreeB := filepath.Join(repoRoot, "wt-b")
	calls := 0
	current := map[string]struct{}{
		worktreeA: {},
	}
	lister := func(string) (map[string]struct{}, error) {
		calls++
		out := make(map[string]struct{}, len(current))
		for path := range current {
			out[path] = struct{}{}
		}
		return out, nil
	}

	fixed := time.Unix(1_700_000_000, 0).UTC()
	require.NoError(t, os.Chtimes(worktreesDir, fixed, fixed))

	first, err := listGitWorktreesCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.Contains(t, first, worktreeA)
	assert.Equal(t, 1, calls)

	require.NoError(t, os.RemoveAll(initialEntry))
	require.NoError(t, os.MkdirAll(filepath.Join(worktreesDir, "entry-b"), 0o755))
	require.NoError(t, os.Chtimes(worktreesDir, fixed, fixed))

	current = map[string]struct{}{
		worktreeB: {},
	}

	second, err := listGitWorktreesCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.Contains(t, second, worktreeB)
	assert.NotContains(t, second, worktreeA)
	assert.Equal(t, 2, calls, "entry-name changes must invalidate the cache even when directory modtime is restored")
}

func TestListGitWorktreesCachedWithListerDoesNotCacheStaleResultWhenProbeChangesDuringList(t *testing.T) {
	repoRoot, _ := setupRepoWithWorktree(t)
	resetGitCommonDirCache()
	worktreesDir := filepath.Join(GitCommonDir(repoRoot), "worktrees")

	initialEntry := filepath.Join(worktreesDir, "entry-a")
	require.NoError(t, os.MkdirAll(initialEntry, 0o755))

	worktreeA := filepath.Join(repoRoot, "wt-a")
	worktreeB := filepath.Join(repoRoot, "wt-b")
	calls := 0
	lister := func(string) (map[string]struct{}, error) {
		calls++
		if calls == 1 {
			require.NoError(t, os.RemoveAll(initialEntry))
			require.NoError(t, os.MkdirAll(filepath.Join(worktreesDir, "entry-b"), 0o755))
			return map[string]struct{}{worktreeA: {}}, nil
		}
		return map[string]struct{}{worktreeB: {}}, nil
	}

	first, err := listGitWorktreesCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.Contains(t, first, worktreeA)

	second, err := listGitWorktreesCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.Contains(t, second, worktreeB)
	assert.NotContains(t, second, worktreeA)
	assert.Equal(t, 2, calls, "probe changes during listing must prevent caching stale worktree sets")
}

func setupRepoWithWorktree(t *testing.T) (repoRoot string, worktreePath string) {
	t.Helper()
	repoRoot = t.TempDir()
	worktreePath = filepath.Join(t.TempDir(), "feature-wt")

	runGit(t, repoRoot, "init", "--initial-branch=main")
	runGit(t, repoRoot, "config", "user.email", "test@example.com")
	runGit(t, repoRoot, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("hello"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(repoRoot, ".slipway.yaml"), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))
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
