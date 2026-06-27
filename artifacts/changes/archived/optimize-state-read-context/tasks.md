# Tasks

## Task List

- [x] `t-01` Add state read context and explicit fast-path state helpers.
  - depends_on: []
  - target_files: [`cmd/state_read_context.go`, `cmd/state_read_context_test.go`, `internal/state/store.go`, `internal/state/store_test.go`, `internal/state/verification.go`, `internal/state/verification_test.go`]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004, REQ-006]
  - acceptance: `go test ./internal/state -run 'Test(.*FastPath|.*Verification.*ForChange|.*LoadChange)' -count=1` passes or the final targeted state test command records the exact implemented test names; explicit missing/archived/sibling authority tests continue to pass.

- [x] `t-02` Wire `status`, `next`, and `validate` through the invocation read context.
  - depends_on: [`t-01`]
  - target_files: [`cmd/common.go`, `cmd/status.go`, `cmd/status_view_build.go`, `cmd/next.go`, `cmd/next_context_build.go`, `cmd/next_handoff.go`, `cmd/validate.go`, `cmd/active_change_resolution_test.go`, `cmd/codebase_map_context_test.go`, `cmd/status_context_repair_test.go`, `cmd/common_test.go`, `cmd/progression_next_test.go`, `cmd/validate_readonly_test.go`, `internal/engine/progression/authority.go`, `internal/engine/progression/evidence.go`, `internal/engine/progression/readiness.go`, `internal/engine/progression/readiness_optimization_test.go`]
  - task_kind: code
  - covers: [REQ-002, REQ-003, REQ-004, REQ-006]
  - acceptance: `go test ./cmd -run 'Test(Status|ResolveExplicitChange|Validate|Next).*' -count=1` passes or the final targeted command test records the exact implemented test names; `status`, `next`, and `validate` JSON route fields remain consistent for bound and explicit invocations.

- [x] `t-03` Add tail-oriented lifecycle event reading for status timeline display.
  - depends_on: [`t-01`]
  - target_files: [`internal/state/lifecycle_event.go`, `internal/state/lifecycle_event_test.go`, `cmd/status_view_build.go`, `cmd/status_view_build_test.go`, `cmd/status_timeline_test.go`, `cmd/health_test.go`]
  - task_kind: code
  - covers: [REQ-005, REQ-006]
  - acceptance: `go test ./internal/state -run Test.*LifecycleEvent -count=1` and `go test ./cmd -run Test.*Status.*Timeline -count=1` pass or the final targeted test commands record the exact implemented test names; malformed retained lifecycle tail lines still surface a status diagnostic.

- [x] `t-04` Record after-baseline performance evidence and final verification.
  - depends_on: [`t-02`, `t-03`]
  - target_files: [`artifacts/changes/optimize-state-read-context/verification/state-read-baseline-before.md`, `artifacts/changes/optimize-state-read-context/verification/state-read-baseline-after.md`]
  - task_kind: verification
  - covers: [REQ-001, REQ-006]
  - acceptance: a built binary after-baseline artifact records the same command matrix and comparable fixture scale as the before-baseline; the regenerated after-baseline fixture may contain 302 `change.yaml` files instead of the original 300 because the `/tmp` before fixture was no longer available and the after artifact documents the regenerated 302-bundle fixture explicitly. Targeted tests pass, and `go test ./... -count=1` passes before S3 review begins.
