package cmd

import (
	"bytes"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootExposesExactlySevenPublicCommands(t *testing.T) {
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
	assert.Equal(t, []string{"doctor", "install", "list", "run", "status", "stop", "uninstall"}, names)

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

func TestRunMachineCommandsRemainHiddenFromHelp(t *testing.T) {
	t.Parallel()
	stdout, stderr, err := executeForTest(t, "run", "--help")
	require.NoError(t, err, stderr)
	for _, hidden := range []string{"submit", "answer", "skip", "resume"} {
		assert.NotContains(t, stdout, "  "+hidden)
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

func executeForTest(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := newRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetIn(strings.NewReader(""))
	root.SetArgs(args)
	err := executeRootCommand(root)
	return stdout.String(), stderr.String(), err
}
