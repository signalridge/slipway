# Issue workflow (non-normative)

> **Non-normative localized guide.** The [Chinese product contract](../../zh/reference/product-contract.md) and [machine schema](../../reference/machine-protocol.schema.json) are authoritative.

Use an Objective only when one outcome necessarily needs multiple independently deliverable Changes. A Change is the only issue-backed Run source. Every Change must contain all effective execution requirements; parent bodies and comments are not runtime inheritance. Tiny, sensitive, emergency, offline, or deliberately untracked work can use `slipway run --budget N --json --root ABSOLUTE_ROOT -- GOAL`.

## Markers, labels, and bodies

An Objective body begins with exactly this layout:

```html
<!-- slipway-level: objective/v1 -->
<!-- slipway-publication-operation: UUID -->
<!-- slipway-publication-item: UUID -->
```

Its title starts `[Objective]`; labels are exactly `level:objective` plus one `kind:*`. Its exact H2 sections are Problem, Outcome, Requirements, Shared constraints, Non-goals, and Changes.

A final Change body begins with the level marker and strict manifest fence, then retains the publication receipt markers after the fence:

````markdown
<!-- slipway-level: change/v2 -->

```slipway-manifest
{
  "manifest_version": 2,
  "profile": "change/v2",
  "parent_requirements_revision": "sha256:...",
  "sections": [
    {
      "key": "outcome",
      "role": "outcome",
      "title": "Outcome",
      "comment_node_id": "IC_...",
      "comment_database_id": 123,
      "body_sha256": "sha256:..."
    }
  ]
}
```
<!-- slipway-publication-operation: UUID -->
<!-- slipway-publication-item: UUID -->
````

A new Change draft has no `change/v2` marker or manifest: its body contains only the operation and item receipt markers. The final title starts `[Change]`; labels are exactly `level:change` plus one `kind:*`, with optional `ready-for-agent`.

Each manifest entry identifies one exact GitHub Issue comment. The comment's first nonempty line is `<!-- slipway-section:v1 key=KEY -->`; everything after it is that chapter's normalized Markdown. The Change profile requires at least one `outcome`, `requirements`, `acceptance_examples`, `constraints`, and `non_goals` role, but may split a role across independently addressable chapters.

The manifest is the only authority: array order is chapter order, the GraphQL comment node ID is object identity, and the body digest detects edits. Slipway fetches only referenced comments and rejects missing, unexpected, minimized, edited, duplicate, cross-Issue, field-inconsistent, or hash-mismatched material. Ordinary discussion comments never enter a Run. Title or label drift remains advisory. A missing, conflicting, Objective, v1, or unknown marker is not a v2 Change source.

## Self-containment and relationships

Decomposition publishes every applicable Objective requirement and shared constraint as self-contained Change chapters. Parent Kind is not inherited. Discussion is non-authoritative until a replacement chapter comment and a new manifest explicitly publish it. Objective→Change uses native sub-issues with one hierarchy level; a plan may reach exactly 100 children. Change dependencies use native blocked-by and may reach exactly 50 blocking and exactly 50 blocked-by dependencies independently per Issue. Stop and report only when a write would exceed a limit; never convert overflow to prose or a gate.

Detect `gh --version`. Use first-class relationship operations at `gh >= 2.94.0`; otherwise use the official REST API with existing authentication or report `environment_unavailable`. Do not invent local authority. Follow transfers only through `github.com`, then refetch repository/Issue node IDs, labels, parent, dependencies, and canonical URL; preserve the old URL alias and still compare markers and revisions. Never trust cross-host redirects.

## Publication and reconciliation

Objective publication is one stage: preview the exact title, complete body, labels, relations, operation UUID, and item UUID; obtain one current confirmation for those exact external writes; refetch mutable targets; create with `--body-file`; reconcile ambiguity by the exact marker pair through a paginated non-search API; and completely read back identity, URL, title/body/digest, markers, labels, and relations. It creates no chapter comments or manifest, asks for no second commit confirmation, and never starts Implement.

Change publication remains two stages because comment IDs do not exist before creation. First show complete chapter drafts, one operation UUID, a stable item UUID, canonical comment-body digests, intended section order/roles, exact labels/relationships, and the expected parent requirements revision; confirm creation of non-authoritative drafts. A new Change draft contains only the receipt markers and no `change/v2` marker; an amendment leaves the accepted body unchanged. Create replacement comments, refetch and verify IDs/body/visibility, then build and show the exact final manifest. Every amendment manifest, including a content-identical replacement, sets `parent_requirements_revision` to the exact pinned revision checked during preview; an initial manifest omits it. A second current confirmation authorizes the commit. Immediately refetch the accepted head, reject parent drift, and update the Issue body manifest last. Preserve the same receipt markers after the manifest fence. Unreferenced comments are drafts, never authority; do not edit accepted chapters in place or silently delete abandoned drafts. Refetch and reconcile the complete graph after the commit point.

GitHub has neither exactly-once Issue creation nor body CAS. On timeout-after-success, partial relation failure, duplicate markers, indexing delay, or ambiguous response, enumerate via paginated non-search Issue API and report each item/label/relation as `created|matched|failed|ambiguous`. Never blindly retry or roll back successes. Zero matches need a fresh preview and confirmation, one may converge, and multiple pause for the user.

A public-repository Issue has no private switch. Warn that Requirements and later journaled goals/answers/command summaries may be sensitive. Use a private repository, private vulnerability reporting only for an actual vulnerability when enabled, an existing security channel, or an ad-hoc Run. Redact recognized credential values while preserving truthful command identity. Do not collect tokens, unreferenced discussion, environment dumps, transcripts, or hidden reasoning. Source import transiently fetches only the exact Issue body and manifest-referenced raw comment fields, passes the raw envelope only to the CLI for consumption, removes the temporary file, and persists only accepted normalized materials plus bounded catalog/provenance.
