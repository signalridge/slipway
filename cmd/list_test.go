package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListReportsMissingManagedCapabilityAsNeedingRefresh(t *testing.T) {
	repository := newCLIRepository(t)
	_, stderr, err := executeForTest(t, "install", "--root", repository, "--tool", "claude", "--json")
	require.NoError(t, err, stderr)
	require.NoError(t, os.Remove(filepath.Join(repository, ".claude", "skills", "slipway-run", "SKILL.md")))

	stdout, stderr, err := executeForTest(t, "list", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	var output listOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &output))
	assert.Equal(t, machineContractVersion, output.ContractVersion)
	require.NotEmpty(t, output.Hosts)
	assert.True(t, output.Hosts[0].Installed)
	assert.True(t, output.Hosts[0].NeedsRefresh)
	assert.NotContains(t, output.Hosts[0].Capabilities, "slipway-run")

	stdout, stderr, err = executeForTest(t, "list", "--root", repository)
	require.NoError(t, err, stderr)
	assert.Contains(t, stdout, "needs_refresh=true")
}
