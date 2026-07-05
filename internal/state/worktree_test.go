package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
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

func recordsContainPath(records []gitWorktreeRecord, path string) bool {
	for _, rec := range records {
		if rec.Path == path {
			return true
		}
	}
	return false
}

func resetNormalizePathTestState(t *testing.T) {
	t.Helper()

	oldEvalSymlinks := normalizePathEvalSymlinks
	normalizePathCache = sync.Map{}
	normalizePathEvalSymlinks = filepath.EvalSymlinks
	t.Cleanup(func() {
		normalizePathCache = sync.Map{}
		normalizePathEvalSymlinks = oldEvalSymlinks
	})
}

func TestNormalizePathCachesFallbackOnEvalSymlinksError(t *testing.T) {
	resetNormalizePathTestState(t)

	path := filepath.Join(t.TempDir(), "missing", "path")
	abs, err := filepath.Abs(path)
	require.NoError(t, err)

	var calls int
	normalizePathEvalSymlinks = func(got string) (string, error) {
		calls++
		assert.Equal(t, abs, got)
		return "", os.ErrNotExist
	}

	first, err := NormalizePath(path)
	require.NoError(t, err)
	second, err := NormalizePath(path)
	require.NoError(t, err)

	assert.Equal(t, filepath.Clean(abs), first)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, calls)
}

func TestNormalizePathDeduplicatesConcurrentFirstCallers(t *testing.T) {
	resetNormalizePathTestState(t)

	path := filepath.Join(t.TempDir(), "target")
	require.NoError(t, os.MkdirAll(path, 0o755))
	abs, err := filepath.Abs(path)
	require.NoError(t, err)

	var calls atomic.Int64
	var startedOnce sync.Once
	started := make(chan struct{})
	release := make(chan struct{})
	normalizePathEvalSymlinks = func(got string) (string, error) {
		calls.Add(1)
		if got != abs {
			return "", os.ErrInvalid
		}
		startedOnce.Do(func() {
			close(started)
		})
		<-release
		return got, nil
	}

	const workers = 16
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	results := make(chan string, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			got, err := NormalizePath(path)
			if err != nil {
				errs <- err
				return
			}
			results <- got
		}()
	}

	<-started
	close(release)
	wg.Wait()
	close(errs)
	close(results)

	for err := range errs {
		require.NoError(t, err)
	}
	count := 0
	for got := range results {
		assert.Equal(t, filepath.Clean(abs), got)
		count++
	}
	assert.Equal(t, workers, count)
	assert.Equal(t, int64(1), calls.Load())
}

func TestPersistScopeWorktreeMetadata(t *testing.T) {
	t.Parallel()
	change := model.NewChange("slug")
	require.NoError(t, PersistScopeWorktreeMetadata(&change, "/tmp/repo", "main"))
	normalizedPath, err := NormalizePath("/tmp/repo")
	require.NoError(t, err)
	assert.Equal(t, normalizedPath, change.WorktreePath)
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
	current := []gitWorktreeRecord{{Path: worktreeA}}
	lister := func(string) ([]gitWorktreeRecord, error) {
		calls++
		return cloneWorktreeRecords(current), nil
	}

	first, err := listGitWorktreeRecordsCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.True(t, recordsContainPath(first, worktreeA))

	second, err := listGitWorktreeRecordsCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.True(t, recordsContainPath(second, worktreeA))
	assert.Equal(t, 1, calls, "unchanged worktree probe should reuse cached listing")

	current = []gitWorktreeRecord{{Path: worktreeB}}
	later := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(worktreesDir, later, later))

	third, err := listGitWorktreeRecordsCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.True(t, recordsContainPath(third, worktreeB))
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
	current := []gitWorktreeRecord{{Path: worktreeA}}
	lister := func(string) ([]gitWorktreeRecord, error) {
		calls++
		return cloneWorktreeRecords(current), nil
	}

	fixed := time.Unix(1_700_000_000, 0).UTC()
	require.NoError(t, os.Chtimes(worktreesDir, fixed, fixed))

	first, err := listGitWorktreeRecordsCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.True(t, recordsContainPath(first, worktreeA))
	assert.Equal(t, 1, calls)

	require.NoError(t, os.RemoveAll(initialEntry))
	require.NoError(t, os.MkdirAll(filepath.Join(worktreesDir, "entry-b"), 0o755))
	require.NoError(t, os.Chtimes(worktreesDir, fixed, fixed))

	current = []gitWorktreeRecord{{Path: worktreeB}}

	second, err := listGitWorktreeRecordsCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.True(t, recordsContainPath(second, worktreeB))
	assert.False(t, recordsContainPath(second, worktreeA))
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
	lister := func(string) ([]gitWorktreeRecord, error) {
		calls++
		if calls == 1 {
			require.NoError(t, os.RemoveAll(initialEntry))
			require.NoError(t, os.MkdirAll(filepath.Join(worktreesDir, "entry-b"), 0o755))
			return []gitWorktreeRecord{{Path: worktreeA}}, nil
		}
		return []gitWorktreeRecord{{Path: worktreeB}}, nil
	}

	first, err := listGitWorktreeRecordsCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.True(t, recordsContainPath(first, worktreeA))

	second, err := listGitWorktreeRecordsCachedWithLister(repoRoot, lister)
	require.NoError(t, err)
	require.True(t, recordsContainPath(second, worktreeB))
	assert.False(t, recordsContainPath(second, worktreeA))
	assert.Equal(t, 2, calls, "probe changes during listing must prevent caching stale worktree sets")
}

