package autopilot

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachineProtocolDocumentationExamplesMatchSchema(t *testing.T) {
	t.Parallel()
	actionSchema := compileMachineSchemaDefinition(t, "action")
	outcomeSchema := compileMachineSchemaDefinition(t, "outcome")
	locales := []string{"en", "zh", "ja"}
	var commandExamples []string
	var referenceExamples []string
	var tutorialExamples []string
	for _, locale := range locales {
		referencePath := filepath.Join("..", "..", "docs", locale, "reference", "machine-protocol.md")
		referenceRaw, err := os.ReadFile(referencePath)
		require.NoError(t, err)
		currentCommands := fencedDocumentationBlocks(string(referenceRaw), "text")
		require.Len(t, currentCommands, 2)
		if commandExamples == nil {
			commandExamples = currentCommands
		} else {
			assert.Equal(t, commandExamples, currentCommands, "machine command syntax drifted in locale %s", locale)
		}
		currentReference := fencedDocumentationBlocks(string(referenceRaw), "json")
		require.Len(t, currentReference, 2)
		if referenceExamples == nil {
			referenceExamples = currentReference
		} else {
			assert.Equal(t, referenceExamples, currentReference, "machine examples drifted in locale %s", locale)
		}

		tutorialPath := filepath.Join("..", "..", "docs", locale, "guides", "machine-protocol-v2.md")
		tutorialRaw, err := os.ReadFile(tutorialPath)
		require.NoError(t, err)
		currentTutorial := tutorialOutcomeExamples(t, string(tutorialRaw))
		require.Len(t, currentTutorial, 3)
		if tutorialExamples == nil {
			tutorialExamples = currentTutorial
		} else {
			assert.Equal(t, tutorialExamples, currentTutorial, "tutorial outcomes drifted in locale %s", locale)
		}
	}

	var action any
	require.NoError(t, json.Unmarshal([]byte(referenceExamples[0]), &action))
	require.NoError(t, actionSchema.Validate(action))
	var outcome any
	require.NoError(t, json.Unmarshal([]byte(referenceExamples[1]), &outcome))
	require.NoError(t, outcomeSchema.Validate(outcome))
	for index, example := range tutorialExamples {
		var value any
		require.NoError(t, json.Unmarshal([]byte(example), &value), "tutorial outcome %d", index)
		require.NoError(t, outcomeSchema.Validate(value), "tutorial outcome %d", index)
	}
}

func TestMachineProtocolSchemaDeclaresStrictDraft202012Unions(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "v2", "machine-protocol.schema.json"))
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(raw, &schema))
	assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", schema["$schema"])
	assert.Equal(t, "https://signalridge.github.io/slipway/reference/v2/machine-protocol.schema.json", schema["$id"])
	require.Len(t, schemaSlice(t, schema, "oneOf"), 11)

	definitions := schemaMap(t, schema, "$defs")
	for _, name := range []string{
		"action", "actionMaterial", "rawSourceEnvelope", "outcome", "protocolState", "cliError",
		"changeReport", "listReport", "doctorReport", "runStatus", "statusList",
	} {
		definition := schemaMap(t, definitions, name)
		assert.False(t, definition["additionalProperties"].(bool), name)
	}
	runStatusProperties := schemaMap(t, schemaMap(t, definitions, "runStatus"), "properties")
	assert.Contains(t, runStatusProperties, "workspace_foreign")
	assert.NotContains(t, runStatusProperties, "decision_suggestions")
	assert.NotContains(t, schemaMap(t, schemaMap(t, definitions, "protocolState"), "properties"), "suggested_actions")
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
		[]string{"contract_version", "hosts", "transaction_outcome", "written", "removed", "preserved", "recovery_artifacts", "warnings"},
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
		[]string{"contract_version", "runs", "unavailable_runs"},
		schemaStrings(t, schemaMap(t, definitions, "statusList"), "required"),
	)

	assertEveryObjectSchemaIsClosed(t, schema, "$")
}

