# Intent

Forward-only governance lifecycle redesign: make lifecycle ownership explicit,
remove retired lifecycle-regression repair surfaces, add a clean S3 review-fix
dispatch path, and keep governance repair inside the current change unless the
user intent itself changes.

## In Scope

- Add explicit primary lifecycle commands:
  - `slipway intake` for S0 intake clarification and authorization.
  - `slipway plan` for S1 plan artifact authoring and same-intent amendments.
  - `slipway implement` for S2 implementation wave orchestration.
  - `slipway review` for S3 review convergence and reviewer feedback repair.
  - `slipway fix` for S3 review-finding repair dispatch in a clean context.
  - `slipway done` for finalization.
- Keep `slipway run` as a shortcut driver only. It delegates to the current
  primary stage command and reports `delegated_to` in JSON.
- Remove retired lifecycle-regression repair command surfaces completely. No
  replacement top-level planning/amendment command is introduced; `fix` is
  limited to S3 review-finding repair dispatch and `repair` remains local
  integrity.
- Treat same-intent scope changes as current-change amendments. The current
  stage updates the relevant artifacts and evidence, then continues forward.
- Treat objective/intent changes as `new_change_required:intent_conflict`; the
  operator starts a new governed change instead of rewriting this one.
- Let review own the plan/code/evidence gates. Review feedback is repaired
  through `slipway fix` by separate fresh-context repair subagents and closed by
  affected-reviewer rereview.
- Keep same-intent `tasks.md` amendments in S3 review/fix once review has
  started; do not force S2 replay solely because the task plan was aligned.
- Keep verification folded into S3: goal-verification is a selected S3 peer, and
  final-closeout is the last ship summary rather than a chained S4 stage.
- Be honest that current shared reviewer inputs can require rerunning the full
  selected review set after code repairs; file-scoped minimal rereview is future
  work, not a promise in this change.
- Keep S2 wave execution continuous across wave boundaries inside the
  wave-orchestration host; no `run` call is required merely to move from one
  wave to the next.
- Preserve the hard fail-closed boundaries for pre-parallel write safety and
  final ship authorization.

## Out Of Scope

- Adding compatibility aliases for retired repair commands.
- Creating a separate top-level amendment command.
- Reusing `slipway repair` for review findings.
- Implementing file-scoped reviewer ownership.
- Letting executor agents silently write outside declared task scope.
- Bypassing review or ship authorization with self-certified repair.

## Acceptance Signals

- Root help, generated command surfaces, docs, and manifest expose
  `intake`, `plan`, `implement`, `review`, `done`, plus `run` as shortcut.
- `run --json` includes `delegated_to`.
- Wrong-stage primary commands fail closed with a current-stage remediation.
- Same-intent scope drift guidance uses scope amendment wording and routes S3
  findings through `slipway fix`.
- Intent conflict review signals use `new_change_required:intent_conflict`.
- Review repair evidence can carry `context_origin:stage=fix=<handle>` from the
  clean repair subagent context.
- The old command surfaces, templates, docs entries, reason codes, and tests are
  absent from the active product surface.
