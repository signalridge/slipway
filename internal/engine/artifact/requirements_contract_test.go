package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluateRequirementsContractReturnsValidResult(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "example-change"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(`# Requirements

### Requirement: Auth
REQ-001: The system MUST authenticate requests before serving protected routes.

#### Scenario: Unauthenticated request is rejected
GIVEN a request without valid credentials
WHEN it reaches a protected route
THEN the system returns 401 and does not serve the resource.
`), 0o644))

	result, err := EvaluateRequirementsContract(bundleDir, slug)
	require.NoError(t, err)
	assert.Equal(t, RequirementsContractStatusValid, result.Status)
	assert.Equal(t, ResolveArtifactPath(bundleDir, slug, "requirements.md"), result.Source)
	assert.Contains(t, result.Message, "validated")
}

func TestEvaluateRequirementsContractReturnsMissingResult(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	slug := "example-change"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	result, err := EvaluateRequirementsContract(bundleDir, slug)
	require.NoError(t, err)
	assert.Equal(t, RequirementsContractStatusMissing, result.Status)
	assert.Equal(t, ResolveArtifactPath(bundleDir, slug, "requirements.md"), result.Source)
	assert.Contains(t, result.Message, "missing")
}

func TestEvaluateRequirementsContractReturnsInvalidResults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		wantMessage string
	}{
		{
			name: "no requirement blocks",
			content: `# Requirements

This file has no requirement blocks.
`,
			wantMessage: "no Requirement blocks found",
		},
		{
			name: "missing stable ids",
			content: `# Requirements

### Requirement: Auth
The system must authenticate requests.
`,
			wantMessage: "missing stable REQ-* IDs",
		},
		{
			name: "body without RFC-2119 keyword",
			content: `# Requirements

### Requirement: Auth
REQ-001: The system authenticates requests.

#### Scenario: Login
GIVEN a user
WHEN they log in
THEN they are authenticated.
`,
			wantMessage: "no RFC-2119 MUST/SHALL/REQUIRED keyword",
		},
		{
			name: "no concrete scenario",
			content: `# Requirements

### Requirement: Auth
REQ-001: The system MUST authenticate requests.
`,
			wantMessage: "no concrete #### Scenario",
		},
		{
			name: "MUST only in scenario line not statement",
			content: `# Requirements

### Requirement: Auth
REQ-001: The system authenticates protected routes.

#### Scenario: Login succeeds
GIVEN the caller MUST present credentials
WHEN it reaches the route
THEN the route returns protected content.
`,
			wantMessage: "no RFC-2119 MUST/SHALL/REQUIRED keyword",
		},
		{
			name: "mechanical placeholder scaffold",
			content: `# Requirements

### Requirement: do the thing
REQ-001: Pending — replace with the normative requirement. Define requirements based on the initial request.

#### Scenario: Pending — replace with a concrete scenario
GIVEN pending — replace with the precondition
WHEN pending — replace with the triggering action
THEN pending — replace with the observable expected outcome
`,
			wantMessage: "placeholder content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			slug := "example-change"
			bundleDir := filepath.Join(root, "artifacts", "changes", slug)
			require.NoError(t, os.MkdirAll(bundleDir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "requirements.md"), []byte(tt.content), 0o644))

			result, err := EvaluateRequirementsContract(bundleDir, slug)
			require.NoError(t, err)
			assert.Equal(t, RequirementsContractStatusInvalid, result.Status)
			assert.Equal(t, ResolveArtifactPath(bundleDir, slug, "requirements.md"), result.Source)
			assert.Contains(t, result.Message, tt.wantMessage)
		})
	}
}

func TestLooksLikeTemplatePlaceholderRecognizesRequirementsTautologies(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		"GIVEN the relevant workflow is exercised",
		"THEN the expected behavior for foo is observed",
		"WHEN the requirement is implemented in the target flow",
		"Define requirements based on the initial request: foo.",
		"Pending — replace with the normative requirement.",
	} {
		assert.True(t, LooksLikeTemplatePlaceholder(s), "should detect placeholder: %q", s)
	}
	for _, s := range []string{
		"The system MUST authenticate requests before serving protected routes.",
		// Issue #91 false-positive guard: a concrete THEN line that shares the
		// "the expected behavior for" fragment but is not the vacuous "… is
		// observed" tautology must NOT be flagged as a placeholder.
		"THEN the expected behavior for an expired token is a 401 response.",
		"THEN the expected behavior for a malformed payload is a validation error.",
		// Issue #91 P2a: an authored line whose middle contains "is observed" but
		// continues past the legacy tautology (anchored to the whole line) is real
		// substance and must NOT be flagged.
		"THEN the expected behavior for an audit-log write is observed in the audit log.",
		// The GIVEN/WHEN legacy lines are likewise whole-line anchored: an authored
		// line that merely starts with the legacy fragment but continues must pass.
		"GIVEN the relevant workflow is exercised by an admin during off-hours",
		"WHEN the requirement is implemented in the target flow for the billing service",
	} {
		assert.False(t, LooksLikeTemplatePlaceholder(s), "should NOT flag concrete prose: %q", s)
	}
}

