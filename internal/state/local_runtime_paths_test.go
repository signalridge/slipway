package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetGitCommonDirCache() {
	gitCommonDirCache.mu.Lock()
	defer gitCommonDirCache.mu.Unlock()
	gitCommonDirCache.entries = map[string]gitCommonDirCacheEntry{}
}

func TestGitScopedPathsUseUnifiedSlipwayNamespace(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	normalizedRoot, err := NormalizePath(root)
	assert.NoError(t, err)

	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway"), GitStateDir(root))
	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway", "locks", "changes", "demo.lock"), ChangeStateLockPath(root, "demo"))
	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway", "locks", "change-create.lock"), ChangeCreateLockPath(root))
	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway", "locks", "repair.lock"), RepairLockPath(root))
	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway", "processes", "demo", "task_pids.json"), TaskPIDFilePath(root, "demo"))
	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway", "cache", "changes", "demo", "governance_snapshot.yaml"), GovernanceSnapshotCachePath(root, "demo"))
	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway", "repair-backups", "config"), ConfigBackupDir(root))
	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway", "runtime", "changes", "demo"), ChangeDir(root, "demo"))
	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway", "runtime", "changes", "demo", "handoff.md"), ChangeHandoffPath(root, "demo"))
}

func TestGitScopedPathsNamespaceNestedScopeByScopeRoot(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))

	normalizedRoot, err := NormalizePath(root)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(normalizedRoot, ".git", "slipway", "scopes", "services", "billing"), GitStateDir(scopeRoot))
	assert.Equal(
		t,
		filepath.Join(normalizedRoot, ".git", "slipway", "scopes", "services", "billing", "runtime", "changes", "demo"),
		ChangeDir(scopeRoot, "demo"),
	)
	assert.Equal(
		t,
		filepath.Join(normalizedRoot, ".git", "slipway", "scopes", "services", "billing", "runtime", "changes", "demo", "handoff.md"),
		ChangeHandoffPath(scopeRoot, "demo"),
	)
}

func TestGitCommonDirCachesResolverPerRoot(t *testing.T) {
	root := t.TempDir()

	originalResolver := gitCommonDirResolver
	resetGitCommonDirCache()
	t.Cleanup(func() {
		gitCommonDirResolver = originalResolver
		resetGitCommonDirCache()
	})

	calls := 0
	gitCommonDirResolver = func(normalizedRoot string) (string, error) {
		calls++
		return filepath.Join(normalizedRoot, ".git"), nil
	}

	first := GitCommonDir(root)
	second := GitCommonDir(root)

	assert.Equal(t, first, second)
	assert.Equal(t, 1, calls)
}

func TestGitCommonDirCacheInvalidatesWhenGitMetadataChanges(t *testing.T) {
	root := t.TempDir()
	gitMetadata := filepath.Join(root, ".git")
	require.NoError(t, os.WriteFile(gitMetadata, []byte("gitdir: first\n"), 0o644))
	normalizedRoot, err := NormalizePath(root)
	require.NoError(t, err)

	originalResolver := gitCommonDirResolver
	resetGitCommonDirCache()
	t.Cleanup(func() {
		gitCommonDirResolver = originalResolver
		resetGitCommonDirCache()
	})

	calls := 0
	gitCommonDirResolver = func(normalizedRoot string) (string, error) {
		calls++
		if calls == 1 {
			return filepath.Join(normalizedRoot, ".git-first"), nil
		}
		return filepath.Join(normalizedRoot, ".git-second"), nil
	}

	first := GitCommonDir(root)

	require.NoError(t, os.WriteFile(gitMetadata, []byte("gitdir: second\n"), 0o644))
	now := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(gitMetadata, now, now))

	second := GitCommonDir(root)

	assert.Equal(t, filepath.Join(normalizedRoot, ".git-first"), first)
	assert.Equal(t, filepath.Join(normalizedRoot, ".git-second"), second)
	assert.Equal(t, 2, calls)
}

func TestGitCommonDirCacheInvalidatesWhenCommondirMetadataChanges(t *testing.T) {
	root := t.TempDir()
	normalizedRoot, err := NormalizePath(root)
	require.NoError(t, err)

	gitDir := filepath.Join(normalizedRoot, ".repo-meta")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(normalizedRoot, ".git"), []byte("gitdir: .repo-meta\n"), 0o644))

	commondirPath := filepath.Join(gitDir, "commondir")
	require.NoError(t, os.WriteFile(commondirPath, []byte("../.git-first\n"), 0o644))

	originalResolver := gitCommonDirResolver
	resetGitCommonDirCache()
	t.Cleanup(func() {
		gitCommonDirResolver = originalResolver
		resetGitCommonDirCache()
	})

	calls := 0
	gitCommonDirResolver = func(normalizedRoot string) (string, error) {
		calls++
		if calls == 1 {
			return filepath.Join(normalizedRoot, ".git-first"), nil
		}
		return filepath.Join(normalizedRoot, ".git-second"), nil
	}

	first := GitCommonDir(root)

	require.NoError(t, os.WriteFile(commondirPath, []byte("../.git-second\n"), 0o644))
	now := time.Now().Add(2 * time.Second)
	require.NoError(t, os.Chtimes(commondirPath, now, now))

	second := GitCommonDir(root)

	assert.Equal(t, filepath.Join(normalizedRoot, ".git-first"), first)
	assert.Equal(t, filepath.Join(normalizedRoot, ".git-second"), second)
	assert.Equal(t, 2, calls)
}

