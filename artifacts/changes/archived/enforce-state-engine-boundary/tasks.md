# Tasks

## Task List

- [x] `t-01` Add lower-level freshness primitives and switch freshness consumers away from engine context
  - depends_on: []
  - target_files: [internal/freshness/freshness.go, internal/freshness/freshness_test.go, internal/engine/context/context.go, internal/engine/context/context_test.go, internal/state/execution_summary.go, internal/state/execution_summary_test.go, internal/engine/progression/readiness.go, internal/engine/progression/freshness_guard_test.go, internal/engine/progression/evidence_digests_test.go, cmd/common.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003]
  - evidence: `go test ./internal/freshness ./internal/engine/context ./internal/state ./internal/engine/progression ./cmd -run 'Test.*Freshness|TestEvaluateEvidenceFreshness|TestProjectExecutionFreshness|TestExecutionSummary' -count=1`
  - acceptance: `internal/state/execution_summary.go` no longer imports `internal/engine/context`, `internal/engine/context` no longer exports freshness helpers, freshness status strings remain `fresh`/`stale`/`unknown`, and the targeted freshness tests pass.

- [x] `t-02` Rehome wave/task-plan primitives and update all production consumers
  - depends_on: []
  - target_files: [internal/wave/wave.go, internal/wave/parse.go, internal/wave/wave_test.go, internal/wave/parse_test.go, internal/wave/objective_parse_regression_test.go, internal/engine/wave/wave.go, internal/engine/wave/parse.go, internal/engine/wave/wave_test.go, internal/engine/wave/parse_test.go, internal/engine/wave/objective_parse_regression_test.go, internal/state/wave_execution.go, internal/state/execution_repair.go, internal/state/health.go, internal/state/wave_execution_test.go, internal/state/wave_execution_transaction_test.go, internal/state/health_test.go, internal/engine/progression/wave_sync.go, internal/engine/progression/wave_sync_test.go, internal/engine/progression/advance_governed.go, internal/engine/progression/validation.go, internal/engine/progression/evidence_digests.go, internal/engine/governance/health.go, internal/engine/scopecontract/evaluate.go, internal/engine/scopecontract/evaluate_test.go, internal/engine/artifact/tasks_contract.go, internal/engine/artifact/manager_test.go, internal/engine/status/view.go, internal/model/wave_execution_test.go, cmd/next_wave_plan.go, cmd/next_wave_plan_test.go, cmd/execution_summary_test_helper_test.go, cmd/fix.go, cmd/run.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003]
  - evidence: `go test ./internal/wave ./internal/state ./internal/engine/progression ./internal/engine/governance ./internal/engine/scopecontract ./internal/engine/artifact ./internal/engine/status ./cmd -run 'Test.*Wave|Test.*TaskPlan|Test.*Scope|Test.*Materialize|Test.*Repair|Test.*Health|Test.*Evidence' -count=1`
  - acceptance: the old `internal/engine/wave` package is deleted with no compatibility layer, all Go imports of `internal/engine/wave` are moved to the lower-level wave package, `internal/state/wave_execution.go` no longer imports `internal/engine/wave`, and wave-plan/task-plan behavior remains covered by targeted tests.

- [x] `t-03` Add architecture gate and run targeted verification
  - depends_on: [t-01, t-02]
  - target_files: [internal/architecture/dependency_direction_test.go, artifacts/changes/enforce-state-engine-boundary/verification/architecture-boundary-tests.md]
  - task_kind: test
  - covers: [REQ-001, REQ-003]
  - evidence: `go test ./internal/architecture -count=1`; `rg -n 'github.com/signalridge/slipway/internal/engine|internal/engine/' internal/state`; `go test ./... -count=1`
  - acceptance: the architecture test fails on future production `internal/state -> internal/engine` imports, the repository has no current production `internal/state -> internal/engine` imports, and full Go tests pass.
