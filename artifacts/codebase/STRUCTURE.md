# Structure

## Repository Layout
- `cmd/`: public CLI commands and command-level tests. Issue #427 touches `fix`, `next`, `validate`, and `evidence` command behavior.
- `internal/model/`: shared reason-code and recovery projection models.
- `internal/engine/progression/`: lifecycle readiness, ship authority, wave synchronization, and workspace changed-file accounting.
- `internal/engine/scopecontract/`: scope-contract evaluation of planned targets against task evidence and workspace changes.
- `internal/toolgen/` and `internal/tmpl/`: generated command surface metadata and prompt template content.
- `docs/`: user-facing command documentation in English, Chinese, and Japanese.

## Generated-Vs-Handwritten Boundaries
- `artifacts/changes/**` and `artifacts/codebase/**` are governed/advisory artifacts for this change.
- Runtime evidence under state directories is engine-owned and should be written through CLI commands, not by hand.

## Change-Relevant Tests
- `cmd/fix_test.go`, `cmd/evidence_task_test.go`, `cmd/s3_inplace_convergence*_test.go`, and `cmd/next_wave_plan_test.go` cover command behavior.
- `internal/model/recovery_test.go` and `internal/model/reason_code_contract_test.go` cover recovery/reason-code contracts.
- `internal/engine/progression/readiness_optimization_test.go` covers scope-contract changed-file filtering.
