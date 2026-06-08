# Command Reference

Most routed commands support `--json` when structured output is useful. Two
exceptions: `slipway init` is setup-only (`--tools`/`--refresh`, no `--json`), and
`slipway validate` emits JSON only (its `--format` flag shapes `--list-focuses`
output, not the main report).

## Core Lifecycle

| Command | Class | Purpose |
| --- | --- | --- |
| `slipway new [description]` | mutation | Create a governed change starting at intake. |
| `slipway next` | query | Inspect the next actionable skill or blocker without advancing state. |
| `slipway run` | mutation | Advance until a skill, blocker, checkpoint, or done-ready outcome is surfaced. |
| `slipway status` | query | Show lifecycle state, blockers, progress, and next actions. |
| `slipway done` | mutation | Finalize a done-ready change and archive it. |

When a governed change has stale evidence, `slipway next` remains read-only and
reports recovery guidance. `slipway run` is the mutating recovery path: it
reopens the earliest affected authority, clears that authority and downstream
verification files, preserves compatible runtime task evidence, and returns the
side effects in JSON and human-readable output. Planning freshness is keyed on
the structural task-plan hash; `wave-plan.yaml` `generated_at` is materialization
time for display/audit and is not a freshness authority.

## Creation Options

```bash
slipway new "add install docs" --preset standard
slipway new "docs-only change" --profile docs
slipway new --from-doc docs/installation.md "refresh install docs"
slipway new "small fix" --trivial
slipway new "auth refactor" --discuss   # carry open questions forward into context
slipway new "schema migration" --full   # force fresh final-closeout evidence before ship
```

Presets control gate strictness: `light`, `standard`, or `strict`.

Workflow profiles shape checks: `code`, `docs`, `research`, `config`, or `meta`.

`--discuss` persists unresolved gray areas into context before execution;
`--full` requires a refreshed final-closeout verification before the ship gate.

## Situational Commands

| Command | Class | Purpose |
| --- | --- | --- |
| `slipway init` | mutation | Initialize `.slipway.yaml` and optional AI-tool adapters. |
| `slipway preset <level>` | mutation | Confirm or change the active change preset. |
| `slipway validate` | query | Recompute evidence and gate readiness without advancing. |
| `slipway review` | mutation | Run explicit artifact-code alignment review (valid in S2_EXECUTE/S3_REVIEW/S4_VERIFY, after execution-summary evidence exists). |
| `slipway checkpoint` | mutation | Pause execution for a task-level human response (S2_EXECUTE; `--task-id` required, `--allowed-responses` required for `--type decision`). |
| `slipway evidence task` | mutation | Record supported runtime task evidence for wave execution. |
| `slipway pivot` | mutation | Reroute an active change (S1_PLAN/S2_EXECUTE/S3_REVIEW/S4_VERIFY) or rescope it (S2_EXECUTE back to S0_INTAKE). |
| `slipway abort` | mutation | Abort the active execution session without archiving the change. |
| `slipway cancel` | mutation | Cancel an active change and archive terminal state. |
| `slipway delete` | mutation | Discard an abandoned governed change: its bundle, runtime binding, optional worktree, or an archived record (dry-run by default). |
| `slipway repair` | mutation | Run bounded local integrity repairs. |

`slipway cancel` and `slipway delete` are not the same operation. `cancel`
takes an **active** change to a terminal `cancelled` status and **archives** it
under `artifacts/changes/archived/<slug>` so the decision stays in the audit
trail. `delete` instead **discards local governed state** for an abandoned,
accidental, or partially-deleted change — its bundle, its runtime binding, and
(with `--worktree`) the bound git worktree — and with `--archived` can purge an
already-archived record. `delete` is dry-run by default: a bare
`slipway delete --change <slug>` prints the removal plan and deletes nothing;
pass `--yes` to execute. It fails closed — it refuses to remove a worktree with
uncommitted tracked changes unless `--force`, and never deletes the
implementation or pushed PR branch. When a change is abandoned, broken, or bound
to another worktree, `slipway status`/`slipway next` and recovery output route
to the exact `slipway delete --change <slug>` command.

## Diagnostics

| Command | Class | Purpose |
| --- | --- | --- |
| `slipway learn --preview` | query | Preview governance learning proposals from lifecycle evidence. |
| `slipway stats` | query | Show repo-wide governance freshness and workflow statistics. |
| `slipway health` | query | Show repo-local integrity and repairability findings. |
| `slipway codebase-map` | mutation | Create or refresh advisory repo-scoped context under `artifacts/codebase/`. |
| `slipway instructions <artifact>` | query | Show the template, quality bar, and — inside a change — resolved output path + dependency graph for a governed artifact or codebase-map doc. |

