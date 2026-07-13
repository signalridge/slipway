package autopilot

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
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
	require.Len(t, schemaSlice(t, schema, "oneOf"), 11)

	definitions := schemaMap(t, schema, "$defs")
	for _, name := range []string{
		"action", "actionMaterial", "rawSourceEnvelope", "outcome", "protocolState", "cliError",
		"changeReport", "listReport", "doctorReport", "runStatus", "statusList",
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
		"contract_version", "action_id", "action_kind", "status", "summary", "observations", "known_issues",
		"suggested_actions", "pause", "implementation", "review",
	}, schemaStrings(t, outcome, "required"))
	assert.Len(t, schemaSlice(t, action, "oneOf"), 6, "Action must distinguish ad-hoc/issue-bound and scoped/unscoped Implement")
	assert.Len(t, schemaSlice(t, outcome, "oneOf"), 12, "Outcome must distinguish every action-specific matrix branch")
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
		"contract_version": ContractVersion,
		"hosts":            []string{"claude"}, "written": []string{}, "removed": []string{}, "preserved": []string{}, "warnings": []string{},
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "listReport"), marshalTestJSON(t, map[string]any{
		"contract_version": ContractVersion,
		"hosts": []map[string]any{{
			"id": "claude", "detected": true, "installed": true, "needs_refresh": false, "capabilities": []string{"slipway-run"},
		}},
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "doctorReport"), marshalTestJSON(t, map[string]any{
		"contract_version": ContractVersion,
		"checks": []map[string]any{{
			"code": "adapter_healthy", "status": "ok", "host_id": "claude", "name": "adapter", "detail": "7 managed files",
		}},
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "protocolState"), marshalTestJSON(t, map[string]any{
		"contract_version": ContractVersion, "run_id": "run-1", "state": "paused", "pause_reason": "decision_required", "next": next,
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "cliError"), marshalTestJSON(t, map[string]any{
		"contract_version": ContractVersion, "code": "invalid_usage", "message": "invalid", "next": next, "exit_code": 2,
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "statusList"), marshalTestJSON(t, map[string]any{
		"contract_version": ContractVersion, "runs": []any{},
	}))
	var runObject map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(marshalTestJSON(t, runFixture), &runObject))
	runObject["next"] = marshalTestJSON(t, next)
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "runStatus"), marshalTestJSON(t, runObject))

	adHoc := testAction()
	require.NoError(t, adHoc.Validate())
	adHocJSON := marshalTestJSON(t, adHoc)
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "action"), adHocJSON)
	actionSchema := compileMachineSchemaDefinition(t, "action")
	require.NoError(t, actionSchema.Validate(machineSchemaValue(t, adHoc)))
	var adHocObject map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(adHocJSON, &adHocObject))
	assert.NotContains(t, adHocObject, "source")
	assert.NotContains(t, adHocObject, "requirements")

	source := testActionSource()
	requirements := testActionRequirements()
	authorization := destructiveAuthorizationForTest(t)
	scoped := testAction()
	scoped.Kind = ActionImplement
	scoped.Source = &source
	scoped.Requirements = &requirements
	scoped.DestructiveAuthorization = &authorization
	require.NoError(t, scoped.Validate())
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "action"), marshalTestJSON(t, scoped))
	require.NoError(t, actionSchema.Validate(machineSchemaValue(t, scoped)))

	rawSourceSchema := compileMachineSchemaDefinition(t, "rawSourceEnvelope")
	require.NoError(t, rawSourceSchema.Validate(machineSchemaValue(t, validSourceEnvelope())))

	material := ActionMaterial{
		ContractVersion:      ContractVersion,
		MessageType:          "action_material",
		RunID:                "run-1",
		ActionID:             "action-1",
		RequirementsRevision: requirements.RequirementsRevision,
		Section: ActionMaterialSection{
			Key:             requirements.Sections[0].Key,
			Role:            requirements.Sections[0].Role,
			Title:           requirements.Sections[0].Title,
			SectionRevision: requirements.Sections[0].SectionRevision,
			Markdown:        "# Outcome\n",
		},
	}
	materialSchema := compileMachineSchemaDefinition(t, "actionMaterial")
	require.NoError(t, materialSchema.Validate(machineSchemaValue(t, material)))

	implementation := implementedTestOutcome(OutcomeCompleted, ImplementationApplied)
	implementation.ActionKind = ActionImplement
	implementationJSON := marshalTestJSON(t, implementation)
	decoded, err := DecodeOutcome(bytes.NewReader(implementationJSON))
	require.NoError(t, err)
	require.NoError(t, decoded.Validate(ActionImplement, "action-1"))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "outcome"), implementationJSON)

	review := reviewedTestOutcome(OutcomeCompleted, ReviewFindings, []Finding{{Location: "a.go:1", Summary: "bug", Detail: "detail"}})
	review.ActionKind = ActionReview
	reviewJSON := marshalTestJSON(t, review)
	decoded, err = DecodeOutcome(bytes.NewReader(reviewJSON))
	require.NoError(t, err)
	require.NoError(t, decoded.Validate(ActionReview, "action-1"))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "outcome"), reviewJSON)
}

