package tmpl

import (
	"io/fs"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func templateFS(t *testing.T) fs.FS {
	t.Helper()
	sub, err := fs.Sub(embeddedTemplates, "templates")
	require.NoError(t, err)
	return sub
}

func TestEmbeddedSurfaceContainsOnlySixCapabilitiesAndOneReference(t *testing.T) {
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
	}, files)
}

func TestCapabilityPromptsEncodeAcceptanceBehavior(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path     string
		contains []string
	}{
		{path: "skills/run/SKILL.md", contains: []string{"explicitly asks", "marker-valid Run", "structured `next.variants`", "ordered chapter catalog", "`grill-me` design-tree discipline", "Finish the branch you opened", "never promote chat prose to decision authority", "confirmed human decisions from Clarify answers", "shared understanding", "Strict Outcome shape", "`action_kind` is mandatory", "`skipped` is emitted only by the CLI", "Review never needs input", "Every waiting Action may be skipped", "another non-ended Run pinned to the same Issue identity", "overrides and discards that pending suggestion", "later revision after a prior Review", "automatic Action queue is empty", "`gh >= 2.94.0`", "official REST fallback", "redirects/transfers only within `github.com`", "accepted chapter materials", "Redact recognized credentials", "source_unavailable", "mutation envelope", "non-null `action`", "canonical safe grammar", "nodes(ids:...)", "“skip this”", "“take over”", "“reorder” or “do X first”"}},
		{path: "skills/clarify/SKILL.md", contains: []string{"explicitly invokes", "design-tree discipline", "exactly one decision", "recommended option", "ask zero questions", "shared understanding", "structured `answer-decision`", "wrap up", "Do not implement", "stateless", "`grill-with-docs`"}},
		{path: "skills/propose/SKILL.md", contains: []string{"explicitly asks", "self-contained", "exactly one `level:change`", "exactly one `level:objective`", "exactly one of `kind:feature|kind:bug|kind:refactor|kind:maintenance|kind:research|kind:docs`", "marker remains authority", "exactly three choices", "`gh >= 2.94.0`", "official GitHub REST API", "100 sub-issues", "50 blocking", "same-host redirect or transfer", "Objective publication: one confirmed external write", "single-stage publication", "one current external-write confirmation", "Change publication: two confirmed phases", "operation UUID", "stable item UUID", "second current confirmation", "Unreferenced comments remain drafts", "timeout-after-success", "indexing delay", "`created`, `matched`, `failed`, or `ambiguous`", "Zero matches", "public repository has no per-Issue private switch", "Redact recognized credential values", "environment_unavailable"}},
		{path: "skills/decompose/SKILL.md", contains: []string{"explicitly asks", "tracer-bullet Changes", "runtime inheritance", "exactly one `level:objective`", "exactly one `level:change`", "exactly 100 children", "exactly 50 blocking dependencies", "exactly 50 blocked-by dependencies", "exceed one of those limits", "`gh >= 2.94.0`", "official REST API", "cross-host redirects", "two confirmed phases", "one operation UUID", "stable item UUID", "second current commit confirmation", "Read back the complete graph", "duplicate marker matches", "Zero marker matches", "public Issue has no private switch", "amendment mode", "Never propagate in the background", "`closed` status does not prove"}},
		{path: "skills/implement/SKILL.md", contains: []string{"explicitly invokes", "pinned Requirements", "`action_kind: \"implement\"`", "activities` (which may be empty)", "actual positive attempt count", "exact command", "exit code", "Never list an activity that did not run", "shell exit 127", "scope SHA-256", "directly outside a Run"}},
		{path: "skills/review/SKILL.md", contains: []string{"explicitly invokes", "always read-only", "Intent", "Quality", "start-to-current difference is only an observation", "`action_kind: \"review\"`", "findings_reported", "leave `suggested_actions` empty", "Do not modify files", "automatic repair or re-review loop"}},
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

func TestSourceBundleReferenceIncludedOnlyWhereConstructed(t *testing.T) {
	t.Parallel()
	essentials := []string{
		`"source_version": 2`,
		`"provider": "github"`,
		`"host": "github.com"`,
		`"repository_id": "REPOSITORY_GRAPHQL_NODE_ID"`,
		`"issue_id": "ISSUE_GRAPHQL_NODE_ID"`,
		`"issue_number": 123`,
		`"canonical_url": "https://github.com/OWNER/REPO/issues/123"`,
		`"updated_at": "RFC3339_TIMESTAMP"`,
		`"fetched_at": "RFC3339_TIMESTAMP"`,
		`"title": "EXACT_ISSUE_TITLE"`,
		`"body": "EXACT_ISSUE_BODY"`,
		`"labels": []`,
		`"comments": [`,
		`"node_id": "COMMENT_GRAPHQL_NODE_ID"`,
		`"database_id": 456`,
		`"author_id": "AUTHOR_GRAPHQL_NODE_ID"`,
		`"is_minimized": false`,
		`"manifest_version": 2`,
		`"profile": "change/v2"`,
		`"parent_requirements_revision": "sha256:64_LOWERCASE_HEX_DIGITS"`,
		`"comment_node_id": "COMMENT_GRAPHQL_NODE_ID"`,
		`"comment_database_id": 456`,
		`"body_sha256": "sha256:64_LOWERCASE_HEX_DIGITS"`,
		"framedRevision(fields...)",
		`"slipway-comment-body/v1"`,
		`"slipway-material/v1"`,
		`"slipway-section/v2"`,
		`"slipway-manifest/v2"`,
		`"slipway-requirements/v2"`,
		`"slipway-source/v2"`,
		`"slipway-source-observation/v2"`,
		"Fetch the Issue identity and exact body before any comment request",
		"labels(first:100){totalCount nodes{name}}",
		"more than 100 observed labels",
		"more than 64 declared comments",
		"nodes(ids:$ids)",
		"... on IssueComment{id databaseId url updatedAt isMinimized body author{login ... on Node{id}} issue{id number url repository{id nameWithOwner}}}",
		"never enumerate Issue comments or ordinary discussion",
		"cross-Issue/repository result",
		"snake_case fields",
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
	} {
		test := test
		t.Run(test.path, func(t *testing.T) {
			t.Parallel()
			content, err := Content(test.path)
			require.NoError(t, err)
			assert.NotContains(t, content, `{{ template "source-bundle" . }}`)
			for _, fragment := range essentials {
				if test.include {
					assert.Contains(t, content, fragment)
				} else {
					assert.NotContains(t, content, fragment)
				}
			}
		})
	}
}

func TestSharedCapabilityBoundariesEncodeTrustAndUserControl(t *testing.T) {
	t.Parallel()
	content, err := Content("_partials/common.tmpl")
	require.NoError(t, err)
	for _, fragment := range []string{
		"untrusted data",
		"exact first body marker is Level authority",
		"trusted attester",
		"Never invent a local Issue",
		"accepted Requirements, user answers, goals, and truthful command summaries may contain sensitive text",
		"public-repository Issue has no private switch",
		"Redact recognized credential values",
		"preserving truthful command identity",
		"tokens, unreferenced discussion comments, environment dumps, full transcripts, or hidden reasoning",
		"transiently fetch only the exact raw Issue body and manifest-referenced comment fields",
		"pass that raw envelope only to the CLI for consumption",
		"persist only parser-accepted normalized materials",
		"exact draft and operation plan",
		"same approved publication plan and receipt",
		"receipt records reconciliation facts and is never Requirements or work authority",
		"Never blindly retry",
		"Natural-language approval alone is not a grant",
	} {
		assert.Contains(t, content, fragment)
	}
}

func TestPublicationLimitsCanonicalRunAndHostControlAreStaticTemplateContracts(t *testing.T) {
	t.Parallel()

	propose, err := Content("skills/propose/SKILL.md")
	require.NoError(t, err)
	objectiveStart := strings.Index(propose, "## Objective publication: one confirmed external write")
	changeStart := strings.Index(propose, "## Change publication: two confirmed phases")
	require.GreaterOrEqual(t, objectiveStart, 0)
	require.Greater(t, changeStart, objectiveStart)
	objective := propose[objectiveStart:changeStart]
	assert.Contains(t, objective, "single-stage publication")
	assert.Contains(t, objective, "one current external-write confirmation")
	assert.Contains(t, objective, "exact title, complete body, exact labels, every relation")
	assert.Contains(t, objective, "gh issue create --body-file FILE")
	assert.Contains(t, objective, "creates no chapter comments or manifest")
	assert.Contains(t, objective, "no second commit confirmation")
	assert.Contains(t, propose, "<!-- slipway-level: objective/v1 -->\n<!-- slipway-publication-operation: UUID -->\n<!-- slipway-publication-item: UUID -->")
	assert.Contains(t, propose, "A new Change draft body contains only these receipt markers and no `change/v2` level marker")
	assert.Contains(t, propose, "<!-- slipway-level: change/v2 -->\n```slipway-manifest\n{...}\n```\n<!-- slipway-publication-operation: UUID -->\n<!-- slipway-publication-item: UUID -->")
	assert.Contains(t, propose[changeStart:], "two confirmed phases")
	assert.Contains(t, propose[changeStart:], "second current confirmation")

	decompose, err := Content("skills/decompose/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, decompose, "may reach exactly 100 children")
	assert.Contains(t, decompose, "exactly 50 blocking dependencies")
	assert.Contains(t, decompose, "exactly 50 blocked-by dependencies")
	assert.Contains(t, decompose, "treat blocking and blocked-by as independent directions")
	assert.Contains(t, decompose, "only when the approved write would exceed")
	assert.Contains(t, decompose, "one operation UUID")
	assert.Contains(t, decompose, "stable item UUID")

	run, err := Content("skills/run/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, run, "slipway run --budget N --json --root ABSOLUTE_ROOT [--no-review] [--source-file FILE] -- GOAL")
	assert.Contains(t, run, "public Cobra command may accept equivalent legal flag placement")
	assert.Contains(t, run, "“skip this” means invoke the exact current structured `skip-action` variant")
	assert.Contains(t, run, "“take over” means first invoke public `slipway stop`, preserve and report the Run ID")
	assert.Contains(t, run, "“reorder” or “do X first” means stop the public Run and hand control back")
	assert.Contains(t, run, "They add no CLI command, state, queue mutation, or gate")
	// These deterministic C assertions prove generated text only. Actual host
	// compliance with natural-language control remains H evidence.
}

func TestGeneratedReviewSkillAvoidsRatingVocabulary(t *testing.T) {
	t.Parallel()
	// Issue #434 §9.4: the protocol has no verdict/approved/gate fields and
	// Review only reports findings. The review skill must therefore avoid
	// rating vocabulary. This is a content-specific structural check on the
	// review skill, not a repo-wide vocabulary ban: ordinary explanatory use
	// of words like "gate" or "lifecycle" elsewhere must not fail CI.
	review, err := Content("skills/review/SKILL.md")
	require.NoError(t, err)
	for _, rating := range []string{" pass ", " fail ", " approved ", " ready ", " verdict "} {
		assert.NotContains(t, " "+strings.ToLower(review)+" ", rating)
	}
}

func TestDecisionInterviewReferenceRetainsMITAttribution(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/clarify/references/decision-interview.md")
	require.NoError(t, err)
	assert.Contains(t, content, "mattpocock/skills")
	assert.Contains(t, content, "MIT License")
	assert.Contains(t, content, "Copyright (c) 2026 Matt Pocock")
	assert.Contains(t, content, "permission notice")
}