func TestMachineProtocolSchemaUnitFixturesMatchGoContract(t *testing.T) {
	t.Parallel()

	// These hand-built values exercise the full contract matrix. Real command
	// emitter bytes are validated separately in cmd/machine_output_test.go.

	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "v2", "machine-protocol.schema.json"))
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
		"hosts":            []string{"claude"}, "transaction_outcome": "committed", "written": []string{}, "removed": []string{},
		"preserved": []string{}, "recovery_artifacts": []string{}, "warnings": []string{},
	}))
	changeReportSchema := compileMachineSchemaDefinition(t, "changeReport")
	require.Error(t, changeReportSchema.Validate(machineSchemaValue(t, map[string]any{
		"contract_version": ContractVersion,
		"hosts":            []string{"claude"}, "transaction_outcome": "rolled_back", "written": []string{"claimed.md"}, "removed": []string{},
		"preserved": []string{}, "recovery_artifacts": []string{}, "warnings": []string{},
	})))
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
		"contract_version": ContractVersion, "run_id": "run-1", "state": "active", "action": testAction(), "next": next,
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "cliError"), marshalTestJSON(t, map[string]any{
		"contract_version": ContractVersion, "code": "invalid_usage", "message": "invalid", "next": next, "exit_code": 2,
	}))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "statusList"), marshalTestJSON(t, map[string]any{
		"contract_version": ContractVersion, "runs": []any{}, "unavailable_runs": []any{},
	}))
	var runObject map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(marshalTestJSON(t, runFixture), &runObject))
	runObject["next"] = marshalTestJSON(t, next)
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "runStatus"), marshalTestJSON(t, runObject))

	foreignWorkspace := t.TempDir()
	foreignNext, err := NewCommandNext(
		NextOperationCommand,
		foreignWorkspace,
		"inspect-run-in-its-workspace",
		[]string{"slipway", "status", "foreign-run", "--root", foreignWorkspace},
		[]NextInput{},
	)
	require.NoError(t, err)
	foreignRunObject := map[string]any{
		"contract_version": ContractVersion,
		"id":               "foreign-run",
		"goal":             "foreign goal",
		"workspace":        foreignWorkspace,
		"workspace_identity": map[string]any{
			"version": 1, "worktree_root": foreignWorkspace, "git_dir": filepath.Join(foreignWorkspace, ".git"),
			"git_common_dir": filepath.Join(foreignWorkspace, ".git"),
			"id":             "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		"workspace_foreign": true,
		"state":             RunActive,
		"created_at":        "2026-01-01T00:00:00Z",
		"next":              foreignNext,
	}
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "runStatus"), marshalTestJSON(t, foreignRunObject))
	runStatusSchema := compileMachineSchemaDefinition(t, "runStatus")
	require.NoError(t, runStatusSchema.Validate(machineSchemaValue(t, foreignRunObject)))
	foreignRunObject["initial_budget"] = 1
	require.Error(t, runStatusSchema.Validate(machineSchemaValue(t, foreignRunObject)), "foreign headers must not expose fully replayed fields")

	adHoc := testAction()
	require.NoError(t, adHoc.Validate())
	adHocJSON := marshalTestJSON(t, adHoc)
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "action"), adHocJSON)
	actionSchema := compileMachineSchemaDefinition(t, "action")
	require.NoError(t, actionSchema.Validate(machineSchemaValue(t, adHoc)))
	whitespaceAction := adHoc
	whitespaceAction.Goal = " \t "
	require.Error(t, whitespaceAction.Validate())
	require.Error(t, actionSchema.Validate(machineSchemaValue(t, whitespaceAction)))
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

	actionRecordSchema := compileMachineSchemaDefinition(t, "actionRecord")
	reviewAction := testAction()
	reviewAction.Kind = ActionReview
	skippedReviewRecord := ActionRecord{
		Action: reviewAction,
		ReviewProjection: &Review{
			Result:        ReviewNotRun,
			Findings:      []Finding{},
			Uncertainties: []string{},
		},
		Skipped: true,
	}
	require.NoError(t, actionRecordSchema.Validate(machineSchemaValue(t, skippedReviewRecord)))
	notSkippedReviewRecord := skippedReviewRecord
	notSkippedReviewRecord.Skipped = false
	require.Error(t, actionRecordSchema.Validate(machineSchemaValue(t, notSkippedReviewRecord)))
	nonReviewRecord := skippedReviewRecord
	nonReviewRecord.Action.Kind = ActionOrient
	require.Error(t, actionRecordSchema.Validate(machineSchemaValue(t, nonReviewRecord)))
	projectionWithOutcomeDigest := skippedReviewRecord
	projectionWithOutcomeDigest.OutcomePayloadSHA256 = "sha256:" + strings.Repeat("a", 64)
	require.Error(t, actionRecordSchema.Validate(machineSchemaValue(t, projectionWithOutcomeDigest)))

	rawSourceSchema := compileMachineSchemaDefinition(t, "rawSourceEnvelope")
	validEnvelope := validSourceEnvelope()
	require.NoError(t, rawSourceSchema.Validate(machineSchemaValue(t, validEnvelope)))
	defaultPortEnvelope := validEnvelope
	defaultPortEnvelope.CanonicalURL = strings.Replace(defaultPortEnvelope.CanonicalURL, "github.com/", "github.com:443/", 1)
	require.Error(t, rawSourceSchema.Validate(machineSchemaValue(t, defaultPortEnvelope)))
	controlEnvelope := validEnvelope
	controlEnvelope.Title += "\u007f"
	require.Error(t, rawSourceSchema.Validate(machineSchemaValue(t, controlEnvelope)))
	markdownControlEnvelope := validEnvelope
	markdownControlEnvelope.Body += "\u0085"
	require.Error(t, rawSourceSchema.Validate(machineSchemaValue(t, markdownControlEnvelope)))

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
	whitespaceMaterial := machineSchemaValue(t, material).(map[string]any)
	whitespaceMaterial["section"].(map[string]any)["markdown"] = " \n\t"
	require.Error(t, materialSchema.Validate(whitespaceMaterial))

	implementation := implementedTestOutcome(OutcomeCompleted, ImplementationApplied)
	implementation.ActionKind = ActionImplement
	implementationJSON := marshalTestJSON(t, implementation)
	decoded, err := DecodeOutcome(bytes.NewReader(implementationJSON))
	require.NoError(t, err)
	require.NoError(t, decoded.Validate(ActionImplement, "action-1"))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "outcome"), implementationJSON)
	whitespaceOutcome := implementation
	whitespaceOutcome.Summary = " \n\t"
	require.Error(t, whitespaceOutcome.Validate(ActionImplement, "action-1"))
	require.Error(t, compileMachineOutcomeSchema(t).Validate(machineSchemaValue(t, whitespaceOutcome)))

	review := reviewedTestOutcome(OutcomeCompleted, ReviewFindings, []Finding{{Location: "a.go:1", Summary: "bug", Detail: "detail"}})
	review.ActionKind = ActionReview
	reviewJSON := marshalTestJSON(t, review)
	decoded, err = DecodeOutcome(bytes.NewReader(reviewJSON))
	require.NoError(t, err)
	require.NoError(t, decoded.Validate(ActionReview, "action-1"))
	assertSchemaObjectFixture(t, schemaMap(t, definitions, "outcome"), reviewJSON)
}

