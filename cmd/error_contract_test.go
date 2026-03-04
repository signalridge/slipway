package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/speclane/internal/bootstrap"
	"github.com/signalridge/speclane/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteFailureEnvelopeNonSplnIntent(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		_, stderr, err := runRootCommand([]string{"new", "how do i set this up?"})
		require.Error(t, err)

		var cliErr *CLIError
		require.True(t, errors.As(err, &cliErr))
		assert.Equal(t, exitCodePrecondition, cliErr.ExitCode)
		assert.Equal(t, "non_spln_intent", cliErr.ErrorCode)

		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, "non_spln_intent", payload.ErrorCode)
		assert.Equal(t, categoryPrecondition, payload.Category)
		assert.Equal(t, exitCodePrecondition, payload.ExitCode)
	})
}

func TestExecuteFailureEnvelopeInvalidUsage(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))

		_, stderr, err := runRootCommand([]string{"review", "--artifact", "design.md"})
		require.Error(t, err)
		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryInvalidUsage, payload.Category)
		assert.Equal(t, exitCodeInvalidUsage, payload.ExitCode)
	})
}

func TestExecuteFailureEnvelopeStateIntegrityForCorruptConfig(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		require.NoError(t, os.WriteFile(filepath.Join(root, ".spln", "config.yaml"), []byte("defaults: ["), 0o644))

		_, stderr, err := runRootCommand([]string{"new", "fix login timeout"})
		require.Error(t, err)
		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryStateIntegrity, payload.Category)
		assert.Equal(t, exitCodeStateIntegrity, payload.ExitCode)
		assert.Contains(t, payload.Remediation, "spln repair")
	})
}

func TestExecuteFailureEnvelopeGovernanceBlocked(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		_, _, err := runRootCommand([]string{"new", "--level", "L1", "fix login timeout"})
		require.NoError(t, err)

		_, stderr, err := runRootCommand([]string{"done"})
		require.Error(t, err)
		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryGovernanceBlocked, payload.Category)
		assert.Equal(t, exitCodeGovernanceBlocked, payload.ExitCode)
	})
}

func TestExecuteFailureEnvelopeStateLockTimeout(t *testing.T) {
	root := t.TempDir()
	withWorkspace(t, root, func() {
		require.NoError(t, bootstrap.InitWorkspace(root, nil, false))
		_, _, err := runRootCommand([]string{"new", "--level", "L1", "fix login timeout"})
		require.NoError(t, err)

		cfg := model.DefaultConfig()
		cfg.Execution.LockWaitTimeoutSeconds = 1
		cfg.Execution.LockStaleAfterSeconds = 1
		require.NoError(t, model.SaveConfig(filepath.Join(root, ".spln", "config.yaml"), cfg))

		stopLockHolder := startStateLockHolder(t, root)
		defer stopLockHolder()

		_, stderr, err := runRootCommand([]string{"do"})
		require.Error(t, err)
		var payload CLIError
		require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
		assert.Equal(t, categoryPrecondition, payload.Category)
		assert.Equal(t, exitCodePrecondition, payload.ExitCode)
		assert.Equal(t, "state_lock_timeout", payload.ErrorCode)
	})
}

func runRootCommand(args []string) (string, string, error) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	rootCmd.SetOut(&out)
	rootCmd.SetErr(&errOut)
	rootCmd.SetArgs(args)
	err := Execute()
	rootCmd.SetArgs(nil)

	return out.String(), errOut.String(), err
}
