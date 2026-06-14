# Architecture

Re-authored for change
`add-engine-enforced-fail-closed-safety-nets-for-shared-workt`.

Question: which Slipway wave-execution seams must turn host-recorded evidence
into fail-closed safety gates for shared-worktree parallel wave execution while
preserving the engine-governs/host-executes boundary?

## Affected Seams

- `internal/engine/wave/wave.go` owns wave planning, static target conflict
  checks, target normalization, and the new coverage helper used by the
  changed-file scope audit.
- `internal/model/wave_execution.go` owns public wave execution data contracts:
  dispatch-mode literals, verification-reference parsing, and wave-run JSON/YAML
  shape.
- `internal/state/wave_execution.go` builds per-wave run records from the
  materialized plan and task evidence. It is the seam where silent dispatch-mode
  inference is removed.
- `internal/engine/progression/wave_sync.go` is the single assembly point for
  task evidence, wave runs, execution summaries, and open blockers. The new
  fail-closed gates attach here so `validate`, `next`, and `status --json`
  observe the same blockers.
- `cmd/next.go` and `cmd/next_wave_plan.go` own the public wave-plan view. The
  new `advisories` field is view-only and must not enter `wave-plan.yaml` or
  freshness hashes.
- `internal/tmpl/templates/skills/wave-orchestration/` owns generated host
  instructions for executor dispatch and evidence references.

## Dependency Flow

`tasks.md` is parsed into wave nodes, materialized as `wave-plan.yaml`, then
rendered through `slipway next --json` for host execution. The host records task
evidence plus wave-orchestration references (`dispatch_mode:*`,
`executor_agent:*`). `SyncGovernedWaveExecution` reads that evidence, builds
`WaveRun` records, adds safety blockers to `ExecutionSummary.OpenBlockers`, and
readiness surfaces those blockers without a separate JSON pipeline.

## Constraints And Invariants

- The engine signals, records, and gates. It does not spawn executor agents.
- Shared-worktree safety depends on accurate planned `target_files` and
  exhaustive recorded `changed_files`.
- Scope coverage must reuse the same target semantics as wave conflict
  detection: exact path, parent/child scope, case-folded aliases, and glob scope.
- Started parallel waves must not infer dispatch mode from plan metadata alone.
- `parallel_subagents` is a public dispatch token; `degraded_sequential` remains
  valid explicit evidence.
- Plan-drift blockers own stale plan/evidence remediation, so the new safety
  gates stay suppressed while plan drift is present.