func TestMachineProtocolSchemaAcceptsSignalStyleActivityExitCode(t *testing.T) {
	t.Parallel()

	activity := Activity{
		Kind:     "test",
		Command:  "killed-test-process",
		ExitCode: -1,
		Summary:  "terminated by signal before an exit status was available",
	}
	activitySchema := compileMachineSchemaDefinition(t, "activity")
	require.NoError(t, activitySchema.Validate(machineSchemaValue(t, activity)))
	require.Error(t, activitySchema.Validate(machineSchemaValue(t, map[string]any{
		"kind": "test", "command": "test", "exit_code": -1.5, "summary": "not an integer",
	})))

	outcome := implementedTestOutcome(OutcomeCompleted, ImplementationApplied)
	outcome.ActionKind = ActionImplement
	outcome.Implementation.Activities = []Activity{activity}
	encoded := marshalTestJSON(t, outcome)
	require.NoError(t, compileMachineOutcomeSchema(t).Validate(machineSchemaValue(t, outcome)))
	decoded, err := DecodeOutcome(bytes.NewReader(encoded))
	require.NoError(t, err)
	require.Equal(t, []Activity{activity}, decoded.Implementation.Activities)
}

func TestMachineProtocolSchemaEnforcesNextFamiliesAndActiveAction(t *testing.T) {
	t.Parallel()
	workspaceID := "sha256:" + strings.Repeat("a", 64)
	nextSchema := compileMachineSchemaDefinition(t, "next")

	variant := func(id string, argv ...string) map[string]any {
		return map[string]any{"id": id, "base_argv": argv, "inputs": []any{}}
	}
	invalid := []map[string]any{
		{"operation": "action", "workspace_identity": workspaceID, "variants": []any{variant("submit", "slipway", "_machine", "resume", "run-1", "--root", "/workspace")}},
		{"operation": "start", "workspace_identity": workspaceID, "variants": []any{variant("retry-run", "slipway", "run", "--budget", "4", "--json", "--root", "/workspace", "--", "goal", "extra")}},
		{"operation": "command", "workspace_identity": workspaceID, "variants": []any{variant("bad-command", "slipway", "run", "--root", "/workspace", "--", "goal")}},
		{"operation": "resume", "workspace_identity": workspaceID, "variants": []any{variant(
			"skip-action", "slipway", "_machine", "skip", "--run", "run-1", "--action", "action-1", "--root", "/workspace", "--extra",
		)}},
		{"operation": "start", "workspace_identity": workspaceID, "variants": []any{variant("skip-action", "slipway", "run", "--budget", "4", "--json", "--root", "/workspace", "--", "goal")}},
		{"operation": "command", "workspace_identity": workspaceID, "variants": []any{variant("skip-action", "slipway", "status", "--root", "/workspace")}},
		{"operation": "action", "workspace_identity": workspaceID, "variants": []any{variant(
			"submit-outcome-stdin", "slipway", "_machine", "submit", "--run", "run-1", "--action", "action-1", "--root", "/workspace", "--outcome-stdin", "--extra",
		)}},
		{"operation": "answer", "workspace_identity": workspaceID, "variants": []any{variant(
			"confirm-destructive", "slipway", "_machine", "answer", "--run", "run-1", "--action", "action-1", "--root", "/workspace", "--confirm-destructive",
		)}},
		{"operation": "resume", "workspace_identity": workspaceID, "variants": []any{variant(
			"resume-ad-hoc", "slipway", "_machine", "resume", "run-1", "--root", "/workspace", "--budget", "1",
		)}},
		{"operation": "start", "workspace_identity": workspaceID, "variants": []any{variant(
			"retry-run", "slipway", "run", "--budget", "0", "--json", "--root", "/workspace", "--", "goal",
		)}},
		{"operation": "command", "workspace_identity": workspaceID, "variants": []any{variant(
			"inspect", "slipway", "status", "--root", "/workspace", "<file>",
		)}},
		{"operation": "command", "workspace_identity": workspaceID, "variants": []any{variant(
			"inspect", "slipway", "status", "--root", "/workspace", "<line\nbreak>",
		)}},
		{"operation": "command", "workspace_identity": workspaceID, "variants": []any{variant(
			"inspect", "slipway", "status", "--root", "/work\x00space",
		)}},
		{"operation": "start", "workspace_identity": workspaceID, "variants": []any{variant(
			"retry-run", "slipway", "run", "--budget", "01", "--json", "--root", "/workspace", "--", "goal",
		)}},
		{"operation": "start", "workspace_identity": workspaceID, "variants": []any{variant(
			"retry-run", "slipway", "run", "--budget", "1001", "--json", "--root", "/workspace", "--", "goal",
		)}},
	}
	for _, fixture := range invalid {
		require.Error(t, nextSchema.Validate(machineSchemaValue(t, fixture)))
	}
	require.NoError(t, nextSchema.Validate(machineSchemaValue(t, map[string]any{
		"operation": "start", "workspace_identity": workspaceID,
		"variants": []any{variant("retry-run", "slipway", "run", "--budget", "4", "--json", "--root", "/workspace", "--", "-leading goal")},
	})))
	require.NoError(t, nextSchema.Validate(machineSchemaValue(t, map[string]any{
		"operation": "start", "workspace_identity": workspaceID,
		"variants": []any{variant("retry-run", "slipway", "run", "--budget", "1000", "--json", "--root", "/workspace", "--", "goal")},
	})))
	require.NoError(t, nextSchema.Validate(machineSchemaValue(t, map[string]any{
		"operation": "start", "workspace_identity": workspaceID,
		"variants": []any{variant("retry-run", "slipway", "run", "--budget", "4", "--json", "--root", "/workspace", "--", "--")},
	})))
	refreshWithBudget := variant(
		"refresh-source", "slipway", "_machine", "resume", "run-1", "--root", "/workspace", "--budget", "9",
	)
	refreshWithBudget["inputs"] = []any{map[string]any{
		"name": "source_file", "type": "path", "flag": "--source-file", "required": true,
	}}
	require.NoError(t, nextSchema.Validate(machineSchemaValue(t, map[string]any{
		"operation": "resume", "workspace_identity": workspaceID, "variants": []any{refreshWithBudget},
	})))

	stateSchema := compileMachineSchemaDefinition(t, "protocolState")
	state := map[string]any{
		"contract_version": ContractVersion,
		"run_id":           "run-1",
		"state":            "active",
		"next":             map[string]any{"operation": "none", "workspace_identity": workspaceID, "variants": []any{}},
	}
	require.Error(t, stateSchema.Validate(machineSchemaValue(t, state)))
	state["state"] = "paused"
	require.NoError(t, stateSchema.Validate(machineSchemaValue(t, state)))

	// resume_operation and budget_applied must appear together and the
	// operation must be one of the closed resume-operation enum values.
	resumeEnum := []string{
		"ad_hoc_resumed", "source_refreshed", "source_candidate_created",
		"source_refresh_skipped", "source_amended", "source_pinned",
	}
	withResume := map[string]any{
		"contract_version": ContractVersion,
		"run_id":           "run-1",
		"state":            "paused",
		"next":             map[string]any{"operation": "none", "workspace_identity": workspaceID, "variants": []any{}},
		"resume_operation": "ad_hoc_resumed",
		"budget_applied":   true,
	}
	require.NoError(t, stateSchema.Validate(machineSchemaValue(t, withResume)))

	for _, value := range resumeEnum {
		paired := map[string]any{}
		for key, value := range withResume {
			paired[key] = value
		}
		paired["resume_operation"] = value
		require.NoError(t, stateSchema.Validate(machineSchemaValue(t, paired)))
	}

	resumesAlone := map[string]any{}
	for key, value := range withResume {
		resumesAlone[key] = value
	}
	delete(resumesAlone, "budget_applied")
	require.Error(t, stateSchema.Validate(machineSchemaValue(t, resumesAlone)))

	budgetAlone := map[string]any{}
	for key, value := range withResume {
		budgetAlone[key] = value
	}
	delete(budgetAlone, "resume_operation")
	require.Error(t, stateSchema.Validate(machineSchemaValue(t, budgetAlone)))

	badEnum := map[string]any{}
	for key, value := range withResume {
		badEnum[key] = value
	}
	badEnum["resume_operation"] = "not_a_real_resume_operation"
	require.Error(t, stateSchema.Validate(machineSchemaValue(t, badEnum)))
}