func TestSourceEnvelopeSchemaValidatesRealEnvelope(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "docs", "reference", "source-envelope.schema.json")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	const schemaURL = "https://signalridge.github.io/slipway/reference/source-envelope.schema.json"
	require.NoError(t, compiler.AddResource(schemaURL, document))
	schema, err := compiler.Compile(schemaURL)
	require.NoError(t, err)
	require.NoError(t, schema.Validate(machineSchemaValue(t, validSourceEnvelope())))

	invalidHead := validSourceEnvelope()
	invalidHead.Body = strings.Replace(
		invalidHead.Body,
		changeSourceMarker,
		"<!-- slipway-level: objective/v1 -->",
		1,
	)
	invalidHead.Comments = []RawSourceComment{}
	require.NoError(t, schema.Validate(machineSchemaValue(t, invalidHead)))

	tooManyLabels := validSourceEnvelope()
	tooManyLabels.Labels = make([]string, maxSourceLabels+1)
	for index := range tooManyLabels.Labels {
		tooManyLabels.Labels[index] = "label-" + jsonNumber(int64(index))
	}
	require.Error(t, schema.Validate(machineSchemaValue(t, tooManyLabels)))

	manifest, err := parseSourceManifest(normalizeLineEndings(validSourceEnvelope().Body))
	require.NoError(t, err)
	manifestSchema, err := compiler.Compile(schemaURL + "#/$defs/sourceManifest")
	require.NoError(t, err)
	require.NoError(t, manifestSchema.Validate(machineSchemaValue(t, manifest)))
	incompleteManifest := machineSchemaValue(t, manifest).(map[string]any)
	manifestSections := incompleteManifest["sections"].([]any)
	incompleteManifest["sections"] = manifestSections[:len(manifestSections)-1]
	require.Error(t, manifestSchema.Validate(incompleteManifest))
}

func TestMachineAndSourceSchemasShareRawEnvelopeDefinitions(t *testing.T) {
	t.Parallel()

	machineRaw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "machine-protocol.schema.json"))
	require.NoError(t, err)
	var machineSchema map[string]any
	require.NoError(t, json.Unmarshal(machineRaw, &machineSchema))

	sourceRaw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "source-envelope.schema.json"))
	require.NoError(t, err)
	var sourceSchema map[string]any
	require.NoError(t, json.Unmarshal(sourceRaw, &sourceSchema))

	machineDefinitions := schemaMap(t, machineSchema, "$defs")
	sourceDefinitions := schemaMap(t, sourceSchema, "$defs")
	for _, name := range []string{"sourceParent", "rawSourceComment", "rawSourceEnvelope"} {
		assert.Equal(t, schemaMap(t, sourceDefinitions, name), schemaMap(t, machineDefinitions, name), name)
	}
}

