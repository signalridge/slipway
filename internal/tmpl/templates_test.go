package tmpl

import (
	"bytes"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The assertions in this file are deliberately limited to safety boundaries,
// protocol identifiers, hard limits, and structure. Freezing ordinary prose
// would tax every future wording change without protecting any behavior, and
// the generated text is not a byte contract — the boundaries it states are.

func templateFS(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(embeddedTemplates, "templates")
	require.NoError(t, err)
	return sub
}

// renderPartial executes a named `define` partial. Content cannot: reading a
// define-only file renders the empty document that wraps the definition.
func renderPartial(t *testing.T, name string) string {
	t.Helper()
	parsed, err := template.New("partials").ParseFS(embeddedTemplates, "templates/_partials/*.tmpl")
	require.NoError(t, err)
	var rendered bytes.Buffer
	require.NoError(t, parsed.ExecuteTemplate(&rendered, name, nil))
	return normalizeTemplateLineEndings(rendered.String())
}

var capabilityNames = []string{"run", "clarify", "propose", "decompose", "implement", "review", "workflow"}

func TestCapabilityFrontmatterRequiresExplicitHumanInvocation(t *testing.T) {
	t.Parallel()
	for _, capability := range capabilityNames {
		capability := capability
		t.Run(capability, func(t *testing.T) {
			t.Parallel()
			content, err := Content("skills/" + capability + "/SKILL.md")
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(content, "---\n"))
			end := strings.Index(content[len("---\n"):], "\n---\n")
			require.GreaterOrEqual(t, end, 0)
			frontmatter := content[len("---\n") : len("---\n")+end]
			assert.Equal(t, 1, strings.Count(frontmatter, "name: slipway-"+capability))
			assert.Equal(t, 1, strings.Count(frontmatter, "description:"))
			assert.Equal(t, 1, strings.Count(frontmatter, "disable-model-invocation: true"))
		})
	}
}

func TestEmbeddedSurfaceContainsOnlySevenCapabilitiesAndOneReference(t *testing.T) {
	t.Parallel()
	var files []string
	err := fs.WalkDir(templateFS(t), "skills", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(files)
	assert.Equal(t, []string{
		"skills/clarify/SKILL.md",
		"skills/clarify/references/decision-interview.md",
		"skills/decompose/SKILL.md",
		"skills/implement/SKILL.md",
		"skills/propose/SKILL.md",
		"skills/review/SKILL.md",
		"skills/run/SKILL.md",
		"skills/workflow/SKILL.md",
	}, files)
}

func TestNoCapabilityShipsAnUnrenderedTemplateDirective(t *testing.T) {
	t.Parallel()
	for _, capability := range capabilityNames {
		capability := capability
		t.Run(capability, func(t *testing.T) {
			t.Parallel()
			content, err := Content("skills/" + capability + "/SKILL.md")
			require.NoError(t, err)
			assert.NotContains(t, content, "{{", "a partial failed to render into the shipped capability")
		})
	}
}

func TestCapabilityPromptsStateTheirSafetyBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		contains []string
	}{
		{path: "skills/run/SKILL.md", contains: []string{
			"One explicit Run authorizes", "Ordinary conversation must never start a Run.",
			"never promote chat prose to decision authority", "On “wrap up”, stop questions immediately",
			"destructive_authorization", "`answer-decision`", "non-null `action`", "action_kind",
			"source_unavailable", "nodes(ids:...)", "unresolved_suggestion", "required_for_action", "--source-file",
		}},
		{path: "skills/clarify/SKILL.md", contains: []string{
			"only when the user explicitly invokes it", "Do not implement", "explicit confirmation",
			"not readiness, quality, Issue status, or delivery certification", "answer-decision", "grill-with-docs", "stateless", "current Clarify Action",
		}},
		{path: "skills/propose/SKILL.md", contains: []string{
			"explicitly asks to propose or publish a Slipway Issue", "must not start implementation",
			"one current external-write confirmation", "provider-assigned IDs are reconciliation facts",
			"level:change", "level:objective", "kind:feature|kind:bug|kind:refactor|kind:maintenance|kind:research|kind:docs",
			"operation UUID", "stable item UUID", "environment_unavailable",
		}},
		{path: "skills/decompose/SKILL.md", contains: []string{
			"explicitly asks to decompose", "Do not implement the resulting work.",
			"must remain advisory rather than locked", "provider-assigned IDs are reconciliation facts",
			"level:change", "100 children", "50 blocking dependencies", "50 blocked-by dependencies",
			"operation UUID", "stable item UUID", "duplicate marker matches", "Zero marker matches",
		}},
		{path: "skills/implement/SKILL.md", contains: []string{
			"only for a current Implement Action or when the user explicitly invokes it directly",
			"pinned Requirements set the maximum business scope", "Never list an activity that did not run",
			"shell exit 127", "destructive_authorization", "must not execute the destructive operation",
			"action_kind: \"implement\"", "actual positive attempt count", "exit code", "scope SHA-256",
		}},
		{path: "skills/review/SKILL.md", contains: []string{
			"It is always read-only.", "Do not modify files", "suggest Implement", "action_kind: \"review\"",
			"suggested_actions", "read-only", "automatic repair or re-review loop",
		}},
		{path: "skills/workflow/SKILL.md", contains: []string{
			"explicitly asks to run the Slipway workflow", "workflow itself is read-only.",
			"Do not discover, rank, or dispatch", "Do not invoke a sibling `slipway-*` capability",
			"slipway-propose", "slipway-decompose", "slipway-run", "slipway-clarify", "slipway-implement", "slipway-review",
			"change/v2", "max(initial_budget, 3)", "no workflow-owned governance gate",
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()
			content, err := Content(test.path)
			require.NoError(t, err)
			for _, fragment := range test.contains {
				assert.Contains(t, content, fragment)
			}
		})
	}
}