func TestSourceEnvelopeSchemaValidatesRealEnvelope(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "docs", "reference", "v2", "source-envelope.schema.json")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	const schemaURL = "https://signalridge.github.io/slipway/reference/v2/source-envelope.schema.json"
	var metadata map[string]any
	require.NoError(t, json.Unmarshal(raw, &metadata))
	assert.Equal(t, schemaURL, metadata["$id"])
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
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

	defaultPort := validSourceEnvelope()
	defaultPort.CanonicalURL = strings.Replace(defaultPort.CanonicalURL, "github.com/", "github.com:443/", 1)
	require.Error(t, schema.Validate(machineSchemaValue(t, defaultPort)))
	controlTitle := validSourceEnvelope()
	controlTitle.Title += "\u007f"
	require.Error(t, schema.Validate(machineSchemaValue(t, controlTitle)))
	controlBody := validSourceEnvelope()
	controlBody.Body += "\u0085"
	require.Error(t, schema.Validate(machineSchemaValue(t, controlBody)))
	whitespaceTitle := validSourceEnvelope()
	whitespaceTitle.Title = "   "
	require.Error(t, schema.Validate(machineSchemaValue(t, whitespaceTitle)))
	whitespaceLabel := validSourceEnvelope()
	whitespaceLabel.Labels = []string{"   "}
	require.Error(t, schema.Validate(machineSchemaValue(t, whitespaceLabel)))

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
	whitespaceManifest := machineSchemaValue(t, manifest).(map[string]any)
	whitespaceSections := whitespaceManifest["sections"].([]any)
	whitespaceSections[0].(map[string]any)["title"] = "   "
	require.Error(t, manifestSchema.Validate(whitespaceManifest))
	whitespaceCommentID := machineSchemaValue(t, manifest).(map[string]any)
	whitespaceCommentSections := whitespaceCommentID["sections"].([]any)
	whitespaceCommentSections[0].(map[string]any)["comment_node_id"] = "   "
	require.Error(t, manifestSchema.Validate(whitespaceCommentID))
	incompleteManifest := machineSchemaValue(t, manifest).(map[string]any)
	manifestSections := incompleteManifest["sections"].([]any)
	incompleteManifest["sections"] = manifestSections[:len(manifestSections)-1]
	require.Error(t, manifestSchema.Validate(incompleteManifest))
}

