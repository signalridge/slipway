# Idea-to-Run workflow

`slipway-workflow` is an explicitly invoked, host-side coordinator for the functions defined by Slipway's Issue workflow. It can orient a rough idea, an existing Objective or Change, or an existing Run to the next user-owned capability or an explicit no-further-action outcome. It does not turn the host's installed skills into a general pipeline, add persistent workflow state, or duplicate the Run scheduler.

This guide uses capability names rather than host-specific invocation syntax. See [Host adapters](../reference/adapters.md) for each entry point.

## What one invocation authorizes

The workflow may inspect current facts, conduct a decision interview, and synthesize a work-item draft when the current stage needs one. It may not cross the next capability boundary or invent one when no further action is chosen:

| Function | What the AI may do autonomously here | What remains user-owned |
| --- | --- | --- |
| Orient and draft | Read repository and lifecycle facts, organize the interview, select Change or Objective, and synthesize the complete draft | Genuine product or risk decisions |
| Publish | Explain the expected publication shape and route | A separate `slipway-propose` invocation and current confirmation of its exact external-write plan |
| Decompose | Explain why an Objective needs self-contained child Changes and route | A separate `slipway-decompose` invocation and current confirmation of its exact operation plan |
| Execute or resume | Explain the source, Run, and budget route | A separate `slipway-run` invocation |

“Stateless” means only that the workflow creates no Slipway Run, journal, or cross-stage cursor. Conversation, documents, tracker artifacts, prototypes, and code changes remain state or side effects. The workflow itself is read-only. Existing external or unmanaged artifacts may be non-authoritative planning input, but valid managed Objective, Change, and Run records retain their contract-defined routing and source authority. Creating or modifying any artifact is a separate explicit detour.

## The shortest valid route

The workflow inspects the observed starting point and skips stages that do not apply:

| Starting point | Immediate next owner or terminal outcome | Conditional downstream route |
| --- | --- | --- |
| Rough idea, clarified conversation, spec, PRD, map, or ticket list | `slipway-propose`, after a complete Change or Objective draft | Published Change → `slipway-run`; published Objective → `slipway-decompose` → selected child Change → `slipway-run` |
| Explicit decision-only request whose endpoint is a bounded summary, not a draft | `slipway-clarify` | Clarify remains stateless and does not materialize or execute |
| Structurally valid Objective | `slipway-decompose` | Successfully published child Change → `slipway-run` |
| Structurally valid, self-contained `change/v2` Issue | `slipway-run` | The Run owns Orient, Clarify when needed, Implement, advisory Review, and Summary |
| Explicit private, tiny, urgent, offline, or deliberately untracked bounded goal | `slipway-run` | Start a new ad-hoc Run on the sharpened goal; no Issue source is implied |
| Active Run | `slipway-run` | Use its exact current Action and submit/skip variants; stop uses the public command, while take-over/reorder first stop and hand control back |
| Paused or stopped Run | `slipway-run` | Use the current structured recovery variant; never reconstruct it from prose |
| Failed, partial, or ambiguous Propose/Decompose publication | The originating `slipway-propose` or `slipway-decompose` owner | Return every available receipt/operation/item/revision fact; the owner decides same-receipt reconciliation or a contract-required fresh preview and confirmation |
| Ended Run or Review findings | One provenance-aware choice including no further action | Stop with no capability; new tracked scope → Propose; same accepted Change scope → fresh-fetch and attest, then a new issue-backed Run; no Change → a new ad-hoc Run |

When the user chooses to continue, the workflow names exactly one immediate next capability and never invokes it. No further Slipway action is a valid terminal outcome for ended work, advisory findings, or an abandoned publication attempt; the workflow reports the exact remaining state and names no capability. Failed, partial, or ambiguous publication remains with its originating Propose or Decompose owner and never advances to Decompose or Run. The workflow never blind-retries, restarts, or invents an operation; the owner may use the same receipt or a contract-required fresh preview and confirmation.

Standalone `slipway-clarify` remains available when a bounded decision summary—not a draft or publication—is the requested endpoint; the workflow does not insert it as a mandatory drafting stage. Direct `slipway-implement` and `slipway-review` remain available only when the user deliberately wants a standalone path without Run attribution or a pinned Issue source. They are not shortcuts around the managed Change-to-Run route. An active Run already owns its Action loop: only submit and skip are Action variants; stop uses public `slipway stop`, and take-over/reorder first stop and hand control back.

