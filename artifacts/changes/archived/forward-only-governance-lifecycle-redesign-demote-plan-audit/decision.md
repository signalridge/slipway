# Decision

## Alternatives Considered

- Keep `run` as the only lifecycle driver. Rejected because it hides the current
  stage from agents and forces workflow meaning into private inference.
- Add a separate rebuild/restart command for scope changes. Rejected because
  same-intent amendments belong inside the current change, while intent
  conflicts should start a new governed change.
- Keep recovery as a lifecycle rewind mechanism. Rejected because review should
  own alignment gates and repair convergence without mutating state backward.
- Reuse `slipway repair` for review findings. Rejected because `repair` is
  bounded local integrity repair; using it for reviewer feedback would recreate
  the ambiguous repair surface this change removes.
- Make goal-verification a post-review chain step. Rejected because the folded
  S3 lifecycle treats goal-verification as an unordered selected review peer;
  final-closeout is the only final summary step.
- Preserve retired command aliases. Rejected because compatibility aliases keep
  teaching agents the removed workflow.

## Selected Approach

Use explicit lifecycle stage commands plus a shortcut driver:

- `new` creates a governed change.
- `intake`, `plan`, `implement`, `review`, and `done` are the primary lifecycle
  commands.
- `fix` is the S3 review-finding repair-dispatch command. It is not a lifecycle
  rewind and not local-integrity repair.
- `run` remains as automation sugar and reports `delegated_to`.
- `next` stays read-only.

Same-intent scope changes are handled as current-change amendments by the stage
that discovers them. They are not separate lifecycle commands. Intent conflicts
are not amendments; they produce `new_change_required:intent_conflict` and a new
governed change.

Review remains the gate for plan/code/evidence alignment. Review feedback is
fixed through `slipway fix`: the host dispatches a fresh-context repair
subagent with one consolidated repair brief for the current `repair_batch_id`,
the subagent changes code/artifacts/evidence inside the current intent, and the
affected reviewers close the findings on rereview. The host collects all
selected-reviewer findings before dispatching repair, then consolidates by root
cause so a file or surface is not churned by piecemeal fixes. Rereview evidence
records the reviewer context and, when a repair occurred, the repair context
with `context_origin:stage=fix=<handle>`.

The live lifecycle is S0/S1/S2/S3/DONE. `S4_VERIFY` is retired. The selected S3
review set includes goal-verification as an unordered peer; final-closeout is
strictly last relative to the selected S3 peer set. The two hard fail-closed
boundaries are pre-parallel dispatch safety and final ship authorization.

Plan-audit is the S1 review that allows S2 to start. It reviews the durable plan
bundle itself, including task dependency and target-file shape, but it does not
approve `wave-plan.yaml` as a frozen execution authority. During S2, Slipway
derives the wave projection from the current `tasks.md`; the persisted
`wave-plan.yaml` is an execution cache/audit artifact whose hash is used for
task evidence freshness, not to forbid normal task updates.

After S3 starts, same-intent `tasks.md` amendments are review inputs, not S2
replay triggers. Raw task-plan hash drift remains diagnosable, but status,
validate, next, and closeout reuse project task-plan-only drift to the S3
review/fix loop so task alignment can converge without moving lifecycle state
backward or re-running wave-orchestration solely for the task edit.

S3 reviewer freshness is anchored by `suite-result.yaml` and shared reviewer
input digests. Because the current reviewer input model includes shared full
suite/SAST/run-summary inputs and an undifferentiated workspace diff, a changed
selected-reviewer digest stales the full selected set. This is a reliability
tradeoff, not a file-scoped rerun promise.

## Why This Design

The previous direction hid lifecycle meaning behind a single advancing command
and attempted to model ordinary governance repair as special repair commands.
That made agents infer private workflow state and encouraged command churn.

The selected design makes the command model match the lifecycle:

- Humans and agents can name the current stage directly.
- Automation can still use `run` when a shortcut is appropriate.
- Scope changes stay inside the approved change when the intent is unchanged.
- Intent changes are explicit new work, not a rewrite of existing authority.
- Review does the gating work instead of pre-review waves blocking on every
  ordinary alignment issue.
- Review-finding repair has a named command and clean-context contract, so the
  next agent does not need private sequencing knowledge.

## Consequences

- Root help and generated surfaces grow by the primary stage commands plus the
  S3-only `fix` repair-dispatch surface.
- `review` moves from situational support to core lifecycle.
- `implement` is the user-facing command for the `S2_IMPLEMENT` state.
- Existing wave-orchestration remains the implementation host, but it now
  explicitly continues across wave boundaries.
- Retired repair command files, tests, templates, docs, and manifest entries
  are removed rather than deprecated.
- Support skills that previously triggered on the ambiguous `repair` command
  trigger on `fix` when they are about review finding repair.

## Interfaces and Data Flow

The CLI command tree exposes the primary lifecycle commands through Cobra.
Stage commands call the same `nextView`/progression machinery as `run`, passing
the command name into lifecycle events and JSON handoff fields. `run` computes
the primary command for the current state and reports it as `delegated_to`.

Evidence repair data flows through blockers and review alignment targets. Stale
evidence no longer deletes downstream verification files or mutates lifecycle
state backward; status, next, validate, and done render repair guidance from the
shared recovery summary.

Review-finding repair data flows through `slipway fix --json`, selected reviewer
verification records, and rereview evidence references. `fix` surfaces the
repair targets, `repair_batch_id`, and fresh-context contract; it does not
hand-edit verification YAML or satisfy the reviewer on its own. The repair
contract tells the host to collect the whole selected-review finding set first
and dispatch one consolidated repair brief instead of responding to findings
one at a time.

S2 wave execution consumes a current task-derived projection. `repair` may
rematerialize `wave-plan.yaml` from `tasks.md`, but task evidence already
recorded for a previous projection is preserved as historical evidence and
reported stale through freshness blockers when the structural task-plan hash no
longer matches.

## Rollout and Rollback

The rollout is a direct source change with generated-surface checks. Rollback is
the normal git revert of this change before archive/finalization. No compatibility
alias is retained for retired command surfaces.

## Risk

The main risk is drift between source commands, generated skill templates,
surface manifest rows, and active governed artifacts. Focused command tests,
toolgen manifest checks, retired-token scans, and `go test ./cmd` cover that
risk. A second risk is stale historical audit events leaking the retired S2 name;
status rendering maps the retired event value to `S2_IMPLEMENT` without editing
the audit log. A third risk is overselling S1 plan-audit as approval of a frozen
execution schedule; the implementation keeps `wave-plan.yaml` as an S2
projection/cache and routes final alignment to review. A fourth risk is
overselling minimal rereview: this design explicitly accepts full selected-set
rereview when shared reviewer inputs change.
