---
name: slipway-workflow
description: Explicitly orient a rough idea, Objective, Change, or Run to the next user-owned Slipway function or a no-further-action outcome without becoming a router or runtime.
disable-model-invocation: true
---

# Slipway Workflow

Use this capability only when the user explicitly asks to run the Slipway workflow, take an idea to a work-item draft, or orient existing Slipway work to its next function. One invocation authorizes only the guidance work for the current stage: inspect current facts, resolve genuine human decisions, and synthesize a draft when one is needed. It does not authorize an external write, Objective decomposition, implementation, or a Run. Ordinary conversation must never start it.

This capability is stateless only in the Slipway sense: it opens no Run, writes no journal, and creates no durable pipeline cursor. Conversation, documents, tracker artifacts, prototypes, and repository edits are still state and side effects. The workflow itself is read-only. It may read an existing external or unmanaged artifact as non-authoritative planning input, but a valid managed Objective, Change, or Run record retains the routing and source authority defined by the contract. It never creates or modifies an artifact as a detour.

## Orchestrate Slipway functions, not installed skills

Coordinate the product functions defined by Slipway's Issue workflow: clarification, publication, Objective decomposition, Run start or resume, and the Run-owned Implement and Review loop. Do not discover, rank, or dispatch the host's general skill catalog. Do not invoke a sibling `slipway-*` capability; every sibling remains a user-invoked boundary and must be named for the user to invoke explicitly.

The workflow is self-contained and must work when no Matt Pocock skill is installed. Never invoke a user-only external front door such as `/grill-me` or `/wayfinder`. Do not invoke an external implementation or review skill even when it is model-reachable: after the Change handoff, Slipway Run owns execution and advisory Review. The only optional external primitive this workflow may invoke is an already-installed, model-invocable `/grilling` interview. Missing primitives never block the workflow and never trigger installation or enablement.

## Choose the shortest valid route

Inspect the user's input, repository, Issue identity and marker, and read-only `slipway status --json` when a Run may already exist. Start from the observed state instead of forcing every request through every stage:

| Observed starting point | Work owned by this invocation | Immediate next owner or terminal outcome |
| --- | --- | --- |
| Rough idea, clarified conversation, or non-authoritative planning artifact | Investigate, settle decisions, choose one Change or Objective, and produce its complete draft | `slipway-propose` |
| Explicit standalone decision-only clarification request, with no draft or materialization desired | Explain the stateless decision-summary boundary without starting the interview here | `slipway-clarify` |
| Structurally valid Objective | Explain its planning role and the need for self-contained child Changes | `slipway-decompose` |
| Structurally valid, self-contained `change/v2` Issue with no selected Run | Confirm the source route and state the effective budget | `slipway-run` |
| Explicit private, tiny, urgent, offline, or deliberately untracked bounded goal | State the sharpened bounded goal and the already-selected no-Issue source route | `slipway-run` for a new ad-hoc Run |
| Active Run | Report the exact current Action and its submit/skip variants; state that stop uses public `slipway stop` and take-over/reorder first stop and hand control back | `slipway-run` |
| Paused or stopped Run | Report the exact Run and its structured recovery choice without changing it | `slipway-run` |
| Failed, partial, or ambiguous Propose/Decompose publication | Preserve and return every available receipt, operation, item, and revision fact; report the exact unresolved state | The originating `slipway-propose` or `slipway-decompose` owner decides same-receipt reconciliation or a contract-required fresh preview and confirmation |
| Ended Run or advisory Review findings | Explain that ended is terminal and no completion was certified; offer no further action, new tracked scope, or a provenance-correct new Run attempt without making findings a gate | No capability when the user stops; otherwise `slipway-propose` or `slipway-run` after that one genuine choice |
| Explicit standalone implementation or review request | Explain that it has no Run attribution or pinned source | `slipway-implement` or `slipway-review`, only when the user deliberately wants the standalone path |

Skip every inapplicable stage. When the user chooses to continue, name exactly one immediate next capability plus the conditional downstream route needed to prevent the next handoff from becoming a guess; never invoke it. No further Slipway action is a valid terminal outcome for ended work, advisory findings, or a user-abandoned publication attempt. Report the exact remaining state without inventing a next capability. A failed, partial, or ambiguous publication stays with its originating Propose or Decompose owner; the workflow never blind-retries, restarts, or invents an operation and never advances it to Decompose or Run. The owner may reconcile the same receipt or, when receipts are unrecoverable or the approved plan materially changed, produce a fresh preview and current confirmation under its existing contract.

Standalone Clarify is not a mandatory stage on the route to a draft: use it only when clarification and a bounded decision summary are themselves the user's requested endpoint. An active Run already owns its Action loop; route back to its exact current Action. Only submit and skip are Action variants. Stop uses the public command, and take-over or reorder means stop and hand control back; never describe those controls as `next.variants`.

An ended Run is terminal and is never resumed. A finding is advisory and may be accepted with no further action. If the user chooses another attempt for findings still within an issue-backed Change, fresh-fetch and attest the canonical Change before starting a new issue-backed Run; never reuse the ended Run's pinned snapshot as new source evidence. For an ended ad-hoc Run or a standalone Review finding with no Change, a chosen new attempt uses an ad-hoc Run on the sharpened goal. Scope the user chooses to track, or that changes the accepted Change contract, routes to Propose.

## Investigate and settle decisions

Investigate before asking. Read the current Git state, relevant code, tests, and repository build, test, typecheck, and lint conventions. Separate facts the model can discover from genuine product or risk decisions only the user can make.