An ended Run is terminal and never resumes. A finding is advisory and may be accepted with no further action. If the user chooses a new issue-backed attempt, the host must fresh-fetch and attest the canonical Change instead of reusing the ended Run's pinned snapshot as source evidence. A chosen retry for an ended ad-hoc Run or standalone Review finding with no Change uses a new ad-hoc Run on the sharpened goal. New or changed tracked scope goes through Propose.

## Decision interviews, not skill-catalog routing

The workflow first checks current Git state, relevant code and tests, and the repository's verification conventions. It investigates discoverable facts instead of asking the user. When a genuine human decision remains, it asks one question at a time with a recommendation, rationale, alternatives, and trade-offs. A complete request needs no interview.

The workflow is self-contained. An already-installed, model-invocable `/grilling` primitive is its only optional external accelerator. If used, it preserves the one-question and shared-understanding rules. That confirmation confirms the understanding used by the draft; it does not grant publication, decomposition, implementation, or Run authority. Missing `/grilling` never blocks the workflow or triggers installation.

The workflow does not discover, rank, or invoke the host's other skills. User-only front doors remain human-only, and external implementation or review skills do not replace Slipway Run. If a separate ADR, report, prototype, or persistent wayfinding map is needed, the workflow explains the detour and stops; the user may invoke that tool separately and later return with its output as non-authoritative planning input.

## Choose the right work-item level

A **Change** is one result that can be delivered, verified, and reverted independently, leaves a safe repository state, and roughly fits one fresh Agent context. It has five independently addressable roles:

- **Outcome**
- **Requirements** — prefer behavior and contract; retain an exact path, format, or example when it is itself a necessary constraint
- **Acceptance examples** — objectively verifiable; user-facing work prefers external behavior, refactor or maintenance work may combine preserved behavior with a measurable internal outcome, and research covers the evidence and conclusion to deliver
- **Constraints**
- **Non-goals**

An **Objective** necessarily needs several independently useful deliveries. It is planning-only and contains:

- **Problem**
- **Outcome**
- **Requirements**
- **Shared constraints**
- **Non-goals**
- **Changes**, including provisional tracer-bullet slices and blocker edges

Only a structurally valid, self-contained `change/v2` Issue whose manifest-addressed chapters pass source validation can start an issue-backed Run. An Objective is never executable. A pure investigation is a `kind:research` Change that delivers an evidence-backed conclusion; later code uses another Change.

## Publication and source handoff

The workflow returns a complete draft and intended publication shape, not Propose's approved publication plan. `slipway-propose` alone owns repository refetch, operation and item identity, exact bodies and digests, relation revisions, preview, reconciliation, and the current confirmation for that plan. `slipway-decompose` owns the corresponding operation for child Changes.

After successful Change publication, the host reports the canonical URL and number. For an Objective, successful decomposition reports every child URL, explains the advisory unblocked frontier, and recommends one Change while preserving the user's selection. Only then does the user explicitly invoke `slipway-run` for the selected Change.

The host fetches and attests that exact Change, builds a temporary Source Bundle envelope, and passes it to the local CLI with `--source-file`. The CLI does not fetch GitHub, and a bare Issue number is not a CLI source.

For a new Run, when the user gives no initial budget override, the host states and uses the contract default of `8`. An explicit override may be `1..1000`. A larger recommendation needs a reason and never promises completion. Resume uses the distinct remaining-budget rules below. For tiny, private, urgent, offline, or deliberately untracked work, the workflow may instead route to an explicit ad-hoc `slipway-run` with the sharpened goal.

The workflow introduces no additional governance gate. Every external-write operation keeps its operation-scoped current confirmation, and Run start remains separately explicit.

## What autonomous execution means

Once explicitly started, a Run advances one Action at a time within pinned Requirements and budget until `paused`, `stopped`, or `ended`. Run Clarify may still pause for a genuine decision. `budget_exhausted` is a normal resumable pause. An explicit resume `--budget N` replaces the remainder; omitting it preserves a positive remainder and replenishes zero to `max(initial_budget, 3)` only when the mutation actually resumes the Run.

Review is read-only and advisory. A finding does not create an automatic Implement/re-review loop. `ended` means only that the automatic Action queue is empty; it does not prove correctness, completion, or ship readiness.
