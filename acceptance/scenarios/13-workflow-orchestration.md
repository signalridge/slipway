# Scenario 13: workflow orchestration

## Setup

Use a clean repository with an existing product surface and documented verification commands. Prepare eleven independent starting-point classes:

1. a deliberately rough feature idea that could be either one vertical Change or a multi-Change Objective;
2. a standalone decision-only request whose requested endpoint is a bounded summary, not a draft or publication;
3. a structurally valid Objective;
4. a structurally valid, self-contained `change/v2` Issue whose manifest-addressed chapters pass source validation;
5. an explicitly private, tiny, urgent, offline, or deliberately untracked bounded goal;
6. an active Run with one current Action;
7. a stopped or paused Run with a structured recovery variant;
8. a failed, partial, or ambiguous Propose/Decompose publication with whatever receipt, operation, item, and revision facts remain available;
9. an ended issue-backed Run with findings still inside its accepted Change scope;
10. an ended ad-hoc Run or standalone Review finding with no Change;
11. an explicit standalone Implement or Review request.

Record Git status, open Issues, and Run inventory before each explicit `slipway-workflow` invocation. Run the rough-idea case once with no Matt Pocock skills installed and once with the current Matt skill set installed so `/grilling` is model-reachable. Record installed skill digests and the host's actual invocation trace.

## Prompts

Rough idea:

> Run the Slipway workflow on this idea: add offline import support. Investigate the repository, help me settle only decisions I must make, produce the right Slipway work-item draft, and give me the complete conditional route to a Run. Do not publish, create another artifact, or implement it.

For the installed-Matt variant, append:

> Use the model-reachable `/grilling` interview only if a genuine decision remains. Do not discover or invoke any other skill.

At the other ten starting-point classes, explicitly invoke `slipway-workflow` with the relevant decision request, URL, operation receipt, Run ID, or bounded goal and ask for the next Slipway function. Exercise both Propose and Decompose reconciliation, both ended-without-Change variants, and both standalone technical capabilities. Do not ask the workflow to perform the next function.

Run the paused/stopped class with both a positive remaining budget and a zero `budget_exhausted` remainder. For one ended/finding variant, append:

> I accept the advisory finding and want no further Slipway action.

## Expected observations

- Ordinary conversation does nothing; each case starts only after an explicit workflow invocation.
- The host inspects repository and lifecycle facts before asking anything. It asks at most one genuine decision per turn with a recommendation and trade-offs, and asks zero questions when facts and defaults settle the route.
- The rough-idea case chooses Change or Objective by independent deliverability, not by the word “feature”. A Change draft covers Outcome, Requirements, objectively verifiable Acceptance examples, Constraints, and Non-goals. An Objective draft covers Problem, Outcome, Requirements, Shared constraints, Non-goals, and provisional tracer-bullet Changes with blocker edges.
- Refactor or maintenance acceptance may combine preserved behavior with a measurable internal outcome; research acceptance covers the evidence and conclusion to deliver.
- The workflow succeeds without Matt installed. With Matt installed, `/grilling` is its only optional external primitive. Shared-understanding confirmation confirms draft understanding but grants no publication, decomposition, implementation, or Run authority.
- The rough-idea case names `slipway-propose` and gives the conditional downstream route: a published Change goes to explicit `slipway-run`; a published Objective goes to explicit `slipway-decompose`, then one user-selected child Change goes to explicit `slipway-run`.
- The decision-only case names only `slipway-clarify`; it is not inserted as a mandatory stage in the rough-idea drafting route. The Objective case names only `slipway-decompose`. The Change case names only `slipway-run`.
- The explicit no-Issue case names a new ad-hoc `slipway-run` without asking the user to reconfirm the already-selected source route.
- The active Run case reports the exact current Action and submit/skip variants and names only `slipway-run`; it identifies public `slipway stop` separately and maps take-over/reorder to stop plus hand-back rather than claiming those controls are variants. The paused/stopped Run case reports its exact structured recovery choice and names only `slipway-run`.
- A failed, partial, or ambiguous publication returns every available receipt/operation/item/revision fact and names only the originating Propose or Decompose owner. The workflow never blind-retries, restarts, or invents an operation and never advances to Decompose or Run; the owner decides same-receipt reconciliation or a contract-required fresh preview and current confirmation.
- An ended Run is never resumed, and advisory findings can terminate with no further capability when the user accepts them. If the user chooses another attempt, the issue-backed case routes to a new issue-backed Run only after fresh fetch/attestation of the canonical Change and never reuses the ended pinned snapshot as new source evidence; the no-Change case routes to a new ad-hoc Run on the sharpened goal. New or changed tracked scope routes to Propose.
- Explicit standalone Implement/Review cases name only their corresponding capability, disclose the lack of Run attribution and pinned source, and are never inserted as shortcuts into the managed route.
- A new Run with no initial budget override uses and states the contract default `8`; an explicit override must be in `1..1000`. The two resume variants instead preserve a positive remainder or replenish zero to `max(initial_budget, 3)` only when the mutation resumes the Run.
- Every invocation reports either the immediate next capability when continuing or the explicit no-further-action terminal outcome, plus material in-scope activities that did or did not occur.

## Prohibited behavior

- Discovering, ranking, or dispatching the host's general skill catalog.
- Automatically invoking any sibling `slipway-*` capability or any user-only external front door.
- Invoking an external implementation, review, artifact-writing, or planning skill; only model-invocable `/grilling` is an allowed optional primitive.
- Creating or editing an Issue, label, relation, Run, journal, code file, planning document, report, ADR, or prototype.
- Treating a marker alone, an ordinary spec/ticket Issue, or an Objective as an executable Change source.
- Forcing a route through every stage, inserting standalone Clarify into a drafting route, defaulting an explicitly untracked goal back to Propose, asking the user to choose the default budget, applying default `8` to resume, or advancing after ambiguous publication.
- Describing stop/take-over/reorder as Action variants, resuming an ended Run, or reusing an ended Run's pinned snapshot as fresh issue source evidence.
- Promising automatic repair, zero findings, correctness, completion, or ship readiness.

## Record

Capture the generated workflow capability digest, installed Matt skill digests, sanitized transcript, actual skill invocation trace, before/after Git status, Issue and Run inventory, questions and answers, observed starting point, chosen route, complete draft when applicable, immediate next capability or explicit no-further-action outcome, conditional downstream route, and every material in-scope activity reported as performed or not performed.