func TestReconcileWorktreeBranchBindingRealignsBranchMismatch(t *testing.T) {
	repoRoot, worktreePath := setupRepoWithWorktree(t)

	// The worktree is actually on "feature"; record a mismatched branch.
	change := model.NewChange("rebind-demo")
	change.WorktreePath = worktreePath
	change.WorktreeBranch = "main"

	reconciled, err := ReconcileWorktreeBranchBinding(repoRoot, &change)
	require.NoError(t, err)
	assert.True(t, reconciled, "a pure branch mismatch on a dedicated worktree must reconcile")
	assert.Equal(t, "feature", change.WorktreeBranch, "recorded branch realigned to the worktree's actual branch")

	// Now that the binding matches reality, a second reconcile is a no-op.
	reconciledAgain, err := ReconcileWorktreeBranchBinding(repoRoot, &change)
	require.NoError(t, err)
	assert.False(t, reconciledAgain)
}

func TestReconcileWorktreeBranchBindingLeavesNonBranchMismatchAlone(t *testing.T) {
	repoRoot, _ := setupRepoWithWorktree(t)

	// An invalid/unregistered worktree path is NOT a pure branch mismatch, so
	// reconcile must fail closed and leave the recorded branch untouched.
	change := model.NewChange("rebind-noop")
	change.WorktreePath = filepath.Join(repoRoot, "missing")
	change.WorktreeBranch = "feature"

	reconciled, err := ReconcileWorktreeBranchBinding(repoRoot, &change)
	require.NoError(t, err)
	assert.False(t, reconciled)
	assert.Equal(t, "feature", change.WorktreeBranch)
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

func initGitRepoAt(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init", "--initial-branch=main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hello"), 0o644))
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "init")
}

func TestEnsureDefaultWorktreeForChange_ProvisionsNonDiscoveryByDefault(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)

	// A non-discovery change is exactly the case that used to be skipped with
	// "discovery_not_required" and run its whole lifecycle in the main checkout.
	change := model.NewChange("my-change")
	change.NeedsDiscovery = false

	binding, err := EnsureDefaultWorktreeForChange(root, &change)
	require.NoError(t, err)
	assert.True(t, binding.Created, "non-discovery change should provision a worktree by default")
	assert.Empty(t, binding.SkippedReason)
	assert.Equal(t, "feat/my-change", binding.Branch)
	assert.Contains(t, filepath.ToSlash(binding.Path), ".worktrees/my-change")
	assert.NotEmpty(t, change.WorktreePath, "binding metadata must be persisted on the change")
}

func TestEnsureDefaultWorktreeForChangeRejectsOptionLikeBaseRef(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)

	change := model.NewChange("my-change")
	change.BaseRef = "--upload-pack=ssh://evil.example/repo"

	_, err := EnsureDefaultWorktreeForChange(root, &change)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base_ref")
	assert.Contains(t, err.Error(), "must not start with '-'")
	assert.NotContains(t, err.Error(), "git worktree add failed")
	assert.NoDirExists(t, filepath.Join(root, ".worktrees", "my-change"))
}

func TestEnsureDefaultWorktreeForChangeRejectsInvalidBaseRefControlChars(t *testing.T) {
	for _, tt := range []struct {
		name    string
		baseRef string
	}{
		{name: "nul", baseRef: "main\x00refs/heads/main"},
		{name: "carriage return", baseRef: "main\rrefs/heads/main"},
		{name: "line feed", baseRef: "main\nrefs/heads/main"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			initGitRepoAt(t, root)

			change := model.NewChange("my-change")
			change.BaseRef = tt.baseRef

			_, err := EnsureDefaultWorktreeForChange(root, &change)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid base_ref")
			assert.Contains(t, err.Error(), "value must be a single git ref")
			assert.NotContains(t, err.Error(), "git worktree add failed")
			assert.NoDirExists(t, filepath.Join(root, ".worktrees", "my-change"))
		})
	}
}

func TestEnsureDefaultWorktreeForChangeRejectsUnknownBaseRefBeforeWorktreeAdd(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)

	change := model.NewChange("my-change")
	change.BaseRef = "definitely-not-a-ref"

	_, err := EnsureDefaultWorktreeForChange(root, &change)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid base_ref")
	assert.Contains(t, err.Error(), "repair the change authority")
	assert.NotContains(t, err.Error(), "git worktree add failed")
	assert.NoDirExists(t, filepath.Join(root, ".worktrees", "my-change"))
}

func TestEnsureDefaultWorktreeForChangeAcceptsTagBaseRef(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)
	runGit(t, root, "tag", "v0.1.0")

	change := model.NewChange("my-change")
	change.BaseRef = "v0.1.0"

	binding, err := EnsureDefaultWorktreeForChange(root, &change)
	require.NoError(t, err)
	assert.True(t, binding.Created)
	assert.Contains(t, filepath.ToSlash(binding.Path), ".worktrees/my-change")
}

func TestEnsureDefaultWorktreeForChange_DisabledByConfig(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".slipway.yaml"),
		[]byte("governance:\n  auto_provision_worktree: false\n"), 0o644))

	change := model.NewChange("my-change")
	binding, err := EnsureDefaultWorktreeForChange(root, &change)
	require.NoError(t, err)
	assert.False(t, binding.Created)
	assert.Equal(t, "worktree_provisioning_disabled", binding.SkippedReason)
	assert.Empty(t, change.WorktreePath)
}

func TestEnsureDefaultWorktreeForChange_SkipsNonGitRepo(t *testing.T) {
	root := t.TempDir() // never `git init`ed

	change := model.NewChange("my-change")
	binding, err := EnsureDefaultWorktreeForChange(root, &change)
	require.NoError(t, err)
	assert.Equal(t, "not_git_repository", binding.SkippedReason)
	assert.False(t, binding.Created)
}
