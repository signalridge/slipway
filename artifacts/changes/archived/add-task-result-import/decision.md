# Decision

## Alternatives Considered

- Wrapper-only result import: add `--result-file`, parse compact JSON, and call
  the existing flag-backed writer internally. This is too weak because the run
  version still has no engine-owned source and re-execution cannot reliably
  advance versions.
- Engine-owned run boundary plus compact import: add explicit active execution
  run state at the wave-plan materialization boundary, derive ledger fields from
  wave-plan/change state, and update B-owned guidance. This satisfies the
  central Workstream B thesis with a bounded model and command change.
- Full S3-to-S2 lifecycle reopen: add a broader lifecycle operation that moves
  review work back into implementation. This may be necessary later, but doing
  it wholesale would widen B beyond the compact result import and run-boundary
  objective.

## Selected Approach

Use an engine-owned active execution run boundary plus compact result import.
Persist the active run version on the smallest durable authority that travels
with the execution plan, prefer the wave-plan materialization transaction as the
place where the version is chosen, and make `evidence task --result-file` derive
all internal ledger fields from the current change and wave plan.

If implementation confirms that review-driven re-execution cannot be modeled by
existing forward-only lifecycle state, add a narrow, tested re-execution hook
that advances the active run version and rematerializes execution authority
without pretending `fix` already re-enters S2.

## Interfaces and Data Flow

- New executor result JSON input:
  `task_id`, `verdict`, `evidence_ref`, `changed_files`, `blockers`, and
  optional `session_id`.
- Rejected result JSON fields:
  `run_summary_version`, `task_kind`, `target_files`, `captured_at`,
  `freshness_inputs`, and `input_hash`.
- Data flow:
  executor result JSON -> `cmd/evidence.go` import parser -> current wave plan
  task lookup -> engine-owned active run version -> `TaskEvidencePayload` ->
  runtime task evidence file -> `progression.ParseTaskEvidence` validation ->
  wave-orchestration evidence -> execution summary materialization.
- Guidance flow:
  source templates and toolgen metadata -> generated command docs/manifest ->
  agent-facing wave-orchestration instructions.

## Rollout and Rollback

Rollout is local to this PR: add tests first, implement result import and
run-boundary changes, update B-owned guidance, regenerate/validate generated
surface artifacts, then run focused and full Go test suites.

Rollback is to remove the result-file parser, active run-version model/state
change, and guidance updates, then restore the old tests and manifest examples.
Verification command for rollback or forward rollout is `go test ./...` plus
`go run ./internal/toolgen/cmd/gen-surface-manifest --check`.

## Risk

- The highest risk is silently reusing stale run versions after S3 repair work.
  This is mitigated by making run version engine-owned and adding a regression
  around re-execution version advancement.
- The second risk is weakening scope safety by over-compressing the result
  schema. This is mitigated by retaining `changed_files` and existing
  scope-contract checks.
- Generated guidance drift is mitigated by updating templates/toolgen/docs and
  asserting the new command shape in tests.
- Compatibility risk from old flags is managed by treating them as
  non-default/internal during this B slice rather than teaching them to agents.
