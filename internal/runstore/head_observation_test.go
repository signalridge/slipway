package runstore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObserveGitRecordsUnbornHeadOnlyForARepositoryWithoutACommit(t *testing.T) {
	t.Parallel()

	unborn := t.TempDir()
	runGitCommand(t, unborn, "init", "-q")
	observation, err := ObserveGit(unborn)
	require.NoError(t, err)
	assert.Equal(t, unbornHead, observation.Head)

	committed := createRepository(t)
	observation, err = ObserveGit(committed)
	require.NoError(t, err)
	assert.NotEqual(t, unbornHead, observation.Head)
	assert.Regexp(t, `^[0-9a-f]{40,64}$`, observation.Head)
}

func TestUnbornHeadClassificationRejectsEveryOtherGitFailure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		err    error
		unborn bool
	}{
		{name: "unborn head", err: &GitCommandError{Args: []string{"rev-parse"}, ExitCode: 1}, unborn: true},
		{name: "damaged repository", err: &GitCommandError{Args: []string{"rev-parse"}, ExitCode: 128, Stderr: "fatal: not a git repository"}, unborn: false},
		{name: "diagnostic without fatal exit", err: &GitCommandError{Args: []string{"rev-parse"}, ExitCode: 1, Stderr: "fatal: bad object"}, unborn: false},
		{name: "wrapped unborn head", err: fmt.Errorf("observe HEAD: %w", &GitCommandError{Args: []string{"rev-parse"}, ExitCode: 1}), unborn: true},
		{name: "start failure", err: errors.New("start git rev-parse: executable file not found"), unborn: false},
		{name: "observation timeout", err: &GitObservationLimitError{Stream: "stdout", Limit: 1}, unborn: false},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.unborn, isUnbornHeadError(test.err))
		})
	}
}

func TestGitCommandErrorReportsExitCodeWhenGitPrintsNoDiagnostic(t *testing.T) {
	t.Parallel()
	err := &GitCommandError{Args: []string{"rev-parse", "--verify", "--quiet", "HEAD"}, ExitCode: 1}
	assert.Equal(t, "git rev-parse --verify --quiet HEAD: exit status 1", err.Error())

	err = &GitCommandError{Args: []string{"status"}, ExitCode: 128, Stderr: "fatal: unreadable index"}
	assert.Equal(t, "git status: fatal: unreadable index", err.Error())
}

func TestGitBytesPreservesTheExitCodeOfAFailedGitCommand(t *testing.T) {
	t.Parallel()
	repository := createRepository(t)
	_, err := gitBytes(repository, "rev-parse", "--verify", "--quiet", "definitely-not-a-revision")
	require.Error(t, err)
	var commandErr *GitCommandError
	require.ErrorAs(t, err, &commandErr)
	assert.Equal(t, 1, commandErr.ExitCode)

	_, err = gitBytes(repository, "cat-file", "-p", "0000000000000000000000000000000000000000")
	require.Error(t, err)
	require.ErrorAs(t, err, &commandErr)
	assert.NotEqual(t, 1, commandErr.ExitCode)
	assert.NotEmpty(t, commandErr.Stderr)
	assert.False(t, isUnbornHeadError(err))
}
