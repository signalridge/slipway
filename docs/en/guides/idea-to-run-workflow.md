# Idea-to-Run workflow

`slipway-workflow` is the explicitly invoked, host-side bridge from a rough idea to a Slipway work-item draft. It gives the AI autonomy over investigation, interview structure, and synthesis without turning those activities into a persistent governance pipeline. It adds no CLI command, Run state, quality gate, or automatic repair loop.

The capability name is shown below without host-specific syntax. Invoke the generated entry in the way your [host adapter](../reference/adapters.md) documents it.

## Authority boundaries

One explicit `slipway-workflow` invocation authorizes only the first row:

| Phase | AI may do autonomously | User still owns |
| --- | --- | --- |
| Investigate and draft | Read repository facts, research bounded unknowns, structure the interview, choose Change versus Objective, and synthesize the draft | Genuine product/risk decisions; any separately requested artifact write |
| Publish | Nothing automatically | A separate `slipway-propose` invocation and one current confirmation for its exact external-write plan |
| Decompose | Nothing automatically | A separate `slipway-decompose` invocation for a published Objective |
| Execute | Nothing automatically | A separate `slipway-run` invocation, its source choice, and its initial budget |

The workflow is “stateless” only because it creates no Slipway Run, journal, or cross-phase cursor. Conversation, documents, tracker Issues, prototypes, and code changes are still state. The default idea-to-draft path is read-only. An artifact-producing detour is allowed only when the user's request already authorizes that artifact and scope; it is reported and never becomes an executable source by itself.

## Autonomous first half

The host first inspects the current Git state, relevant code and tests, and repository verification conventions. It asks no question whose answer can be discovered. When a real human decision remains, it asks exactly one question, recommends an option based on observed facts, explains alternatives and trade-offs, and waits. A complete request can proceed with zero questions.

Matt's installed, model-invocable `/grilling` primitive is an optional accelerator for the decision interview. The workflow may run the `/grilling` skill because it is model-reachable, and then honors its one-question-at-a-time and shared-understanding confirmation rules. Its absence never blocks the workflow and never causes an installation.

Artifact-producing primitives are not a read-only shortcut: `/domain-modeling` writes glossary or ADR material, `/research` writes a Markdown report, and `/prototype` writes throwaway code. The workflow invokes one only when the user's existing request separately authorizes that exact artifact and scope; otherwise it investigates directly and reports the remaining uncertainty. A large, unresolved decision map that cannot fit safely in one context is not silently persisted: the host returns a bounded map and names either a fresh explicit workflow continuation or the human-invoked `/wayfinder` option when it is installed.

## Mapping Matt Pocock's methods

Matt's `/grill-me`, `/grill-with-docs`, `/wayfinder`, `/to-spec`, `/to-tickets`, `/implement`, and `/ask-matt` front doors are user-invoked: another skill must not fire them. Slipway therefore internalizes the useful disciplines and leaves those original commands as an optional wizard path. Regardless of `code-review`'s invocation setting, this workflow does not invoke it because execution and Review belong to Slipway Run after handoff.

| Matt method or output | Slipway workflow meaning |
| --- | --- |
| `grill-me` / `grill-with-docs` | Their model-invocable `/grilling` primitive may be reused when installed; the front doors remain human-only and documentation remains a separately authorized write |
| `wayfinder` destination or issue map | One Objective when several independent deliveries are necessary; a durable multi-session map remains a separately invoked external workflow |
| `to-spec` spec/PRD | Planning input normalized into the Change or Objective sections below |
| `to-tickets` tracer bullets | Provisional Objective Changes, then marker-valid child Changes created by explicit `slipway-decompose` |
| `implement` / `code-review` | Not used before handoff; Slipway Run owns Implement and advisory Review |

`slipway-workflow` never invokes `/grill-me`, `/grill-with-docs`, `/wayfinder`, `/to-spec`, `/to-tickets`, `/implement`, `/code-review`, or `/ask-matt`. A human may still invoke the user-only commands directly as a separate, multi-command wizard.

## Draft the right level

A **Change** is one independently deliverable, verifiable, reversible result that leaves a safe repository state and roughly fits one fresh Agent context. It contains five independently addressable roles:

- Outcome
- Requirements
- Acceptance examples
- Constraints
- Non-goals

An **Objective** is a larger destination that necessarily needs several independently useful deliveries. It is planning-only and contains:

- Problem
- Outcome
- Requirements
- Shared constraints
- Non-goals
- Changes, including provisional tracer-bullet slices and blocker edges

Only a self-contained, marker-valid `change/v2` Issue can start an issue-backed Run. An Objective cannot. A pure investigation is a `kind:research` Change that delivers an evidence-backed conclusion rather than code.

## Publication and source handoff

The workflow returns the complete draft and intended publication shape, then stops. It does not claim to have produced the exact approved publication plan. `slipway-propose` owns the repository re-fetch, operation identities, exact bodies and digests, relationship revisions, preview, and one current user confirmation. See [Using GitHub Issues](github-issues.md).

An ordinary spec or tracker Issue—including an Issue created by `to-spec` or `to-tickets`—is non-authoritative planning input. Do not pass it directly to a Run. After a Change is published, the host reports its canonical URL and number. A later explicit `slipway-run` invocation makes the host fetch and attest that exact Change, build a temporary Source Bundle envelope, and pass it to the local CLI through `--source-file`. The CLI does not fetch GitHub, and a bare Issue number is not the CLI source.

For small, private, urgent, offline, or deliberately untracked work, the workflow may instead recommend a separate ad-hoc `slipway-run` on the sharpened goal. That is an explicit source choice, not a shortcut that lets the workflow start execution.

## What “run autonomously” means

Start an issue-backed Run with a deliberate budget in `1..1000`. Within the pinned Requirements and available budget, the Run advances one Action at a time until `paused`, `stopped`, or `ended`. Run Clarify can still pause for a genuine decision; a strong draft reduces this need but does not disable it. `budget_exhausted` is a normal resumable pause. On resume, explicit `--budget N` replaces the remaining budget with `N`; omitting it preserves a positive remainder and replenishes zero to `max(initial_budget, 3)`. The replacement applies only when the mutation actually resumes the Run.

Review is read-only and advisory. Findings do not create an automatic Implement/re-review loop, and `ended` means only that the automatic Action queue is empty—not that the change is correct, complete, or shippable. The user can place findings in a new Change or start another Run on the same Change. See [Runs, recovery, and privacy](runs-and-recovery.md).
