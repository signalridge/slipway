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
			name: "resolved checklist",
			content: `## Open Questions
- [x] Resolved by local reference survey.
`,
			want: false,
		},
		{
			name: "unchecked checklist blocks",
			content: `## Open Questions
- [ ] Which installer path should be documented?
`,
			want: true,
		},
		{
			name: "unchecked star checklist blocks",
			content: `## Open Questions
* [ ] Which installer path should be documented?
`,
			want: true,
		},
		{
			name: "unchecked entry blocks even when its text says none",
			content: `## Open Questions
- [ ] None.
`,
			want: true,
		},
		{
			// Contract: only checklist items count. A bare bullet is documentation;
			// the intake-clarification skill must promote a real question to `- [ ]`.
			name: "plain bullet is documentation, not a blocker",
			content: `## Open Questions
- Which docs build command should be used?
`,
			want: false,
		},
		{
			name: "plain prose is documentation, not a blocker",
			content: `## Open Questions
Need to decide which adapter layout should be documented.
`,
			want: false,
		},
		{
			// Regression for #104: a sentinel followed by an explanatory clause must
			// not read as an open question. Prose never blocks under the checklist
			// contract, so this advances instead of detouring to research.
			name: "sentinel with explanatory prose does not block (#104)",
			content: `## Open Questions
None requiring research — the page model is already specified.
`,
			want: false,
		},
		{
			name: "n/a with explanatory prose does not block (#104)",
			content: `## Open Questions
N/A — page model already defined.
`,
			want: false,
		},
		{
			// A genuinely-open question written as prose does NOT block the engine;
			// surfacing it as a `- [ ]` is the intake-clarification skill's job. The
			// engine deliberately gates on structure only.
			name: "real open question in prose is not an engine blocker",
			content: `## Open Questions
None of the auth flows are specified yet — need to decide token TTL.
`,
			want: false,
		},
		{
			name: "mixed list blocks on the unchecked item",
			content: `## Open Questions
- [x] Installer path resolved by research.
- [ ] Token TTL still undecided.
`,
			want: true,
		},
		{
			name: "last canonical section wins",
			content: `## Summary
Copied source.

## Open Questions
- [ ] Copied unresolved question.

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
			name: "nested unchecked checklist still blocks",
			content: `## Open Questions
- [x] Parent question — RESOLVED.
  - [ ] Nested follow-up still open.
`,
			want: true,
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

func TestFirstBlockingOpenQuestion(t *testing.T) {
	t.Parallel()

	t.Run("returns the first unchecked entry", func(t *testing.T) {
		t.Parallel()
		content := `## Open Questions
- [x] Installer path resolved.
- [ ] Token TTL still undecided.
- [ ] Second open item.
`
		assert.Equal(t, "- [ ] Token TTL still undecided.", FirstBlockingOpenQuestion(content))
	})

	t.Run("empty when nothing blocks", func(t *testing.T) {
		t.Parallel()
		content := `## Open Questions
- [x] All resolved.
None requiring research.
`
		assert.Equal(t, "", FirstBlockingOpenQuestion(content))
	})

	t.Run("empty section", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "", FirstBlockingOpenQuestion("## Open Questions\n<!-- placeholder -->\n"))
	})
}
