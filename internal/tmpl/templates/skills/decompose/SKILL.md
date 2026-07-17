---
name: slipway-decompose
description: Explicitly split an Objective into self-contained vertical Changes with native relationships.
disable-model-invocation: true
---

# Slipway Decompose

Use this capability only when the user explicitly asks to decompose a marker-valid Objective or amend its open child Changes. Do not implement the resulting work.

Read the current Objective, repository facts, existing sub-issues and dependencies, provider node identities, and current Issue revisions. Only the exact first marker `<!-- slipway-level: objective/v1 -->` and valid Objective body content determine decomposition eligibility. Inspect title and labels, but missing or conflicting labels never block decomposition: report projection drift as a warning. Repairing title or labels is a separate, explicitly confirmed external write and must not be bundled into decomposition merely to make the Objective appear valid. Treat the Objective as planning input, never as runtime inheritance.

Produce coherent tracer-bullet Changes that each have one observable result, can be independently delivered, verified, rolled back, leave the repository meaningful and safe, and roughly fit one fresh Agent context. Split only independently deliverable outcomes; keep non-deliverable implementation steps as checklists. Materialize every applicable Objective requirement and shared constraint into each child's manifest-addressed chapter comments. Each child starts with `<!-- slipway-level: change/v2 -->`, has exactly one `level:change` and exactly one of `kind:feature|kind:bug|kind:refactor|kind:maintenance|kind:research|kind:docs`, and its manifest covers the required Outcome, Requirements, Acceptance examples, Constraints, and Non-goals roles; no child may depend on rereading its parent or unreferenced discussion. Keep Kind independent of the parent. Research delivers an evidence-backed conclusion and leaves later code to another Change. Pure refactors state preserved behavior and a measurable internal outcome; wide refactors use `expand`, independently deliverable migration batches, then `contract`.

Show a numbered decomposition before writing: every Change's complete effective Requirements, delivery result, acceptance examples, blockers, exact labels, parent relation, publication item UUID, every pre-existing mutable target's provider identity and observed revision, expected parent Requirements revision, and any missing-label creation with exact attributes. Use GitHub native sub-issues for Objective-to-Change and native blocked-by dependencies for Change-to-Change. Keep one hierarchy level. A plan may reach exactly 100 children, exactly 50 blocking dependencies, and exactly 50 blocked-by dependencies per Issue; treat blocking and blocked-by as independent directions. Stop and report only when the approved write would exceed one of those limits, and recommend how to repartition the Objective into smaller coherent Objectives or publication operations. Never convert overflow to prose or a gate.

GitHub `closed` status does not prove that a blocker Change was actually delivered. The planning frontier may use that state as a signal, but it must remain advisory rather than locked; the user retains authority to override it.

{{ template "source-bundle" . }}

## GitHub compatibility, transfer, and publication

Detect `gh --version`. For `gh >= 2.94.0`, use first-class parent/sub-issue/dependency operations; otherwise use the official REST API with existing authentication or report `environment_unavailable`. Never make a local relationship graph authoritative. Follow redirects or transfers only within `github.com`; refetch repository/Issue node IDs, canonical URL, labels, parent, dependencies, and revisions, preserve the old URL alias, and still compare body marker and source/Requirements revisions. Do not trust cross-host redirects.

{{ template "github-rest" . }}

Use one confirmed operation for the complete decomposition. It has one operation UUID, and each child keeps one stable item UUID through preview, receipt creation, chapter creation, deterministic manifest construction, relations, and final readback. Before any write, show full chapter drafts and one approved operation plan with exact chapter bodies/digests, intended section keys/order/roles/titles, the deterministic rule that inserts provider-returned comment IDs into the final manifest, target repositories, every pre-existing mutable target's provider identity and observed revision, expected parent Requirements revisions, exact labels, any missing-label creation with exact attributes, parent, dependencies, and the exact receipt-only intermediate body. Obtain one current external-write confirmation for that complete operation.

Use private body files or equivalent cross-platform safe input for every Issue/comment create or PATCH; never use a POSIX heredoc. Because provider comment IDs do not exist before their Issue, each new child may temporarily contain only its receipt markers and no `change/v2` marker:

```html
<!-- slipway-publication-operation: UUID -->
<!-- slipway-publication-item: UUID -->
```

This receipt-only Issue is a short-lived, non-authoritative reconciliation resource, not a Change source, a Run/lifecycle state, or a second authorization boundary. Create and refetch chapter comments, then deterministically build each final manifest from the approved keys/order/roles/titles, exact body digests, and provider-returned IDs. The provider-assigned IDs are reconciliation facts and must not trigger another confirmation. Every amendment manifest, including a content-identical replacement, must set `parent_requirements_revision` to the exact expected pinned revision; each initial manifest omits it. Immediately before each final body PATCH or pre-existing relationship/label mutation, refetch every mutable target and reject any approved revision drift. Remote drift, a missing receipt, ambiguity, or any material chapter/construction-rule change requires a new preview and current confirmation; otherwise update each body last through the safe body-file path under the original approval. Each final child starts with its `change/v2` marker and manifest fence and retains the same operation and stable item markers after the fence:

````markdown
<!-- slipway-level: change/v2 -->
```slipway-manifest
{...}
```
<!-- slipway-publication-operation: UUID -->
<!-- slipway-publication-item: UUID -->
````

Read back the complete graph. Unreferenced comments remain drafts; accepted chapters are replaced, never edited in place, and abandoned drafts are not silently deleted.

On timeout-after-success, a partial relationship failure, duplicate marker matches, indexing delay, or any ambiguous response, reconcile through paginated non-search Issue APIs. Return every item, label, and relationship as `created`, `matched`, `failed`, or `ambiguous`; never blindly retry, close duplicates, roll back successful creations, edit the Objective body, or close the Objective unless separately requested. Zero marker matches require a new preview and confirmation, one may converge after readback, and multiple matches pause for the user.

Before publication, warn that Requirements can be sensitive and that a public Issue has no private switch. Use a private repository, private vulnerability reporting only for an actual vulnerability when enabled, an existing security channel, or an ad-hoc Run. Redact recognized credential values while retaining truthful command identity. Do not collect tokens, unreferenced discussion, environment dumps, full transcripts, or hidden reasoning; if source validation is needed, transiently fetch only the manifest-referenced raw comment envelope for immediate CLI consumption and persist only accepted normalized materials/catalog.

When explicitly invoked in amendment mode, compare only affected open children and show every Requirements diff, expected source revision, and PATCH in one amendment publication plan. Obtain one current confirmation for that exact plan before applying its items sequentially. Pause on concurrent edits, unclear applicability, or any failed item. Never propagate in the background, silently patch children, rewrite closed deliveries, or turn synchronization into a Run prerequisite; use a new superseding Change for delivered work.