`slipway instructions <artifact>` serves the artifact template plus its quality
bar so an authoring skill writes the real file directly — the engine owns
structure, the skill owns substance, and there is no seeded body to replace.
Inside a governed change it also returns the resolved output path, the
dependency/unlock graph, and tagged background (`context`/`rules`) the skill must
honor but never copy into the artifact. It covers the six governed bundle
artifacts (`intent`, `requirements`, `decision`, `research`, `tasks`,
`assurance`) and the repo-scoped codebase-map docs (`stack`, `architecture`,
`structure`, `conventions`, `integrations`, `testing`, `concerns`).
In `--json`, `context_is_baseline: true` marks codebase-map baseline context
that should be preserved and extended into the authored doc; when absent or
false, `context` is background to honor but not copy.

`codebase-map --json` reports `status: "baseline"` when documents contain only
CLI-detected repository facts. Baseline docs are useful starting context, not
authored brownfield analysis; callers should refine them with source-backed
findings before relying on them for planning or review.

Codebase maps under `artifacts/codebase/` are git-tracked by default — durable
brownfield context is meant to be reviewed and shared, not hidden as local-only
state. Existing repositories auto-migrate the next time `slipway new`,
`slipway codebase-map`, or `slipway init` rewrites the managed `.gitignore`
block (`next`/`run`/`status`/`repair` do not reconcile it); the `evidence/`,
`events/`, `verification/`, and `.worktrees/` paths stay ignored.

`next --json` and `run --json` include `input_context.codebase_map_status`
(and per-doc `input_context.codebase_map_doc_states`) in the default,
non-`--diagnostics` handoff so callers can tell whether the referenced map is
durable. Values mirror the `slipway codebase-map` assessment (`missing`,
`scaffold_only`, `baseline`, `partial`, `populated`); a missing map reports
`"missing"` with each doc `missing` rather than an omitted field. When a
map-consuming planning skill (research-orchestration or plan-audit) is next and
the status is `scaffold_only` or `baseline`, `warnings` carries a non-blocking
codebase-map advisory.

## Output And Hydration Flags

Query and review commands share a consistent output-and-hydration surface, kept
aligned with the CLI by a reverse flag-contract test:

- `--format <text|yaml|json>` — `status` supports the full set; `review`,
  `validate`, `repair`, and `health` use `--format` only to shape
  `--list-focuses` output (`text|json`). `--json` is shorthand for
  `--format json` where supported.
- `--hydrate` / `--hydrate-ref <skill-id>/<name>` — `status`, `review`, and
  `health` append selected hydrate reference bodies to text output;
  `--hydrate-ref` restricts hydration to a named reference (repeatable).
- `--focus <alias>` / `--list-focuses` — `status`, `health`, `review`,
  `validate`, and `repair` accept a public focus override; run
  `<command> --list-focuses` to enumerate. Known aliases: `status`/`health` →
  `incident`; `review` → `sast`, `calibration`; `validate` → `sast`, `property`,
  `mutation`, `spec-trace`; `repair` currently exposes none.
- `status --root` prints the canonical Slipway scope root; `status --stats`
  shows workspace diagnostics (active count, stale summaries, integrity issues).
- `next --no-auto-pass` reports skill eligibility instead of auto-passing;
  `next --context-guard` emits context-budget guard messages in hook format.
- `done --all-ready` archives every active change that is currently done-ready.
- `pivot --reroute` re-evaluates the routing/discovery decision and re-enters
  S1_PLAN (valid in S1_PLAN/S2_EXECUTE/S3_REVIEW/S4_VERIFY); `pivot --rescope`
  (valid only in S2_EXECUTE) returns the change to S0_INTAKE to amend scope,
  clearing the Approved Summary for re-confirmation. Both set
  `needs_discovery=true`.

## Useful JSON Invocations

```bash
slipway new --json "refresh docs"
slipway next --json --diagnostics
slipway run --json --diagnostics
slipway status --json
slipway validate --json
slipway evidence task --task-id t-01 --run-summary-version 1 --task-kind code --verdict pass --evidence-ref "test:go-test" --json
slipway health --doctor --json
```

`next --json` includes `next_skill.name` for AI-tool handoff. The host tool derives the local `SKILL.md` path from its own adapter conventions.

When diagnostics are enabled, review-state handoff JSON can also include:

- `next_skill.display_name`, `next_skill.blocking_name`, and `next_skill.resolution_reason` when the conceptual stage differs from the actionable missing skill.
- `next_skill.review_context.required_artifact_layers` and `next_skill.review_context.required_implementation_layers`, which map to exact gate tokens such as `layer:R0=pass`, `layer:R3=pass`, `layer:IR1=pass`, and `layer:IR3=pass`.
- top-level `confirmation_requirement`, which reports whether a hard stop needs fresh user confirmation, whether prior authorization is sufficient, whether `--resume-response` is supported at this stop (`resume_response_supported`), the next operator action as human prose (`next_action`), a machine-readable `next_action_kind` (`skill_handoff` | `checkpoint_resume` | `preset_confirmation` | `command` | `blocker_resolution` | `confirmation` | `none`), and the exact `next_command` to run when one is runnable as-is. `next_command` is empty for stops that need operator-supplied input — notably `checkpoint_resume`, which requires a `--resume-response` argument and is therefore signaled by `resume_response_supported` rather than an exact command. Branch on `next_action_kind`/`next_command`; treat `next_action` as display prose only.
- `freshness_diagnostics`, which reports stale source/evidence pairs, field-level execution input mismatches, path authority, and the next regeneration action.

`validate --json` is the active-readiness authority: it answers whether the
current governed state can advance now and mirrors actionable review handoff
through `actionable_next_skill`, including `required_tokens` for the exact layer
references the actionable skill must supply. `run --json` is the mutating
transition surface: `advanced` reports what this invocation changed, while
`blockers` reports the current stop condition after any transition. A successful
advance can therefore be followed by error-severity blockers for the next
required skill. `health --governance --json` is diagnostic health feedback; use
it to inspect controls and traceability details, not as the lifecycle authority
for whether `run` just advanced.

`status --json` includes `freshness_diagnostics` when execution evidence is known stale and marks each `artifact_dag` node with `blocking` plus `blocking_reason` so draft planning artifacts are not mistaken for current review blockers.

`validate --change <slug>` selects an explicit active change. If the slug names
an archived terminal change, the command fails with
`archived_change_not_validatable` and returns the terminal status plus archived
`change.yaml` path instead of the generic no-active diagnostic. This is an
active-readiness contract: `validate` proves the currently active governed state
before `done`; it is not a post-archive audit surface for frozen bundles.

`slipway evidence task` writes the flat runtime task JSON consumed by
wave-orchestration sync. Required flags: `--task-id`, `--run-summary-version`,
`--task-kind`, `--verdict`, `--evidence-ref`. Optional: `--changed-file` and
`--target-file` (each repeatable), `--blocker <code[:detail]>` (repeatable;
supply for non-pass verdicts), `--captured-at <RFC3339Nano>` (defaults to now),
`--session-id`, and `--change <slug>`. The command computes `freshness_inputs`,
validates task kind/verdict/blockers, and refuses unknown or path-unsafe task IDs
instead of relying on hand-written JSON.
`freshness_inputs` includes the current wave-plan `tasks_plan_hash` so task
evidence cannot be reused after `tasks.md` semantically changes.

Accepted governance skill evidence is additionally bound by
`verification/evidence-digests.yaml`, an engine-owned local file that records the
content digest of the inputs each passing skill certified. The entry also stores
the accepted verification verdict timestamp so a newer host re-run verdict can
replace a stale digest during mutating advancement. Read-only commands only
compare the stored digest with current inputs; mutating advancement paths stamp
the file when passing evidence is accepted. Diff-class review digests certify the current
working diff (`git diff HEAD` plus non-ignored untracked reviewable files,
excluding Slipway governed/runtime artifacts under `artifacts/changes/**`), so
a commit between review and finalization can make read-only projections report
the review stale until the owning review stage is run again through
`slipway run` against the new diff boundary. If required digest evidence is
missing or stale, the owning governance skill is reported stale and must be
re-run.

`repair --json` separates `applied_repairs` from `unrepaired_drift`. Applied repairs are bounded local fixes that were actually performed; unrepaired drift includes a target, reason, and `next_action` for evidence or artifact work that Slipway did not mutate automatically. Ready execution summaries that are stale only because runtime task evidence is newer can be rebuilt from current wave-backed task evidence; stale planning-source drift remains unrepaired. Empty orphan active-bundle directories left behind after archive cleanup are removed as `empty_orphan_bundle` applied repairs; non-empty orphan bundles remain operator-reviewed integrity findings. Missing task-evidence blockers include the runtime task evidence path, `record_command=slipway evidence task`, and the required flat JSON fields: `task_id,run_summary_version,task_kind,verdict,evidence_ref,captured_at,freshness_inputs`. `health --json` findings include `active_change_blocking` and `active_change_impact`; advisory codebase-map warnings are marked non-blocking for the active change.

`done --json` archives done-ready worktree-bound changes even when source files
or non-active governance artifacts are still uncommitted, returning a
non-blocking `worktree_dirty_warning` with `worktree_dirty_files` so operators
commit those files together with the archived bundle. `done` never removes the
worktree, and `git worktree remove` already refuses to drop a dirty worktree, so
the advisory replaces a hard block. The active `artifacts/changes/<slug>/` bundle
is excluded from the advisory because `done` rewrites it into
`artifacts/changes/archived/<slug>/`; sibling or archived bundles are listed.

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