When a draft is needed, apply a design-tree interview. Ask exactly one genuine decision at a time with a recommendation grounded in observed repository facts, its rationale, and concrete alternatives with trade-offs; wait for the answer. Finish the current decision branch before opening an independent branch. Settle the testing seams, slice granularity, and blocking edges that would otherwise prevent a self-contained downstream draft. Ask zero questions when the request already determines the work, and stop interviewing immediately when the user asks to wrap up.

The optional `/grilling` primitive is an accelerator, never a dependency or gate. If used, honor its one-question-at-a-time and shared-understanding rules. Shared-understanding confirmation only confirms the understanding used by the draft; the initial explicit workflow invocation already authorized drafting, and the confirmation grants no publication, decomposition, implementation, or Run authority. If `/grilling` is absent, apply the same discipline directly.

This is not a durable wayfinding state machine. If the unresolved decision map cannot fit responsibly in the current context, stop with a bounded map of settled decisions, open questions, and the recommended next explicit workflow invocation. Never silently create a cross-session Issue map.

## Choose one coherent work-item level

Choose the level before drafting:

- One observable result that is independently deliverable, verifiable, reversible, leaves a safe repository state, and roughly fits one fresh Agent context is a Change. Keep a cross-layer result as one tracer-bullet vertical slice; keep non-deliverable implementation steps as a checklist.
- A result that necessarily requires several independently useful deliveries is an Objective for later explicit decomposition. Draft provisional vertical slices and their blocker edges, but do not pretend the Objective itself is executable.
- A pure investigation with no deliverable code is a `kind:research` Change whose result is an evidence-backed conclusion; any later code belongs to a separate Change.

For a Change, produce all five independently addressable roles with no runtime inheritance from unreferenced conversation:

- **Outcome** — the observable result and the problem it resolves.
- **Requirements** — behavior, user stories, interfaces, schemas, and contract decisions. Avoid unnecessary implementation prescription, but retain an exact path, format, or example when it is itself a contract or necessary constraint.
- **Acceptance examples** — objectively verifiable checks. Prefer external behavior for user-facing work; for refactor or maintenance work, cover preserved behavior and a measurable internal outcome; for research, cover the evidence and conclusion to deliver.
- **Constraints** — architectural decisions, preserved behavior, and boundaries.
- **Non-goals** — explicit exclusions.

For an Objective, instead produce its distinct planning shape:

- **Problem** — why the larger effort exists.
- **Outcome** — the observable destination.
- **Requirements** — behavior shared across the effort.
- **Shared constraints** — boundaries every child must preserve.
- **Non-goals** — explicit exclusions.
- **Changes** — provisional tracer-bullet deliveries, dependencies, and blocking edges for later validation by `slipway-decompose`.

Map any upstream spec, PRD, wayfinding map, or ticket list into these roles without inheriting its execution ownership. Such material is non-authoritative planning input. Slipway Decompose and Run own decomposition and execution after publication.

## Preserve every explicit boundary

Present the complete work-item draft and its intended publication shape, not an approved publication plan. `slipway-propose` alone owns repository re-fetch, operation and item identities, exact bodies and digests, relation revisions, the exact external-write plan, and one current user confirmation for that plan.

Give the complete conditional route without executing it:

- one Change draft -> explicit `slipway-propose`; only after successful reconciliation, report its canonical URL and number and use explicit `slipway-run`;
- one Objective draft -> explicit `slipway-propose`; only after successful publication, use explicit `slipway-decompose`; only after successful child publication, report every child URL, explain the advisory unblocked frontier, recommend one Change, and use explicit `slipway-run` after the user selects it;
- work deliberately kept private, urgent, tiny, offline, or untracked -> an explicit ad-hoc `slipway-run` on the sharpened goal, with no Issue.

Never publish, patch, label, or relate an Issue from this capability. Never invent a second Issue format. Only a structurally valid, self-contained `change/v2` Issue whose manifest-addressed chapters pass source validation can be an executable Issue source; an Objective is never executable.

For an issue-backed Run, the host reports the canonical Change URL and number, fetches and attests that exact Issue, builds the Source Bundle envelope, and passes it with `--source-file`; never hand a bare Issue number to the CLI or make the CLI fetch the network. For a new Run, if the user gives no initial budget override, state and use the contract default of `8`; an explicit override must be in `1..1000`. Do not ask the user to choose a value that the default already settles. A larger recommendation needs a reason and is never a promise of completion. For resume, preserve the distinct remaining-budget rules below.

This workflow adds no workflow-owned governance gate. Each Propose or Decompose external-write operation retains its own exact-plan confirmation, and Run start remains a separate explicit authorization.

## Report the Run boundary faithfully

Once explicitly started elsewhere, a Run advances one Action at a time within its pinned Requirements and budget until it is `paused`, `stopped`, or `ended`. It honors skip, stop, take-over, and reorder immediately. Run Clarify may still pause for a genuine decision; a high-quality draft reduces that need but cannot disable it.

Review is advisory and read-only and creates no automatic repair or re-review loop. `ended` means only that the automatic Action queue is empty—not that the work is correct, complete, or shippable. A `budget_exhausted` pause is normal and resumable. On resume, explicit `--budget N` replaces the remaining budget with `N`; omitting it preserves a positive remainder and replenishes zero to `max(initial_budget, 3)`. The replacement applies only when the mutation actually resumes the Run. New findings remain visible for the user to place in another Change, a new issue-backed Run after fresh source fetch and attestation, or a new ad-hoc Run when no Change owns the scope.

Certify nothing. Report the facts investigated, decisions settled, route selected, exact draft produced when applicable, remaining uncertainties, and either the immediate next explicit capability when continuing or the explicit no-further-action terminal outcome. State whether the material in-scope activities—Issue mutation, artifact writes, implementation, Run start, and requested verification—actually occurred; do not enumerate irrelevant activities merely to say they were absent.
