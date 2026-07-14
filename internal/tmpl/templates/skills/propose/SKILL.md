---
name: slipway-propose
description: Explicitly materialize clarified requirements as one self-contained Change or one planning Objective.
disable-model-invocation: true
---

# Slipway Propose

Use this capability only when the user explicitly asks to propose or publish a Slipway Issue. It may use the current conversation directly; it does not require a prior Clarify session and must not start implementation.

Choose the smallest coherent level:

- one independently deliverable, verifiable, and reversible result becomes a Change;
- a result that necessarily needs multiple independent deliveries becomes an Objective for later explicit decomposition.

A managed Change starts with the exact first non-empty line `<!-- slipway-level: change/v2 -->`, has a `[Change]` title, exactly one `level:change` label, exactly one of `kind:feature|kind:bug|kind:refactor|kind:maintenance|kind:research|kind:docs`, and one strict Issue-body manifest. Publish Outcome, Requirements, Acceptance examples, Constraints, and Non-goals as independently addressable chapter comments; each starts with its exact `<!-- slipway-section:v1 key=KEY -->` marker. The manifest's ordered entries bind each key/role/title to the comment node ID, database ID hint, and exact body digest. It must be self-contained; unreferenced discussion and a parent Objective are not runtime inheritance. `ready-for-agent` is optional navigation only.

A managed Objective starts with the exact first non-empty line `<!-- slipway-level: objective/v1 -->`, immediately followed by `<!-- slipway-publication-operation: UUID -->` and `<!-- slipway-publication-item: UUID -->`. It has an `[Objective]` title, exactly one `level:objective` label, exactly one of `kind:feature|kind:bug|kind:refactor|kind:maintenance|kind:research|kind:docs`, and the H2 sections `Problem`, `Outcome`, `Requirements`, `Shared constraints`, `Non-goals`, and `Changes`. It is never an executable Run source and does not receive `ready-for-agent`. The body marker remains authority: title or level-label drift is a warning and may be repaired only after confirmation; it never blocks a marker-valid Change Run.

For an existing Issue without a valid marker, do not rewrite it silently. Present exactly three choices: the user manually applies a normalized body; create a separately confirmed managed Change linked to it; or use a bounded summary in an explicit ad-hoc Run.

{{ template "source-bundle" . }}

## GitHub compatibility and identity

Detect `gh --version`. For `gh >= 2.94.0`, use first-class parent, sub-issue, and dependency operations. With an older or missing `gh`, use the official GitHub REST API with the user's existing authentication, or report `environment_unavailable`; never invent local authority. Respect the platform limits of 100 sub-issues per parent and 50 blocking plus 50 blocked-by dependencies per Issue.

Trust redirects and transfers only while every hop remains on `github.com`. After a same-host redirect or transfer, refetch the repository node ID, Issue node ID, canonical URL, labels, parent, dependencies, and current revision; preserve the old URL as an alias and still compare the marker and source/Requirements revisions. Reject cross-host redirect trust.

## Objective publication: one confirmed external write

An Objective is a single-stage publication. Show one preview containing the exact title, complete body, exact labels, every relation, one operation UUID, and one item UUID. Its body layout is exactly:

```markdown
<!-- slipway-level: objective/v1 -->
<!-- slipway-publication-operation: UUID -->
<!-- slipway-publication-item: UUID -->

## Problem
...
```

After one current external-write confirmation for that exact preview, refetch mutable relation targets and create the Objective with a private temporary body file (for example, `gh issue create --body-file FILE`), never a POSIX heredoc. On timeout-after-success or another ambiguous response, reconcile the exact operation/item marker pair through a paginated non-search Issue API before any retry. Then perform a complete readback of identity, URL, exact title/full body and digest, both markers, labels, and relations, reporting each write as `created`, `matched`, `failed`, or `ambiguous`. Objective publication creates no chapter comments or manifest, asks for no second commit confirmation, and never starts Implement.

## Change publication: two confirmed phases

A Change retains two confirmed phases because remote comment IDs do not exist before creation. First show every complete chapter draft and a draft-resource plan containing one operation UUID, one stable item UUID, target repository, exact comment body digests, intended section order/roles, exact labels, parent/dependencies, and expected parent requirements revision. Confirm creation of non-authoritative drafts. A new Change draft body contains only these receipt markers and no `change/v2` level marker:

```html
<!-- slipway-publication-operation: UUID -->
<!-- slipway-publication-item: UUID -->
```

For an amendment, leave the accepted body unchanged. Create chapter comments, refetch and verify IDs/bodies/visibility, then build and show the exact final manifest. Every amendment manifest, including a content-identical replacement, must set `parent_requirements_revision` to the exact expected pinned revision; an initial manifest omits it. Obtain a second current confirmation for the commit, immediately refetch and reject parent drift, then update the body last. The final body starts with the `change/v2` marker and manifest fence and retains the same receipt markers after that fence:

````markdown
<!-- slipway-level: change/v2 -->
```slipway-manifest
{...}
```
<!-- slipway-publication-operation: UUID -->
<!-- slipway-publication-item: UUID -->
````

Read back the complete graph. Unreferenced comments remain drafts and never become authority; never edit an accepted chapter in place or silently delete abandoned drafts.

On timeout-after-success, partial relation failure, duplicate marker matches, an ambiguous response, or indexing delay, enumerate with the paginated non-search Issue API and reconcile each operation/item marker. Report every Issue, label, and relationship as `created`, `matched`, `failed`, or `ambiguous`; never blindly retry, delete partial success, or claim exactly-once behavior. Zero matches require a fresh preview and confirmation before retry, one match may converge and read back, and multiple matches pause for the user.

Before showing or publishing a draft, warn that accepted Requirements and later journaled answers/command summaries may contain sensitive text. A public repository has no per-Issue private switch. Offer a private repository, enabled private vulnerability reporting only for an actual vulnerability, an existing security channel, or an ad-hoc Run. Redact recognized credential values while preserving truthful command identity. Do not collect tokens, unreferenced discussion, environment dumps, full transcripts, or hidden reasoning; if source validation is needed, transiently fetch only the manifest-referenced raw comment envelope for immediate CLI consumption and persist only accepted normalized materials/catalog.