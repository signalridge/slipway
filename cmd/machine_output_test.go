package cmd

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicMachineSuccessEnvelopesHaveExactVersionedShapes(t *testing.T) {
	repository := newCLIRepository(t)

	installJSON, stderr, err := executeForTest(t, "install", "--root", repository, "--tool", "claude", "--json")
	require.NoError(t, err, stderr)
	install := exactJSONObject(t, installJSON, "contract_version", "hosts", "written", "removed", "preserved", "warnings")
	assertContractVersion(t, install)
	assertJSONArray(t, install, "hosts")
	assertJSONArray(t, install, "written")
	assertJSONArray(t, install, "removed")
	assertJSONArray(t, install, "preserved")
	assertJSONArray(t, install, "warnings")

	listJSON, stderr, err := executeForTest(t, "list", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	list := exactJSONObject(t, listJSON, "contract_version", "hosts")
	assertContractVersion(t, list)
	hosts := rawJSONArray(t, list, "hosts")
	require.NotEmpty(t, hosts)
	exactRawJSONObject(t, hosts[0], "id", "detected", "installed", "needs_refresh", "capabilities")
	emptyListJSON, err := json.Marshal(makeListOutput(nil))
	require.NoError(t, err)
	emptyList := exactJSONObject(t, string(emptyListJSON), "contract_version", "hosts")
	assertContractVersion(t, emptyList)
	assert.Empty(t, rawJSONArray(t, emptyList, "hosts"))

	emptyRepository := newCLIRepository(t)
	emptyStatusJSON, stderr, err := executeForTest(t, "status", "--root", emptyRepository, "--json")
	require.NoError(t, err, stderr)
	emptyStatus := exactJSONObject(t, emptyStatusJSON, "contract_version", "runs")
	assertContractVersion(t, emptyStatus)
	assert.Empty(t, rawJSONArray(t, emptyStatus, "runs"))

	doctorJSON := executeDoctorWithRunner(t, repository, &fakeDoctorCommandRunner{
		pathErr: errors.New("not found"), bounded: true,
	})
	doctor := exactJSONObject(t, doctorJSON, "contract_version", "checks")
	assertContractVersion(t, doctor)
	checks := rawJSONArray(t, doctor, "checks")
	require.NotEmpty(t, checks)
	for _, check := range checks {
		exactRawJSONObject(t, check, "code", "status", "host_id", "name", "detail")
	}

	actionJSON, stderr, err := executeForTest(t, "run", "inspect output shapes", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	action := exactJSONObject(
		t,
		actionJSON,
		"contract_version", "run_id", "action_id", "kind", "goal", "brief", "context", "remaining_budget",
	)
	assertContractVersion(t, action)
	var decodedAction autopilot.Action
	require.NoError(t, json.Unmarshal([]byte(actionJSON), &decodedAction))

	statusJSON, stderr, err := executeForTest(t, "status", decodedAction.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	status := exactJSONObject(
		t,
		statusJSON,
		"contract_version", "id", "goal", "workspace", "workspace_identity", "state",
		"review_enabled", "review_pending", "initial_budget", "remaining_budget", "initial_git", "current_git",
		"final_git_observed", "current_action", "actions", "created_at", "updated_at", "next",
	)
	assertContractVersion(t, status)

	stopJSON, stderr, err := executeForTest(t, "stop", decodedAction.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	stop := exactJSONObject(t, stopJSON, "contract_version", "run_id", "state", "next")
	assertContractVersion(t, stop)

	uninstallJSON, stderr, err := executeForTest(t, "uninstall", "--root", repository, "--tool", "claude", "--json")
	require.NoError(t, err, stderr)
	uninstall := exactJSONObject(t, uninstallJSON, "contract_version", "hosts", "written", "removed", "preserved", "warnings")
	assertContractVersion(t, uninstall)
	assertJSONArray(t, uninstall, "written")
	assertJSONArray(t, uninstall, "warnings")
}

func TestMachineErrorsHaveExactVersionedShape(t *testing.T) {
	repository := newCLIRepository(t)
	stdout, stderr, err := executeForTest(t, "status", "missing-run", "--root", repository, "--json")
	require.Error(t, err)
	assert.Empty(t, stdout)
	machineError := exactJSONObject(t, stderr, "contract_version", "code", "message", "next", "exit_code")
	assertContractVersion(t, machineError)
	assertJSONString(t, machineError, "code")
	assertJSONString(t, machineError, "message")
	exactRawJSONObject(t, machineError["next"], "operation", "workspace_identity", "variants")
}

func exactJSONObject(t *testing.T, raw string, keys ...string) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(raw), &object))
	assert.ElementsMatch(t, keys, mapKeys(object))
	return object
}

func exactRawJSONObject(t *testing.T, raw json.RawMessage, keys ...string) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &object))
	assert.ElementsMatch(t, keys, mapKeys(object))
	return object
}

func rawJSONArray(t *testing.T, object map[string]json.RawMessage, key string) []json.RawMessage {
	t.Helper()
	var values []json.RawMessage
	require.NoError(t, json.Unmarshal(object[key], &values))
	require.NotNil(t, values)
	return values
}

func assertJSONArray(t *testing.T, object map[string]json.RawMessage, key string) {
	t.Helper()
	_ = rawJSONArray(t, object, key)
}

func assertContractVersion(t *testing.T, object map[string]json.RawMessage) {
	t.Helper()
	var version int
	require.NoError(t, json.Unmarshal(object["contract_version"], &version))
	assert.Equal(t, machineContractVersion, version)
}

func assertJSONString(t *testing.T, object map[string]json.RawMessage, key string) {
	t.Helper()
	var value string
	require.NoError(t, json.Unmarshal(object[key], &value))
	assert.NotEmpty(t, value)
}
