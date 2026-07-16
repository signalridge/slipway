package autopilot

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutcomeValidationAcceptsEveryLegalMatrixRow(t *testing.T) {
	t.Parallel()

	destructive := destructiveRequestForTest(t)
	tests := []struct {
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
		{name: "implement destructive", kind: ActionImplement, outcome: pausedTestOutcome(PauseDestructiveConfirm, &destructive)},
		{name: "implement environment", kind: ActionImplement, outcome: pausedTestOutcome(PauseEnvironmentUnavailable, nil)},
		{name: "review no findings", kind: ActionReview, outcome: reviewedTestOutcome(OutcomeCompleted, ReviewNoFindings, nil)},
		{name: "review findings", kind: ActionReview, outcome: reviewedTestOutcome(OutcomeCompleted, ReviewFindings, []Finding{{Location: "a.go:1", Summary: "bug", Detail: "nil is dereferenced"}})},
		{name: "review partial", kind: ActionReview, outcome: reviewedTestOutcome(OutcomePartial, ReviewInconclusive, nil)},
		{name: "review error", kind: ActionReview, outcome: reviewedTestOutcome(OutcomeError, ReviewError, nil)},
		{name: "summarize completed", kind: ActionSummarize, outcome: testOutcome(OutcomeCompleted)},
		{name: "summarize error", kind: ActionSummarize, outcome: testOutcome(OutcomeError)},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			outcome := test.outcome
			outcome.ActionKind = test.kind
			require.NoError(t, outcome.Validate(test.kind, "action-1"))
		})
	}
}

