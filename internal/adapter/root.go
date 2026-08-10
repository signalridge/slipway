package adapter

import (
	"context"
	"os/exec"

	"github.com/signalridge/slipway/internal/fsutil"
)

// ResolveRepositoryRoot discovers the worktree used for project-local host
// capabilities without exposing filesystem discovery to the CLI layer.
func ResolveRepositoryRoot(start string) (string, error) {
	repository, err := fsutil.DiscoverGit(start)
	if err != nil {
		return "", err
	}
	return repository.WorktreeRoot, nil
}

// RepositoryGitCommandContext constructs a repository-safe Git subprocess for
// command-layer repository inspection.
func RepositoryGitCommandContext(ctx context.Context, executable string, args ...string) *exec.Cmd {
	return fsutil.GitCommandContext(ctx, executable, args...)
}
