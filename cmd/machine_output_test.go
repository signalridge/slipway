package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/signalridge/slipway/internal/autopilot"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicMachineSuccessEnvelopesHaveExactVersionedShapes(t *testing.T) {
	repository := newCLIRepository(t)

	installJSON, stderr, err := executeForTest(t, "install", "--root", repository, "--tool", "claude", "--json")
	require.NoError(t, err, stderr)
	assertMachineSchemaOutput(t, "changeReport", installJSON)
	install := exactJSONObject(t, installJSON, "contract_version", "hosts", "transaction_outcome", "written", "removed", "preserved", "recovery_artifacts", "warnings")
	assertContractVersion(t, install)
	assertJSONArray(t, install, "hosts")
	assertJSONString(t, install, "transaction_outcome")
	assertJSONArray(t, install, "written")
	assertJSONArray(t, install, "removed")
	assertJSONArray(t, install, "preserved")
	assertJSONArray(t, install, "recovery_artifacts")
	assertJSONArray(t, install, "warnings")

	listJSON, stderr, err := executeForTest(t, "list", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	assertMachineSchemaOutput(t, "listReport", listJSON)
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
	assertMachineSchemaOutput(t, "statusList", emptyStatusJSON)
	emptyStatus := exactJSONObject(t, emptyStatusJSON, "contract_version", "runs")
	assertContractVersion(t, emptyStatus)
	assert.Empty(t, rawJSONArray(t, emptyStatus, "runs"))

	doctorJSON := executeDoctorWithRunner(t, repository, &fakeDoctorCommandRunner{
		pathErr: errors.New("not found"), bounded: true,
	})
	assertMachineSchemaOutput(t, "doctorReport", doctorJSON)
	doctor := exactJSONObject(t, doctorJSON, "contract_version", "checks")
	assertContractVersion(t, doctor)
	checks := rawJSONArray(t, doctor, "checks")
	require.NotEmpty(t, checks)
	for _, check := range checks {
		var identity struct {
			Code string `json:"code"`
		}
		require.NoError(t, json.Unmarshal(check, &identity))
		keys := []string{"code", "status", "host_id", "name", "detail"}
		if identity.Code == "runstore_durability_full" || identity.Code == "runstore_durability_limited" {
			keys = append(keys, "durability")
		}
		exactRawJSONObject(t, check, keys...)
	}

	actionJSON, stderr, err := executeForTest(t, "run", "inspect output shapes", "--root", repository, "--json")
	require.NoError(t, err, stderr)
	assertMachineSchemaOutput(t, "protocolState", actionJSON)
	mutation := exactJSONObject(t, actionJSON, "contract_version", "run_id", "state", "action", "next")
	assertContractVersion(t, mutation)
	exactRawJSONObject(
		t,
		mutation["action"],
		"contract_version", "run_id", "action_id", "kind", "goal", "brief", "context", "remaining_budget",
	)
	decodedAction := decodeMutationAction(t, actionJSON)

	statusJSON, stderr, err := executeForTest(t, "status", decodedAction.RunID, "--root", repository, "--json")
	require.NoError(t, err, stderr)
	assertMachineSchemaOutput(t, "runStatus", statusJSON)
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
	assertMachineSchemaOutput(t, "protocolState", stopJSON)
	stop := exactJSONObject(t, stopJSON, "contract_version", "run_id", "state", "action", "next")
	assertContractVersion(t, stop)

	issueRepository := newCLIRepository(t)
	sourcePath := writeCLISource(t, cliSourceEnvelope())
	issueJSON, stderr, err := executeForTest(
		t, "run", "inspect material output", "--root", issueRepository, "--source-file", sourcePath, "--json",
	)
	require.NoError(t, err, stderr)
	assertMachineSchemaOutput(t, "protocolState", issueJSON)
	issueAction := decodeMutationAction(t, issueJSON)
	materialJSON, stderr, err := executeForTest(
		t, "_machine", "material", "--root", issueRepository,
		"--run", issueAction.RunID, "--action", issueAction.ActionID, "--section", "requirements",
	)
	require.NoError(t, err, stderr)
	assertMachineSchemaOutput(t, "actionMaterial", materialJSON)
	uninstallJSON, stderr, err := executeForTest(t, "uninstall", "--root", repository, "--tool", "claude", "--json")
	require.NoError(t, err, stderr)
	assertMachineSchemaOutput(t, "changeReport", uninstallJSON)
	uninstall := exactJSONObject(t, uninstallJSON, "contract_version", "hosts", "transaction_outcome", "written", "removed", "preserved", "recovery_artifacts", "warnings")
	assertContractVersion(t, uninstall)
	assertJSONString(t, uninstall, "transaction_outcome")
	assertJSONArray(t, uninstall, "written")
	assertJSONArray(t, uninstall, "recovery_artifacts")
	assertJSONArray(t, uninstall, "warnings")
}

func TestProtocolResultRejectsActiveStateWithoutAction(t *testing.T) {
	t.Parallel()
	err := writeProtocolResult(&cobra.Command{}, autopilot.Run{State: autopilot.RunActive})
	assert.EqualError(t, err, "active protocol result requires a current action")
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
	assertMachineSchemaOutput(t, "cliError", stderr)
}

func TestUnknownHostAdapterMachineErrorHasExactUsageShape(t *testing.T) {
	repository := newCLIRepository(t)
	canonicalRepository, resolveErr := resolveRoot(repository)
	require.NoError(t, resolveErr)
	stdout, stderr, err := executeForTest(t, "install", "--root", repository, "--tool", "missing-host", "--json")
	require.Error(t, err)
	assert.Empty(t, stdout)
	machineError := exactJSONObject(t, stderr, "contract_version", "code", "message", "next", "exit_code")
	assertContractVersion(t, machineError)
	assert.Equal(t, `"unknown_host_adapter"`, string(machineError["code"]))
	assert.Equal(t, "2", string(machineError["exit_code"]))
	next := exactRawJSONObject(t, machineError["next"], "operation", "workspace_identity", "variants")
	assert.Equal(t, `"command"`, string(next["operation"]))
	variants := rawJSONArray(t, next, "variants")
	require.Len(t, variants, 1)
	variant := exactRawJSONObject(t, variants[0], "id", "base_argv", "inputs")
	assert.Equal(t, `"list-host-adapters"`, string(variant["id"]))
	assert.JSONEq(t, `["slipway","list","--root",`+mustJSONQuote(t, canonicalRepository)+`]`, string(variant["base_argv"]))
	assertMachineSchemaOutput(t, "cliError", stderr)
}

func mustJSONQuote(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
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

func assertMachineSchemaOutput(t *testing.T, definition, raw string) {
	t.Helper()

	schemaRaw, err := os.ReadFile(filepath.Join("..", "docs", "reference", "machine-protocol.schema.json"))
	require.NoError(t, err)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaRaw))
	require.NoError(t, err)

	const schemaURL = "https://signalridge.github.io/slipway/reference/machine-protocol.schema.json"
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	require.NoError(t, compiler.AddResource(schemaURL, document))
	schema, err := compiler.Compile(schemaURL + "#/$defs/" + definition)
	require.NoError(t, err)
	value, err := jsonschema.UnmarshalJSON(bytes.NewBufferString(raw))
	require.NoError(t, err, "real machine emitter did not produce one JSON value: %s", raw)
	require.NoError(t, schema.Validate(value), "real machine emitter diverged from $defs/%s: %s", definition, raw)
}
