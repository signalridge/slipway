# Research

Question: why does unscoped `slipway status --json` from a bound worktree
recommend deleting the active slug when `--change <slug>` and `validate` can
load that same change?

## Alternatives Considered

### Architecture
- Affected modules: status command routing in `cmd/status.go`, shared active
  change resolution helpers in `cmd/common.go`, and worktree binding authority
  in `internal/state/worktree_binding.go`.
- Entry point: `makeStatusCmd` handles unscoped status; the current path checks
  delete recovery before route selection in the no-`--change` branch
  (`cmd/status.go:243`, `cmd/status.go:257`, `cmd/status.go:274`).
- Dependency chains: `status` -> `state.ListChanges` / `deleteRecoveryStatusView`
  -> `state.OrphanBundleSlugs`; explicit `--change` follows
  `loadStatusChangeBySlug` -> `state.LoadChange`, which already knows how to
  load the worktree authority for a bound change.
- Blast radius: status routing and worktree-binding resolution only. Existing
  delete recovery must continue to route genuinely partially deleted bundles to
  `slipway delete`.
- Constraints: `change.yaml` remains the current-state authority; git-local
  `worktree-binding.yaml` remains machine-local binding metadata and is not
  written into tracked `change.yaml`.

### Patterns
- Existing convention: worktree binding is git-local runtime state under
  `.git/slipway/runtime/changes/<slug>/worktree-binding.yaml` and is read by
  `readWorktreeBinding` (`internal/state/worktree_binding.go:72`).
- Existing explicit-change behavior is already safe because `LoadChange` can
  fall back to the bound worktree authority even when a root bundle directory is
  orphaned. Existing coverage: `TestLoadChangeFallsBackToSiblingWorktreeAuthorityWhenLocalBundleDirIsOrphaned`.
- Status already has a multi-active route that tries to use current worktree
  context, but it calls `FindActiveChangeForWorktree`, which depends on
  `ListChanges`; that is too late for this issue because orphan/stale scans can
  win first.

### Risks
- High: presenting `slipway delete --change <active-slug>` for a valid active
  change can cause data loss or a workflow dead end.
- Medium: suppressing legitimate orphan/stale diagnostics globally would break
  repairability and existing delete recovery tests.
- Low: reading runtime binding directly is local and reversible; if no matching
  active bound change exists, status falls back to the existing global path.
- Guardrail domains: no auth, PII, financial, schema migration, or external API
  contracts touched. The sensitive part is destructive recovery advice.
- Reversibility: the fix is a small routing priority change plus tests and can
  be rolled back independently.

### Test Strategy
- Existing coverage: delete recovery tests in `cmd/delete_test.go`; bound
  worktree status tests in `cmd/status_context_repair_test.go`; worktree binding
  tests in `internal/state/worktree_binding_test.go`.
- New regression coverage:
  - `TestStatusFromBoundWorktreePrefersActiveAuthorityOverRootOrphanSameSlug`
    reproduces issue #266 at the CLI/status layer.
  - `TestFindActiveChangeByWorktreeBindingIgnoresRootOrphanSameSlug` pins the
    new state resolver against same-slug root orphan residue.
- Verification approach: focused cmd and state tests first, then broader
  `go test ./cmd`, `go test ./internal/state`, and `go test ./...`.

### Options
- Option 1: Filter `OrphanBundleSlugs` globally when another workspace has an
  authority for the same slug. Tradeoff: broad semantic change could hide
  legitimate health/repair findings that intentionally scan all worktrees.
- Option 2: Reorder unscoped status to select any active change before delete
  recovery. Tradeoff: would likely break existing behavior that reports stale
  runtime bindings even when another active change exists.
- Option 3: Add a narrow runtime-binding resolver for the current git worktree
  and let unscoped status use it before global orphan/stale diagnostics.
  Tradeoff: adds a small state API, but keeps existing global repair behavior
  when the invocation is not inside a valid active bound worktree.
- Selected: Option 3. It matches issue #266 exactly: current bound worktree
  status should prioritize the valid active bound change, while root-level
  status and genuinely deleted bundles keep the existing delete recovery path.

## Unknowns
- Resolved: whether unscoped status should prefer the active bound worktree over
  root orphan residue for the same slug -> yes, confirmed by user on
  2026-06-19 and reproduced by the new failing test before the fix.
- Remaining: None.

## Assumptions
- The issue is caused by status route priority, not by Lattice-specific state,
  because the new cmd-level test reproduced the exact diagnostics payload with
  `orphaned_change_bundle` and `slipway delete --change <active-slug>`.
- Runtime binding is an acceptable local source for choosing the current
  invocation's active worktree before global diagnostics because it is already
  the documented machine-local binding authority and is ignored when written by
  another repository checkout.
- Existing delete recovery semantics should remain observable from root-level
  status, supported by `TestStatusRoutesToDeleteAfterPartialDelete` and
  `TestStatusReportsStaleRuntimeBindingWithAnotherActiveChange`.

## Canonical References
- `cmd/status.go:220` explicit status route.
- `cmd/status.go:243` current-worktree binding priority in unscoped status.
- `cmd/status.go:257` fallback `ListChanges` path.
- `cmd/status.go:274` global delete recovery diagnostics.
- `cmd/common.go:277` `resolveActiveChangeRef` worktree-based resolution.
- `internal/state/worktree_binding.go:72` runtime binding reader.
- `internal/state/worktree_binding.go:98` current-worktree runtime binding resolver.
- `cmd/delete_test.go:252` issue #266 status regression test.
- `internal/state/worktree_binding_test.go:69` state resolver regression test.
