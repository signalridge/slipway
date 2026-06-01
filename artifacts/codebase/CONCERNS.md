# Concerns

- Architectural pressure points:
  - `slipway new` must distinguish "known active authority" from "blocking
    workspace collision"; state discovery intentionally exposes hidden sibling
    authorities to the guard.
  - The prospective target workspace for a discovery change must stay aligned
    with `state.EnsureDefaultWorktreeForChange`.
- Brittle areas:
  - Unbound active changes have no persisted `WorktreePath`; the fallback
    workspace root is the local project root until a binding exists.
  - Worktree visibility markers affect normal reads but should not be confused
    with workspace collision semantics.
- Migration traps:
  - A broad storage migration for per-scope active-change authority is not
    required for the #48/#50 fix; changing it here would expand the blast
    radius beyond create-guard behavior.
- Recheck routing:
  - Re-run targeted `cmd` tests for unbound and bound active-change create
    scenarios, then full `go test -count=1 ./...` before closeout.
- Notes:
  - Source references: `cmd/common.go`, `cmd/new.go`,
    `internal/state/store.go`, `internal/state/paths.go`.
