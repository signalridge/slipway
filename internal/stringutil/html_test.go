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
			name: "explicit none bullet",
			content: `## Open Questions
- None.
`,
			want: false,
		},
		{
			name: "explicit no open questions bullet",
			content: `## Open Questions
* No open questions.
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
			name: "unchecked none checklist remains blocking",
			content: `## Open Questions
- [ ] None.
`,
			want: true,
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
Need to decide which adapter layout should be documented.
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
		{
			name: "resolved checklist wraps across lines",
			content: `## Open Questions
- [x] Part C mechanism — RESOLVED: Option A keeps the change to SKILL.md prose
  only, anchored on the existing task_kind=test->code + depends_on model, so
  no parser or hash change is needed.
`,
			want: false,
		},
		{
			name: "uppercase RESOLVED prose is documentation",
			content: `## Open Questions
All intake questions RESOLVED during research; see research.md.
`,
			want: false,
		},
		{
			name: "lowercase resolved prose is documentation",
			content: `## Open Questions
Domain classification resolved: verification.
`,
			want: false,
		},
		{
			name: "unresolved prose still blocks",
			content: `## Open Questions
Adapter layout is still unresolved.
`,
			want: true,
		},
		{
			name: "nested unchecked checklist still blocks",
			content: `## Open Questions
- [x] Parent question — RESOLVED.
  - [ ] Nested follow-up still open.
`,
			want: true,
		},
		{
			name: "resolved checklist continuation with question mark does not block",
			content: `## Open Questions
- [x] Which host emits the hint — RESOLVED: wave-orchestration only. Why not
  tdd-governance? Because it is ExportOnlyExtra and never a next-skill.
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