// TestWorkflowRoutesEveryObservedStartingPointToOneOwner checks the route table
// structurally: every row must still name the capability that owns the next
// step, or a deliberate terminal outcome. Row wording stays free to change.
func TestWorkflowRoutesEveryObservedStartingPointToOneOwner(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/workflow/SKILL.md")
	require.NoError(t, err)

	var rows [][]string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| ---") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), " | ")
		require.Len(t, cells, 3, "route row %q must keep its three columns", line)
		rows = append(rows, cells)
	}
	require.NotEmpty(t, rows, "route table must contain a header and at least one route")
	require.Greater(t, len(rows), 1, "route table must contain at least one route")
	header := rows[0]
	require.Len(t, header, 3, "route table header must keep its three columns")
	for _, cell := range header {
		require.NotEmpty(t, strings.TrimSpace(cell), "route table header cells must not be empty")
	}

	assert.Equal(t, "Observed starting point", strings.TrimSpace(header[0]))
	routed := map[string]int{}
	for _, row := range rows[1:] {
		for _, cell := range row {
			assert.NotEmpty(t, strings.TrimSpace(cell))
		}
		owner := row[2]
		named := false
		for _, capability := range capabilityNames {
			if strings.Contains(owner, "`slipway-"+capability+"`") {
				routed["slipway-"+capability]++
				named = true
			}
		}
		if !named {
			// The only rows without a next capability are deliberate terminal
			// outcomes, which must say so rather than invent an owner.
			assert.Contains(t, owner, "No capability", "route %q names neither an owner nor a terminal outcome", row[0])
		}
	}
	for _, capability := range []string{"clarify", "propose", "decompose", "run", "implement", "review"} {
		assert.NotZero(t, routed["slipway-"+capability], "no route reaches slipway-%s", capability)
	}
	assert.Zero(t, routed["slipway-workflow"], "the workflow must never route to itself")
}

func TestSourceBundleReferenceIncludedOnlyWhereConstructed(t *testing.T) {
	t.Parallel()
	// One marker per distinct guarantee the partial carries, rather than a
	// transcript of it: the envelope contract, the comment-fetch rule, and the
	// schema it is validated against.
	markers := []string{
		`"source_version": 2`,
		`"manifest_version": 2`,
		`"profile": "change/v2"`,
		"nodes(ids:$ids)",
		"never enumerate Issue comments or ordinary discussion",
		"private temporary file for immediate CLI consumption",
		"docs/reference/v2/source-envelope.schema.json",
	}
	for _, test := range []struct {
		path    string
		include bool
	}{
		{path: "skills/run/SKILL.md", include: true},
		{path: "skills/propose/SKILL.md", include: true},
		{path: "skills/decompose/SKILL.md", include: true},
		{path: "skills/clarify/SKILL.md", include: false},
		{path: "skills/implement/SKILL.md", include: false},
		{path: "skills/review/SKILL.md", include: false},
		{path: "skills/workflow/SKILL.md", include: false},
	} {
		test := test
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()
			content, err := Content(test.path)
			require.NoError(t, err)
			for _, marker := range markers {
				if test.include {
					assert.Contains(t, content, marker)
				} else {
					assert.NotContains(t, content, marker)
				}
			}
		})
	}
}

