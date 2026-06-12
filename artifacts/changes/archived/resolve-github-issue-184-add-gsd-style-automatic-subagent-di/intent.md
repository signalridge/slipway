# Intent

## Summary
Resolve GitHub issue #184: add GSD-style automatic subagent dispatch contract for parallel waves
## Complexity Assessment
complex

This is a generated-surface contract change with tests and sync requirements.
It changes how the `wave-orchestration` host tells agents to dispatch
`parallel: true` waves, including Codex-specific subagent adapter semantics,
fallback behavior, executor result shape, evidence ownership, and single
worktree dispatch safety.

## Guardrail Domains
None detected. The work changes generated instructions, references, and tests;
it does not alter auth/authz, credentials/PII, financial logic, schema
migrations, irreversible operations, or external API contracts.

## In Scope
- Resolve GitHub issue #184 as the authoritative scope.
- Update `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` so
  `parallel: true` means real executor subagent fan-out instead of same-context
  inline work.
- Update
  `internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md`
  with executable runtime adapter semantics for Claude-style runtimes, Codex,
  fallback/degraded operation, spawn/wait/collect/close, and a stable executor
  result contract.
- Remove or demote the old Codex `codex -q --task` path so it is not the
  primary fan-out mechanism.
- Preserve Slipway governance authority: task evidence is written through
  `slipway evidence task`; executors do not self-stamp governed freshness.
- Define the single worktree parallelization contract: executor agents share
  the current worktree, so the coordinator must preflight the current wave's
  target scope before dispatch, record executor handles, wait with bounded
  recovery semantics, and run post-wave conflict/integration checks before the
  next wave.
- Make Codex's explicit subagent authorization boundary visible: when the
  runtime policy requires user authorization for `spawn_agent`, the coordinator
  must stop and ask instead of forcing a spawn or silently inlining the wave.
- Add or update generated-surface contract tests under the existing template or
  toolgen test surfaces.
- Regenerate tracked generated agent surfaces if the repository expects them to
  stay in sync with template sources.

## Out of Scope
- Building a full internal Slipway scheduler or executor runtime inside the Go
  CLI.
- Copying GSD project-state files, phase model, commit protocol, or fail-soft
  skip paths wholesale.
- Moving governed evidence stamping or verification ownership into subagents.
- Weakening `execution.parallelization: off`; explicit sequential opt-out must
  remain clean.
- Running shared-worktree-wide integration commands concurrently inside every
  executor.
- Adding per-executor git worktree isolation, worktree merge/cleanup machinery,
  or GSD's commit protocol.

## Constraints
- Use the current Slipway CLI and governed workflow as lifecycle authority.
- Use local `github.com/open-gsd/gsd-core` only as an implementation reference;
  Slipway artifacts and behavior remain repo-native.
- Keep coordinator context thin by passing paths and bounded task briefs to
  executors; executor source reads and implementation debugging belong in
  executor contexts.
- Capable runtimes must not silently inline a `parallel: true` wave in the
  coordinator context.
- Incapable runtimes must report degraded or unsupported dispatch explicitly and
  stop or ask for operator direction when inline execution would pollute the
  coordinator context.
- Single worktree executor fan-out is allowed only for dependency-free,
  target-disjoint tasks; the coordinator owns pre-dispatch target checks,
  post-result changed-file conflict checks, and post-wave integration.

## Acceptance Signals
- Template/reference tests prove Codex generated guidance uses `spawn_agent`
  or equivalent spawn/wait/collect/close semantics instead of relying on
  `codex -q --task` as the primary fan-out path.
- Tests require fresh-context wording such as `fork_context: false` where the
  available Codex tool supports it.
- Tests require coordinator stop-work wording while executor agents are active.
- Tests require executor result fields: `task_id`, `verdict`, `changed_files`,
  `test_summary`, `evidence_ref`, and `blockers`.
- Tests require `slipway evidence task` to remain the task evidence writer and
  forbid subagent self-stamping of governed freshness.
- Tests reject silent same-context inline execution for `parallel: true` waves
  on runtimes with a real subagent primitive.
- Tests require single worktree preflight wording for target overlap, structured
  executor agent/session references, wait/stall recovery, and Codex explicit
  subagent authorization handling.
- Focused verification passes:
  `go test -count=1 ./internal/tmpl ./internal/toolgen`.
- Final verification passes:
  `go test -count=1 ./...`, `git diff --check`, and `go run . validate --json`.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] Which exact GSD Codex adapter semantics should be borrowed into Slipway,
  and which fail-soft or worktree-isolation behavior must stay out of scope?
- [x] Which generated surfaces are tracked and must be regenerated after editing
  the `wave-orchestration` template/reference sources?
- [x] Which existing tests should be expanded versus where new contract tests
  should live?

## Deferred Ideas
- A typed dispatch-mode field in wave-orchestration verification instead of
  string references.
- Runtime capability hints from `slipway next --json`.
- A helper command/reference generator that emits one bounded executor brief per
  task.
- Typed executor dispatch records with per-task handles and wait state.

## Approved Summary
User objective received on 2026-06-12: use the Slipway governed workflow to
resolve all issues described in GitHub issue #184 until done-ready, make the
best choice when encountering non-sensitive blockers, and reference local GSD
while implementing.

Approved scope: strengthen Slipway's generated `wave-orchestration` contract so
`parallel: true` waves require real executor subagent fan-out where the runtime
supports it, Codex guidance maps dispatch to `spawn_agent`-style
spawn/wait/collect/close behavior, capable runtimes cannot silently inline the
wave in coordinator context, incapable runtimes report degraded or unsupported
dispatch explicitly, single worktree fan-out carries target preflight,
executor-handle evidence, wait/stall recovery, and post-wave conflict checks,
and task evidence remains owned by `slipway evidence task`. Do not build an
internal scheduler, copy GSD's lifecycle, add per-executor worktrees, or move
governed freshness stamping into subagents. Primary acceptance is
generated-surface contract tests plus focused and full Go/Slipway verification.
