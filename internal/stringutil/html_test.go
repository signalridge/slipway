package stringutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripHTMLComments(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "alpha  omega", StripHTMLComments("alpha <!-- hidden --> omega"))
	assert.Equal(t, "alpha <!-- hidden", StripHTMLComments("alpha <!-- hidden"))
	assert.Equal(t, "alpha  omega  done", StripHTMLComments("alpha <!-- one --> omega <!-- two --> done"))
}

func TestLastMarkdownSectionContentUsesLastMatchingSection(t *testing.T) {
	t.Parallel()

	content := `## Summary
Copied source content.

## Acceptance Signals
Source document says verification exists.

## Acceptance Signals
Canonical acceptance criteria.
`

	assert.Equal(t, "Canonical acceptance criteria.", LastMarkdownSectionContent(content, "## Acceptance Signals"))
}

func TestLastMarkdownSectionContentStripsHTMLComments(t *testing.T) {
	t.Parallel()

	content := `## In Scope
<!-- placeholder -->
- keep the canonical section content
`

	assert.Equal(t, "- keep the canonical section content", LastMarkdownSectionContent(content, "## In Scope"))
}
