# Decision

## Project Context
- Tech Stack: Go CLI
- Conventions: minimal command-layer changes, source-backed worktree authority,
  and command-level regression tests for user-visible behavior
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Alternative 1: Only correct the unbound remediation text
- Tradeoff: Low implementation risk and fixes the most misleading #48 message.
- Rejected because it leaves #48's non-destructive unblock problem and #50's
  bound sibling-worktree block intact.

### Alternative 2: Add an explicit bind/park command for intake-stage changes
- Tradeoff: Gives operators a direct escape hatch for unbound intake changes.
- Rejected for this change because it adds a new public command surface and does
  not address #50's bound-worktree global guard without additional guard work.

### Alternative 3: Scope create-guard rejection to workspace collisions
- Tradeoff: Requires moving the guard after slug/change construction so the
  prospective target workspace can be known before `EnsureDefaultWorktreeForChange`.
- Selected because it fixes both confirmed issue scenarios, preserves hidden
  active-change discovery for fail-closed diagnostics, and keeps the code change
  local to `cmd/new.go` and `cmd/common.go`.

## Selected Approach

Use Alternative 3. `slipway new` will construct the prospective change first,
then call `rejectIfConflictingChange(root, change)`. The guard will still load
active changes through `state.ListChangesForCreateGuard`, but it will reject
only when an active change owns the current bound worktree or the new change's
prospective target workspace.

For a discovery change with a valid git `HEAD`, the prospective target is
`state.DefaultWorktreePath(repoRoot, slug)`, matching the early-bind path. For a
non-discovery, non-git, or unborn-HEAD change, the target remains the current
workspace. Same-workspace unbound conflicts continue to return
`active_change_exists`, with remediation that tells the operator to finish or
cancel rather than running `slipway next`.

## Interfaces and Data Flow

- `cmd/new.go`: reorder create flow so slug/change construction happens before
  conflict checking; no persisted state is written before the guard passes.
- `cmd/common.go`: change `rejectIfConflictingChange` to accept the prospective
  `model.Change`, compute the new target workspace, compare it with active
  changes, and produce scoped error messages.
- `internal/state/*`: no storage or discovery API changes. The guard continues
  to consume `ListChangesForCreateGuard`, `WorkspaceRootForChange`,
  `ResolveGitWorkspaceRoot`, and `DefaultWorktreePath`.
- Tests: add command-level regressions for #48 and #50, and update the existing
  active-change rejection test to preserve same-workspace fail-closed behavior.

## Rollout and Rollback

Rollout is a normal code change through the governed Slipway lifecycle. Verify
with the targeted command test, then `go test -count=1 ./...`, `go build ./...`,
and Slipway validation/closeout.

Rollback is straightforward: revert the changes in `cmd/common.go`,
`cmd/new.go`, and `cmd/new_test.go`, then rerun the same targeted and full test
commands. No data migration or runtime state rewrite is required.

## Risk

- Medium: target workspace calculation could drift from
  `EnsureDefaultWorktreeForChange`. Mitigation: the guard mirrors the same
  discovery/HEAD/default-worktree conditions and tests assert the created change
  binds to a separate worktree.
- Low: permitting parallel governed changes could weaken true collision
  protection. Mitigation: same-workspace unbound rejection remains covered, and
  bound current-worktree rejection remains fail-closed.
- Low: remediation text could regress. Mitigation: command test asserts the
  unbound same-workspace conflict no longer references `slipway next`.