func TestRequirementSubstanceBlockers(t *testing.T) {
	t.Parallel()

	authored := `# Requirements
### Requirement: Auth
REQ-001: The system MUST authenticate requests before serving protected routes.

#### Scenario: Rejects unauthenticated request
GIVEN a request without credentials
WHEN it reaches a protected route
THEN the system returns 401.
`
	assert.Empty(t, RequirementSubstanceBlockers(authored), "authored requirements should have no substance blockers")

	// Issue #91: REQUIRED is accepted as an equivalent strong-obligation keyword,
	// so a requirement phrased with "is REQUIRED to …" is not hard-blocked.
	requiredKeyword := `# Requirements
### Requirement: Auth
REQ-001: The system is REQUIRED to authenticate requests before serving protected routes.

#### Scenario: Rejects unauthenticated request
GIVEN a request without credentials
WHEN it reaches a protected route
THEN the system returns 401.
`
	assert.Empty(t, RequirementSubstanceBlockers(requiredKeyword),
		"a REQUIRED-keyword requirement should satisfy the RFC-2119 substance check")

	// Issue #91 false-positive guard: a concrete scenario whose THEN line shares
	// the "the expected behavior for" fragment is real substance, not a placeholder.
	concreteExpectedBehavior := `# Requirements
### Requirement: Auth
REQ-001: The system MUST reject requests that carry an expired token.

#### Scenario: Expired token is rejected
GIVEN a request carrying an expired token
WHEN it reaches a protected route
THEN the expected behavior for an expired token is a 401 response.
`
	assert.Empty(t, RequirementSubstanceBlockers(concreteExpectedBehavior),
		"a concrete 'expected behavior for' scenario must not be treated as placeholder")

	mechanical := `# Requirements
### Requirement: do the thing
REQ-001: Pending — replace with the normative requirement. Define requirements based on the initial request.

#### Scenario: Pending — replace with a concrete scenario
GIVEN pending — replace with the precondition
WHEN pending — replace with the triggering action
THEN pending — replace with the observable expected outcome
`
	assert.NotEmpty(t, RequirementSubstanceBlockers(mechanical), "mechanical scaffold must be rejected")
}

// Blocker #3 regression (issue #91): the RFC-2119 substance check must apply to
// the requirement statement, not to scenario GIVEN/WHEN/THEN lines.
func TestRequirementSubstanceBlockersScopesRFC2119ToStatement(t *testing.T) {
	t.Parallel()

	// MUST appears only in a scenario line; the requirement statement has no
	// normative keyword → must be blocked.
	scenarioOnly := `# Requirements
### Requirement: Auth
REQ-001: The system authenticates protected routes.

#### Scenario: Login succeeds
GIVEN the caller MUST present credentials
WHEN it reaches the route
THEN the route returns protected content.
`
	blockers := RequirementSubstanceBlockers(scenarioOnly)
	require.NotEmpty(t, blockers, "MUST in a scenario line must not satisfy the statement gate")
	assert.Contains(t, strings.Join(blockers, "; "), "no RFC-2119 MUST/SHALL/REQUIRED keyword")

	// MUST on a continuation line of a multi-line statement (before any scenario)
	// must still pass — guards against narrowing the check to the REQ-* line only.
	multiLineStatement := `# Requirements
### Requirement: Engine yields an honest scaffold
REQ-001: The engine default seed for requirements.md and tasks.md
MUST NOT fabricate plausible normative requirements or tautology scenarios.

#### Scenario: Default seed is detectably non-substantive
GIVEN a change with no source document
WHEN the engine seeds the artifacts
THEN the seeded body reads as an honest placeholder.
`
	assert.Empty(t, RequirementSubstanceBlockers(multiLineStatement),
		"multi-line statement with MUST before the scenario must pass")
}

