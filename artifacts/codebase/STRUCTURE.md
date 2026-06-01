# Structure

- Directory layout: cmd/, docs/, internal/
- Entry points: README.md, go.mod, main.go
- Generated versus handwritten boundaries:
  - `cmd/*.go`, `internal/state/*.go`, and their tests are handwritten source.
  - `artifacts/changes/*` and `artifacts/codebase/*` are governed context and
    evidence artifacts, not runtime code.
- Ownership hints:
  - CLI command behavior and user-facing errors live in `cmd/`.
  - Worktree, bundle, local runtime, and active-change authority helpers live in
    `internal/state/`.
- Notes:
  - This change is scoped to `slipway new` create-time behavior and command
    tests; it does not touch generated skill/template surfaces.
