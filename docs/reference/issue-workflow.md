# Issue workflow (non-normative)

> **Non-normative localized guide.** The [Chinese product contract](../zh/reference/product-contract.md) and [machine schema](machine-protocol.schema.json) are authoritative.

Use an Objective only when one outcome necessarily needs multiple independently deliverable Changes. A Change is the only issue-backed Run source. Every Change must contain all effective execution requirements; parent bodies and comments are not runtime inheritance. Tiny, sensitive, emergency, offline, or deliberately untracked work can use `slipway run "<ad-hoc goal>"`.

## Markers, labels, and bodies

An Objective's first non-empty line is exactly:

```html
<!-- slipway-level: objective/v1 -->
```

Its title starts `[Objective]`; labels are exactly `level:objective` plus one `kind:*`. Its exact H2 sections are Problem, Outcome, Requirements, Shared constraints, Non-goals, and Changes.

A Change's first non-empty line is exactly:

```html
<!-- slipway-level: change/v2 -->
```

Its title starts `[Change]`; labels are exactly `level:change` plus one `kind:*`, with optional `ready-for-agent`. The marker is followed by one strict manifest fence:

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
````

Each manifest entry identifies one exact GitHub Issue comment. The comment's first nonempty line is `<!-- slipway-section:v1 key=KEY -->`; everything after it is that chapter's normalized Markdown. The Change profile requires at least one `outcome`, `requirements`, `acceptance_examples`, `constraints`, and `non_goals` role, but may split a role across independently addressable chapters.

The manifest is the only authority: array order is chapter order, the GraphQL comment node ID is object identity, and the body digest detects edits. Slipway fetches only referenced comments and rejects missing, unexpected, minimized, edited, duplicate, or hash-mismatched material. Ordinary discussion comments never enter a Run. Title or label drift remains advisory. A missing, conflicting, Objective, v1, or unknown marker is not a v2 Change source.

## Self-containment and relationships

Decomposition publishes every applicable Objective requirement and shared constraint as self-contained Change chapters. Parent Kind is not inherited. Discussion is non-authoritative until a replacement chapter comment and a new manifest explicitly publish it. Objective→Change uses native sub-issues with one hierarchy level and a maximum of 100. Change dependencies use native blocked-by with a maximum of 50 blocking and 50 blocked-by per Issue. Stop and report limits; do not hide overflow in prose.

Detect `gh --version`. Use first-class relationship operations at `gh >= 2.94.0`; otherwise use the official REST API with existing authentication or report `environment_unavailable`. Do not invent local authority. Follow transfers only through `github.com`, then refetch repository/Issue node IDs, labels, parent, dependencies, and canonical URL; preserve the old URL alias and still compare markers and revisions. Never trust cross-host redirects.

## Publication and reconciliation

Publication uses two confirmations because comment IDs do not exist before creation. First show complete chapter drafts, operation/item UUIDs, canonical comment-body digests, intended section order/roles, exact labels/relationships, and the expected parent requirements revision; confirm creation of non-authoritative drafts. A new Change begins as a reconcilable draft Issue shell with publication markers but no `change/v2` marker; an amendment leaves the accepted body unchanged. Create replacement comments, refetch and verify IDs/body/visibility, then build and show the exact final manifest. Every amendment manifest, including a content-identical replacement, sets `parent_requirements_revision` to the exact pinned revision checked during preview; an initial manifest omits it. A second current confirmation authorizes the commit. Immediately refetch the accepted head, reject parent drift, and update the Issue body manifest last. Preserve reconciliation markers after the manifest fence. Unreferenced comments are drafts, never authority; do not edit accepted chapters in place or silently delete abandoned drafts. Refetch and reconcile the complete graph after the commit point.

GitHub has neither exactly-once Issue creation nor body CAS. On timeout-after-success, partial relation failure, duplicate markers, indexing delay, or ambiguous response, enumerate via paginated non-search Issue API and report each item/label/relation as `created|matched|failed|ambiguous`. Never blindly retry or roll back successes. Zero matches need a fresh preview and confirmation, one may converge, and multiple pause for the user.

A public-repository Issue has no private switch. Warn that Requirements and later journaled goals/answers/command summaries may be sensitive. Use a private repository, private vulnerability reporting only for an actual vulnerability when enabled, an existing security channel, or an ad-hoc Run. Redact recognized credential values while preserving truthful command identity; do not collect tokens, raw comments, environment dumps, transcripts, or hidden reasoning.
