package state

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPlanWorktreeTargetRefusesUnmanagedWorktreeEvenWithForce proves the
// execution-surface gate: a worktree at the default path but on a hand-named
// branch (NOT feat/<slug>) was not provisioned by Slipway, so --worktree must be
// refused even with --force (issue #285).
func TestPlanWorktreeTargetRefusesUnmanagedWorktreeEvenWithForce(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)
	const slug = "fix-283"
	// External worktree at the DEFAULT path but on a hand-named branch: not provisioned by Slipway.
	path := filepath.Join(root, ".worktrees", slug)
	runGit(t, root, "worktree", "add", "-b", "hand/local-work", path)

	target := planWorktreeTarget(root, slug, DeleteOptions{RemoveWorktree: true, Force: true})
	assert.Equal(t, DeleteTargetWorktree, target.Kind)
	assert.Equal(t, DeleteActionRefused, target.Action)
	assert.Contains(t, target.Reason, "did not provision")
}

// TestPlanWorktreeTargetAllowsManagedWorktree confirms a worktree Slipway DID
// provision (default path + feat/<slug> branch) stays removable.
func TestPlanWorktreeTargetAllowsManagedWorktree(t *testing.T) {
	root := t.TempDir()
	initGitRepoAt(t, root)
	const slug = "demo"
	path := filepath.Join(root, ".worktrees", slug)
	runGit(t, root, "worktree", "add", "-b", DefaultWorktreeBranch(slug), path)

	target := planWorktreeTarget(root, slug, DeleteOptions{RemoveWorktree: true})
	assert.Equal(t, DeleteActionDelete, target.Action, "a Slipway-provisioned worktree (default path + feat/<slug>) must be removable")
}

func TestPlanWorktreeTargetRefusesResolverErrorWhenWorktreeRequested(t *testing.T) {
	t.Parallel()

	target := planWorktreeTargetWithResolver(
		t.TempDir(),
		"broken-worktree",
		DeleteOptions{RemoveWorktree: true},
		func(string, string) (string, error) {
			return "", errors.New("git worktree list failed")
		},
	)

	assert.Equal(t, DeleteTargetWorktree, target.Kind)
	assert.Equal(t, DeleteActionRefused, target.Action)
	assert.Contains(t, target.Reason, "could not determine bound worktree")
	assert.Contains(t, target.Reason, "git worktree list failed")
}

func TestPlanWorktreeTargetSkipsResolverErrorWhenWorktreeNotRequested(t *testing.T) {
	t.Parallel()

	target := planWorktreeTargetWithResolver(
		t.TempDir(),
		"broken-worktree",
		DeleteOptions{},
		func(string, string) (string, error) {
			return "", errors.New("git worktree list failed")
		},
	)

	assert.Equal(t, DeleteTargetWorktree, target.Kind)
	assert.Equal(t, DeleteActionSkip, target.Action)
	assert.Contains(t, target.Reason, "could not determine bound worktree")
}
