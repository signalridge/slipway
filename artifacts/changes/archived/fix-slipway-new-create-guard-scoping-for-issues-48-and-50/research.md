# Research

## Research Findings

### Architecture
- Affected modules: `cmd/new.go` owns `slipway new` setup, slug allocation,
  create-guard invocation, and default worktree binding (`cmd/new.go:391-476`).
- Affected modules: `cmd/common.go` owns the shared create-guard error contract
  and current invocation worktree resolution (`cmd/common.go:851-985`).
- Affected modules: `internal/state/store.go` discovers active changes across
  workspace roots for normal reads and for the create guard
  (`internal/state/store.go:127-180`, `internal/state/store.go:244-256`,
  `internal/state/store.go:471-484`).
- Dependency chains: `new` resolves classification and constructs a
  prospective `model.Change` before calling `rejectIfConflictingChange`; the
  guard uses `state.ListChangesForCreateGuard`, `state.WorkspaceRootForChange`,
  and `state.DefaultWorktreePath` to compare workspace authority before
  `state.EnsureDefaultWorktreeForChange` materializes the target worktree.
- Blast radius: create-time lifecycle gating only. Runtime progression,
  artifact validation, and done/cancel semantics are not changed.
- Constraints: same-workspace active change protection must remain fail-closed;
  sibling worktree active changes must remain discoverable for diagnostics but
  should not block unrelated `new` calls when the new workspace authority does
  not collide.

### Patterns
- Existing conventions: CLI precondition failures use `newPreconditionError`
  with `error_code=active_change_exists` and actionable remediation
  (`cmd/common.go:942-985`).
- Existing conventions: path comparisons normalize through `state.NormalizePath`
  or fall back to cleaned paths, matching nearby command/path handling
  (`cmd/common.go:927-940`).
- Existing conventions: tests use `withWorkspace`, `initTestWorkspace`,
  `recordingIntentClassifier`, and real temporary git worktrees for lifecycle
  behavior (`cmd/new_test.go:1174-1284`).
- Reusable abstractions: `state.ResolveGitWorkspaceRoot`,
  `state.DefaultWorktreePath`, and `state.WorkspaceRootForChange` provide enough
  authority information for a local create-guard fix without adding a new
  command or storage model.
- Convention deviations: the create guard still calls
  `ListChangesForCreateGuard` so hidden sibling authorities are known, but the
  rejection decision is now scoped to the current or prospective workspace
  collision instead of any active change anywhere.

### Risks
- Technical risks: medium - allowing parallel governed changes could accidentally
  permit same-workspace collisions if the target workspace calculation diverges
  from `EnsureDefaultWorktreeForChange`.
- Technical risks: low - the unbound remediation could become less specific;
  targeted assertion ensures it no longer points to `slipway next`.
- Technical risks: low - `git rev-parse --verify HEAD` is duplicated in the cmd
  layer only to mirror whether a discovery change can be early-bound; unborn or
  non-git repositories fall back to root-scoped collision checks.
- Guardrail domains: none. This changes local CLI lifecycle behavior, not auth,
  secrets, PII, financial flows, schema migration, irreversible operations, or
  externally consumed web/API contracts.
- Reversibility: high. The change is localized to `cmd/common.go`,
  `cmd/new.go`, and command tests; rollback restores the prior global guard.

### Test Strategy
- Existing coverage: `TestNewCommandRejectsWhenActiveChangeAlreadyExists`
  protects same-workspace active-change rejection and now verifies the unbound
  remediation does not mention `slipway next` (`cmd/new_test.go:1174-1201`).
- New coverage for #48: `TestNewCommandAllowsDiscoveryChangeWhenUnboundIntakeChangeOwnsRoot`
  failed before the fix with `active_change_exists` and now verifies a discovery
  follow-up can bind a separate worktree while the original unbound intake
  remains active (`cmd/new_test.go:1204-1237`).
- New coverage for #50: `TestNewCommandAllowsBoundSiblingWorktreeActiveChange`
  failed before the fix with `active_change_exists` and now verifies a hidden
  sibling bound worktree does not block a separate discovery follow-up
  (`cmd/new_test.go:1239-1284`).
- Infrastructure needs: no new external fixtures. Existing temporary git repos
  and worktree helpers cover the behavior.
- Verification approach: run the targeted `go test ./cmd -run ... -count=1`,
  then broaden to package/full-suite testing and Slipway validation before
  closeout.

## Alternatives Considered

- Approach 1: Keep global serialization and only fix remediation. Tradeoff:
  small change, but #50 remains a real behavior bug and #48 still requires
  destructive cancellation for unrelated work.
- Approach 2: Add a new `bind` / `park` command. Tradeoff: explicit operator
  control, but larger UX/API surface and unnecessary because the existing new
  command can compute whether its own target workspace collides.
- Approach 3: Scope `slipway new` rejection to current/prospective workspace
  collisions while preserving all active-change discovery. Tradeoff: requires
  moving the guard until after prospective change construction, but keeps hidden
  authority visibility and fixes both reported cases with a narrow local change.
- Selected: Approach 3. It directly matches the #48/#50 failure modes, preserves
  same-workspace fail-closed behavior, avoids a new command surface, and stays
  localized to the create path.

## Unknowns

- Resolved: Are #48 and #50 real? -> Yes. The added targeted tests failed under
  the prior implementation with `active_change_exists` for both issue scenarios
  and pass after the guard change.
- Resolved: Does the fix require the broader #46 storage migration? -> No. The
  current state APIs expose enough workspace authority to compare the existing
  active change with the new change's prospective workspace.
- Remaining: None for planning. Broader policy questions about configurable
  repo-wide serialization remain deferred.

## Assumptions

- A discovery change with a valid git HEAD will early-bind to
  `state.DefaultWorktreePath(repoRoot, slug)` before its bundle is persisted.
  Evidence: `cmd/new.go:471-476` calls the create guard before
  `state.EnsureDefaultWorktreeForChange`, and `cmd/common.go:899-911` mirrors
  the same target calculation.
- An active change with no `WorktreePath` owns the current workspace root until
  it is bound. Evidence: `state.WorkspaceRootForChange` falls back to the
  project root for unbound changes, consumed by `cmd/common.go:914-920`.
- Hidden sibling worktree authorities should remain discoverable but should not
  create a block unless their workspace authority collides. Evidence:
  `internal/state/store.go:471-484` still feeds the guard with hidden
  authorities, while `cmd/common.go:880-893` now filters by workspace collision.

## Canonical References

- `https://github.com/signalridge/slipway/issues/48`
- `https://github.com/signalridge/slipway/issues/50`
- `cmd/new.go:391-476`
- `cmd/common.go:851-985`
- `internal/state/store.go:127-180`
- `internal/state/store.go:244-256`
- `internal/state/store.go:471-484`
- `cmd/new_test.go:1174-1284`