func TestOutcomeValidationRejectsIllegalMatrixCombinations(t *testing.T) {
	t.Parallel()

	destructive := destructiveRequestForTest(t)
	finding := []Finding{{Location: "a.go:1", Summary: "bug", Detail: "detail"}}
	tests := []struct {
		name    string
		kind    ActionKind
		outcome Outcome
		want    string
	}{
		{name: "host skipped status", kind: ActionOrient, outcome: outcomeWithStatus(OutcomeStatus("skipped")), want: "invalid outcome status"},
		{name: "clarify partial", kind: ActionClarify, outcome: testOutcome(OutcomePartial), want: "clarify does not support"},
		{name: "review needs input", kind: ActionReview, outcome: pausedTestOutcome(PauseDecisionRequired, nil), want: "review does not support needs_input"},
		{name: "review suggestion", kind: ActionReview, outcome: withSuggestion(reviewedTestOutcome(OutcomeCompleted, ReviewFindings, finding), ActionImplement), want: "review outcomes cannot suggest"},
		{name: "review not run", kind: ActionReview, outcome: reviewedTestOutcome(OutcomeCompleted, ReviewNotRun, nil), want: "completed review result"},
		{name: "completed implement partial result", kind: ActionImplement, outcome: implementedTestOutcome(OutcomeCompleted, ImplementationPartial), want: "completed implement result"},
		{name: "partial implement applied result", kind: ActionImplement, outcome: implementedTestOutcome(OutcomePartial, ImplementationApplied), want: "partial implement result"},
		{name: "error implement applied result", kind: ActionImplement, outcome: implementedTestOutcome(OutcomeError, ImplementationApplied), want: "error implement result"},
		{name: "findings result without findings", kind: ActionReview, outcome: reviewedTestOutcome(OutcomeCompleted, ReviewFindings, nil), want: "at least one finding"},
		{name: "no findings result with finding", kind: ActionReview, outcome: reviewedTestOutcome(OutcomeCompleted, ReviewNoFindings, finding), want: "zero findings"},
		{name: "implementation on orient", kind: ActionOrient, outcome: implementedTestOutcome(OutcomeCompleted, ImplementationApplied), want: "implementation is only valid"},
		{name: "review on clarify", kind: ActionClarify, outcome: reviewedTestOutcome(OutcomeCompleted, ReviewNoFindings, nil), want: "review is only valid"},
		{name: "implementation missing", kind: ActionImplement, outcome: testOutcome(OutcomeCompleted), want: "requires implementation"},
		{name: "review missing", kind: ActionReview, outcome: testOutcome(OutcomeCompleted), want: "requires review"},
		{name: "implementation on paused implement", kind: ActionImplement, outcome: withImplementation(pausedTestOutcome(PauseDecisionRequired, nil), ImplementationApplied), want: "requires implementation null"},
		{name: "pause on completed", kind: ActionOrient, outcome: withPause(testOutcome(OutcomeCompleted), PauseDecisionRequired, nil), want: "only valid with needs_input"},
		{name: "missing pause", kind: ActionOrient, outcome: testOutcome(OutcomeNeedsInput), want: "requires pause"},
		{name: "budget pause", kind: ActionOrient, outcome: pausedTestOutcome(PauseBudgetExhausted, nil), want: "supported host pause"},
		{name: "destructive pause outside implement", kind: ActionOrient, outcome: pausedTestOutcome(PauseDestructiveConfirm, &destructive), want: "only valid for implement"},
		{name: "destructive pause without request", kind: ActionImplement, outcome: pausedTestOutcome(PauseDestructiveConfirm, nil), want: "requires destructive_request"},
		{name: "request on decision pause", kind: ActionImplement, outcome: pausedTestOutcome(PauseDecisionRequired, &destructive), want: "only valid for destructive"},
		{name: "blank decision supersession", kind: ActionOrient, outcome: withDecisionSupersession(pausedTestOutcome(PauseDecisionRequired, nil), " "), want: "supersedes_answer_action_id is required"},
		{name: "supersession on environment pause", kind: ActionOrient, outcome: withDecisionSupersession(pausedTestOutcome(PauseEnvironmentUnavailable, nil), "prior-action"), want: "only valid for decision_required"},
		{name: "supersession on destructive pause", kind: ActionImplement, outcome: withDecisionSupersession(pausedTestOutcome(PauseDestructiveConfirm, &destructive), "prior-action"), want: "only valid for decision_required"},
		{name: "needs input suggestion", kind: ActionOrient, outcome: withSuggestion(pausedTestOutcome(PauseDecisionRequired, nil), ActionClarify), want: "empty suggested_actions"},
		{name: "implement suggestion", kind: ActionImplement, outcome: withSuggestion(implementedTestOutcome(OutcomeCompleted, ImplementationApplied), ActionSummarize), want: "implement outcomes cannot suggest"},
		{name: "summary suggestion", kind: ActionSummarize, outcome: withSuggestion(testOutcome(OutcomeCompleted), ActionClarify), want: "summarize outcomes cannot suggest"},
		{name: "summary partial", kind: ActionSummarize, outcome: testOutcome(OutcomePartial), want: "summarize does not support"},
		{name: "zero attempts", kind: ActionImplement, outcome: withAttempts(implementedTestOutcome(OutcomeCompleted, ImplementationApplied), 0), want: "attempts must be positive"},
		{name: "file path tab control", kind: ActionImplement, outcome: withFilesChanged(implementedTestOutcome(OutcomeCompleted, ImplementationApplied), "a.go\tb.go"), want: "disallowed control"},
		{name: "file path newline control", kind: ActionImplement, outcome: withFilesChanged(implementedTestOutcome(OutcomeCompleted, ImplementationApplied), "a.go\nb.go"), want: "disallowed control"},
		{name: "file path carriage-return control", kind: ActionImplement, outcome: withFilesChanged(implementedTestOutcome(OutcomeCompleted, ImplementationApplied), "a.go\rb.go"), want: "disallowed control"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			outcome := test.outcome
			outcome.ActionKind = test.kind
			err := outcome.Validate(test.kind, "action-1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestOutcomeValidationRequiresMatchingActionKind(t *testing.T) {
	t.Parallel()

	outcome := testOutcome(OutcomeCompleted)
	outcome.ActionKind = ActionClarify
	err := outcome.Validate(ActionOrient, "action-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match current action kind")

	outcome.ActionKind = ActionKind("unknown")
	err = outcome.Validate(ActionOrient, "action-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid outcome action_kind")
}

func TestOutcomeValidationEnforcesSuggestionsAndText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		outcome Outcome
		want    string
	}{
		{name: "two suggestions", outcome: withSuggestions(testOutcome(OutcomeCompleted), []SuggestedAction{{Kind: ActionClarify, Brief: "first"}, {Kind: ActionImplement, Brief: "second"}}), want: "at most one"},
		{name: "orient suggestion", outcome: withSuggestion(testOutcome(OutcomeCompleted), ActionOrient), want: "invalid kind"},
		{name: "review kind suggestion", outcome: withSuggestion(testOutcome(OutcomeCompleted), ActionReview), want: "invalid kind"},
		{name: "blank suggestion", outcome: withSuggestions(testOutcome(OutcomeCompleted), []SuggestedAction{{Kind: ActionClarify, Brief: " "}}), want: "brief is required"},
		{name: "oversize suggestion", outcome: withSuggestions(testOutcome(OutcomeCompleted), []SuggestedAction{{Kind: ActionClarify, Brief: strings.Repeat("x", maxSuggestedActionBriefBytes+1)}}), want: "brief exceeds"},
		{name: "invalid summary utf8", outcome: withSummary(testOutcome(OutcomeCompleted), string([]byte{0xff})), want: "valid utf-8"},
		{name: "summary nul control", outcome: withSummary(testOutcome(OutcomeCompleted), "facts\x00hidden"), want: "disallowed control"},
		{name: "summary del control", outcome: withSummary(testOutcome(OutcomeCompleted), "facts\u007fhidden"), want: "disallowed control"},
		{name: "summary c1 control", outcome: withSummary(testOutcome(OutcomeCompleted), "facts\u0085hidden"), want: "disallowed control"},
		{name: "nil observations", outcome: withNilObservations(testOutcome(OutcomeCompleted)), want: "observations must be a non-null array"},
		{name: "nil known issues", outcome: withNilKnownIssues(testOutcome(OutcomeCompleted)), want: "known_issues must be a non-null array"},
		{name: "nil suggestions", outcome: withNilSuggestions(testOutcome(OutcomeCompleted)), want: "suggested_actions must be a non-null array"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := test.outcome.Validate(ActionOrient, "action-1")
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestDecodeOutcomeRequiresExactStrictJSON(t *testing.T) {
	t.Parallel()

	valid := `{"contract_version":2,"action_id":"action-1","action_kind":"orient","status":"completed","summary":"facts","observations":[],"known_issues":[],"suggested_actions":[],"pause":null,"implementation":null,"review":null}`
	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{name: "missing action kind", raw: []byte(`{"contract_version":2,"action_id":"action-1","status":"completed","summary":"facts","observations":[],"known_issues":[],"suggested_actions":[],"pause":null,"implementation":null,"review":null}`), want: "action_kind"},
		{name: "missing array", raw: []byte(`{"contract_version":2,"action_id":"action-1","action_kind":"orient","status":"completed","summary":"facts","known_issues":[],"suggested_actions":[],"pause":null,"implementation":null,"review":null}`), want: "observations"},
		{name: "null array", raw: []byte(strings.Replace(valid, `"observations":[]`, `"observations":null`, 1)), want: "array, not null"},
		{name: "wrong array type", raw: []byte(strings.Replace(valid, `"observations":[]`, `"observations":{}`, 1)), want: "cannot unmarshal"},
		{name: "unknown field", raw: []byte(strings.Replace(valid, `"review":null`, `"review":null,"verdict":true`, 1)), want: "unknown field"},
		{name: "duplicate key", raw: []byte(strings.Replace(valid, `"summary":"facts"`, `"summary":"facts","summary":"other"`, 1)), want: "duplicate object key"},
		{name: "nested unknown field", raw: []byte(`{"contract_version":2,"action_id":"action-1","action_kind":"orient","status":"needs_input","summary":"wait","observations":[],"known_issues":[],"suggested_actions":[],"pause":{"reason":"decision_required","question":"choose","destructive_request":null,"extra":true},"implementation":null,"review":null}`), want: "unknown field"},
		{name: "nested duplicate key", raw: []byte(`{"contract_version":2,"action_id":"action-1","action_kind":"orient","status":"needs_input","summary":"wait","observations":[],"known_issues":[],"suggested_actions":[],"pause":{"reason":"decision_required","question":"choose","question":"again","destructive_request":null},"implementation":null,"review":null}`), want: "duplicate object key"},
		{name: "explicit null decision supersession", raw: []byte(`{"contract_version":2,"action_id":"action-1","action_kind":"orient","status":"needs_input","summary":"wait","observations":[],"known_issues":[],"suggested_actions":[],"pause":{"reason":"decision_required","question":"choose","destructive_request":null,"supersedes_answer_action_id":null},"implementation":null,"review":null}`), want: "not null"},
		{name: "nested missing field", raw: []byte(`{"contract_version":2,"action_id":"action-1","action_kind":"implement","status":"completed","summary":"done","observations":[],"known_issues":[],"suggested_actions":[],"pause":null,"implementation":{"result":"applied","files_changed":[],"activities":[],"uncertainties":[]},"review":null}`), want: "attempts"},
		{name: "nested null array", raw: []byte(`{"contract_version":2,"action_id":"action-1","action_kind":"implement","status":"completed","summary":"done","observations":[],"known_issues":[],"suggested_actions":[],"pause":null,"implementation":{"result":"applied","files_changed":null,"activities":[],"uncertainties":[],"attempts":1},"review":null}`), want: "array, not null"},
		{name: "trailing value", raw: []byte(valid + ` {}`), want: "trailing json value"},
		{name: "invalid utf8", raw: append([]byte(`{"contract_version":2,"action_id":"`), append([]byte{0xff}, []byte(`","action_kind":"orient","status":"completed","summary":"facts","observations":[],"known_issues":[],"suggested_actions":[],"pause":null,"implementation":null,"review":null}`)...)...), want: "valid utf-8"},
		{name: "bom", raw: append([]byte{0xef, 0xbb, 0xbf}, []byte(valid)...), want: "bom"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeOutcome(bytes.NewReader(test.raw))
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}

	decoded, err := DecodeOutcome(strings.NewReader(valid))
	require.NoError(t, err)
	assert.Equal(t, OutcomeCompleted, decoded.Status)
	assert.NotNil(t, decoded.Observations)
}

func TestDecodeOutcomeReturnsVersionErrorAfterStrictDecode(t *testing.T) {
	t.Parallel()

	raw := `{"contract_version":1,"action_id":"action-1","action_kind":"orient","status":"completed","summary":"facts","observations":[],"known_issues":[],"suggested_actions":[],"pause":null,"implementation":null,"review":null}`
	_, err := DecodeOutcome(strings.NewReader(raw))
	var versionErr *VersionError
	require.ErrorAs(t, err, &versionErr)
	assert.Equal(t, 1, versionErr.Received)

	_, err = DecodeOutcome(strings.NewReader(strings.Repeat("x", maxOutcomeBytes+1)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payload exceeds")
}

func TestActionValidationEnforcesSourceAuthorizationAndBounds(t *testing.T) {
	t.Parallel()

	source := testActionSource()
	requirements := testActionRequirements()
	authorization := destructiveAuthorizationForTest(t)
	tests := []struct {
		name   string
		mutate func(*Action)
		want   string
	}{
		{name: "source without requirements", mutate: func(action *Action) { action.Source = &source }, want: "both be present"},
		{name: "requirements without source", mutate: func(action *Action) { action.Requirements = &requirements }, want: "both be present"},
		{name: "invalid source digest", mutate: func(action *Action) {
			invalidSource := source
			invalidSource.SourceRevision = "sha256:ABC"
			action.Source, action.Requirements = &invalidSource, &requirements
		}, want: "lowercase"},
		{name: "authorization outside implement", mutate: func(action *Action) { action.DestructiveAuthorization = &authorization }, want: "only valid for implement"},
		{name: "authorization digest mismatch", mutate: func(action *Action) {
			invalidAuthorization := authorization
			invalidAuthorization.ScopeSHA256 = "sha256:" + strings.Repeat("0", 64)
			action.Kind = ActionImplement
			action.DestructiveAuthorization = &invalidAuthorization
		}, want: "does not match"},
		{name: "authorization bad timestamp", mutate: func(action *Action) {
			invalidAuthorization := authorization
			invalidAuthorization.ConfirmedAt = "yesterday"
			action.Kind = ActionImplement
			action.DestructiveAuthorization = &invalidAuthorization
		}, want: "UTC Z notation"},
		{name: "authorization non utc timestamp", mutate: func(action *Action) {
			invalidAuthorization := authorization
			invalidAuthorization.ConfirmedAt = "2026-07-12T11:11:12+01:00"
			action.Kind = ActionImplement
			action.DestructiveAuthorization = &invalidAuthorization
		}, want: "UTC Z notation"},
		{name: "authorization scope version", mutate: func(action *Action) {
			invalidAuthorization := authorization
			invalidAuthorization.ScopeVersion = 2
			action.Kind = ActionImplement
			action.DestructiveAuthorization = &invalidAuthorization
		}, want: "scope_version must be 1"},
		{name: "brief too large", mutate: func(action *Action) { action.Brief = strings.Repeat("b", maxActionBriefBytes+1) }, want: "brief exceeds"},
		{name: "context too large", mutate: func(action *Action) { action.Context = strings.Repeat("c", maxActionContextBytes+1) }, want: "context exceeds"},
		{name: "encoded action too large", mutate: func(action *Action) {
			oversized := testActionRequirements()
			oversized.Sections[0].Title = strings.Repeat("r", maxActionBytes)
			action.Source, action.Requirements = &source, &oversized
		}, want: "encoded action exceeds"},
		{name: "invalid utf8", mutate: func(action *Action) { action.Goal = string([]byte{0xff}) }, want: "valid utf-8"},
		{name: "goal nul control", mutate: func(action *Action) { action.Goal = "goal\x00hidden" }, want: "disallowed control"},
		{name: "brief del control", mutate: func(action *Action) { action.Brief = "brief\u007fhidden" }, want: "disallowed control"},
		{name: "context c1 control", mutate: func(action *Action) { action.Context = "context\u0085hidden" }, want: "disallowed control"},
		{name: "blank identifier", mutate: func(action *Action) { action.ActionID = " " }, want: "action_id is required"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			action := testAction()
			test.mutate(&action)
			err := action.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}

	boundaryAction := testAction()
	boundaryAction.Brief = strings.Repeat("b", maxActionBriefBytes)
	boundaryAction.Context = strings.Repeat("c", maxActionContextBytes)
	require.NoError(t, boundaryAction.Validate())
	boundaryJSON, err := json.Marshal(boundaryAction)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(boundaryJSON), maxActionBytes)

	issueAction := testAction()
	issueAction.Source, issueAction.Requirements = &source, &requirements
	require.NoError(t, issueAction.Validate())
	encoded, err := json.Marshal(issueAction)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"source":`)
	assert.Contains(t, string(encoded), `"requirements":`)

	adHoc, err := json.Marshal(testAction())
	require.NoError(t, err)
	assert.NotContains(t, string(adHoc), `"source"`)
	assert.NotContains(t, string(adHoc), `"requirements"`)
	assert.NotContains(t, string(adHoc), `"destructive_authorization"`)

	scoped := testAction()
	scoped.Kind = ActionImplement
	scoped.DestructiveAuthorization = &authorization
	require.NoError(t, scoped.Validate())
}

func TestDestructiveScopeCanonicalizationRejectsAmbiguousTargets(t *testing.T) {
	t.Parallel()

	targets := []DestructiveTarget{
		{Kind: DestructiveTargetExternalResource, Value: "service/<prod>&\u2028"},
		{Kind: DestructiveTargetPath, Value: "/tmp/\"quoted\""},
	}
	impact := "delete <records>&\npermanently"
	digest, err := ComputeDestructiveScopeSHA256("11111111-1111-4111-8111-111111111111", targets, impact)
	require.NoError(t, err)
	canonical := "{\"impact\":\"delete <records>&\\npermanently\",\"request_id\":\"11111111-1111-4111-8111-111111111111\",\"scope_version\":1,\"targets\":[{\"kind\":\"external_resource\",\"value\":\"service/<prod>&\u2028\"},{\"kind\":\"path\",\"value\":\"/tmp/\\\"quoted\\\"\"}]}"
	want := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(canonical)))
	assert.Equal(t, want, digest)

	for name, requestID := range map[string]string{
		"non uuid":  "request-1",
		"nil uuid":  "00000000-0000-0000-0000-000000000000",
		"non rfc":   "11111111-1111-4111-7111-111111111111",
		"uppercase": "11111111-1111-4111-8111-AAAAAAAAAAAA",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := ComputeDestructiveScopeSHA256(requestID, targets, impact)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "canonical lowercase non-nil RFC UUID")
		})
	}

	tests := []struct {
		name    string
		targets []DestructiveTarget
		want    string
	}{
		{name: "nil", targets: nil, want: "at least one"},
		{name: "empty", targets: []DestructiveTarget{}, want: "at least one"},
		{name: "duplicate", targets: []DestructiveTarget{{Kind: DestructiveTargetPath, Value: "/x"}, {Kind: DestructiveTargetPath, Value: "/x"}}, want: "duplicates"},
		{name: "unsorted kind", targets: []DestructiveTarget{{Kind: DestructiveTargetPath, Value: "/x"}, {Kind: DestructiveTargetGitRef, Value: "main"}}, want: "bytewise sorted"},
		{name: "unsorted value", targets: []DestructiveTarget{{Kind: DestructiveTargetPath, Value: "/z"}, {Kind: DestructiveTargetPath, Value: "/a"}}, want: "bytewise sorted"},
		{name: "unknown kind", targets: []DestructiveTarget{{Kind: DestructiveTargetKind("database"), Value: "prod"}}, want: "unsupported kind"},
		{name: "blank value", targets: []DestructiveTarget{{Kind: DestructiveTargetPath, Value: " "}}, want: "value is required"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := ComputeDestructiveScopeSHA256("11111111-1111-4111-8111-111111111111", test.targets, "delete")
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}

	request := destructiveRequestForTest(t)
	request.ScopeSHA256 = "sha256:" + strings.Repeat("0", 64)
	_, err = NormalizeDestructiveRequest(request)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestDecideUsesIssueSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input TransitionInput
		want  Transition
	}{
		{name: "orient without suggestion summarizes", input: TransitionInput{Kind: ActionOrient, Outcome: testOutcome(OutcomeCompleted)}, want: Transition{Next: ActionSummarize}},
		{name: "clarify without suggestion summarizes", input: TransitionInput{Kind: ActionClarify, Outcome: testOutcome(OutcomeCompleted)}, want: Transition{Next: ActionSummarize}},
		{name: "orient changed reviews", input: TransitionInput{Kind: ActionOrient, Outcome: testOutcome(OutcomeCompleted), CodeChanged: true, ReviewEnabled: true}, want: Transition{Next: ActionReview}},
		{name: "clarify changed reviews", input: TransitionInput{Kind: ActionClarify, Outcome: testOutcome(OutcomeCompleted), CodeChanged: true, ReviewEnabled: true}, want: Transition{Next: ActionReview}},
		{name: "pending suggestion routes first", input: TransitionInput{Kind: ActionOrient, Outcome: testOutcome(OutcomeCompleted), Pending: []SuggestedAction{{Kind: ActionImplement, Brief: "apply"}}}, want: Transition{Next: ActionImplement, Brief: "apply"}},
		{name: "new revision reviews before pending suggestion", input: TransitionInput{Kind: ActionOrient, Outcome: testOutcome(OutcomeCompleted), CodeChanged: true, ReviewEnabled: true, Pending: []SuggestedAction{{Kind: ActionSummarize, Brief: "report"}}}, want: Transition{Next: ActionReview}},
		{name: "applied changed reviews", input: TransitionInput{Kind: ActionImplement, Outcome: implementedTestOutcome(OutcomeCompleted, ImplementationApplied), CodeChanged: true, ReviewEnabled: true}, want: Transition{Next: ActionReview}},
		{name: "not needed changed reviews", input: TransitionInput{Kind: ActionImplement, Outcome: implementedTestOutcome(OutcomeCompleted, ImplementationNotNeeded), CodeChanged: true, ReviewEnabled: true}, want: Transition{Next: ActionReview}},
		{name: "partial changed reviews", input: TransitionInput{Kind: ActionImplement, Outcome: implementedTestOutcome(OutcomePartial, ImplementationPartial), CodeChanged: true, ReviewEnabled: true}, want: Transition{Next: ActionReview}},
		{name: "unable changed reviews", input: TransitionInput{Kind: ActionImplement, Outcome: implementedTestOutcome(OutcomeError, ImplementationUnable), CodeChanged: true, ReviewEnabled: true}, want: Transition{Next: ActionReview}},
		{name: "changed review disabled summarizes", input: TransitionInput{Kind: ActionImplement, Outcome: implementedTestOutcome(OutcomeCompleted, ImplementationApplied), CodeChanged: true}, want: Transition{Next: ActionSummarize}},
		{name: "unchanged summarizes", input: TransitionInput{Kind: ActionImplement, Outcome: implementedTestOutcome(OutcomeCompleted, ImplementationApplied), ReviewEnabled: true}, want: Transition{Next: ActionSummarize}},
		{name: "review always summarizes", input: TransitionInput{Kind: ActionReview, Outcome: reviewedTestOutcome(OutcomeError, ReviewError, nil)}, want: Transition{Next: ActionSummarize}},
		{name: "summary ends", input: TransitionInput{Kind: ActionSummarize, Outcome: testOutcome(OutcomeCompleted)}, want: Transition{End: true}},
		{name: "nested pause reason", input: TransitionInput{Kind: ActionClarify, Outcome: pausedTestOutcome(PauseDecisionRequired, nil)}, want: Transition{PauseReason: PauseDecisionRequired}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.want, Decide(test.input))
		})
	}
}

func TestBudgetIsConsumedOnlyWhenAnActionIsIssued(t *testing.T) {
	t.Parallel()
	require.NoError(t, ValidateBudget(1))
	remaining, err := ConsumeBudget(1)
	require.NoError(t, err)
	assert.Zero(t, remaining)
	_, err = ConsumeBudget(0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exhausted")
	assert.Error(t, ValidateBudget(0))
	assert.Error(t, ValidateBudget(1001))
}

func testAction() Action {
	return Action{
		ContractVersion: ContractVersion,
		RunID:           "run-1",
		ActionID:        "action-1",
		Kind:            ActionOrient,
		Goal:            "ship the change",
		Brief:           "inspect facts",
		Context:         "confirmed context",
		RemainingBudget: 7,
	}
}

func testActionSource() ActionSource {
	return ActionSource{
		Kind:                 ActionSourceChangeIssue,
		CanonicalURL:         "https://github.com/signalridge/slipway/issues/434",
		IssueID:              "I_kwDOExample",
		SourceRevision:       "sha256:" + strings.Repeat("a", 64),
		ManifestRevision:     "sha256:" + strings.Repeat("c", 64),
		RequirementsRevision: "sha256:" + strings.Repeat("b", 64),
	}
}

func testActionRequirements() ActionRequirements {
	keys := []string{"outcome", "requirements", "acceptance-examples", "constraints", "non-goals"}
	roles := []SourceSectionRole{
		SourceSectionOutcome,
		SourceSectionRequirements,
		SourceSectionAcceptanceExamples,
		SourceSectionConstraints,
		SourceSectionNonGoals,
	}
	revisionDigits := []string{"1", "2", "3", "4", "5"}
	materialDigits := []string{"6", "7", "8", "9", "a"}
	sections := make([]ActionRequirementSection, len(keys))
	for index, key := range keys {
		sections[index] = ActionRequirementSection{
			Key:             key,
			Role:            roles[index],
			Title:           key,
			SectionRevision: "sha256:" + strings.Repeat(revisionDigits[index], 64),
			MaterialSHA256:  "sha256:" + strings.Repeat(materialDigits[index], 64),
			Bytes:           32,
		}
	}
	return ActionRequirements{
		RequirementsRevision: "sha256:" + strings.Repeat("b", 64),
		Sections:             sections,
		RequiredForAction:    append([]string(nil), keys...),
		Reader: ActionMaterialReader{
			Operation: "read_material",
			BaseArgv: []string{
				"slipway", "_machine", "material", "--root", "/workspace",
				"--run", "run-1", "--action", "action-1",
			},
			Input: ActionMaterialInput{
				Name:     "section",
				Type:     "enum",
				Flag:     "--section",
				Required: true,
				Choices:  append([]string(nil), keys...),
			},
		},
	}
}

func destructiveRequestForTest(t *testing.T) DestructiveRequest {
	t.Helper()
	targets := []DestructiveTarget{{Kind: DestructiveTargetPath, Value: "/absolute/target"}}
	digest, err := ComputeDestructiveScopeSHA256("11111111-1111-4111-8111-111111111111", targets, "permanent deletion")
	require.NoError(t, err)
	return DestructiveRequest{RequestID: "11111111-1111-4111-8111-111111111111", Targets: targets, Impact: "permanent deletion", ScopeSHA256: digest}
}

func destructiveAuthorizationForTest(t *testing.T) DestructiveAuthorization {
	t.Helper()
	request := destructiveRequestForTest(t)
	return DestructiveAuthorization{
		RequestID:           request.RequestID,
		OriginatingActionID: "action-origin",
		ScopeVersion:        DestructiveScopeVersion,
		ScopeSHA256:         request.ScopeSHA256,
		Targets:             append([]DestructiveTarget(nil), request.Targets...),
		Impact:              request.Impact,
		ConfirmedAt:         "2026-07-12T10:11:12Z",
	}
}

func testOutcome(status OutcomeStatus) Outcome {
	return Outcome{
		ContractVersion:  ContractVersion,
		ActionID:         "action-1",
		ActionKind:       ActionOrient,
		Status:           status,
		Summary:          "observed facts",
		Observations:     []string{},
		KnownIssues:      []string{},
		SuggestedActions: []SuggestedAction{},
	}
}

func outcomeWithStatus(status OutcomeStatus) Outcome {
	return testOutcome(status)
}

func pausedTestOutcome(reason PauseReason, request *DestructiveRequest) Outcome {
	outcome := testOutcome(OutcomeNeedsInput)
	outcome.Pause = &Pause{Reason: reason, Question: "What should happen?", DestructiveRequest: request}
	return outcome
}

func implementedTestOutcome(status OutcomeStatus, result ImplementationResult) Outcome {
	outcome := testOutcome(status)
	outcome.Implementation = &Implementation{
		Result:        result,
		FilesChanged:  []string{},
		Activities:    []Activity{},
		Uncertainties: []string{},
		Attempts:      1,
	}
	return outcome
}

func reviewedTestOutcome(status OutcomeStatus, result ReviewResult, findings []Finding) Outcome {
	outcome := testOutcome(status)
	if findings == nil {
		findings = []Finding{}
	}
	outcome.Review = &Review{Result: result, Findings: findings, Uncertainties: []string{}}
	return outcome
}

func withSuggestion(outcome Outcome, kind ActionKind) Outcome {
	return withSuggestions(outcome, []SuggestedAction{{Kind: kind, Brief: "next bounded action"}})
}

func withSuggestions(outcome Outcome, suggestions []SuggestedAction) Outcome {
	outcome.SuggestedActions = suggestions
	return outcome
}

func withImplementation(outcome Outcome, result ImplementationResult) Outcome {
	outcome.Implementation = implementedTestOutcome(OutcomeCompleted, result).Implementation
	return outcome
}

func withAttempts(outcome Outcome, attempts int) Outcome {
	outcome.Implementation.Attempts = attempts
	return outcome
}

func withFilesChanged(outcome Outcome, paths ...string) Outcome {
	outcome.Implementation.FilesChanged = paths
	return outcome
}

func withPause(outcome Outcome, reason PauseReason, request *DestructiveRequest) Outcome {
	outcome.Pause = &Pause{Reason: reason, Question: "question", DestructiveRequest: request}
	return outcome
}

func withDecisionSupersession(outcome Outcome, actionID string) Outcome {
	outcome.Pause.SupersedesAnswerActionID = &actionID
	return outcome
}

func withSummary(outcome Outcome, summary string) Outcome {
	outcome.Summary = summary
	return outcome
}

func withNilObservations(outcome Outcome) Outcome {
	outcome.Observations = nil
	return outcome
}

func withNilKnownIssues(outcome Outcome) Outcome {
	outcome.KnownIssues = nil
	return outcome
}

func withNilSuggestions(outcome Outcome) Outcome {
	outcome.SuggestedActions = nil
	return outcome
}

func marshalTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return encoded
}
