# Testing

- Question: What tests prove the public lifecycle contract is consistent without
  only testing private helper behavior?
- Existing resolver tests in `cmd/active_change_resolution_test.go:17-95`
  cover helper-level bound-elsewhere behavior for `resolveActiveChangeRef`,
  `next --change`, and root `run`, but they do not cover root unscoped
  `status`, root unscoped `validate`, `done`, or `evidence` as a shared
  contract matrix.
- Existing archived-local regression tests in
  `cmd/active_change_resolution_test.go:98-175` and `cmd/status.go:400-416`
  should be preserved and extended only as needed; this behavior must not be
  rewritten away while introducing shared routing.
- Existing validate zero-write tests in `cmd/validate_readonly_test.go:52-96`
  cover no-active diagnostics and archived explicit slugs. They should be
  extended so explicit missing slugs fail closed with `change_not_found` instead
  of returning diagnostics.
- Existing review-action consistency tests in
  `cmd/progression_next_test.go:1065-1183` compare `next`, `validate`, and
  `run` for S3 review-batch state. They do not assert `status` exposes the same
  action kind, so this change should add a status-side action contract assertion.
- Freshness tests in `cmd/common_test.go:496-580` prove
  `projectFreshnessForExecMode` tracks execution evidence and deliberately
  ignores non-freshness blockers. New tests should keep that behavior while
  asserting new readiness/governance freshness fields become stale/blocked when
  required skills or ship gates are missing.
- Auto-mode tests in `cmd/auto_mode_test.go:173-205` and
  `cmd/auto_mode_test.go:263-280` show current `prior_authorization_sufficient`
  softening for review batches and non-sensitive skill handoffs, with security
  review staying hard-stop. #339 tests should add capability-unavailable cases
  so prior authorization alone is not reported sufficient when the required host
  mechanism is absent.
- Black-box command helpers exist through `runRootCommandIn` in
  `cmd/error_contract_test.go:267-280`; command-level tests should use these or
  `commandForRoot` instead of only calling private route helpers.
- Minimum focused verification before implementation closeout: new route/action
  matrix tests, freshness field tests, host-capability tests, `go test ./cmd
  -count=1`, `go test ./... -count=1`, and `golangci-lint run ./...`.