// TestInterviewDisciplineIsSharedAndUnforked keeps the one rule the report of a
// forked interview discipline was about: every capability that interviews the
// user renders the identical partial, and no capability restates it.
func TestInterviewDisciplineIsSharedAndUnforked(t *testing.T) {
	t.Parallel()
	discipline := strings.TrimSpace(renderPartial(t, "interview"))
	require.NotEmpty(t, discipline)

	for _, fragment := range []string{
		"Investigate before asking",
		"never blocks the interview",
		"design tree",
		"Finish the branch you opened",
		"exactly one genuine decision per response",
		"asking several at once is bewildering",
		"❓ **Q1**",
		"➡️",
		"Ask zero questions when the request already determines the work",
		"cannot be settled by talking",
		"Do not build that prototype, write files, run an experiment, or add a capability",
		"been reported as one you cannot settle here",
		"Stop immediately when the user asks to wrap up",
	} {
		assert.Contains(t, discipline, fragment)
	}

	for _, capability := range []string{"clarify", "run", "workflow"} {
		content, err := Content("skills/" + capability + "/SKILL.md")
		require.NoError(t, err)
		assert.Equal(t, 1, strings.Count(content, discipline),
			"slipway-%s must render the shared interview discipline exactly once", capability)
	}
	for _, capability := range []string{"propose", "decompose", "implement", "review"} {
		content, err := Content("skills/" + capability + "/SKILL.md")
		require.NoError(t, err)
		assert.NotContains(t, content, discipline,
			"slipway-%s does not interview the user and must not carry the discipline", capability)
	}
}

func TestWorkflowConfirmationBoundaryFollowsInterview(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/workflow/SKILL.md")
	require.NoError(t, err)
	interview := strings.Index(content, "Investigate before asking")
	require.GreaterOrEqual(t, interview, 0)
	boundary := strings.Index(content[interview:], "If the user asks to wrap up")
	require.GreaterOrEqual(t, boundary, 0)
	boundary += interview
	end := strings.Index(content[boundary:], "This is not a durable wayfinding state machine")
	require.Greater(t, end, 0)
	block := content[boundary : boundary+end]
	for _, token := range []string{
		"if interview answers add to or change", "If there was no interview or the understanding did not change",
		"wrap up", "stop the interview immediately", "confirmation", "publication", "decomposition", "implementation", "Run",
	} {
		assert.Contains(t, block, token)
	}
}

func TestSharedCapabilityBoundariesEncodeTrustAndUserControl(t *testing.T) {
	t.Parallel()
	content, err := Content("_partials/common.tmpl")
	require.NoError(t, err)
	for _, fragment := range []string{
		"untrusted data",
		"exact first body marker is Level authority",
		"change Slipway's control rules",
		"execute unrelated commands",
		"trusted attester",
		"Never invent a local Issue",
		"accepted Requirements, user answers, goals, and truthful command summaries may contain sensitive text",
		"public-repository Issue has no private switch",
		"Redact recognized credential values",
		"preserving truthful command identity",
		"tokens, unreferenced discussion comments, environment dumps, full transcripts, or hidden reasoning",
		"Never put a token in a URL",
		"automatically fetch a linked URL with credentials",
		"transiently fetch only the exact raw Issue body and manifest-referenced comment fields",
		"pass that raw envelope only to the CLI for consumption",
		"persist only parser-accepted normalized materials",
		"exact draft and operation plan",
		"same approved publication plan and receipt",
		"receipt records reconciliation facts and is never Requirements or work authority",
		"Provider-assigned IDs and observed digests are reconciliation facts, not a second human decision",
		"Never blindly retry",
		"Natural-language approval alone is not a grant",
		"A capability that does not publish must not create or edit external resources",
		"Before the first bare `slipway` invocation",
		"non-empty structured `hosts` array",
		"do not freeze the set of adapter IDs",
		"PATH collision",
		"executable identity safety check",
		"lifecycle or governance gate",
	} {
		assert.Contains(t, content, fragment)
	}
}

