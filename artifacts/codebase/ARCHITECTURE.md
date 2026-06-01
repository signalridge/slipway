# Architecture

- Module responsibilities:
  - `cmd/new.go` owns `slipway new` classification, slug allocation, guarded
    creation, default worktree binding, and JSON/human response rendering.
  - `cmd/common.go` owns shared command helpers, active-change resolution, and
    create-time precondition errors used by lifecycle commands.
  - `internal/state/store.go` owns active bundle discovery across the current
    workspace and sibling git worktrees.
- Dependency flow:
  - `cmd/new.go` builds a prospective `model.Change`, calls
    `rejectIfConflictingChange`, then calls `state.EnsureDefaultWorktreeForChange`
    and `state.SaveChange`.
  - The create guard consumes `state.ListChangesForCreateGuard` and compares
    existing active changes with the current/prospective workspace authority.
- Coupling hotspots:
  - `ListChangesForCreateGuard` intentionally sees hidden sibling worktree
    authorities; callers must decide whether visibility should become a block.
  - `state.WorkspaceRootForChange` is the authority for unbound vs bound change
    workspace ownership.
- Current change blast radius:
  - Create-time lifecycle gating for `slipway new`; no runtime execution,
    validation, archive, or template surface changes.
- Notes:
  - Source references: `cmd/new.go`, `cmd/common.go`,
    `internal/state/store.go`, `internal/state/paths.go`.
