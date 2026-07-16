---
name: slipway-decompose
description: Explicitly split an Objective into self-contained vertical Changes with native relationships.
disable-model-invocation: true
---

# Slipway Decompose

Use this capability only when the user explicitly asks to decompose a marker-valid Objective or amend its open child Changes. Do not implement the resulting work.

Read the current Objective, repository facts, existing sub-issues and dependencies, provider node identities, and current Issue revisions. The Objective must have the exact first marker `<!-- slipway-level: objective/v1 -->`, exactly one `level:objective`, and exactly one of `kind:feature|kind:bug|kind:refactor|kind:maintenance|kind:research|kind:docs`; marker authority wins over title/label drift, which is warning-only and repairable only after confirmation. Treat the Objective as planning input, never as runtime inheritance.

Produce coherent tracer-bullet Changes that each have one observable result, can be independently delivered, verified, rolled back, leave the repository meaningful and safe, and roughly fit one fresh Agent context. Split only independently deliverable outcomes; keep non-deliverable implementation steps as checklists. Materialize every applicable Objective requirement and shared constraint into each child's manifest-addressed chapter comments. Each child starts with `<!-- slipway-level: change/v2 -->`, has exactly one `level:change` and exactly one of `kind:feature|kind:bug|kind:refactor|kind:maintenance|kind:research|kind:docs`, and its manifest covers the required Outcome, Requirements, Acceptance examples, Constraints, and Non-goals roles; no child may depend on rereading its parent or unreferenced discussion. Keep Kind independent of the parent. Research delivers an evidence-backed conclusion and leaves later code to another Change. Pure refactors state preserved behavior and a measurable internal outcome; wide refactors use `expand`, independently deliverable migration batches, then `contract`.

Show a numbered decomposition before writing: every Change's complete effective Requirements, delivery result, acceptance examples, blockers, exact labels, parent relation, publication item UUID, every pre-existing mutable target's provider identity and observed revision, expected parent Requirements revision, and any missing-label creation with exact attributes. Use GitHub native sub-issues for Objective-to-Change and native blocked-by dependencies for Change-to-Change. Keep one hierarchy level. A plan may reach exactly 100 children, exactly 50 blocking dependencies, and exactly 50 blocked-by dependencies per Issue; treat blocking and blocked-by as independent directions. Stop and report only when the approved write would exceed one of those limits. Never convert overflow to prose or a gate.

GitHub `closed` status does not prove that a blocker Change was actually delivered. The planning frontier may use that state as a signal, but it must remain advisory rather than locked; the user retains authority to override it.

{{ template "source-bundle" . }}

## GitHub compatibility, transfer, and publication

Detect `gh --version`. For `gh >= 2.94.0`, use first-class parent/sub-issue/dependency operations; otherwise use the official REST API with existing authentication or report `environment_unavailable`. Never make a local relationship graph authoritative. Follow redirects or transfers only within `github.com`; refetch repository/Issue node IDs, canonical URL, labels, parent, dependencies, and revisions, preserve the old URL alias, and still compare body marker and source/Requirements revisions. Do not trust cross-host redirects.

{{ template "github-rest" . }}

Use two confirmed phases because remote comment IDs are unknown before creation. The complete decomposition uses one operation UUID, and each child keeps one stable item UUID through preview, draft creation, reconciliation, relations, and final readback. First show full chapter drafts and the draft-resource plan with exact chapter digests, intended section order/roles, target repositories, every pre-existing mutable target's provider identity and observed revision, expected parent Requirements revisions, exact labels, any missing-label creation with exact attributes, parent, and dependencies. Confirm all non-authoritative draft and label-creation writes. Use private body files or equivalent cross-platform safe input for every Issue/comment create or PATCH; never use a POSIX heredoc. Each new child draft has no `change/v2` marker and contains only its receipt markers:

```html
<!-- slipway-publication-operation: UUID -->
<!-- slipway-publication-item: UUID -->
```

Create and refetch chapter comments, then build and show each exact final manifest from observed IDs. Every amendment manifest, including a content-identical replacement, must set `parent_requirements_revision` to the exact expected pinned revision; each initial manifest omits it. Obtain a second current commit confirmation, immediately refetch every pre-existing mutable target and reject any approved revision drift, and update each body last through the safe body-file path. Each final child starts with its `change/v2` marker and manifest fence and retains the same operation and stable item markers after the fence:

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

When explicitly invoked in amendment mode, compare only affected open children, show each Requirements diff and expected source revision, and obtain approval before each planned PATCH. Pause on concurrent edits, unclear applicability, or any failed item. Never propagate in the background, silently patch children, rewrite closed deliveries, or turn synchronization into a Run prerequisite; use a new superseding Change for delivered work.
