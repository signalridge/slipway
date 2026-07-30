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

func TestCapabilityFrontmatterRequiresExplicitHumanInvocation(t *testing.T) {
	t.Parallel()
	for _, capability := range []string{"run", "clarify", "propose", "decompose", "implement", "review", "workflow"} {
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

func TestCapabilityPromptsEncodeAcceptanceBehavior(t *testing.T) {
	t.Parallel()
	workflowRouteFragments := []string{
		"| Rough idea, clarified conversation, or non-authoritative planning artifact | Investigate, settle decisions, choose one Change or Objective, and produce its complete draft | `slipway-propose` |",
		"| Explicit standalone decision-only clarification request, with no draft or materialization desired | Explain the stateless decision-summary boundary without starting the interview here | `slipway-clarify` |",
		"| Structurally valid Objective awaiting initial child Changes | Explain its planning role and the need for self-contained child Changes | `slipway-decompose` |",
		"| Explicit Objective amendment that may affect open child Changes | Explain that propagation is a user-invoked amendment convenience, not inheritance or background synchronization; require per-child diffs and current publication confirmation | `slipway-decompose` in amendment mode |",
		"| Structurally valid, self-contained `change/v2` Issue with no selected Run | Confirm the source route and state the effective budget | `slipway-run` |",
		"| Explicit private, tiny, urgent, offline, or deliberately untracked bounded goal | State the sharpened bounded goal and the already-selected no-Issue source route | `slipway-run` for a new ad-hoc Run |",
		"| Active issue-backed Run whose user requests a Change amendment | Explain that chat prose cannot revise pinned Requirements; first stop through public `slipway stop`, report the pinned revision, then wait for explicit resume and its refresh/use-pinned/candidate choice | `slipway-run` |",
		"| Active Run | Report the exact current Action and its submit/skip variants; state that stop uses public `slipway stop` and take-over/reorder first stop and hand control back | `slipway-run` |",
		"| Paused or stopped Run | Report the exact Run and its structured recovery choice without changing it | `slipway-run` |",
		"| Failed, partial, or ambiguous Propose/Decompose publication | Preserve and return every available receipt, operation, item, and revision fact; report the exact unresolved state | The originating `slipway-propose` or `slipway-decompose` owner decides same-receipt reconciliation or a contract-required fresh preview and confirmation |",
		"| Ended Run or advisory Review findings | Explain that ended is terminal and no completion was certified; offer no further action, new tracked scope, or a provenance-correct new Run attempt without making findings a gate | No capability when the user stops; otherwise `slipway-propose` or `slipway-run` after that one genuine choice |",
		"| Explicit standalone implementation or review request | Explain that it has no Run attribution or pinned source | `slipway-implement` or `slipway-review`, only when the user deliberately wants the standalone path |",
	}
	workflowFragments := []string{"explicitly asks to run the Slipway workflow", "stateless only in the Slipway sense", "workflow itself is read-only", "Orchestrate Slipway functions, not installed skills", "Do not discover, rank, or dispatch", "Do not invoke a sibling `slipway-*` capability", "self-contained and must work when no Matt Pocock skill is installed", "only optional external primitive", "model-invocable `/grilling`", "Choose the shortest valid route"}
	workflowFragments = append(workflowFragments, workflowRouteFragments...)
	workflowFragments = append(workflowFragments, "external or unmanaged artifact as non-authoritative planning input", "retains the routing and source authority defined by the contract", "Standalone Clarify is not a mandatory stage", "Only submit and skip are Action variants", "An ended Run is terminal", "No further Slipway action is a valid terminal outcome", "fresh-fetch and attest the canonical Change", "same receipt", "one genuine decision at a time", "initial explicit workflow invocation already authorized drafting", "not a durable wayfinding state machine", "`kind:research` Change", "measurable internal outcome", "For an Objective, instead produce its distinct planning shape", "non-authoritative planning input", "not an approved publication plan", "advisory unblocked frontier", "structurally valid, self-contained `change/v2` Issue", "never hand a bare Issue number to the CLI", "For a new Run", "contract default of `8`", "For resume, preserve the distinct remaining-budget rules below", "no workflow-owned governance gate", "`paused`, `stopped`, or `ended`", "automatic repair or re-review loop", "`budget_exhausted` pause is normal", "`max(initial_budget, 3)`", "Certify nothing", "material in-scope activities")
	tests := []struct {
		path     string
		contains []string
	}{
		{path: "skills/run/SKILL.md", contains: []string{"explicitly asks", "marker-valid Run", "structured `next.variants`", "ordered chapter catalog", "`grill-me` design-tree discipline", "Finish the branch you opened", "never promote chat prose to decision authority", "confirmed human decisions from Clarify answers", "shared understanding", "Strict Outcome shape", "`action_kind` is mandatory", "`skipped` is emitted only by the CLI", "Review never needs input", "Every waiting Action may be skipped", "another non-ended Run pinned to the same Issue identity", "skippable, read-only advisory Review", "later revision after a prior Review", "automatic Action queue is empty", "`gh >= 2.94.0`", "official REST fallback", "redirects/transfers only within `github.com`", "trusted host attests the GitHub fetch identity and visibility observations", "does not contact GitHub and cannot independently revalidate remote visibility", "accepted chapter materials", "Redact recognized credentials", "source_unavailable", "mutation envelope", "non-null `action`", "canonical safe grammar", "nodes(ids:...)", "“skip this”", "“take over”", "“reorder” or “do X first”"}},
		{path: "skills/clarify/SKILL.md", contains: []string{"explicitly invokes", "design-tree discipline", "filesystem", "CLI state", "permitted external facts", "Finish the branch you opened", "exactly one decision", "recommended option", "ask zero questions", "explicit confirmation", "structured `answer-decision`", "wrap up", "Do not implement", "stateless", "superseded by later answers", "`grill-with-docs`"}},
		{path: "skills/propose/SKILL.md", contains: []string{"explicitly asks", "self-contained", "exactly one `level:change`", "exactly one `level:objective`", "exactly one of `kind:feature|kind:bug|kind:refactor|kind:maintenance|kind:research|kind:docs`", "marker remains authority", "exactly three choices", "`gh >= 2.94.0`", "official GitHub REST API", "100 sub-issues", "50 blocking", "same-host redirect or transfer", "API stays on `api.github.com`", "every resulting `html_url` stays on `github.com`", "Objective publication: one confirmed external write", "single-stage publication", "one current external-write confirmation", "Change publication: one confirmed operation", "operation UUID", "stable item UUID", "non-authoritative reconciliation resource", "provider-assigned IDs are reconciliation facts", "must not trigger a second confirmation", "Unreferenced comments remain drafts", "timeout-after-success", "indexing delay", "`created`, `matched`, `failed`, or `ambiguous`", "Zero matches", "public repository has no per-Issue private switch", "Redact recognized credential values", "environment_unavailable"}},
		{path: "skills/decompose/SKILL.md", contains: []string{"explicitly asks", "tracer-bullet Changes", "runtime inheritance", "missing or conflicting labels never block decomposition", "exactly one `level:change`", "exactly 100 children", "exactly 50 blocking dependencies", "exactly 50 blocked-by dependencies", "exceed one of those limits", "repartition the Objective", "`gh >= 2.94.0`", "official REST API", "cross-host redirects", "one confirmed operation", "one operation UUID", "stable item UUID", "provider-assigned IDs are reconciliation facts", "must not trigger another confirmation", "Read back the complete graph", "duplicate marker matches", "Zero marker matches", "public Issue has no private switch", "amendment mode", "one amendment publication plan", "one current confirmation for that exact plan", "Never propagate in the background", "`closed` status does not prove"}},
		{path: "skills/implement/SKILL.md", contains: []string{"explicitly invokes", "pinned Requirements", "`action_kind: \"implement\"`", "activities` (which may be empty)", "actual positive attempt count", "exact command", "exit code", "Never list an activity that did not run", "shell exit 127", "scope SHA-256", "directly outside a Run", "standalone invocation cannot obtain", "must not execute the destructive operation", "must not silently start a Run"}},
		{path: "skills/review/SKILL.md", contains: []string{"explicitly invokes", "always read-only", "Intent", "Quality", "start-to-current difference is only an observation", "`action_kind: \"review\"`", "findings_reported", "leave `suggested_actions` empty", "Do not modify files", "automatic repair or re-review loop"}},
		{path: "skills/workflow/SKILL.md", contains: workflowFragments},
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
		{path: "skills/workflow/SKILL.md", include: false},
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
	assert.Contains(t, objective, "gh issue create --body-file FILE")
	assert.Contains(t, objective, "creates no chapter comments or manifest")
	assert.Contains(t, objective, "no second commit confirmation")
	assert.Contains(t, propose, "<!-- slipway-level: objective/v1 -->\n<!-- slipway-publication-operation: UUID -->\n<!-- slipway-publication-item: UUID -->")
	assert.Contains(t, propose, "A new Change may temporarily use a receipt-only body")
	assert.Contains(t, propose, "not a Change source or a new lifecycle state")
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
	assert.Contains(t, decompose, "one operation UUID")
	assert.Contains(t, decompose, "stable item UUID")
	assert.Contains(t, decompose, "one current external-write confirmation")
	assert.Contains(t, decompose, "must not trigger another confirmation")
	assert.NotContains(t, decompose, "second current commit confirmation")
	assert.NotContains(t, decompose, "approval before each planned PATCH")

	run, err := Content("skills/run/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, run, "slipway run --budget N --json --root ABSOLUTE_ROOT [--no-review] --goal-file GOAL_FILE [--source-file SOURCE_FILE]")
	assert.Contains(t, run, "Write the exact goal to a private temporary regular non-symlink file")
	assert.Contains(t, run, "required `goal_file` path input")
	assert.Contains(t, run, "host-generated/machine canonical form")
	assert.Contains(t, run, "required `text_file` path")
	assert.Contains(t, run, "resolve it through `--text-file`")
	assert.Contains(t, run, "“skip this” means invoke the exact current structured `skip-action` variant")
	assert.Contains(t, run, "“take over” means first invoke public `slipway stop`, preserve and report the Run ID")
	assert.Contains(t, run, "“reorder” or “do X first” means stop the public Run and hand control back")
	assert.Contains(t, run, "They add no CLI command, state, queue mutation, or gate")
	// These deterministic C assertions prove generated text only. Actual host
	// compliance with natural-language control remains H evidence.
}

func TestDecisionInterviewReferenceRetainsMITAttribution(t *testing.T) {
	t.Parallel()
	content, err := Content("skills/clarify/references/decision-interview.md")
	require.NoError(t, err)
	assert.Contains(t, content, "mattpocock/skills")
	assert.Contains(t, content, "finish an opened decision branch")
	assert.Contains(t, content, "all available environment facts")
	assert.Contains(t, content, "user explicitly confirms the shared understanding")
	assert.Contains(t, content, "MIT License")
	assert.Contains(t, content, "Copyright (c) 2026 Matt Pocock")
	assert.Contains(t, content, "permission notice")
}
