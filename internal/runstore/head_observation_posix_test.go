//go:build !windows

package runstore

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestObserveGitRefusesToCallAnUnresolvedHeadUnborn drives the one case a real
// repository cannot reproduce: HEAD is readable in principle, but the Git
// invocation that reads it fails. Recording that as an unborn HEAD would let a
// run-start snapshot claim a repository state it never observed.
func TestObserveGitRefusesToCallAnUnresolvedHeadUnborn(t *testing.T) {
	repository := createRepository(t)
	observed, err := ObserveGit(repository)
	require.NoError(t, err)
	require.NotEqual(t, unbornHead, observed.Head)

	installFailingHeadLookupGit(t)
	failed, err := ObserveGit(repository)
	require.Error(t, err)
	assert.ErrorContains(t, err, "observe HEAD")
	assert.False(t, isUnbornHeadError(err))
	assert.Empty(t, failed.Head)

	var commandErr *GitCommandError
	require.ErrorAs(t, err, &commandErr)
	assert.Equal(t, 128, commandErr.ExitCode)
}

// installFailingHeadLookupGit puts a `git` wrapper ahead of the real binary on
// PATH. It fails only the HEAD resolution so workspace discovery, the index
// read, and the status read all still succeed and the observation reaches the
// HEAD branch under test.
func installFailingHeadLookupGit(t *testing.T) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	require.NoError(t, err)
	directory := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
for argument do
  if [ "$argument" = "--verify" ]; then
    printf 'fatal: unable to read HEAD\n' >&2
    exit 128
  fi
done
exec %q "$@"
`, realGit)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "git"), []byte(script), 0o700)) //nolint:gosec // the shim must be executable
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
}
