# Tasks

## Task List

- [x] `t-01` Implement shared invocation route and explicit change error taxonomy.
  - depends_on: []
  - target_files: [cmd/common.go, cmd/freshness_diagnostics.go, cmd/status.go, cmd/validate.go, cmd/next.go, cmd/next_handoff.go, cmd/done.go, cmd/evidence.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - acceptance:
    - `status`, `next`, and `validate` expose the same additive invocation route fields for a locally bound active change.
    - Root or wrong-worktree invocation cannot advertise locally executable lifecycle actions for a change bound elsewhere.
    - `validate --change definitely-not-a-change --json` returns `change_not_found` with exit 3, and archived explicit active-command usage remains fail-closed without writes.
    - Existing archived-local status behavior from #283 remains covered and unchanged.

- [x] `t-02` Add readiness-safe freshness fields and shared current action projection.
  - depends_on: [t-01]
  - target_files: [cmd/common.go, cmd/status.go, cmd/status_view_build.go, cmd/validate.go, cmd/next.go]
  - task_kind: code
  - covers: [REQ-003, REQ-004]
  - acceptance:
    - `status --json` and `validate --json` include `execution_evidence_freshness`, `governance_evidence_freshness`, and `overall_readiness_freshness`.
    - Missing or stale required skill evidence prevents `overall_readiness_freshness` from being reported as `fresh`.
    - The legacy `evidence_freshness` execution-evidence contract remains compatible with existing tests.
    - S3 review-batch pending states expose `review_batch` as the current action kind through `next`, `status`, and `validate`.

- [x] `t-03` Add host capability visibility and fail-closed fallback contract for selected delegated actions.
  - depends_on: [t-02]
  - target_files: [cmd/next.go, cmd/next_handoff.go, cmd/next_skill_view.go, cmd/run.go, cmd/validate.go, internal/engine/capability/resolver.go, internal/engine/progression/confirmation_boundaries.go, internal/model/reason_code.go, internal/model/recovery.go]
  - task_kind: code
  - covers: [REQ-005]
  - acceptance:
    - Selected actions that require unavailable delegation, subagent, or independent-review capability do not report `prior_authorization_sufficient=true`.
    - `next`, `run`, and `validate` surface the same fail-closed capability blocker or explicit remediation for unavailable required capability.
    - Manual fallback, when selected or supported, names the degraded mode and the evidence requirement needed to proceed.
    - Ordinary non-sensitive skill handoffs that do not require unavailable host capability keep the existing auto-mode behavior.

- [x] `t-04` Add public CLI regression coverage for route, freshness, action, and capability contracts.
  - depends_on: [t-01, t-02, t-03]
  - target_files: [cmd/active_change_resolution_test.go, cmd/validate_readonly_test.go, cmd/status_view_build_test.go, cmd/progression_next_test.go, cmd/auto_mode_test.go, cmd/common_test.go, cmd/run_contract_test.go, cmd/next_skill_capability_hints_test.go, cmd/test_main_test.go, internal/engine/capability/resolver_test.go, internal/engine/capability/routes_test.go, internal/model/reason_code_contract_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - acceptance:
    - Focused command tests fail on the current root/wrong-worktree actionability drift and pass after the route contract is implemented.
    - Tests cover explicit missing `--change`, archived explicit active-command usage, archived-local status, readiness-safe freshness, review-batch action kind consistency, and host capability fail-closed behavior.
    - Package tests for changed capability resolver behavior are added when resolver behavior changes; otherwise command-level tests explicitly prove no resolver package change was needed.
    - `go test ./cmd -count=1`, `go test ./... -count=1`, and `golangci-lint run ./...` are the required implementation verification commands before S3 closeout.
