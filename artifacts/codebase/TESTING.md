# Testing

Re-authored for change
`make-two-confirmed-downstream-reported-lattice-slipway-gover`
(GitHub issues #207 and #211).

## Existing Coverage

- `internal/engine/progression/readiness_optimization_test.go` covers the
  readiness changed-file optimization, including the `artifacts/codebase/**`
  scope-contract exemption path.
- `internal/engine/progression/scope_contract_gate_test.go` covers scope-contract
  gate behavior (pass/blocked) at the progression layer.
- `cmd/validate_test.go` covers the validate JSON view, including the
  `scope_contract` sub-view assembled by `buildScopeContractView`.
- `cmd/status_test.go` covers the status JSON/human view, including `progress`.
- `cmd/evidence_task_test.go` / `cmd/evidence_test.go` cover `evidence task`
  validation, including the `run_summary_version >= 1` rejection.

## Gaps Closed By This Change

- #207: a dirty, tracked `artifacts/codebase/*.md` not in any task's
  `target_files` is surfaced in `scope_contract.exempt_context_files` (unit test
  at the readiness layer; cmd test at the view layer) while it stays out of
  `changed_files` and `scope_contract.status` stays `pass`.
- #211: `status --json` (and `next --json`) omit `progress.run_summary_version`
  when no execution summary exists (value `0`), and report the real value once a
  run exists (cmd test asserts both states).
- #211: the `evidence task` help/guidance surface states the correct first run
  version (`1`), and `evidence task --run-summary-version 0` still fails with
  `evidence_task_run_summary_version_invalid` (cmd test pins both).

## Verification Plan

- Run affected packages after each task:
  `go test ./internal/engine/scopecontract ./internal/engine/progression ./internal/engine/status ./cmd`.
- Run full repository verification: `go build ./...`, `go vet ./...`,
  `go test ./...`, and `gofmt -s -l` (the lint gate is golangci-lint gofmt
  **simplify**, not plain `gofmt -l`).
- Manually confirm with current-worktree Slipway: with a dirty
  `artifacts/codebase/*.md`, `validate --json` / `status --json` show
  `scope_contract.exempt_context_files` listing it while `changed_files` omits it
  and status is `pass`; early-S2 `status --json` omits `run_summary_version`.