func TestGitCommonDirUsesAncestorGitMetadataDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nestedRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core]\nrepositoryformatversion = 0\n"), 0o644))
	require.NoError(t, os.MkdirAll(nestedRoot, 0o755))

	normalizedRoot, err := NormalizePath(root)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(normalizedRoot, ".git"), GitCommonDir(nestedRoot))
}

func TestGitCommonDirUsesLinkedWorktreeCommondirMetadata(t *testing.T) {
	t.Parallel()

	worktreeRoot := t.TempDir()
	nestedRoot := filepath.Join(worktreeRoot, "services", "billing")
	commonGitDir := filepath.Join(t.TempDir(), "repo", ".git")
	gitDir := filepath.Join(commonGitDir, "worktrees", "feature")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("../..\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeRoot, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644))
	require.NoError(t, os.MkdirAll(nestedRoot, 0o755))

	assert.Equal(t, filepath.Clean(commonGitDir), GitCommonDir(nestedRoot))
}

func TestGitWorkspaceRootUsesAncestorGitMetadataDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nestedRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[core]\nrepositoryformatversion = 0\n"), 0o644))
	require.NoError(t, os.MkdirAll(nestedRoot, 0o755))

	normalizedRoot, err := NormalizePath(root)
	require.NoError(t, err)

	got, err := gitWorkspaceRoot(nestedRoot)
	require.NoError(t, err)
	assert.Equal(t, normalizedRoot, got)
}

func TestGitWorkspaceRootUsesLinkedWorktreeMetadata(t *testing.T) {
	t.Parallel()

	worktreeRoot := t.TempDir()
	nestedRoot := filepath.Join(worktreeRoot, "services", "billing")
	gitDir := filepath.Join(t.TempDir(), "repo", ".git", "worktrees", "feature")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "commondir"), []byte("../..\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeRoot, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644))
	require.NoError(t, os.MkdirAll(nestedRoot, 0o755))

	normalizedRoot, err := NormalizePath(worktreeRoot)
	require.NoError(t, err)

	got, err := gitWorkspaceRoot(nestedRoot)
	require.NoError(t, err)
	assert.Equal(t, normalizedRoot, got)
}

func TestGitWorkspaceRootRejectsIncompleteMetadataFastPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))

	_, err := ResolveGitWorkspaceRoot(root)
	require.Error(t, err)
}

func TestGitBranchFromMetadataUsesGitDirectoryHead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nestedRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
	require.NoError(t, os.MkdirAll(nestedRoot, 0o755))

	branch, ok := gitBranchFromMetadata(nestedRoot)
	require.True(t, ok)
	assert.Equal(t, "main", branch)
}

func TestGitBranchFromMetadataUsesLinkedWorktreeHead(t *testing.T) {
	t.Parallel()

	worktreeRoot := t.TempDir()
	nestedRoot := filepath.Join(worktreeRoot, "services", "billing")
	gitDir := filepath.Join(t.TempDir(), "repo", ".git", "worktrees", "feature")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature/demo\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeRoot, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644))
	require.NoError(t, os.MkdirAll(nestedRoot, 0o755))

	branch, ok := gitBranchFromMetadata(nestedRoot)
	require.True(t, ok)
	assert.Equal(t, "feature/demo", branch)
}

func TestGitBranchFromMetadataReportsDetachedHead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "HEAD"), []byte("0123456789abcdef\n"), 0o644))

	branch, ok := gitBranchFromMetadata(root)
	require.True(t, ok)
	assert.Equal(t, "HEAD", branch)
}

func TestScopeRootInWorkspaceMapsNestedScopeIntoWorkspace(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))

	workspaceRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workspaceRoot, "services", "billing"), 0o755))

	got, err := scopeRootInWorkspace(scopeRoot, workspaceRoot)
	require.NoError(t, err)

	expected, err := NormalizePath(filepath.Join(workspaceRoot, "services", "billing"))
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestScopeRootInWorkspaceKeepsWorkspaceRootForRepoRoot(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	workspaceRoot := t.TempDir()

	got, err := scopeRootInWorkspace(root, workspaceRoot)
	require.NoError(t, err)

	expected, err := NormalizePath(workspaceRoot)
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}

func TestScopeRootInWorkspaceNormalizesResolvedNestedPath(t *testing.T) {
	t.Parallel()

	root := createRuntimeRepoLayout(t)
	scopeRoot := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(scopeRoot, 0o755))

	workspaceRoot := t.TempDir()
	realServices := filepath.Join(workspaceRoot, "real-services")
	require.NoError(t, os.MkdirAll(filepath.Join(realServices, "billing"), 0o755))
	require.NoError(t, os.Symlink(realServices, filepath.Join(workspaceRoot, "services")))

	got, err := scopeRootInWorkspace(scopeRoot, workspaceRoot)
	require.NoError(t, err)

	expected, err := NormalizePath(filepath.Join(workspaceRoot, "services", "billing"))
	require.NoError(t, err)
	assert.Equal(t, expected, got)
}