func TestMachineProtocolPinnedSourceSchemaRequiresChangeProfileRoles(t *testing.T) {
	t.Parallel()

	schema := compileMachineSchemaDefinition(t, "pinnedSource")
	pinned := mustParseSource(t, validSourceEnvelope())
	require.NoError(t, schema.Validate(machineSchemaValue(t, pinned)))

	incomplete := machineSchemaValue(t, pinned).(map[string]any)
	sections := incomplete["sections"].([]any)
	incomplete["sections"] = sections[:len(sections)-1]
	require.Error(t, schema.Validate(incomplete))

	tooManyAliases := mustParseSource(t, validSourceEnvelope())
	tooManyAliases.URLAliases = make([]string, maxSourceURLAliases+1)
	for index := range tooManyAliases.URLAliases {
		tooManyAliases.URLAliases[index] = "https://github.com/example/repository/issues/" + jsonNumber(int64(1000+index))
	}
	require.Error(t, schema.Validate(machineSchemaValue(t, tooManyAliases)))
}

func TestMachineProtocolSourceCandidateSchemaMatchesGoContract(t *testing.T) {
	t.Parallel()

	schema := compileMachineSchemaDefinition(t, "sourceCandidate")
	valid := newSourceCandidate(sourceCandidateForTest(t, validSourceEnvelope()))
	require.NoError(t, schema.Validate(machineSchemaValue(t, valid)))

	invalidEnvelope := validSourceEnvelope()
	invalidEnvelope.Body = strings.Replace(
		invalidEnvelope.Body,
		changeSourceMarker,
		"<!-- slipway-level: objective/v1 -->",
		1,
	)
	invalid := newSourceCandidate(sourceCandidateForTest(t, invalidEnvelope))
	require.False(t, invalid.Valid)
	require.NoError(t, schema.Validate(machineSchemaValue(t, invalid)))

	validWithoutSnapshot := machineSchemaValue(t, valid).(map[string]any)
	delete(validWithoutSnapshot, "snapshot")
	invalidWithoutError := machineSchemaValue(t, invalid).(map[string]any)
	delete(invalidWithoutError, "classification_error")
	invalidWithSnapshot := machineSchemaValue(t, invalid).(map[string]any)
	invalidWithSnapshot["snapshot"] = machineSchemaValue(t, *valid.Snapshot)
	invalidWithSnapshot["requirements_revision"] = valid.RequirementsRevision
	validWithError := machineSchemaValue(t, valid).(map[string]any)
	validWithError["classification_error"] = "must be absent"
	invalidWithValidCode := machineSchemaValue(t, invalid).(map[string]any)
	invalidWithValidCode["classification_code"] = SourceClassificationValidChange
	missingSourceRevision := machineSchemaValue(t, valid).(map[string]any)
	delete(missingSourceRevision, "source_revision")

	illegal := []struct {
		name  string
		value any
	}{
		{name: "valid without snapshot", value: validWithoutSnapshot},
		{name: "invalid without classification error", value: invalidWithoutError},
		{name: "invalid with valid snapshot", value: invalidWithSnapshot},
		{name: "valid with classification error", value: validWithError},
		{name: "invalid with valid classification code", value: invalidWithValidCode},
		{name: "missing source revision", value: missingSourceRevision},
	}
	for _, test := range illegal {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, schema.Validate(test.value))
		})
	}
}

func TestMachineProtocolSchemaAcceptsSourceIssueMismatchErrorDetails(t *testing.T) {
	t.Parallel()

	workspaceIdentity := "sha256:" + strings.Repeat("a", 64)
	value := map[string]any{
		"contract_version": ContractVersion,
		"code":             "source_issue_mismatch",
		"message":          "refreshed source belongs to another issue",
		"next":             NoneNext(workspaceIdentity),
		"exit_code":        3,
		"details": map[string]any{
			"run_id":             "run-1",
			"state":              RunActive,
			"pinned_issue_id":    "I_pinned",
			"refreshed_issue_id": "I_refreshed",
		},
	}
	schema := compileMachineSchemaDefinition(t, "cliError")
	require.NoError(t, schema.Validate(machineSchemaValue(t, value)))

	value["details"].(map[string]any)["unexpected"] = true
	require.Error(t, schema.Validate(machineSchemaValue(t, value)))
}

