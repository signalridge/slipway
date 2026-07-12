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

A managed Change starts with the exact first non-empty line `<!-- slipway-level: change/v1 -->`, has a `[Change]` title, exactly one `level:change` label, exactly one `kind:*` label, and the exact H2 sections `Outcome`, `Requirements`, `Acceptance examples`, `Constraints`, and `Non-goals`. It must be self-contained; comments and a parent Objective are not runtime inheritance. `ready-for-agent` is optional navigation only.

A managed Objective starts with the exact first non-empty line `<!-- slipway-level: objective/v1 -->`, has an `[Objective]` title, exactly one `level:objective` label, exactly one `kind:*` label, and the H2 sections `Problem`, `Outcome`, `Requirements`, `Shared constraints`, `Non-goals`, and `Changes`. It is never an executable Run source and does not receive `ready-for-agent`. The body marker remains authority: title or level-label drift is a warning and may be repaired only after confirmation; it never blocks a marker-valid Change Run.

For an existing Issue without a valid marker, do not rewrite it silently. Present exactly three choices: the user manually applies a normalized body; create a separately confirmed managed Change linked to it; or use a bounded summary in an explicit ad-hoc Run.

## GitHub compatibility and identity

Detect `gh --version`. For `gh >= 2.94.0`, use first-class parent, sub-issue, and dependency operations. With an older or missing `gh`, use the official GitHub REST API with the user's existing authentication, or report `environment_unavailable`; never invent local authority. Respect the platform limits of 100 sub-issues per parent and 50 blocking plus 50 blocked-by dependencies per Issue.

Trust redirects and transfers only while every hop remains on `github.com`. After a same-host redirect or transfer, refetch the repository node ID, Issue node ID, canonical URL, labels, parent, dependencies, and current revision; preserve the old URL as an alias and still compare the marker and source/Requirements revisions. Reject cross-host redirect trust.

## Confirmed, reconcilable publication

Before any GitHub write, show every complete draft and an approved publication plan containing an operation UUID, stable item UUID, target repository, canonical body SHA-256, exact labels, parent/dependencies, and expected current revisions. Obtain confirmation for that exact plan. Put typed `slipway-publication-operation` and `slipway-publication-item` UUID markers immediately after the level marker, use body files or equivalent safe argv input, refetch mutable targets immediately before writing, and read back body digest, markers, labels, parent, dependencies, node identities, and URL.

On timeout-after-success, partial relation failure, duplicate marker matches, an ambiguous response, or indexing delay, enumerate with the paginated non-search Issue API and reconcile each operation/item marker. Report every Issue, label, and relationship as `created`, `matched`, `failed`, or `ambiguous`; never blindly retry, delete partial success, or claim exactly-once behavior. Zero matches require a fresh preview and confirmation before retry, one match may converge and read back, and multiple matches pause for the user.

Before showing or publishing a draft, warn that accepted Requirements and later journaled answers/command summaries may contain sensitive text. A public repository has no per-Issue private switch. Offer a private repository, enabled private vulnerability reporting only for an actual vulnerability, an existing security channel, or an ad-hoc Run. Redact recognized credential values while preserving truthful command identity; never publish tokens, raw comments, environment dumps, full transcripts, or hidden reasoning.