func TestMachineAndSourceSchemasShareRawEnvelopeDefinitions(t *testing.T) {
	t.Parallel()

	machineRaw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "v2", "machine-protocol.schema.json"))
	require.NoError(t, err)
	var machineSchema map[string]any
	require.NoError(t, json.Unmarshal(machineRaw, &machineSchema))

	sourceRaw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "v2", "source-envelope.schema.json"))
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
	assert.Empty(t, valid.ObservationSHA256)
	assert.True(t, validSHA256(valid.SourceRevision))
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
	invalidWithSourceRevision := machineSchemaValue(t, invalid).(map[string]any)
	invalidWithSourceRevision["source_revision"] = valid.SourceRevision
	invalidWithWhitespaceError := machineSchemaValue(t, invalid).(map[string]any)
	invalidWithWhitespaceError["classification_error"] = "   "
	validWithObservation := machineSchemaValue(t, valid).(map[string]any)
	validWithObservation["observation_sha256"] = invalid.ObservationSHA256
	missingSourceRevision := machineSchemaValue(t, valid).(map[string]any)
	delete(missingSourceRevision, "source_revision")
	missingObservation := machineSchemaValue(t, invalid).(map[string]any)
	delete(missingObservation, "observation_sha256")

	illegal := []struct {
		name  string
		value any
	}{
		{name: "valid without snapshot", value: validWithoutSnapshot},
		{name: "invalid without classification error", value: invalidWithoutError},
		{name: "invalid with valid snapshot", value: invalidWithSnapshot},
		{name: "valid with classification error", value: validWithError},
		{name: "invalid with valid classification code", value: invalidWithValidCode},
		{name: "invalid with source revision", value: invalidWithSourceRevision},
		{name: "invalid with whitespace classification error", value: invalidWithWhitespaceError},
		{name: "valid with observation digest", value: validWithObservation},
		{name: "missing source revision", value: missingSourceRevision},
		{name: "missing observation digest", value: missingObservation},
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

func TestMachineProtocolSchemaAcceptsAdapterMutationErrorDetails(t *testing.T) {
	t.Parallel()

	workspaceIdentity := "sha256:" + strings.Repeat("a", 64)
	base := map[string]any{
		"contract_version": ContractVersion,
		"code":             "install_failed",
		"message":          "transaction failed",
		"next":             NoneNext(workspaceIdentity),
		"exit_code":        3,
		"details": map[string]any{
			"transaction_outcome": "rolled_back",
			"report": map[string]any{
				"contract_version":    ContractVersion,
				"hosts":               []any{"claude"},
				"transaction_outcome": "rolled_back",
				"written":             []any{},
				"removed":             []any{},
				"preserved":           []any{},
				"recovery_artifacts":  []any{},
				"warnings":            []any{},
			},
		},
	}
	schema := compileMachineSchemaDefinition(t, "cliError")
	require.NoError(t, schema.Validate(machineSchemaValue(t, base)))

	for _, outcome := range []string{"committed", "rolled_back", "not_committed", "ambiguous"} {
		variant := map[string]any{}
		for key, value := range base {
			variant[key] = value
		}
		details := map[string]any{}
		for key, value := range base["details"].(map[string]any) {
			details[key] = value
		}
		details["transaction_outcome"] = outcome
		details["report"] = map[string]any{
			"contract_version":    ContractVersion,
			"hosts":               []any{"claude"},
			"transaction_outcome": outcome,
			"written":             []any{},
			"removed":             []any{},
			"preserved":           []any{},
			"recovery_artifacts":  []any{},
			"warnings":            []any{},
		}
		variant["details"] = details
		require.NoError(t, schema.Validate(machineSchemaValue(t, variant)))
	}

	missingOutcome := map[string]any{}
	for key, value := range base {
		missingOutcome[key] = value
	}
	details := map[string]any{"report": base["details"].(map[string]any)["report"]}
	missingOutcome["details"] = details
	require.Error(t, schema.Validate(machineSchemaValue(t, missingOutcome)))

	badOutcome := map[string]any{}
	for key, value := range base {
		badOutcome[key] = value
	}
	badDetails := map[string]any{}
	for key, value := range base["details"].(map[string]any) {
		badDetails[key] = value
	}
	badDetails["transaction_outcome"] = "partially_committed"
	badOutcome["details"] = badDetails
	require.Error(t, schema.Validate(machineSchemaValue(t, badOutcome)))

	base["details"].(map[string]any)["unexpected"] = true
	require.Error(t, schema.Validate(machineSchemaValue(t, base)))
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
		{name: "orient decision supersession", kind: ActionOrient, outcome: withDecisionSupersession(pausedTestOutcome(PauseDecisionRequired, nil), "prior-action")},
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
	invalidRequestID := destructive
	invalidRequestID.RequestID = "request-1"
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
		{name: "environment decision supersession", value: machineSchemaValue(t, outcomeForSchema(ActionOrient, withDecisionSupersession(pausedTestOutcome(PauseEnvironmentUnavailable, nil), "prior-action")))},
		{name: "destructive request non uuid", value: machineSchemaValue(t, outcomeForSchema(ActionImplement, pausedTestOutcome(PauseDestructiveConfirm, &invalidRequestID)))},
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

	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "reference", "v2", "machine-protocol.schema.json"))
	require.NoError(t, err)
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	require.NoError(t, err)

	const schemaURL = "https://signalridge.github.io/slipway/reference/v2/machine-protocol.schema.json"
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