func TestMachineProtocolResumeResultSchemaEnforcesReceiptMatrix(t *testing.T) {
	t.Parallel()

	schema := compileMachineSchemaDefinition(t, "resumeResult")
	legal := []ResumeResult{
		{Operation: ResumeOperationAdHoc, BudgetApplied: true},
		{Operation: ResumeOperationSourceRefreshed, BudgetApplied: true},
		{Operation: ResumeOperationSourceRefreshSkipped, BudgetApplied: true},
		{Operation: ResumeOperationSourceCandidate, BudgetApplied: false, CandidateID: "candidate-1"},
		{Operation: ResumeOperationSourceAmended, BudgetApplied: true, CandidateID: "candidate-1"},
		{Operation: ResumeOperationSourcePinned, BudgetApplied: true},
		{Operation: ResumeOperationSourcePinned, BudgetApplied: true, CandidateID: "candidate-1"},
	}
	for _, result := range legal {
		require.NoError(t, schema.Validate(machineSchemaValue(t, result)), result.Operation)
	}

	illegal := []ResumeResult{
		{Operation: "invented", BudgetApplied: true},
		{Operation: ResumeOperationAdHoc, BudgetApplied: true, CandidateID: "candidate-1"},
		{Operation: ResumeOperationSourceCandidate, BudgetApplied: true, CandidateID: "candidate-1"},
		{Operation: ResumeOperationSourceCandidate, BudgetApplied: false},
		{Operation: ResumeOperationSourceAmended, BudgetApplied: true},
		{Operation: ResumeOperationSourcePinned, BudgetApplied: false},
	}
	for _, result := range illegal {
		require.Error(t, schema.Validate(machineSchemaValue(t, result)), result.Operation)
	}
}

