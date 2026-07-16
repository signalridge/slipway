package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/adapter"
	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdapterMutationErrorPreservesReportAndDisablesBlindRetry(t *testing.T) {
	t.Parallel()
	report := adapter.ChangeReport{
		Hosts:              []string{"claude"},
		TransactionOutcome: adapter.TransactionOutcomeAmbiguous,
		RecoveryArtifacts:  []string{".claude/slipway/recovery-file"},
		Warnings:           []string{"rollback preserved a concurrent edit"},
	}

	cliErr := adapterMutationError("install_failed", errors.New("transaction failed"), "/workspace", report)
	assert.Equal(t, autopilot.NextOperationCommand, cliErr.Next.Operation)
	require.Len(t, cliErr.Next.Variants, 1)
	assert.Equal(t, "inspect-host-adapters", cliErr.Next.Variants[0].ID)
	assert.Equal(t, []string{"slipway", "list", "--root", "/workspace"}, cliErr.Next.Variants[0].BaseArgv)
	require.NotNil(t, cliErr.Details)
	encodedReport, ok := cliErr.Details["report"].(changeReportOutput)
	require.True(t, ok)
	assert.Equal(t, makeChangeReportOutput(report), encodedReport)
	assert.Equal(t, string(adapter.TransactionOutcomeAmbiguous), cliErr.Details["transaction_outcome"])
}

func TestInstallAndUninstallUnknownHostAreUsageErrorsWithoutMutation(t *testing.T) {
	tests := []struct {
		name    string
		command string
		json    bool
	}{
		{name: "install human", command: "install"},
		{name: "install json", command: "install", json: true},
		{name: "uninstall human", command: "uninstall"},
		{name: "uninstall json", command: "uninstall", json: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newCLIRepository(t)
			canonicalRepository, resolveErr := resolveRoot(repository)
			require.NoError(t, resolveErr)
			before := snapshotCLIWorkspace(t, repository)
			args := []string{test.command, "--root", repository, "--tool", "missing-host"}
			if test.json {
				args = append(args, "--json")
			}

			stdout, stderr, err := executeForTest(t, args...)
			require.Error(t, err)
			assert.Empty(t, stdout)
			var cliErr CLIError
			require.NoError(t, json.Unmarshal([]byte(stderr), &cliErr))
			assert.Equal(t, "unknown_host_adapter", cliErr.Code)
			assert.Equal(t, exitCodeUsage, cliErr.ExitCode)
			assert.Nil(t, cliErr.Details, "unknown selection must not include a transaction report")
			assert.Equal(t, autopilot.NextOperationCommand, cliErr.Next.Operation)
			require.Len(t, cliErr.Next.Variants, 1)
			assert.Equal(t, []string{"slipway", "list", "--root", canonicalRepository}, cliErr.Next.Variants[0].BaseArgv)
			require.NoError(t, cliErr.Next.Validate())
			assert.Equal(t, before, snapshotCLIWorkspace(t, repository))
			_, statErr := os.Stat(filepath.Join(repository, ".git", "slipway"))
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

func TestInstallAndUninstallRejectExplicitEmptyToolSelection(t *testing.T) {
	selections := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "comma only", value: ","},
	}
	for _, command := range []string{"install", "uninstall"} {
		for _, selection := range selections {
			t.Run(command+"/"+selection.name, func(t *testing.T) {
				repository := newCLIRepository(t)
				before := snapshotCLIWorkspace(t, repository)
				stdout, stderr, err := executeForTest(t, command, "--root", repository, "--tool", selection.value)
				require.Error(t, err)
				assert.Empty(t, stdout)
				var cliErr CLIError
				require.NoError(t, json.Unmarshal([]byte(stderr), &cliErr))
				assert.Equal(t, "host_adapter_required", cliErr.Code)
				assert.Equal(t, exitCodeUsage, cliErr.ExitCode)
				assert.Equal(t, before, snapshotCLIWorkspace(t, repository))
			})
		}
	}
}

func TestInstallSurfaceValidationIsAUsageError(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "non kiro", args: []string{"--tool", "claude", "--surface", "ide"}},
		{name: "kiro missing", args: []string{"--tool", "kiro"}},
		{name: "kiro invalid", args: []string{"--tool", "kiro", "--surface", "editor"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := newCLIRepository(t)
			args := append([]string{"install", "--root", repository}, test.args...)
			stdout, stderr, err := executeForTest(t, args...)
			require.Error(t, err)
			assert.Empty(t, stdout)
			var cliErr CLIError
			require.NoError(t, json.Unmarshal([]byte(stderr), &cliErr))
			assert.Equal(t, "invalid_adapter_surface", cliErr.Code)
			assert.Equal(t, exitCodeUsage, cliErr.ExitCode)
			assert.Equal(t, autopilot.NextOperationCommand, cliErr.Next.Operation)
			require.Len(t, cliErr.Next.Variants, 1)
			assert.Equal(t, "install-kiro", cliErr.Next.Variants[0].ID)
			require.Len(t, cliErr.Next.Variants[0].Inputs, 1)
			assert.Equal(t, []string{"ide", "cli"}, cliErr.Next.Variants[0].Inputs[0].Choices)
		})
	}
}

func snapshotCLIWorkspace(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == filepath.Join(root, ".git") {
			return filepath.SkipDir
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			snapshot[filepath.ToSlash(relative)] = "directory"
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	require.NoError(t, err)
	return snapshot
}
