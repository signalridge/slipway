package fsutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindProjectRootReturnsGitRepoRootWithProjectConfigOnly(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Slipway Test")
	writeProjectConfig(t, root)
	start := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(start, 0o755))

	got, err := FindProjectRoot(start)
	require.NoError(t, err)
	expectedRoot, _ := filepath.EvalSymlinks(root)
	assert.Equal(t, expectedRoot, got)
}

func TestFindProjectRootReturnsErrorOutsideGitRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	start := filepath.Join(root, "x", "y")
	require.NoError(t, os.MkdirAll(start, 0o755))

	_, err := FindProjectRoot(start)
	require.ErrorIs(t, err, ErrProjectRootNotFound)
	require.ErrorContains(t, err, "slipway requires a git repository")
	require.ErrorContains(t, err, "is not inside a git working tree")
}

func TestFindProjectRootAcceptsFileStartPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Slipway Test")
	writeProjectConfig(t, root)
	writeSharedScopeMarker(t, root)

	startFile := filepath.Join(root, "a", "b", "c", "notes.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(startFile), 0o755))
	require.NoError(t, os.WriteFile(startFile, []byte("note\n"), 0o644))

	got, err := FindProjectRoot(startFile)
	require.NoError(t, err)
	expectedRoot, _ := filepath.EvalSymlinks(root)
	assert.Equal(t, expectedRoot, got)
}

func TestFindProjectRootPrefersRegisteredNestedScopeInMainWorktree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Slipway Test")
	writeProjectConfig(t, root)
	writeSharedScopeMarker(t, root)

	nested := filepath.Join(root, "services", "billing")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	writeProjectConfig(t, nested)
	writeSharedScopeMarker(t, nested)

	start := filepath.Join(nested, "internal", "api")
	require.NoError(t, os.MkdirAll(start, 0o755))

	got, err := FindProjectRoot(start)
	require.NoError(t, err)

	expectedRoot, _ := filepath.EvalSymlinks(nested)
	gotResolved, _ := filepath.EvalSymlinks(got)
	assert.Equal(t, expectedRoot, gotResolved)
}

func TestFindProjectRootResolvesNestedScopeFromLinkedWorktree(t *testing.T) {
	t.Parallel()
	mainRepo := t.TempDir()
	runGit(t, mainRepo, "init", "-b", "main")
	runGit(t, mainRepo, "config", "user.email", "test@example.com")
	runGit(t, mainRepo, "config", "user.name", "Slipway Test")
	writeProjectConfig(t, mainRepo)
	writeSharedScopeMarker(t, mainRepo)

	nested := filepath.Join(mainRepo, "services", "billing")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	writeProjectConfig(t, nested)
	writeSharedScopeMarker(t, nested)

	wtPath := filepath.Join(t.TempDir(), "my-worktree")
	runGit(t, mainRepo, "worktree", "add", wtPath, "-b", "wt-branch")
	writeProjectConfig(t, filepath.Join(wtPath, "services", "billing"))
	writeWorkspaceScopeMarker(t, wtPath, filepath.Join("services", "billing"))

	start := filepath.Join(wtPath, "services", "billing", "internal", "api")
	require.NoError(t, os.MkdirAll(start, 0o755))

	got, err := FindProjectRoot(start)
	require.NoError(t, err)

	expectedRoot, _ := filepath.EvalSymlinks(nested)
	gotResolved, _ := filepath.EvalSymlinks(got)
	assert.Equal(t, expectedRoot, gotResolved)
}

func TestFindProjectRootUsesCanonicalNestedScopeForMarkerOnlyBoundWorktree(t *testing.T) {
	t.Parallel()
	mainRepo := t.TempDir()
	runGit(t, mainRepo, "init", "-b", "main")
	runGit(t, mainRepo, "config", "user.email", "test@example.com")
	runGit(t, mainRepo, "config", "user.name", "Slipway Test")
	writeProjectConfig(t, mainRepo)
	writeSharedScopeMarker(t, mainRepo)

	nested := filepath.Join(mainRepo, "services", "billing")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	writeProjectConfig(t, nested)
	writeSharedScopeMarker(t, nested)

	wtPath := filepath.Join(t.TempDir(), "stale-worktree")
	runGit(t, mainRepo, "worktree", "add", wtPath, "-b", "wt-branch")
	require.NoError(t, os.MkdirAll(filepath.Join(wtPath, "services", "billing"), 0o755))
	writeWorkspaceScopeMarker(t, wtPath, filepath.Join("services", "billing"))

	start := filepath.Join(wtPath, "services", "billing", "internal", "api")
	require.NoError(t, os.MkdirAll(start, 0o755))

	got, err := FindProjectRoot(start)
	require.NoError(t, err)

	expectedRoot, _ := filepath.EvalSymlinks(nested)
	gotResolved, _ := filepath.EvalSymlinks(got)
	assert.Equal(t, expectedRoot, gotResolved)
}

