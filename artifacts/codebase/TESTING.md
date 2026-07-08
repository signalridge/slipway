# Testing

## Existing Coverage
- `cmd/s3_inplace_convergence_test.go` and `cmd/s3_inplace_convergence_forward_test.go` cover the in-place S3 convergence path for added and edited tasks.
- `cmd/evidence_task_test.go` covers result-file import, wave-orchestration evidence preconditions, and S3 task-plan drift behavior.
- `cmd/fix_test.go` covers `fix --start-reexecution` state transitions and now needs the additive-convergence guard.
- `internal/model/recovery_test.go` covers recovery step construction and ordering.
- `internal/engine/progression/readiness_optimization_test.go` covers workspace changed-file filtering before scope-contract evaluation.

## Verification Strategy
- Run focused tests for modified packages while iterating: `go test ./cmd`, `go test ./internal/model`, `go test ./internal/engine/progression`, `go test ./internal/toolgen`, and `go test ./internal/tmpl`.
- Run `go test ./...` before closeout to catch generated-surface and integration contract drift.

## Fixture Notes
- Command tests commonly create temporary git workspaces and governed bundles through helpers in `cmd/lifecycle_commands_test.go`.
- Evidence tests write task-result JSON files in temporary workspaces to exercise the same public `slipway evidence task --result-file` path operators use.
