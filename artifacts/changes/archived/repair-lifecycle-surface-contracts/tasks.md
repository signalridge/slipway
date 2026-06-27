# Tasks

## Task List

- [x] `t-01` Add invocation route output to done and evidence command JSON surfaces.
  - depends_on: []
  - target_files: [`cmd/done.go`, `cmd/evidence.go`, `cmd/freshness_diagnostics.go`]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]
  - acceptance: `done --json`, `evidence skill --json`, and `evidence task --json` include `invocation_route` for successful single-change operations without changing their mutation semantics.
  - evidence: targeted command tests exercise successful route projection and explicit missing slug fail-closed behavior.

- [x] `t-02` Add and run black-box lifecycle surface regression tests for the route completion patch.
  - depends_on: [`t-01`]
  - target_files: [`cmd/active_change_resolution_test.go`, `cmd/evidence_skill_test.go`, `cmd/evidence_task_test.go`, `cmd/lifecycle_commands_test.go`, `artifacts/changes/repair-lifecycle-surface-contracts/verification/lifecycle-surface-route-tests.md`]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-004]
  - acceptance: tests assert route output for touched commands, including single-result `evidence task --json` and batch `evidence task --result-file ... --result-file ... --json` output with a batch-level `invocation_route`; tests explicitly cover each REQ-002 fail-closed semantic touched by route projection: explicit missing evidence target returns `change_not_found`, root bound-elsewhere `done` remains `change_bound_to_other_worktree`, archived explicit targets remain fail-closed, no-active unscoped evidence/done remains fail-closed, and wrong-state `done` and evidence commands remain fail-closed; existing route/freshness/action/capability regression tests continue to pass for `status`, `next`, `validate`, and `run`.
  - evidence: targeted `go test` transcript names the new route-output tests, fail-closed tests, evidence task batch JSON test, and existing P0 contract tests; final package/full-suite transcript is recorded in verification notes.
