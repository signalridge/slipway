---
name: slipway-decompose
description: Explicitly split an Objective into self-contained vertical Changes with native relationships.
disable-model-invocation: true
---

# Slipway Decompose

Use this capability only when the user explicitly asks to decompose a marker-valid Objective or amend its open child Changes. Do not implement the resulting work.

Read the current Objective, repository facts, existing sub-issues and dependencies, provider node identities, and current Issue revisions. The Objective must have the exact first marker `<!-- slipway-level: objective/v1 -->`, exactly one `level:objective`, and exactly one `kind:*`; marker authority wins over title/label drift, which is warning-only and repairable only after confirmation. Treat the Objective as planning input, never as runtime inheritance.

Produce the smallest tracer-bullet Changes that each have one observable result, can be independently delivered, verified, rolled back, and leave the repository meaningful and safe. Materialize every applicable Objective requirement and shared constraint into each child body. Each child starts with `<!-- slipway-level: change/v1 -->`, has exactly one `level:change` and exactly one `kind:*`, and contains the exact H2 sections `Outcome`, `Requirements`, `Acceptance examples`, `Constraints`, and `Non-goals`; no child may depend on rereading its parent or comments. Keep Kind independent of the parent. For wide refactors, prefer `expand`, independently deliverable migration batches, then `contract`.

Show a numbered decomposition before writing: every Change's complete effective Requirements, delivery result, acceptance examples, blockers, exact labels, parent relation, expected revision, and publication item UUID. Use GitHub native sub-issues for Objective-to-Change and native blocked-by dependencies for Change-to-Change. Keep one hierarchy level, stop before 100 sub-issues or 50 dependencies in either direction, and report the platform limit instead of hiding overflow in prose.

## GitHub compatibility, transfer, and publication

Detect `gh --version`. For `gh >= 2.94.0`, use first-class parent/sub-issue/dependency operations; otherwise use the official REST API with existing authentication or report `environment_unavailable`. Never make a local relationship graph authoritative. Follow redirects or transfers only within `github.com`; refetch repository/Issue node IDs, canonical URL, labels, parent, dependencies, and revisions, preserve the old URL alias, and still compare body marker and source/Requirements revisions. Do not trust cross-host redirects.

Build one approved publication plan with an operation UUID, stable item UUIDs, canonical body SHA-256 values, target repository, expected current revisions, exact labels, parent, and dependencies. Show full drafts and obtain confirmation for all exact external writes. Add typed operation/item UUID markers immediately after each level marker, use body files or equivalent safe argv, refetch mutable Issues immediately before changes, and read back bodies, markers, labels, node identities, parent, and dependencies.

On timeout-after-success, a partial relationship failure, duplicate marker matches, indexing delay, or any ambiguous response, reconcile through paginated non-search Issue APIs. Return every item, label, and relationship as `created`, `matched`, `failed`, or `ambiguous`; never blindly retry, close duplicates, roll back successful creations, edit the Objective body, or close the Objective unless separately requested. Zero marker matches require a new preview and confirmation, one may converge after readback, and multiple matches pause for the user.

Before publication, warn that Requirements can be sensitive and that a public Issue has no private switch. Use a private repository, private vulnerability reporting only for an actual vulnerability when enabled, an existing security channel, or an ad-hoc Run. Redact recognized credential values while retaining truthful command identity; do not store or publish tokens, raw comments, environment dumps, full transcripts, or hidden reasoning.

When explicitly invoked in amendment mode, compare only affected open children, show each Requirements diff and expected source revision, and obtain approval before each planned PATCH. Pause on concurrent edits, unclear applicability, or any failed item. Never propagate in the background, silently patch children, rewrite closed deliveries, or turn synchronization into a Run prerequisite; use a new superseding Change for delivered work.
