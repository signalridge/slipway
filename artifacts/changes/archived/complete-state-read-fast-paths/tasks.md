# Tasks

## Task List

- [x] `t-01` Complete invocation-scoped read context reuse in command read paths.
  - depends_on: []
  - target_files: [`cmd/state_read_context.go`, `cmd/status.go`, `cmd/next.go`, `cmd/validate.go`, `cmd/common.go`]
  - task_kind: code
  - covers: [REQ-001, REQ-004]
  - acceptance:
    - `status`, `next`, and `validate` pass one invocation-scoped read context through downstream lifecycle authority, path, verification, and display reads.
    - A completed command does not persist or reuse read-context facts across later command invocations.
    - Existing bound-worktree, archived-local, no-active, and multi-active authority semantics remain unchanged.
  - evidence:
    - Targeted tests in `cmd/state_read_context_test.go` demonstrate in-command reuse and no cross-command cache.
    - Targeted command tests continue to pass for bound, archived, missing, no-active, and multi-active lifecycle authority cases.

- [x] `t-02` Add explicit `--change` fast-path coverage and preserve missing/archived semantics.
  - depends_on: [`t-01`]
  - target_files: [`cmd/state_read_context_test.go`, `cmd/common_test.go`, `cmd/status_context_repair_test.go`, `cmd/resolve_explicit_change_authority_test.go`]
  - task_kind: test
  - covers: [REQ-002, REQ-004]
  - acceptance:
    - Successful explicit `--change <slug>` reads for `status`, `next`, and `validate` avoid enumerating every active change bundle.
    - Missing explicit slugs still return stable `change_not_found` semantics and exit code 3.
    - Archived explicit and archived-local resolution behavior stays preferred over unrelated active changes.
  - evidence:
    - Targeted tests use instrumentation or focused fixtures to prove the successful explicit-slug path does not call the broad active-change scan.
    - Existing and new resolver tests pass in `cmd/common_test.go`, `cmd/status_context_repair_test.go`, and `cmd/resolve_explicit_change_authority_test.go`.

- [x] `t-03` Verify and complete tail-oriented status timeline reads.
  - depends_on: [`t-01`]
  - target_files: [`cmd/status_view_build.go`, `cmd/status_context_repair_test.go`, `internal/state/lifecycle_event.go`, `internal/state/lifecycle_event_test.go`]
  - task_kind: code
  - covers: [REQ-003, REQ-004]
  - acceptance:
    - Default `status` timeline display uses a bounded lifecycle-event tail read plus required predecessor transition context.
    - Malformed JSON in the retained tail or predecessor context still fails closed with a lifecycle event log read error.
    - Full-log integrity readers remain available for health, repair, or verification surfaces and are not replaced by display-only tail reads.
  - evidence:
    - `internal/state/lifecycle_event_test.go` covers bounded tail reads and malformed retained-line failures.
    - `cmd/status_context_repair_test.go` or an equivalent command-level test proves the status display path uses the bounded timeline read.

- [x] `t-04` Run targeted and full verification, including performance-sensitive state-read checks.
  - depends_on: [`t-01`, `t-02`, `t-03`]
  - target_files: [`artifacts/changes/complete-state-read-fast-paths/verification/implementation-verification.md`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
  - acceptance:
    - Targeted command/state/progression packages pass after implementation.
    - Full repository tests pass or any failure is documented as unrelated with a targeted rerun showing this change is not the cause.
    - Verification notes record commands, outcomes, and any residual risk.
  - evidence:
    - `go test ./cmd ./internal/state ./internal/engine/progression -count=1`
    - `go test ./... -count=1`
    - `artifacts/changes/complete-state-read-fast-paths/verification/implementation-verification.md`
