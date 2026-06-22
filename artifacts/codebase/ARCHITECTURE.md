# Architecture

- Question: What seams must Workstream B change to make task evidence recording
  result-import based while keeping Slipway the execution ledger authority?
- Public task evidence entrypoint: `makeEvidenceTaskCmd` in `cmd/evidence.go`
  currently owns validation, wave-plan lookup, payload construction, runtime
  task evidence writes, parser verification, and lifecycle events. It requires
  `--task-id`, `--run-summary-version`, `--task-kind`, `--verdict`, and
  `--evidence-ref`, then writes `.git/slipway/runtime/changes/<slug>/evidence/tasks/<task_id>.json`.
  Evidence: `cmd/evidence.go:600-876`.
- Current derivation split: `evidence task` already derives `captured_at` when
  omitted, defaults `target_files` from the current wave plan when omitted, and
  computes `freshness_inputs` through
  `state.ExpectedExecutionTaskFreshnessInputs`; it still makes the agent supply
  `run_summary_version` and `task_kind`. Evidence: `cmd/evidence.go:659-810`,
  `internal/state/execution_summary.go:363-375`.
- Wave-plan authority: `state.MaterializeWavePlanTransactionOpAt` builds
  `wave-plan.yaml` from `tasks.md`, preserving each planned task's
  `target_files` and `task_kind`, and applies effective parallel flags. The
  current `model.WavePlan` has generated time and task hashes but no active
  run-summary version. Evidence: `internal/state/wave_execution.go:141-190`,
  `internal/model/wave_execution.go:14-43`.
- Run-version gap: `model.Change` has no active execution run field, and the
  only current run-version signal is agent-supplied task evidence plus
  wave-orchestration skill evidence. Evidence: `internal/model/change.go:14-67`,
  `cmd/evidence.go:949-1054`.
- Execution summary sync: `SyncGovernedWaveExecution` uses the latest passing
  wave-orchestration verification run version, loads task evidence for that
  version, builds `wave-runs`, and writes `execution-summary.yaml`. Evidence:
  `internal/engine/progression/wave_sync.go:82-205`.
- Lifecycle boundary: the normal forward transition into S2 materializes the
  wave plan in the same transaction as `change.yaml`; this is the safest
  existing execution-run boundary. Evidence:
  `internal/engine/progression/advance_governed.go:461-489`.
- S3 repair boundary: `fix` is S3-only and surfaces a repair contract; the
  current lifecycle path is forward-only and has no automatic S3-to-S2
  rematerialization. Evidence: `cmd/fix.go:82-147`,
  `internal/engine/action/workflow.go:10-17`,
  `internal/engine/progression/advance_governed.go:745-754`.
