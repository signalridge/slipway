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
<!-- slipway-level: change/v1 -->
```

Its title starts `[Change]`; labels are exactly `level:change` plus one `kind:*`, with optional `ready-for-agent`. Its accepted body is:

```markdown
<!-- slipway-level: change/v1 -->

## Outcome
A single observable result.

## Requirements
All behavior needed for this delivery.

## Acceptance examples
Concrete observable examples.

## Constraints
Technical and product boundaries.

## Non-goals
Explicit exclusions.

## Implementation checklist
Optional execution notes; excluded from revisions.
```

The first exact marker is Level authority. Title or label drift is a warning and can be repaired only after confirmation; it never blocks a marker-valid Change Run. A missing, conflicting, Objective, or unknown marker is not a Change source. An ordinary unmarked Issue offers three explicit paths: manual normalization, a separately confirmed linked Change, or a bounded ad-hoc Run.

## Self-containment and relationships

Decomposition copies every applicable Objective requirement and shared constraint into each child. Parent Kind is not inherited. Comments must be folded into the Change body before they can enter a new snapshot. Objective→Change uses native sub-issues with one hierarchy level and a maximum of 100. Change dependencies use native blocked-by with a maximum of 50 blocking and 50 blocked-by per Issue. Stop and report limits; do not hide overflow in prose.

Detect `gh --version`. Use first-class relationship operations at `gh >= 2.94.0`; otherwise use the official REST API with existing authentication or report `environment_unavailable`. Do not invent local authority. Follow transfers only through `github.com`, then refetch repository/Issue node IDs, labels, parent, dependencies, and canonical URL; preserve the old URL alias and still compare markers and revisions. Never trust cross-host redirects.

## Publication and reconciliation

Before a write, show complete drafts and a plan containing an operation UUID, stable item UUIDs, repository, canonical body SHA-256, exact labels/relationships, and expected revisions. Confirm that exact plan. Put typed operation/item markers immediately after the level marker, use body files, refetch mutable Issues, and read everything back.

GitHub has neither exactly-once Issue creation nor body CAS. On timeout-after-success, partial relation failure, duplicate markers, indexing delay, or ambiguous response, enumerate via paginated non-search Issue API and report each item/label/relation as `created|matched|failed|ambiguous`. Never blindly retry or roll back successes. Zero matches need a fresh preview and confirmation, one may converge, and multiple pause for the user.

A public-repository Issue has no private switch. Warn that Requirements and later journaled goals/answers/command summaries may be sensitive. Use a private repository, private vulnerability reporting only for an actual vulnerability when enabled, an existing security channel, or an ad-hoc Run. Redact recognized credential values while preserving truthful command identity; do not collect tokens, raw comments, environment dumps, transcripts, or hidden reasoning.