func TestFindProjectRootIgnoresLinkedWorktreeScopeMarkerWithoutCanonicalConfig(t *testing.T) {
	t.Parallel()
	mainRepo := t.TempDir()
	runGit(t, mainRepo, "init", "-b", "main")
	runGit(t, mainRepo, "config", "user.email", "test@example.com")
	runGit(t, mainRepo, "config", "user.name", "Slipway Test")
	writeProjectConfig(t, mainRepo)
	writeSharedScopeMarker(t, mainRepo)

	nested := filepath.Join(mainRepo, "services", "billing")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	writeSharedScopeMarker(t, nested)

	wtPath := filepath.Join(t.TempDir(), "stale-worktree")
	runGit(t, mainRepo, "worktree", "add", wtPath, "-b", "wt-branch")
	require.NoError(t, os.MkdirAll(filepath.Join(wtPath, "services", "billing"), 0o755))
	writeWorkspaceScopeMarker(t, wtPath, filepath.Join("services", "billing"))

	start := filepath.Join(wtPath, "services", "billing", "internal", "api")
	require.NoError(t, os.MkdirAll(start, 0o755))

	got, err := FindProjectRoot(start)
	require.NoError(t, err)

	expectedRoot, _ := filepath.EvalSymlinks(mainRepo)
	gotResolved, _ := filepath.EvalSymlinks(got)
	assert.Equal(t, expectedRoot, gotResolved)
}

func TestResolveGitWorkspaceInfoRequiresAbsolutePathFormatBeforeShowTopLevel(t *testing.T) {
	root := t.TempDir()
	fakeBin := filepath.Join(root, "fake-bin")
	require.NoError(t, os.MkdirAll(fakeBin, 0o755))

	expectedRepo := filepath.Join(root, "repo")
	expectedGitDir := filepath.Join(expectedRepo, ".git")
	fakeGit := filepath.Join(fakeBin, "git")
	require.NoError(t, os.WriteFile(fakeGit, []byte(fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
args=("$@")
path_idx=-1
show_idx=-1
for i in "${!args[@]}"; do
  if [[ "${args[$i]}" == "--path-format=absolute" && $path_idx -lt 0 ]]; then
    path_idx=$i
  fi
  if [[ "${args[$i]}" == "--show-toplevel" && $show_idx -lt 0 ]]; then
    show_idx=$i
  fi
done
if [[ $path_idx -lt 0 || $show_idx -lt 0 || $path_idx -gt $show_idx ]]; then
  echo "expected --path-format=absolute before --show-toplevel" >&2
  exit 1
fi
printf '%%s\n%%s\n%%s\n' %q %q %q
`, filepath.ToSlash(expectedRepo), filepath.ToSlash(expectedGitDir), filepath.ToSlash(expectedGitDir))), 0o755))
	if runtime.GOOS == "windows" {
		wrapperPath := filepath.Join(fakeBin, "git.bat")
		wrapper := "@echo off\r\nbash \"%~dp0git\" %*\r\nexit /b %ERRORLEVEL%\r\n"
		require.NoError(t, os.WriteFile(wrapperPath, []byte(wrapper), 0o755))
	}

	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	info, err := resolveGitWorkspaceInfo(root)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(expectedRepo), info.worktreeRoot)
	assert.Equal(t, filepath.Clean(expectedGitDir), info.commonDir)
	assert.Equal(t, filepath.Clean(expectedGitDir), info.localGitDir)
}

func writeSharedScopeMarker(t *testing.T, root string) {
	t.Helper()
	info, err := resolveGitWorkspaceInfo(root)
	require.NoError(t, err)
	scopeRoot := root
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		scopeRoot = resolved
	}
	scopeRel := scopeRelativePathWithinWorkspace(info.worktreeRoot, scopeRoot)
	path := scopeMarkerPath(info.commonDir, scopeRel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("scope\n"), 0o644))
}

func writeWorkspaceScopeMarker(t *testing.T, worktreeRoot string, rel string) {
	t.Helper()
	info, err := resolveGitWorkspaceInfo(worktreeRoot)
	require.NoError(t, err)
	path := scopeMarkerPath(info.localGitDir, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte("scope\n"), 0o644))
}

func writeProjectConfig(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ProjectConfigFileName), []byte("defaults:\n  artifact_schema: expanded\n"), 0o644))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}
