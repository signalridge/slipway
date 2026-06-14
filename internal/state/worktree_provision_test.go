package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/signalridge/slipway/internal/toolgen"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests exercise the wiring of the injected provisioner through
// EnsureDefaultWorktreeForChange. The provisioning logic itself is unit-tested in
// internal/toolgen; here we assert the create and reuse branches invoke it and
// fail closed on its error. The real toolgen provisioner is injected (a test
// file may import the surface renderer; production state code may not).

// writeUnder writes data to root/rel, creating parent directories.
func writeUnder(t *testing.T, root, rel, data string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(data), 0o644))
}

const generatedSlipwaySkillDir = ".claude/skills/slipway-wave-orchestration"

func TestEnsureDefaultWorktreeForChange_ProvisionsHostSurfacesOnCreate(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)
	writeUnder(t, root, ".claude/skills/golang-foo/SKILL.md", "third party")
	writeUnder(t, root, ".codex/skills/golang-foo/SKILL.md", "third party codex")

	change := model.NewChange("provision-me")
	binding, err := EnsureDefaultWorktreeForChange(root, &change, toolgen.ProvisionWorktreeHostSurfaces)
	require.NoError(t, err)
	require.True(t, binding.Created)

	assert.FileExists(t, filepath.Join(binding.Path, ".claude/skills/golang-foo/SKILL.md"))
	assert.DirExists(t, filepath.Join(binding.Path, generatedSlipwaySkillDir))
	assert.FileExists(t, filepath.Join(binding.Path, ".claude/settings.json"))
	assert.FileExists(t, filepath.Join(binding.Path, ".claude/hooks/slipway-session-start.sh"))
	assert.FileExists(t, filepath.Join(binding.Path, ".codex/skills/golang-foo/SKILL.md"))
}

func TestEnsureDefaultWorktreeForChange_CreateFailsClosedOnProvisionError(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)

	// A stub provisioner isolates the create-branch fail-closed wiring: making the
	// real provisioner fail on a freshly created worktree is awkward, but the
	// contract under test is only that a provisioner error aborts the binding.
	boom := errors.New("provision boom")
	stub := func(_, _ string) error { return boom }

	change := model.NewChange("create-fail")
	_, err := EnsureDefaultWorktreeForChange(root, &change, stub)
	require.Error(t, err, "the create branch must fail closed when provisioning errors")
	assert.ErrorIs(t, err, boom, "the provisioner error must propagate to the binding caller")
	assert.Empty(t, change.WorktreePath,
		"a failed create-path provision must not persist a worktree binding (provision runs before persist)")
}

func TestEnsureDefaultWorktreeForChange_ReuseBackfillsPreservingLocalEdit(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)
	writeUnder(t, root, ".claude/skills/golang-foo/SKILL.md", "repo version")

	repoRoot, err := gitWorkspaceRoot(root)
	require.NoError(t, err)
	slug := "reuse-me"
	branch := DefaultWorktreeBranch(slug)
	wtPath, err := NormalizePath(DefaultWorktreePath(repoRoot, slug))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))
	runGit(t, repoRoot, "worktree", "add", "-b", branch, wtPath, "HEAD")

	// A worktree-local third-party edit must survive backfill; the rest of
	// .claude is missing and must be provisioned.
	writeUnder(t, wtPath, ".claude/skills/golang-foo/SKILL.md", "WORKTREE LOCAL EDIT")

	change := model.NewChange(slug)
	binding, err := EnsureDefaultWorktreeForChange(root, &change, toolgen.ProvisionWorktreeHostSurfaces)
	require.NoError(t, err)
	require.False(t, binding.Created, "an already-registered worktree takes the reuse branch")

	got, err := os.ReadFile(filepath.Join(wtPath, ".claude/skills/golang-foo/SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, "WORKTREE LOCAL EDIT", string(got), "reuse must preserve the worktree-local edit")
	assert.DirExists(t, filepath.Join(wtPath, generatedSlipwaySkillDir))
	assert.FileExists(t, filepath.Join(wtPath, ".claude/settings.json"))
}

func TestEnsureDefaultWorktreeForChange_ReuseFailsClosedOnProvisionError(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)
	writeUnder(t, root, ".claude/skills/golang-foo/SKILL.md", "repo version")

	repoRoot, err := gitWorkspaceRoot(root)
	require.NoError(t, err)
	slug := "reuse-fail"
	branch := DefaultWorktreeBranch(slug)
	wtPath, err := NormalizePath(DefaultWorktreePath(repoRoot, slug))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(wtPath), 0o755))
	runGit(t, repoRoot, "worktree", "add", "-b", branch, wtPath, "HEAD")

	// Destination .claude is a regular file → provisioning cannot create the
	// tool root and must fail closed from the binding call.
	require.NoError(t, os.WriteFile(filepath.Join(wtPath, ".claude"), []byte("not a dir"), 0o644))

	change := model.NewChange(slug)
	_, err = EnsureDefaultWorktreeForChange(root, &change, toolgen.ProvisionWorktreeHostSurfaces)
	require.Error(t, err, "binding must fail closed when worktree provisioning errors")
}
