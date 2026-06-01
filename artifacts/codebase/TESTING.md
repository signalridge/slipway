# Testing

- Test layout: Go *_test.go files
- Coverage hotspots:
  - `cmd/new_test.go` covers `slipway new` intake setup, JSON/classifier
    behavior, worktree binding, active-change create-guard behavior, and
    user-facing precondition errors.
  - `internal/state/store_test.go` covers workspace/worktree discovery,
    visibility, and hidden authority behavior for active bundles.
- Coverage gaps:
  - Cross-worktree create-guard behavior must be asserted at the command layer
    because state discovery intentionally reports more authorities than should
    necessarily block `new`.
- Verification commands: go build ./...; go test ./...
- Fixture patterns:
  - Command tests use `withWorkspace`, `initTestWorkspace`, temporary git repos,
    `recordingIntentClassifier`, and real `git worktree add` calls.
  - State tests use temporary repositories plus helper worktrees to exercise
    visibility and fallback paths.
- Notes:
  - Source references: `cmd/new_test.go`, `internal/state/store_test.go`.
