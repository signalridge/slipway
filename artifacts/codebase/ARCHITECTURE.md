# Architecture

## Affected Modules
- `cmd/fix.go`: S3 review repair surface and `--start-reexecution` transition.
- `cmd/evidence.go`: task evidence import, wave-orchestration evidence preconditions, and user-facing remediation.
- `cmd/next.go` plus `cmd/next_wave_plan.go`: read-only next payload and live wave-plan projection.
- `cmd/validate.go`: read-only validation payload.
- `internal/model/recovery.go` and `internal/model/reason_code.go`: canonical blockers and recovery ordering.
- `internal/engine/progression/authority.go`: ship authority blocker projection for S3 task-plan drift.
- `internal/engine/progression/readiness.go`: readiness diagnostics and workspace changed-file filtering.

## Dependency Flow
- Public commands load governed change state through `internal/state`, evaluate readiness through `internal/engine/progression`, then render JSON/text views from command-local structs.
- Recovery is intentionally centralized: commands and CLI errors pass reason codes into `model.BuildRecovery` rather than inventing command-specific recovery order.
- Wave projections derive from `tasks.md` through `internal/wave` and are materialized into engine-owned `wave-plan.yaml` through `internal/state`.

## Invariants
- `wave-plan.yaml` is a cache/projection, not planning authority; current `tasks.md` remains authoritative.
- S3 additive task convergence should keep the same `run_summary_version` and preserve existing task evidence.
- Evidence ledgers and verification YAML are engine-owned; public commands should surface decisions and prerequisites before mutating them.