func TestEvaluateRequirementsContractReturnsErrorForUnreadableFile(t *testing.T) {
	root := t.TempDir()
	slug := "example-change"
	bundleDir := filepath.Join(root, "artifacts", "changes", slug)
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))

	reqPath := filepath.Join(bundleDir, "requirements.md")
	require.NoError(t, os.Mkdir(reqPath, 0o755))
	t.Cleanup(func() {
		_ = os.RemoveAll(reqPath)
	})

	_, err := EvaluateRequirementsContract(bundleDir, slug)
	require.Error(t, err)
}

// Issue #91 (false-positive guard): the requirements substance gate uses the
// requirements-specific LooksLikeRequirementsPlaceholder, NOT the broad
// LooksLikeTemplatePlaceholder. A legitimately-authored requirement that
// contains a generic sentinel substring (e.g. a real case status "pending
// investigation", or "replace with concrete") must NOT be rejected.
func TestRequirementSubstanceBlockersDoesNotFlagGenericSentinelProse(t *testing.T) {
	t.Parallel()

	// "pending investigation" is a generic decision/research sentinel in
	// LooksLikeTemplatePlaceholder; here it is legitimate domain terminology.
	pendingInvestigation := `# Requirements
### Requirement: Case retention
REQ-001: The system MUST retain cases whose status is "pending investigation" until manually closed.

#### Scenario: A pending-investigation case is retained
GIVEN a case in "pending investigation" status
WHEN the nightly cleanup runs
THEN the case is retained, not purged.
`
	assert.Empty(t, RequirementSubstanceBlockers(pendingInvestigation),
		"a real requirement containing the generic phrase 'pending investigation' must not be flagged")

	// "replace with concrete" is another generic sentinel; legitimate here.
	replaceWithConcrete := `# Requirements
### Requirement: Config migration
REQ-001: The migrator MUST replace with concrete values every placeholder it finds in the config.

#### Scenario: Placeholders are replaced
GIVEN a config containing placeholder tokens
WHEN the migrator runs
THEN every token is replaced with a concrete value.
`
	assert.Empty(t, RequirementSubstanceBlockers(replaceWithConcrete),
		"a real requirement containing the generic phrase 'replace with concrete' must not be flagged")
}

// Issue #91 (false-positive guard, parity with tasks_contract_test): an authored
// requirements.md that RETAINS the seeded authoring-guidance HTML comment must
// still pass — the comment's prose must not trip the substance gate.
func TestRequirementSubstanceBlockersAllowsRetainedGuidanceComment(t *testing.T) {
	t.Parallel()

	withComment := `# Requirements

## Requirements

<!--
Authoring guidance — the engine owns structure, the authoring skill owns substance:
- Each requirement is "### Requirement: <title>" + a stable "REQ-" identifier line
  whose body states what the system MUST, SHALL, or is REQUIRED to do.
- Replace the seeded placeholder below; an unedited scaffold is rejected by the
  requirements substance gate and cannot reach done.
-->

### Requirement: Auth
REQ-001: The system MUST authenticate requests before serving protected routes.

#### Scenario: Unauthenticated request is rejected
GIVEN a request without valid credentials
WHEN it reaches a protected route
THEN the system returns 401.
`
	assert.Empty(t, RequirementSubstanceBlockers(withComment),
		"authored requirements retaining the guidance comment must pass")
}

// Issue #91 (F5b): GIVEN/WHEN/THEN must all appear within a SINGLE "#### Scenario"
// segment. A requirement whose keywords are scattered across two separate
// scenarios (each individually incomplete) has no concrete scenario.
func TestRequirementSubstanceBlockersRequiresCompleteSingleScenario(t *testing.T) {
	t.Parallel()

	splitScenarios := `# Requirements
### Requirement: Auth
REQ-001: The system MUST authenticate requests before serving protected routes.

#### Scenario: Only given and when
GIVEN a request without credentials
WHEN it reaches a protected route

#### Scenario: Only then
THEN the system returns 401.
`
	blockers := RequirementSubstanceBlockers(splitScenarios)
	require.NotEmpty(t, blockers, "GIVEN/WHEN/THEN split across separate scenarios must not satisfy the gate")
	assert.Contains(t, strings.Join(blockers, "; "), "no concrete #### Scenario")

	// A single complete scenario alongside an incomplete one still passes.
	oneComplete := `# Requirements
### Requirement: Auth
REQ-001: The system MUST authenticate requests before serving protected routes.

#### Scenario: Incomplete
GIVEN a request without credentials

#### Scenario: Complete
GIVEN a request without credentials
WHEN it reaches a protected route
THEN the system returns 401.
`
	assert.Empty(t, RequirementSubstanceBlockers(oneComplete),
		"at least one complete scenario must satisfy the gate")
}