func TestMachineProtocolOutcomeSchemaEnforcesGoMatrix(t *testing.T) {
	t.Parallel()

	schema := compileMachineOutcomeSchema(t)
	destructive := destructiveRequestForTest(t)
	legal := []struct {
		name    string
		kind    ActionKind
		outcome Outcome
	}{
		{name: "orient completed", kind: ActionOrient, outcome: testOutcome(OutcomeCompleted)},
		{name: "orient partial", kind: ActionOrient, outcome: testOutcome(OutcomePartial)},
		{name: "orient error", kind: ActionOrient, outcome: testOutcome(OutcomeError)},
		{name: "orient decision", kind: ActionOrient, outcome: pausedTestOutcome(PauseDecisionRequired, nil)},
		{name: "orient environment", kind: ActionOrient, outcome: pausedTestOutcome(PauseEnvironmentUnavailable, nil)},
		{name: "clarify completed", kind: ActionClarify, outcome: testOutcome(OutcomeCompleted)},
		{name: "clarify error", kind: ActionClarify, outcome: testOutcome(OutcomeError)},
		{name: "clarify decision", kind: ActionClarify, outcome: pausedTestOutcome(PauseDecisionRequired, nil)},
		{name: "clarify environment", kind: ActionClarify, outcome: pausedTestOutcome(PauseEnvironmentUnavailable, nil)},
		{name: "implement applied", kind: ActionImplement, outcome: implementedTestOutcome(OutcomeCompleted, ImplementationApplied)},
		{name: "implement not needed", kind: ActionImplement, outcome: implementedTestOutcome(OutcomeCompleted, ImplementationNotNeeded)},
		{name: "implement partial", kind: ActionImplement, outcome: implementedTestOutcome(OutcomePartial, ImplementationPartial)},
		{name: "implement error", kind: ActionImplement, outcome: implementedTestOutcome(OutcomeError, ImplementationUnable)},
		{name: "implement decision", kind: ActionImplement, outcome: pausedTestOutcome(PauseDecisionRequired, nil)},
		{name: "implement destructive pause", kind: ActionImplement, outcome: pausedTestOutcome(PauseDestructiveConfirm, &destructive)},
		{name: "implement environment", kind: ActionImplement, outcome: pausedTestOutcome(PauseEnvironmentUnavailable, nil)},
		{name: "review no findings", kind: ActionReview, outcome: reviewedTestOutcome(OutcomeCompleted, ReviewNoFindings, nil)},
		{name: "review findings", kind: ActionReview, outcome: reviewedTestOutcome(OutcomeCompleted, ReviewFindings, []Finding{{Location: "a.go:1", Summary: "bug", Detail: "detail"}})},
		{name: "review partial", kind: ActionReview, outcome: reviewedTestOutcome(OutcomePartial, ReviewInconclusive, nil)},
		{name: "review error", kind: ActionReview, outcome: reviewedTestOutcome(OutcomeError, ReviewError, nil)},
		{name: "summarize completed", kind: ActionSummarize, outcome: testOutcome(OutcomeCompleted)},
		{name: "summarize error", kind: ActionSummarize, outcome: testOutcome(OutcomeError)},
	}
	for _, test := range legal {
		test := test
		t.Run("accepts "+test.name, func(t *testing.T) {
			t.Parallel()
			outcome := test.outcome
			outcome.ActionKind = test.kind
			require.NoError(t, schema.Validate(machineSchemaValue(t, outcome)))
		})
	}

	missingKind := machineSchemaValue(t, outcomeForSchema(ActionOrient, testOutcome(OutcomeCompleted))).(map[string]any)
	delete(missingKind, "action_kind")
	illegal := []struct {
		name  string
		value any
	}{
		{name: "missing action kind", value: missingKind},
		{name: "unknown action kind", value: machineSchemaValue(t, outcomeForSchema(ActionKind("unknown"), testOutcome(OutcomeCompleted)))},
		{name: "kind and payload mismatch", value: machineSchemaValue(t, outcomeForSchema(ActionImplement, testOutcome(OutcomeCompleted)))},
		{name: "clarify partial", value: machineSchemaValue(t, outcomeForSchema(ActionClarify, testOutcome(OutcomePartial)))},
		{name: "orient implementation", value: machineSchemaValue(t, outcomeForSchema(ActionOrient, implementedTestOutcome(OutcomeCompleted, ImplementationApplied)))},
		{name: "orient destructive pause", value: machineSchemaValue(t, outcomeForSchema(ActionOrient, pausedTestOutcome(PauseDestructiveConfirm, &destructive)))},
		{name: "implement result mismatch", value: machineSchemaValue(t, outcomeForSchema(ActionImplement, implementedTestOutcome(OutcomeCompleted, ImplementationPartial)))},
		{name: "implement suggestion", value: machineSchemaValue(t, outcomeForSchema(ActionImplement, withSuggestion(implementedTestOutcome(OutcomeCompleted, ImplementationApplied), ActionSummarize)))},
		{name: "review needs input", value: machineSchemaValue(t, outcomeForSchema(ActionReview, pausedTestOutcome(PauseDecisionRequired, nil)))},
		{name: "review findings mismatch", value: machineSchemaValue(t, outcomeForSchema(ActionReview, reviewedTestOutcome(OutcomeCompleted, ReviewFindings, nil)))},
		{name: "summarize partial", value: machineSchemaValue(t, outcomeForSchema(ActionSummarize, testOutcome(OutcomePartial)))},
		{name: "summarize suggestion", value: machineSchemaValue(t, outcomeForSchema(ActionSummarize, withSuggestion(testOutcome(OutcomeCompleted), ActionClarify)))},
	}
	for _, test := range illegal {
		test := test
		t.Run("rejects "+test.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, schema.Validate(test.value))
		})
	}
}

func compileMachineOutcomeSchema(t *testing.T) *jsonschema.Schema {
	return compileMachineSchemaDefinition(t, "outcome")
}

func compileMachineSchemaDefinition(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "machine-protocol.schema.json"))
	require.NoError(t, err)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)

	const schemaURL = "https://signalridge.github.io/slipway/reference/machine-protocol.schema.json"
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	require.NoError(t, compiler.AddResource(schemaURL, document))
	schema, err := compiler.Compile(schemaURL + "#/$defs/" + name)
	require.NoError(t, err)
	return schema
}

func machineSchemaValue(t *testing.T, value any) any {
	t.Helper()

	decoded, err := jsonschema.UnmarshalJSON(bytes.NewReader(marshalTestJSON(t, value)))
	require.NoError(t, err)
	return decoded
}

func outcomeForSchema(kind ActionKind, outcome Outcome) Outcome {
	outcome.ActionKind = kind
	return outcome
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
