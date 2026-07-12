package autopilot

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachineProtocolSchemaDeclaresStrictDraft202012Unions(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "machine-protocol.schema.json"))
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(raw, &schema))
	assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	require.Len(t, schemaSlice(t, schema, "oneOf"), 9)

	definitions := schemaMap(t, schema, "$defs")
	for _, name := range []string{
		"action", "outcome", "protocolState", "cliError", "changeReport", "listReport", "doctorReport", "runStatus", "statusList",
	} {
		definition := schemaMap(t, definitions, name)
		assert.False(t, definition["additionalProperties"].(bool), name)
	}
	action := schemaMap(t, definitions, "action")
	outcome := schemaMap(t, definitions, "outcome")
	next := schemaMap(t, definitions, "next")
	assert.False(t, action["additionalProperties"].(bool))
	assert.False(t, outcome["additionalProperties"].(bool))
	assert.False(t, next["additionalProperties"].(bool))
	assert.ElementsMatch(t, []string{"operation", "workspace_identity", "variants"}, schemaStrings(t, next, "required"))
	nextProperties := schemaMap(t, next, "properties")
	assert.Equal(t, "^sha256:[0-9a-f]{64}$", schemaMap(t, nextProperties, "workspace_identity")["pattern"])
	assert.ElementsMatch(t, []string{
		"contract_version", "run_id", "action_id", "kind", "goal", "brief", "context", "remaining_budget",
	}, schemaStrings(t, action, "required"))
	assert.ElementsMatch(t, []string{
		"contract_version", "action_id", "status", "summary", "observations", "known_issues",
		"suggested_actions", "pause", "implementation", "review",
	}, schemaStrings(t, outcome, "required"))
	assert.Len(t, schemaSlice(t, action, "oneOf"), 6, "Action must distinguish ad-hoc/issue-bound and scoped/unscoped Implement")
	assert.Len(t, schemaSlice(t, outcome, "oneOf"), 9, "Outcome must distinguish every structurally unique matrix branch")
	assert.ElementsMatch(t,
		[]string{"contract_version", "hosts", "written", "removed", "preserved", "warnings"},
		schemaStrings(t, schemaMap(t, definitions, "changeReport"), "required"),
	)
	assert.ElementsMatch(t,
		[]string{"contract_version", "hosts"},
		schemaStrings(t, schemaMap(t, definitions, "listReport"), "required"),
	)
	assert.ElementsMatch(t,
		[]string{"contract_version", "checks"},
		schemaStrings(t, schemaMap(t, definitions, "doctorReport"), "required"),
	)
	assert.ElementsMatch(t,
		[]string{"code", "status", "host_id", "name", "detail"},
		schemaStrings(t, schemaMap(t, definitions, "doctorCheck"), "required"),
	)
	assert.ElementsMatch(t,
		[]string{"contract_version", "runs"},
		schemaStrings(t, schemaMap(t, definitions, "statusList"), "required"),
	)

	assertEveryObjectSchemaIsClosed(t, schema, "$")
}

