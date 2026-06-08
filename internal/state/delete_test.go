package state

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
