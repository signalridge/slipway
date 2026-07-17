package cmd

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The published surface is the seven user commands plus the machine protocol.
// The protocol group is deliberately among them: its operations are a published
// contract that a generated adapter must be able to discover, so the CLI must
// not present them as an implementation detail. Only `_help` stays hidden.
func TestRootExposesTheUserCommandsAndTheMachineProtocol(t *testing.T) {
	t.Parallel()
	root := newRootCmd()
	root.InitDefaultHelpCmd()
	var names []string
	for _, command := range root.Commands() {
		if command.IsAvailableCommand() {
			names = append(names, command.Name())
		}
	}
	sort.Strings(names)
	assert.Equal(t, []string{"doctor", "install", "list", "protocol", "run", "status", "stop", "uninstall"}, names)

	stdout, stderr, err := executeForTest(t, "--help")
	require.NoError(t, err, stderr)
	for _, name := range names {
		assert.Contains(t, stdout, "  "+name)
	}
	assert.NotContains(t, stdout, "\n  help        ")
	for _, retired := range []string{"check", "validate", "done", "next", "new", "intake", "plan", "implement", "review", "fix", "evidence", "preset", "repair", "handoff", "abort", "cancel", "delete"} {
		assert.NotContains(t, stdout, "  "+retired+" ")
	}
}

// The protocol operations are public, but they belong to `slipway protocol`
// alone. `run` starts a Run and must never regrow them as subcommands.
func TestRunDoesNotRegrowTheProtocolOperations(t *testing.T) {
	t.Parallel()
	stdout, stderr, err := executeForTest(t, "run", "--help")
	require.NoError(t, err, stderr)
	for _, operation := range []string{"submit", "answer", "skip", "resume"} {
		assert.NotContains(t, stdout, "  "+operation)
	}
	assert.Contains(t, stdout, "--no-review")
	assert.Contains(t, stdout, "--budget")
}

func TestRetiredCommandReturnsStructuredActionableError(t *testing.T) {
	t.Parallel()
	stdout, stderr, err := executeForTest(t, "check")
	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `"code":"invalid_usage"`)
	assert.NotContains(t, stderr, `"next_command"`)
	assert.Contains(t, stderr, `"operation":"none"`)
}

func TestRawRootArgumentStopsAtSeparator(t *testing.T) {
	t.Parallel()
	explicit, found := rawRootArgument([]string{"run", "goal", "--root=/repo/b", "--", "--root", "/repo/a"})
	require.True(t, found)
	assert.Equal(t, "/repo/b", explicit)

	explicit, found = rawRootArgument([]string{"run", "--", "--root=/repo/b"})
	assert.False(t, found)
	assert.Empty(t, explicit)
}

func TestRootForEarlyErrorDoesNotRequireRepositoryDiscovery(t *testing.T) {
	t.Parallel()
	explicit := filepath.Join(t.TempDir(), "not-a-repository")
	assert.Equal(t, explicit, rootForEarlyError([]string{"status", "--root", explicit}))
}

func executeForTest(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := newRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(args)
	err := executeRootCommand(root, args...)
	return stdout.String(), stderr.String(), err
}
