package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusDoesNotCreateRunstoreNamespace(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "status", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var output statusListOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &output))
	assert.Empty(t, output.Runs)
	assert.Empty(t, output.UnavailableRuns)
	_, statErr := os.Stat(filepath.Join(repository, ".git", "slipway"))
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestStatusDoesNotCreateLockOrRepairJournalTail(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "run", "read-only status", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var started mutationEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &started))

	runDirectory := filepath.Join(repository, ".git", "slipway", "runs", started.RunID)
	lockPath := filepath.Join(runDirectory, "run.lock")
	journalPath := filepath.Join(runDirectory, "journal.jsonl")
	require.NoError(t, os.Remove(lockPath))
	file, err := os.OpenFile(journalPath, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = file.WriteString(`{"sequence":`)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	before, err := os.ReadFile(journalPath)
	require.NoError(t, err)

	stdout, stderr, err = executeForTest(t, "status", started.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var status runStatusOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	assert.Equal(t, started.RunID, status.ID)
	after, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(before, after), "status must not repair an interrupted tail")
	_, statErr := os.Stat(lockPath)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestStatusDistinguishesMissingAndCorruptRuns(t *testing.T) {
	repository := newCLIRepository(t)

	_, stderr, err := executeForTest(t, "status", "missing-run", "--root", repository, "--json")
	require.Error(t, err)
	var missing CLIError
	require.NoError(t, json.Unmarshal([]byte(stderr), &missing))
	assert.Equal(t, "run_not_found", missing.Code)

	_, stderr, err = executeForTest(t, "status", "../bad", "--root", repository, "--json")
	require.Error(t, err)
	var invalid CLIError
	require.NoError(t, json.Unmarshal([]byte(stderr), &invalid))
	assert.Equal(t, "invalid_run_id", invalid.Code)
	assert.Equal(t, exitCodeUsage, invalid.ExitCode)

	stdout, stderr, err := executeForTest(t, "run", "corrupt status", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var started mutationEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &started))
	journalPath := filepath.Join(repository, ".git", "slipway", "runs", started.RunID, "journal.jsonl")
	file, err := os.OpenFile(journalPath, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = file.WriteString("not-json\n")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	_, stderr, err = executeForTest(t, "status", started.RunID, "--root", repository, "--json")
	require.Error(t, err)
	var corrupt CLIError
	require.NoError(t, json.Unmarshal([]byte(stderr), &corrupt))
	assert.Equal(t, "run_journal_invalid", corrupt.Code)
	assert.Equal(t, autopilot.NextOperationCommand, corrupt.Next.Operation)

	stdout, stderr, err = executeForTest(t, "status", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var listed statusListOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &listed))
	require.Len(t, listed.UnavailableRuns, 1)
	assert.Equal(t, started.RunID, listed.UnavailableRuns[0].ID)
	assert.Equal(t, "run_journal_invalid", listed.UnavailableRuns[0].Code)

	stdout, stderr, err = executeForTest(t, "run", "still active", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var healthy mutationEnvelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &healthy))
	_, stderr, err = executeForTest(t, "stop", "--root", repository, "--json")
	require.Error(t, err)
	var selection CLIError
	require.NoError(t, json.Unmarshal([]byte(stderr), &selection))
	assert.Equal(t, "run_id_required", selection.Code)
	stdout, stderr, err = executeForTest(t, "status", healthy.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var healthyStatus runStatusOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &healthyStatus))
	assert.Equal(t, autopilot.RunActive, healthyStatus.State)
	assert.NotEmpty(t, listed.UnavailableRuns[0].Detail)
}
