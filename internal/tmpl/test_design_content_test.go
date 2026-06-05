package tmpl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestDesignTemplatesStayLanguageNeutral(t *testing.T) {
	t.Parallel()

	templates := []string{
		"skills/test-design/SKILL.md",
		"skills/test-design/references/test-doubles.md",
		"skills/test-design/references/behavior-vs-implementation.md",
		"skills/test-design/references/case-enumeration.md",
		"skills/test-design/references/property-reasoning.md",
		"skills/test-design/references/test-data.md",
	}
	bannedTokens := []string{
		"t.Run",
		"#[test]",
		"pytest",
		"def test_",
		"describe(",
		"it(",
		"expect(",
		"@Test",
	}

	for _, name := range templates {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			content, err := Content(name)
			require.NoError(t, err, "test-design template %s must exist", name)

			for _, token := range bannedTokens {
				assert.NotContains(t, content, token, "%s must stay language-neutral", name)
			}
		})
	}
}

func TestTestDesignReferencesTeachCoreJudgmentTopics(t *testing.T) {
	t.Parallel()

	referencePaths := []string{
		"skills/test-design/references/test-doubles.md",
		"skills/test-design/references/behavior-vs-implementation.md",
		"skills/test-design/references/case-enumeration.md",
		"skills/test-design/references/property-reasoning.md",
		"skills/test-design/references/test-data.md",
	}
	var corpus strings.Builder
	for _, name := range referencePaths {
		content, err := Content(name)
		require.NoError(t, err, "test-design reference %s must exist", name)
		corpus.WriteString("\n")
		corpus.WriteString(strings.ToLower(content))
	}
	text := corpus.String()

	assert.Contains(t, text, "test double")
	assert.Contains(t, text, "test level")
	assert.Contains(t, text, "fake")
	assert.Contains(t, text, "mock")
	assert.Contains(t, text, "behavior")
	assert.Contains(t, text, "implementation")
	assert.Contains(t, text, "equivalence")
	assert.Contains(t, text, "boundary")
	assert.Contains(t, text, "decision table")
	assert.Contains(t, text, "impossible")
	assert.Contains(t, text, "uncovered")
	assert.Contains(t, text, "oracle")
	assert.Contains(t, text, "property")
	assert.Contains(t, text, "invariant")
	assert.Contains(t, text, "generator")
	assert.Contains(t, text, "stronger properties")
	assert.Contains(t, text, "stateful")
	assert.Contains(t, text, "command sequences")
	assert.Contains(t, text, "interchangeable")
	assert.Contains(t, text, "design signal")
	assert.Contains(t, text, "fixture")
	assert.Contains(t, text, "masked")
	assert.Contains(t, text, "secrets")
	assert.Contains(t, text, "shared mutable")
}
