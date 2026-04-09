package intake

import (
	"strings"
	"testing"

	"github.com/signalridge/slipway/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDocExtractsSummaryAndSections(t *testing.T) {
	doc := `# Session timeout

## In Scope
- expire idle sessions after 15 minutes

## Out of Scope
- redesign the login screen

## Constraints
- keep the existing middleware contract

## Acceptance Criteria
- verify idle sessions expire after 15 minutes
`

	parsed := ParseDoc(doc)
	assert.Equal(t, "Session timeout", parsed.Summary)
	assert.Contains(t, parsed.Scope, "15 minutes")
	assert.Contains(t, parsed.OutOfScope, "login screen")
	assert.Contains(t, parsed.Constraints, "middleware contract")
	assert.Contains(t, parsed.Acceptance, "verify idle sessions")
}

func TestParseDocSupportsColonSuffixedHeadingsAndIgnoresInlineHeadingMentions(t *testing.T) {
	doc := `# Session timeout

This draft mentions ## In Scope in prose before the actual section.

## In Scope:
- expire idle sessions after 15 minutes

## Constraints:
- keep existing middleware contract
`

	parsed := ParseDoc(doc)
	assert.Contains(t, parsed.Scope, "15 minutes")
	assert.Contains(t, parsed.Constraints, "middleware contract")
	assert.NotContains(t, parsed.Scope, "mentions ## In Scope in prose")
}

func TestSeedIntentContentPopulatesEmptySectionsAndAppendsSourceDocument(t *testing.T) {
	intent := `# Intent

## Summary
Describe the change objective.

## In Scope
<!-- What is explicitly included -->

## Out of Scope
<!-- What is explicitly excluded -->

## Constraints
<!-- Technical / business / time constraints -->

## Acceptance Signals
<!-- What verifiable signals indicate completion -->
`

	docContent := `# Session timeout

## In Scope
- expire idle sessions after 15 minutes
`
	updated := SeedIntentContent(intent, docContent, DocSeed{
		Scope:       "- expire idle sessions after 15 minutes",
		OutOfScope:  "- redesign the login screen",
		Constraints: "- keep the existing middleware contract",
		Acceptance:  "- verify idle sessions expire after 15 minutes",
	})

	assert.Contains(t, updated, "## In Scope\n- expire idle sessions after 15 minutes")
	assert.Contains(t, updated, "## Out of Scope\n- redesign the login screen")
	assert.Contains(t, updated, "## Constraints\n- keep the existing middleware contract")
	assert.Contains(t, updated, "## Acceptance Signals\n- verify idle sessions expire after 15 minutes")
	assert.Contains(t, updated, "### Source Document")
	assert.Contains(t, updated, "# Session timeout")
}

func TestSeedIntentContentDoesNotOverwriteNonEmptySections(t *testing.T) {
	intent := `# Intent

## Summary
Describe the change objective.

## In Scope
- keep the existing in-scope text

## Constraints
<!-- Technical / business / time constraints -->
`

	updated := SeedIntentContent(intent, "# Source", DocSeed{
		Scope:       "- new scope that should not replace existing content",
		Constraints: "- keep the existing middleware contract",
	})

	assert.Contains(t, updated, "## In Scope\n- keep the existing in-scope text")
	assert.NotContains(t, updated, "new scope that should not replace")
	assert.Contains(t, updated, "## Constraints\n- keep the existing middleware contract")
}

func TestSeedIntentContentTruncatesLongSourceDocument(t *testing.T) {
	intent := `# Intent

## Summary
Describe the change objective.
`

	docContent := strings.Repeat("a", maxSourceDocumentLength+200)
	updated := SeedIntentContent(intent, docContent, DocSeed{})

	assert.Contains(t, updated, "<!-- truncated: original document was longer -->")
	require.Contains(t, updated, "### Source Document")
}

func TestBuildInteractivePromptPayloadUsesStableOrdering(t *testing.T) {
	payload := BuildInteractivePromptPayload("/tmp/repo", model.ProjectContext{
		TechStack:  "Go",
		Languages:  []string{"Go", "YAML"},
		RecentWork: "abc123 add command\nfff222 tighten tests",
	})

	require.Equal(t, "Project context (auto-detected):", payload.Header)
	require.Equal(t, "What change do you want to make? ", payload.Question)
	require.Len(t, payload.Lines, 5)
	assert.Equal(t, "  Tech Stack: Go", payload.Lines[0])
	assert.Equal(t, "  Languages:  Go, YAML", payload.Lines[1])
	assert.Equal(t, "  Recent work:", payload.Lines[2])
	assert.Equal(t, "    abc123 add command", payload.Lines[3])
	assert.Equal(t, "    fff222 tighten tests", payload.Lines[4])
}
