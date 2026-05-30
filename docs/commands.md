# Command Reference

All routed commands support `--json` when structured output is useful.

## Core Lifecycle

| Command | Class | Purpose |
| --- | --- | --- |
| `slipway new [description]` | mutation | Create a governed change starting at intake. |
| `slipway next` | query | Inspect the next actionable skill or blocker without advancing state. |
| `slipway run` | mutation | Advance until a skill, blocker, checkpoint, or done-ready outcome is surfaced. |
| `slipway status` | query | Show lifecycle state, blockers, progress, and next actions. |
| `slipway done` | mutation | Finalize a done-ready change and archive it. |

## Creation Options

```bash
slipway new "add install docs" --preset standard
slipway new "docs-only change" --profile docs
slipway new --from-doc docs/installation.md "refresh install docs"
slipway new "small fix" --trivial
```

Presets control gate strictness: `light`, `standard`, or `strict`.

Workflow profiles shape checks: `code`, `docs`, `research`, `config`, or `meta`.

## Situational Commands

| Command | Class | Purpose |
| --- | --- | --- |
| `slipway init` | mutation | Initialize `.slipway.yaml` and optional AI-tool adapters. |
| `slipway preset <level>` | mutation | Confirm or change the active change preset. |
| `slipway validate` | query | Recompute evidence and gate readiness without advancing. |
| `slipway review` | mutation | Run explicit artifact-code alignment review from execution onward. |
| `slipway checkpoint` | mutation | Pause execution for a task-level human response. |
| `slipway pivot` | mutation | Reroute or rescope an active change from execution onward. |
| `slipway abort` | mutation | Abort the active execution session without archiving the change. |
| `slipway cancel` | mutation | Cancel an active change and archive terminal state. |
| `slipway repair` | mutation | Run bounded local integrity repairs. |

## Diagnostics

| Command | Class | Purpose |
| --- | --- | --- |
| `slipway learn --preview` | query | Preview governance learning proposals from lifecycle evidence. |
| `slipway stats` | query | Show repo-wide governance freshness and workflow statistics. |
| `slipway health` | query | Show repo-local integrity and repairability findings. |
| `slipway codebase-map` | mutation | Create or refresh advisory repo-scoped context under `artifacts/codebase/`. |

`codebase-map --json` reports `status: "baseline"` when documents contain only
CLI-detected repository facts. Baseline docs are useful starting context, not
authored brownfield analysis; callers should refine them with source-backed
findings before relying on them for planning or review.

## Useful JSON Invocations

```bash
slipway new --json "refresh docs"
slipway next --json --diagnostics
slipway run --json --diagnostics
slipway status --json
slipway validate --json
slipway health --doctor --json
```

`next --json` includes `next_skill.name` for AI-tool handoff. The host tool derives the local `SKILL.md` path from its own adapter conventions.

When diagnostics are enabled, review-state handoff JSON can also include:

- `next_skill.display_name`, `next_skill.blocking_name`, and `next_skill.resolution_reason` when the conceptual stage differs from the actionable missing skill.
- `next_skill.review_context.required_artifact_layers` and `next_skill.review_context.required_implementation_layers`, which map to exact gate tokens such as `layer:R0=pass`, `layer:R3=pass`, `layer:IR1=pass`, and `layer:IR3=pass`.
- top-level `confirmation_requirement`, which reports whether a hard stop needs fresh user confirmation, whether prior authorization is sufficient, and whether the current boundary is a command-only or evidence-continuation step.
- `freshness_diagnostics`, which reports stale source/evidence pairs, field-level execution input mismatches, path authority, and the next regeneration action.

`validate --json` mirrors actionable review handoff through `actionable_next_skill`, including `required_tokens` for the exact layer references the actionable skill must supply. `status --json` includes `freshness_diagnostics` when execution evidence is known stale and marks each `artifact_dag` node with `blocking` plus `blocking_reason` so draft planning artifacts are not mistaken for current review blockers.

`validate --change <slug>` selects an explicit active change. If the slug names
an archived terminal change, the command fails with
`archived_change_not_validatable` and returns the terminal status plus archived
`change.yaml` path instead of the generic no-active diagnostic.

`repair --json` separates `applied_repairs` from `unrepaired_drift`. Applied repairs are bounded local fixes that were actually performed; unrepaired drift includes a target, reason, and `next_action` for evidence or artifact work that Slipway did not mutate automatically. Empty orphan active-bundle directories left behind after archive cleanup are removed as `empty_orphan_bundle` applied repairs; non-empty orphan bundles remain operator-reviewed integrity findings. `health --json` findings include `active_change_blocking` and `active_change_impact`; advisory codebase-map warnings are marked non-blocking for the active change.

## Resume And Checkpoints

If execution pauses on a checkpoint:

```bash
slipway status --json
slipway run --resume-response "approved"
```

If an execution session is resumable:

```bash
slipway run --resume --json
```

Use `health --doctor` before repair or resume when state looks interrupted or inconsistent.

`run --resume` only applies to resumable execution states such as `S2_EXECUTE`.
If the active change is already in review or verify, JSON errors include
`current_state`, `resumable_states`, and a `next_action` directing the operator
back to the normal run, validate, or review-evidence flow.
