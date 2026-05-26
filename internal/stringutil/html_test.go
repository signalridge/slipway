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

func TestHasBlockingOpenQuestions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "empty section",
			content: `## Open Questions
<!-- placeholder -->
`,
			want: false,
		},
		{
			name: "explicit none",
			content: `## Open Questions
(none)
`,
			want: false,
		},
		{
			name: "resolved checklist",
			content: `## Open Questions
- [x] Resolved by local reference survey.
`,
			want: false,
		},
		{
			name: "unchecked checklist",
			content: `## Open Questions
- [ ] Which installer path should be documented?
`,
			want: true,
		},
		{
			name: "plain bullet question",
			content: `## Open Questions
- Which docs build command should be used?
`,
			want: true,
		},
		{
			name: "plain prose question",
			content: `## Open Questions
Need to decide whether OpenCode commands are flat or nested.
`,
			want: true,
		},
		{
			name: "last canonical section wins",
			content: `## Summary
Copied source.

## Open Questions
- Copied unresolved question.

## Open Questions
(none)
`,
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, HasBlockingOpenQuestions(tt.content))
		})
	}
}
