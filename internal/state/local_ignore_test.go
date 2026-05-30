package state

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureLocalStateGitIgnoreIsIdempotentAndPreservesUserEntries(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n.DS_Store\n"), 0o644))

	first, err := EnsureLocalStateGitIgnore(root)
	require.NoError(t, err)
	assert.True(t, first.Changed)

	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "node_modules/\n.DS_Store\n")
	assert.Contains(t, string(content), LocalStateGitIgnoreBlock())

	second, err := EnsureLocalStateGitIgnore(root)
	require.NoError(t, err)
	assert.False(t, second.Changed)

	again, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, string(content), string(again))
}

func TestEnsureLocalStateGitIgnoreReplacesExistingManagedBlock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stale := "build/\n\n" +
		localStateGitIgnoreStart + "\n" +
		"/old-slipway-local-state/\n" +
		localStateGitIgnoreEnd + "\n\n" +
		"dist/\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(stale), 0o644))

	update, err := EnsureLocalStateGitIgnore(root)
	require.NoError(t, err)
	assert.True(t, update.Changed)

	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "build/\n")
	assert.Contains(t, string(content), "dist/\n")
	assert.NotContains(t, string(content), "/old-slipway-local-state/")
	assert.Contains(t, string(content), "/artifacts/changes/**/verification/")
}

func TestEnsureLocalStateGitIgnoreRefusesOrphanedStartMarker(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	orphaned := "build/\n" + localStateGitIgnoreStart + "\n/half-written/\n"
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte(orphaned), 0o644))

	update, err := EnsureLocalStateGitIgnore(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), localStateGitIgnoreStart)
	assert.Contains(t, err.Error(), localStateGitIgnoreEnd)
	assert.False(t, update.Changed)

	content, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err)
	assert.Equal(t, orphaned, string(content))
}

func TestLocalStateGitIgnoreRulesHideProofDirsButNotGovernedRecords(t *testing.T) {
	t.Parallel()
	root := createRuntimeRepoLayout(t)
	_, err := EnsureLocalStateGitIgnore(root)
	require.NoError(t, err)

	ignored := []string{
		"artifacts/codebase/ARCHITECTURE.md",
		"artifacts/changes/demo/evidence/governance/review.yaml",
		"artifacts/changes/demo/events/lifecycle.jsonl",
		"artifacts/changes/demo/verification/final-closeout.yaml",
		"artifacts/changes/archived/demo/evidence/tasks/t-01.json",
		"artifacts/changes/archived/demo/events/lifecycle.jsonl",
		"artifacts/changes/archived/demo/verification/final-closeout.yaml",
		".worktrees/demo/change.yaml",
	}
	for _, rel := range ignored {
		assertGitCheckIgnore(t, root, rel, true)
	}

	trackable := []string{
		"artifacts/changes/demo/change.yaml",
		"artifacts/changes/demo/intent.md",
		"artifacts/changes/demo/research.md",
		"artifacts/changes/demo/requirements.md",
		"artifacts/changes/demo/decision.md",
		"artifacts/changes/demo/tasks.md",
		"artifacts/changes/demo/assurance.md",
		"artifacts/changes/archived/demo/change.yaml",
	}
	for _, rel := range trackable {
		assertGitCheckIgnore(t, root, rel, false)
	}
}

func assertGitCheckIgnore(t *testing.T, root, rel string, wantIgnored bool) {
	t.Helper()
	cmd := exec.Command("git", "check-ignore", "--quiet", "--", rel)
	cmd.Dir = root
	err := cmd.Run()
	if wantIgnored {
		require.NoErrorf(t, err, "expected %s to be ignored", rel)
		return
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return
	}
	require.NoErrorf(t, err, "expected %s to be trackable", rel)
}