func fencedDocumentationBlocks(content, language string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	opening := "```" + language
	var blocks []string
	for index := 0; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != opening {
			continue
		}
		start := index + 1
		index = start
		for index < len(lines) && strings.TrimSpace(lines[index]) != "```" {
			index++
		}
		if index >= len(lines) {
			break
		}
		blocks = append(blocks, strings.Join(lines[start:index], "\n"))
	}
	return blocks
}

func tutorialOutcomeExamples(t *testing.T, content string) []string {
	t.Helper()
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	var examples []string
	for index := 0; index < len(lines); index++ {
		if !strings.Contains(lines[index], "jq -n") {
			continue
		}
		var object []string
		for ; index < len(lines); index++ {
			line := lines[index]
			if len(object) == 0 {
				marker := strings.Index(line, "'{")
				if marker < 0 {
					continue
				}
				object = append(object, line[marker+1:])
				continue
			}
			if marker := strings.Index(line, "}' >"); marker >= 0 {
				object = append(object, line[:marker+1])
				break
			}
			object = append(object, line)
		}
		require.NotEmpty(t, object, "jq outcome example has no object")
		examples = append(examples, normalizeTutorialOutcome(strings.Join(object, "\n")))
	}
	return examples
}

var tutorialObjectKey = regexp.MustCompile(`(?m)^(\s*)([a-z_][a-z0-9_]*):`)

func normalizeTutorialOutcome(example string) string {
	example = strings.ReplaceAll(example, "$action", `"action-id"`)
	example = strings.ReplaceAll(example, "$check_command", `"git diff --check"`)
	return tutorialObjectKey.ReplaceAllString(example, `$1"$2":`)
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