func TestMachineProtocolSchemaFixturesMatchGoContract(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "machine-protocol.schema.json"))
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(raw, &schema))
	definitions := schemaMap(t, schema, "$defs")

	workspace, err := filepath.Abs(".")
	require.NoError(t, err)
	runFixture := Run{
		ID: "run-1", Workspace: workspace, State: RunActive,
		CurrentAction: &Action{ActionID: "action-1", Kind: ActionOrient},
	}
	runFixture.WorkspaceIdentity.ID = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	next, err := DeriveNext(runFixture)
	require.NoError(t, err)
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "next"), marshalTestJSON(t, next))

	assertSchemaObjectFixture(t, schemaMap(t, definitions, "changeReport"), marshalTestJSON(t, map[string]any{
		"contract_version": 1,
		"hosts":            []string{"claude"}, "written": []string{}, "removed": []string{}, "preserved": []string{}, "warnings": []string{},
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "listReport"), marshalTestJSON(t, map[string]any{
		"contract_version": 1,
		"hosts": []map[string]any{{
			"id": "claude", "detected": true, "installed": true, "needs_refresh": false, "capabilities": []string{"slipway-run"},
		}},
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "doctorReport"), marshalTestJSON(t, map[string]any{
		"contract_version": 1,
		"checks": []map[string]any{{
			"code": "adapter_healthy", "status": "ok", "host_id": "claude", "name": "adapter", "detail": "7 managed files",
		}},
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "protocolState"), marshalTestJSON(t, map[string]any{
		"contract_version": 1, "run_id": "run-1", "state": "paused", "pause_reason": "decision_required", "next": next,
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "cliError"), marshalTestJSON(t, map[string]any{
		"contract_version": 1, "code": "invalid_usage", "message": "invalid", "next": next, "exit_code": 2,
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "statusList"), marshalTestJSON(t, map[string]any{
		"contract_version": 1, "runs": []any{},
	}))
	var runObject map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(marshalTestJSON(t, runFixture), &runObject))
	runObject["next"] = marshalTestJSON(t, next)
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "runStatus"), marshalTestJSON(t, runObject))

	adHoc := testAction()
	require.NoError(t, adHoc.Validate())
	adHocJSON := marshalTestJSON(t, adHoc)
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "action"), adHocJSON)
	var adHocObject map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(adHocJSON, &adHocObject))
	assert.NotContains(t, adHocObject, "source")
	assert.NotContains(t, adHocObject, "requirements")

	source := testActionSource()
	requirements := testAcceptedRequirements()
	authorization := destructiveAuthorizationForTest(t)
	scoped := testAction()
	scoped.Kind = ActionImplement
	scoped.Source = &source
	scoped.Requirements = &requirements
	scoped.DestructiveAuthorization = &authorization
	require.NoError(t, scoped.Validate())
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "action"), marshalTestJSON(t, scoped))

	implementation := implementedTestOutcome(OutcomeCompleted, ImplementationApplied)
	implementationJSON := marshalTestJSON(t, implementation)
	decoded, err := DecodeOutcome(bytes.NewReader(implementationJSON))
	require.NoError(t, err)
	require.NoError(t, decoded.Validate(ActionImplement, "action-1"))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "outcome"), implementationJSON)

	review := reviewedTestOutcome(OutcomeCompleted, ReviewFindings, []Finding{{Location: "a.go:1", Summary: "bug", Detail: "detail"}})
	reviewJSON := marshalTestJSON(t, review)
	decoded, err = DecodeOutcome(bytes.NewReader(reviewJSON))
	require.NoError(t, err)
	require.NoError(t, decoded.Validate(ActionReview, "action-1"))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "outcome"), reviewJSON)
}

func assertEveryObjectSchemaIsClosed(t *testing.T, value any, path string) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		if typed["type"] == "object" {
			closed, exists := typed["additionalProperties"]
			require.True(t, exists, "object schema %s must declare additionalProperties", path)
			assert.Equal(t, false, closed, "object schema %s must reject unknown fields", path)
		}
		for key, child := range typed {
			assertEveryObjectSchemaIsClosed(t, child, path+"."+key)
		}
	case []any:
		for _, child := range typed {
			assertEveryObjectSchemaIsClosed(t, child, path+"[]")
		}
	}
}

func assertSchemaObjectFixture(t *testing.T, objectSchema map[string]any, raw []byte) {
	t.Helper()
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &object))
	properties := schemaMap(t, objectSchema, "properties")
	for key := range object {
		assert.Contains(t, properties, key, "fixture contains a field rejected by additionalProperties:false")
	}
	for _, required := range schemaStrings(t, objectSchema, "required") {
		assert.Contains(t, object, required, "fixture omits a required field")
	}
}

func schemaMap(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()
	value, exists := object[key]
	require.True(t, exists, "schema key %q is missing", key)
	result, ok := value.(map[string]any)
	require.True(t, ok, "schema key %q must be an object", key)
	return result
}

func schemaSlice(t *testing.T, object map[string]any, key string) []any {
	t.Helper()
	value, exists := object[key]
	require.True(t, exists, "schema key %q is missing", key)
	result, ok := value.([]any)
	require.True(t, ok, "schema key %q must be an array", key)
	return result
}

func schemaStrings(t *testing.T, object map[string]any, key string) []string {
	t.Helper()
	values := schemaSlice(t, object, key)
	result := make([]string, len(values))
	for index, value := range values {
		text, ok := value.(string)
		require.True(t, ok, "schema key %q item %d must be a string", key, index)
		result[index] = text
	}
	return result
}
