# Testing

Re-authored for change
`add-engine-enforced-fail-closed-safety-nets-for-shared-workt`.

## Existing Coverage

- `internal/engine/wave/wave_test.go` covers wave planning, static target
  conflict behavior, parsing, and now target coverage plus narrowing advisories.
- `internal/model/wave_execution_test.go` covers dispatch-mode validation and
  verification-reference parsing, including conflict handling.
- `internal/state/wave_execution_test.go` covers wave-run construction and the
  removal of silent parallel dispatch inference.
- `internal/engine/progression/wave_sync_test.go` covers task evidence parsing,
  sync/mutate behavior, execution-summary persistence, read-only readiness, and
  the new safety-net blockers.
- `cmd/next_wave_plan_test.go` covers derived and persisted wave-plan views,
  including view-only `advisories`.
- `internal/tmpl/wave_isolation_content_test.go` and `internal/toolgen` cover
  generated host instruction contracts.

## Gaps Closed By This Change

- `parallel_subagents` is parsed and emitted as the public parallel dispatch
  token; the retired `parallel` dispatch token is not accepted through the new
  contract.
- A started parallel wave without explicit valid dispatch evidence records no
  inferred dispatch mode and surfaces
  `dispatch_mode_absent_on_started_parallel_wave`.
- A `parallel_subagents` wave missing a per-task executor handle surfaces
  `executor_agent_missing`; `degraded_sequential` requires no handles.
- A task whose recorded `changed_files` escapes planned `target_files` surfaces
  `task_changed_file_scope_escape`, including the fail-closed empty
  `target_files` case.
- Two tasks in the same parallel wave recording the same canonical changed file
  surface `parallel_wave_changed_file_overlap`; sequential sharing remains
  allowed.
- `broad_target_files` and `fully_serial_plan` advisories are exposed only in
  the view layer and are excluded from persisted wave plans and freshness hashes.

## Verification Plan

- Run the focused progression scope-escape blocker tests after repair.
- Run affected packages:
  `go test ./internal/model ./internal/state ./internal/engine/wave ./internal/engine/progression ./cmd ./internal/tmpl ./internal/toolgen`.
- Run full repository verification:
  `gofmt -l`, `go test ./...`, and `git diff --check`.
- Use current-worktree Slipway outputs (`status --json`, `validate --json`,
  `next --json --diagnostics`) as lifecycle authority after each evidence
  refresh.