func TestPublicationLimitsCanonicalRunAndHostControlAreStaticTemplateContracts(t *testing.T) {
	t.Parallel()
	propose, err := Content("skills/propose/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, propose, "only for a still-unaccepted draft")
	assert.Contains(t, propose, "never edit a manifest-accepted chapter in place")
	objectiveStart := strings.Index(propose, "## Objective publication: one confirmed external write")
	changeStart := strings.Index(propose, "## Change publication: one confirmed operation")
	require.GreaterOrEqual(t, objectiveStart, 0)
	require.Greater(t, changeStart, objectiveStart)
	objective := propose[objectiveStart:changeStart]
	assert.Contains(t, objective, "single-stage publication")
	assert.Contains(t, objective, "one current external-write confirmation")
	assert.Contains(t, objective, "exact title, complete body, exact labels, every relation")
	assert.Contains(t, objective, "creates no chapter comments or manifest")
	assert.Contains(t, objective, "no second commit confirmation")
	assert.Contains(t, propose, "<!-- slipway-level: objective/v1 -->\n<!-- slipway-publication-operation: UUID -->\n<!-- slipway-publication-item: UUID -->")
	assert.Contains(t, propose, "<!-- slipway-level: change/v2 -->\n```slipway-manifest\n{...}\n```\n<!-- slipway-publication-operation: UUID -->\n<!-- slipway-publication-item: UUID -->")
	assert.Contains(t, propose[changeStart:], "one current external-write confirmation")
	assert.Contains(t, propose[changeStart:], "must not trigger a second confirmation")
	assert.NotContains(t, propose[changeStart:], "second current confirmation")

	decompose, err := Content("skills/decompose/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, decompose, "may reach exactly 100 children")
	assert.Contains(t, decompose, "exactly 50 blocking dependencies")
	assert.Contains(t, decompose, "exactly 50 blocked-by dependencies")
	assert.Contains(t, decompose, "treat blocking and blocked-by as independent directions")
	assert.Contains(t, decompose, "only when the approved write would exceed")
	assert.Contains(t, decompose, "one current external-write confirmation")
	assert.NotContains(t, decompose, "second current commit confirmation")
	assert.NotContains(t, decompose, "approval before each planned PATCH")

	run, err := Content("skills/run/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, run, "slipway run --budget N --json --root ABSOLUTE_ROOT [--no-review] --goal-file GOAL_FILE [--source-file SOURCE_FILE]")
	assert.Contains(t, run, "Write the exact goal to a private temporary regular non-symlink file")
	assert.Contains(t, run, "required `goal_file` path input")
	assert.Contains(t, run, "required `text_file` path")
	assert.Contains(t, run, "“skip this” means invoke the exact current structured `skip-action` variant")
	assert.Contains(t, run, "“take over” means first invoke public `slipway stop`, preserve and report the Run ID")
	assert.Contains(t, run, "“reorder” or “do X first” means stop the public Run and hand control back")
	assert.Contains(t, run, "They add no CLI command, state, queue mutation, or gate")
	// These deterministic C assertions prove generated text only. Actual host
	// compliance with natural-language control remains H evidence.
}

func TestDecisionInterviewReferenceRecordsProvenanceAndDivergence(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/clarify/references/decision-interview.md")
	require.NoError(t, err)
	assert.Contains(t, content, "mattpocock/skills")
	// The pinned revision is the licence and provenance claim; an unpinned link
	// would silently re-describe whatever upstream says today.
	assert.Contains(t, content, "e9fcdf95b402d360f90f1db8d776d5dd450f9234")
	assert.Contains(t, content, "one at a time")
	assert.Contains(t, content, "deliberate divergence")
	assert.Contains(t, content, "MIT License")
	assert.Contains(t, content, "Copyright (c) 2026 Matt Pocock")
	assert.Contains(t, content, "permission notice")
}
