---
name: slipway-workflow
description: Explicitly orchestrate a rough idea into a publication-ready Slipway Change or Objective draft, then stop at the propose and run authorization boundaries.
disable-model-invocation: true
---

# Slipway Workflow

Use this capability only when the user explicitly asks to run the workflow or to take a rough idea through to a Slipway work-item draft. One explicit invocation authorizes the bounded first-half orchestration in this document: investigate repository facts, settle genuine human decisions, and synthesize one Change or Objective draft. It does not authorize an external write, implementation, or a Run. Ordinary conversation must never start it.

This capability is stateless only in the Slipway sense: it opens no Run, writes no journal, and creates no durable Slipway pipeline cursor. Conversation, documents, tracker artifacts, prototypes, and repository edits are still state and side effects. The default idea-to-draft path is read-only. Create a planning document or throwaway prototype only when the user's explicit request already authorizes that artifact and scope; report it, keep it out of production implementation, and never treat it as an executable Change source.

The workflow owns its orchestration but not another skill's invocation authority. It is self-contained and must work when no Matt Pocock skill is installed. Never invoke a user-only front door such as `/grill-me`, `/grill-with-docs`, `/wayfinder`, `/to-spec`, `/to-tickets`, `/implement`, or `/ask-matt`; their names remain commands only the human may explicitly invoke. Do not invoke `code-review` even when it is model-reachable: execution and Review belong to Slipway Run after the Change handoff, not to this planning half.

## Investigate and settle decisions

Investigate before asking. Read the current Git state, relevant code, tests, and repository build, test, typecheck, and lint conventions. Separate facts the model can discover from genuine product or risk decisions only the user can make.

Then apply a design-tree interview. Ask exactly one genuine decision at a time with a recommendation grounded in observed repository facts, its rationale, and concrete alternatives with trade-offs; wait for the answer. Finish the current branch by settling the parent decision and its immediate consequences before opening an independent branch. Settle the testing seams, slice granularity, and blocking edges that would otherwise prevent a self-contained downstream draft. Ask zero questions when the request already determines the work, and stop interviewing immediately when the user asks to wrap up.

Model-invocable primitives are optional accelerators, never dependencies or gates. When Matt's model-invocable `/grilling` primitive is installed and model-reachable, this capability may run the `/grilling` skill for the decision interview; honor its one-question-at-a-time and shared-understanding confirmation rules. That confirmation authorizes only the draft, never publication, implementation, or a Run. Do not install or enable a primitive merely to continue this workflow. If `/grilling` is absent, apply the same interview discipline directly.

Artifact-producing primitives are a different class. Matt's `/domain-modeling` writes glossary or ADR material, `/research` writes a Markdown report, and `/prototype` writes throwaway code. Invoke one only when the user's existing request separately authorizes that exact artifact and scope. Otherwise do not invoke it: investigate vocabulary, primary-source facts, and runnable uncertainty through the default read-only path, and report anything that cannot be resolved without a new side effect. No primitive may silently widen the task.

This is not a durable wayfinding state machine. If the unresolved decision map cannot fit responsibly in the current context, stop with a bounded map of settled decisions, open questions, and recommended next step. When available, name `/wayfinder` as a separate human-invoked option; otherwise name a fresh explicit `slipway-workflow` continuation. Never silently create a cross-session Issue map.

## Choose one coherent work-item level

Choose the level before drafting:

- One observable result that is independently deliverable, verifiable, reversible, leaves a safe repository state, and roughly fits one fresh Agent context is a Change. Keep a cross-layer result as one tracer-bullet vertical slice; keep non-deliverable implementation steps as a checklist.
- A result that necessarily requires several independently useful deliveries is an Objective for later explicit decomposition. Draft provisional vertical slices and their blocker edges, but do not pretend the Objective itself is executable.
- A pure investigation with no deliverable code is a `kind:research` Change whose result is an evidence-backed conclusion; any later code belongs to a separate Change.

For a Change, produce all five independently addressable roles with no runtime inheritance from unreferenced conversation:

- **Outcome** — the observable user result and the problem it resolves.
- **Requirements** — behavior, user stories, interfaces, schemas, and contract decisions, without file paths or code snippets.
- **Acceptance examples** — externally observable checks derived from the testing decisions.
- **Constraints** — architectural decisions, preserved behavior, and boundaries.
- **Non-goals** — explicit exclusions.

For an Objective, instead produce its distinct planning shape:

- **Problem** — why the larger effort exists.
- **Outcome** — the observable destination.
- **Requirements** — behavior shared across the effort.
- **Shared constraints** — boundaries every child must preserve.
- **Non-goals** — explicit exclusions.
- **Changes** — provisional tracer-bullet deliveries, dependencies, and blocking edges for later validation by `slipway-decompose`.

Map upstream planning without inheriting its execution ownership: a foggy multi-result destination becomes an Objective; a spec or PRD becomes the appropriate Change or Objective roles above; an external ticket list is only planning input. Matt's `to-tickets -> implement -> code-review` path stops at this boundary—Slipway Decompose and Run own decomposition and execution after publication.

## Stop at the publication boundary

Present the complete work-item draft and its intended publication shape, not an approved publication plan. `slipway-propose` alone owns repository re-fetch, operation and item identities, exact bodies and digests, relation revisions, the exact external-write plan, and one current user confirmation for that plan.

Name one next command without invoking it:

- one Change draft -> explicit `slipway-propose`;
- one Objective draft -> explicit `slipway-propose`, followed after publication by explicit `slipway-decompose`;
- work deliberately kept private, urgent, tiny, or untracked -> an explicit ad-hoc `slipway-run` on the sharpened goal, with no Issue.

Never publish, patch, label, or relate an Issue from this capability. Never invent a second Issue format. Treat an ordinary spec or tracker Issue—including output from `to-spec` or `to-tickets`—as non-authoritative planning input. Only a marker-valid `change/v2` Issue can be an executable Issue source; an Objective is never executable.

After publication, the host must report the canonical Change URL and number. The user then separately invokes `slipway-run` on that Change with a deliberate `--budget N` in `1..1000`. The host fetches and attests the Issue, builds the Source Bundle envelope, and passes it with `--source-file`; never hand a bare Issue number to the CLI or make the CLI fetch the network. Publication and Run start are two deliberate authorization boundaries, so this workflow stops before both.

## Report the Run boundary faithfully

Once explicitly started elsewhere, a Run advances one Action at a time within its pinned Requirements and budget until it is `paused`, `stopped`, or `ended`. It honors skip, stop, take-over, and reorder immediately. Run Clarify may still pause for a genuine decision; the quality of this first-half draft reduces that need but cannot disable it.

Review is advisory and read-only and creates no automatic repair or re-review loop. `ended` means only that the automatic Action queue is empty—not that the work is correct, complete, or shippable. A `budget_exhausted` pause is normal and resumable. On resume, explicit `--budget N` replaces the remaining budget with `N`; omitting it preserves a positive remainder and replenishes zero to `max(initial_budget, 3)`. The replacement applies only when the mutation actually resumes the Run. New findings remain visible for the user to place in another Change or another Run on the same Change.

Certify nothing. Report repository facts investigated, decisions settled, the exact draft produced, optional artifacts actually created, remaining uncertainties, and the explicit next command. Name every activity not performed.

When Matt Pocock's user-invoked skills are installed, the human may independently choose `/grill-with-docs` or `/grill-me`, `/wayfinder`, `/to-spec`, or `/to-tickets`. That wizard path is an alternative set of explicit commands, never an implicit subroutine of this workflow.
