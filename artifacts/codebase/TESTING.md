# Testing

- Question: Which focused tests prove state-read reuse without changing
  fail-closed lifecycle semantics?
- Existing coverage: explicit missing slug is pinned by
  `TestResolveExplicitChangeRejectsUnknownSlug` (`cmd/common_test.go:124`);
  archived/missing active authority fail-closed semantics are pinned in
  `cmd/resolve_explicit_change_authority_test.go:23` and
  `cmd/resolve_explicit_change_authority_test.go:68`; bound-worktree default
  status routing is pinned in `cmd/status_context_repair_test.go:36`.
- Existing state tests: bound worktree loading and location fallback are covered
  by `internal/state/worktree_binding_test.go:51` and
  `internal/state/worktree_binding_test.go:98`; the process worktree-list cache
  invalidation behavior is covered by `internal/state/worktree_test.go:102`,
  `internal/state/worktree_test.go:136`, and
  `internal/state/worktree_test.go:174`.
- Existing lifecycle tests: append/readback and missing-log semantics are covered
  in `internal/state/lifecycle_event_test.go:13` and
  `internal/state/lifecycle_event_test.go:56`; malformed lifecycle logs are
  surfaced by health diagnostics in `cmd/health_test.go:122`.
- New required tests: add focused command/state tests for invocation-scoped
  reuse counters, explicit `--change` success without global bundle scan,
  missing explicit slug still returning `change_not_found`, resolved-change
  verification inventory reuse, and tail-oriented lifecycle timeline reads that
  still report malformed retained lines.
- Performance verification: use a built binary, not `go run`. The current
  before-baseline is recorded in
  `artifacts/changes/optimize-state-read-context/verification/state-read-baseline-before.md`
  with 26 worktrees including main, 300 `change.yaml` files, 100 verification
  records, and 1200 lifecycle events.
- Final commands: run targeted tests for `cmd`, `internal/state`, and any
  readiness helper touched; finish with `go test ./... -count=1`.
