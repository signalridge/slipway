package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/signalridge/slipway/internal/fsutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var legacyResidueCodesForTest = map[string]string{
	"runtime":        "legacy_runtime_residue",
	"cache":          "legacy_cache_residue",
	"scope-root":     "legacy_scope_root_residue",
	"scopes":         "legacy_scopes_residue",
	"locks":          "legacy_locks_residue",
	"processes":      "legacy_processes_residue",
	"repair-backups": "legacy_repair_backups_residue",
}

type legacyResidueFixture struct {
	name            string
	path            string
	payload         []byte
	mode            os.FileMode
	quarantinedMode os.FileMode
	modTime         time.Time
}

func TestLegacyRunstoreResidueIsAdvisoryAndUntouchedByCommands(t *testing.T) {
	repository := newCLIRepository(t)
	gitRepository, err := fsutil.DiscoverGit(repository)
	require.NoError(t, err)
	namespace := filepath.Join(gitRepository.CommonDir, "slipway")
	require.NoError(t, os.MkdirAll(namespace, 0o700))

	names := make([]string, 0, len(legacyResidueCodesForTest)+1)
	for name := range legacyResidueCodesForTest {
		names = append(names, name)
	}
	names = append(names, "future-format")
	sort.Strings(names)
	fixtures := make([]legacyResidueFixture, 0, len(names))
	for _, name := range names {
		path := filepath.Join(namespace, name)
		require.NoError(t, os.Mkdir(path, 0o700))
		payload := []byte("opaque legacy payload for " + name + "\n")
		require.NoError(t, os.WriteFile(filepath.Join(path, "private.bin"), payload, 0o600))
		originalInfo, err := os.Lstat(path)
		require.NoError(t, err)
		require.NoError(t, os.Chmod(path, 0))
		quarantinedInfo, err := os.Lstat(path)
		require.NoError(t, err)
		fixtures = append(fixtures, legacyResidueFixture{
			name:            name,
			path:            path,
			payload:         payload,
			mode:            originalInfo.Mode(),
			quarantinedMode: quarantinedInfo.Mode(),
			modTime:         quarantinedInfo.ModTime(),
		})
	}
	t.Cleanup(func() {
		for _, fixture := range fixtures {
			_ = os.Chmod(fixture.path, 0o700)
		}
	})

	installJSON, stderr, err := executeForTest(t, "install", "--root", repository, "--tool", "claude", "--json")
	require.NoError(t, err, stderr)
	assert.Equal(t, machineContractVersion, decodeChangeReportOutput(t, installJSON).ContractVersion)
	refreshJSON, stderr, err := executeForTest(t, "install", "--root", repository, "--tool", "claude", "--refresh", "--json")
	require.NoError(t, err, stderr)
	assert.Equal(t, machineContractVersion, decodeChangeReportOutput(t, refreshJSON).ContractVersion)

	listJSON, stderr, err := executeForTest(t, "list", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var listed listOutput
	require.NoError(t, json.Unmarshal([]byte(listJSON), &listed))
	assert.Equal(t, machineContractVersion, listed.ContractVersion)
	require.Len(t, listed.Hosts, 10)

	runJSON, stderr, err := executeForTest(t, "run", "inspect legacy isolation", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	action := decodeMutationAction(t, runJSON)
	require.NotEmpty(t, action.RunID)
	statusListJSON, stderr, err := executeForTest(t, "status", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var statusList statusListOutput
	require.NoError(t, json.Unmarshal([]byte(statusListJSON), &statusList))
	require.Len(t, statusList.Runs, 1)
	statusJSON, stderr, err := executeForTest(t, "status", action.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var status runStatusOutput
	require.NoError(t, json.Unmarshal([]byte(statusJSON), &status))
	assert.Equal(t, action.RunID, status.ID)

	doctorJSON := executeDoctorWithRunner(t, repository, &fakeDoctorCommandRunner{
		pathErr: errors.New("not found"), bounded: true,
	})
	var doctor doctorOutput
	require.NoError(t, json.Unmarshal([]byte(doctorJSON), &doctor))
	legacyChecks := map[string][]string{}
	for _, check := range doctor.Checks {
		if check.Name == "legacy_runstore" {
			legacyChecks[check.Code] = append(legacyChecks[check.Code], check.Detail)
			assert.Equal(t, "warning", check.Status)
		}
	}
	for name, code := range legacyResidueCodesForTest {
		require.Len(t, legacyChecks[code], 1, name)
		assert.Contains(t, legacyChecks[code][0], `"`+name+`"`)
		assert.Contains(t, legacyChecks[code][0], "manually clean")
	}
	require.Len(t, legacyChecks["legacy_unknown_residue"], 1)
	assert.Contains(t, legacyChecks["legacy_unknown_residue"][0], `"future-format"`)
	for _, check := range doctor.Checks {
		if check.Name == "legacy_runstore" {
			assert.NotContains(t, check.Detail, `"runs"`)
		}
	}

	uninstallJSON, stderr, err := executeForTest(t, "uninstall", "--root", repository, "--tool", "claude", "--json")
	require.NoError(t, err, stderr)
	assert.Equal(t, machineContractVersion, decodeChangeReportOutput(t, uninstallJSON).ContractVersion)

	for _, fixture := range fixtures {
		info, err := os.Lstat(fixture.path)
		require.NoError(t, err, fixture.name)
		assert.Equal(t, fixture.quarantinedMode, info.Mode(), fixture.name)
		assert.Equal(t, fixture.modTime, info.ModTime(), fixture.name)
		require.NoError(t, os.Chmod(fixture.path, fixture.mode.Perm()))
		payload, err := os.ReadFile(filepath.Join(fixture.path, "private.bin"))
		require.NoError(t, err, fixture.name)
		assert.Equal(t, fixture.payload, payload, fixture.name)
	}

	if runtime.GOOS != "windows" {
		for _, fixture := range fixtures {
			require.NoError(t, os.Chmod(fixture.path, 0))
			_, err := os.ReadDir(fixture.path)
			assert.Error(t, err, fixture.name)
		}
	}
}

func executeDoctorWithRunner(t *testing.T, repository string, runner doctorCommandRunner) string {
	t.Helper()
	command := makeDoctorCmdWithRunner(runner)
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"--root", repository, "--json"})
	require.NoError(t, command.Execute(), stderr.String())
	return stdout.String()
}

func decodeChangeReportOutput(t *testing.T, raw string) changeReportOutput {
	t.Helper()
	var output changeReportOutput
	require.NoError(t, json.Unmarshal([]byte(raw), &output))
	return output
}
