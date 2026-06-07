# Research

Investigation mapped each #86 dead-end against current `main` (release 0.10.0,
which already contains PR #99 and PR #102). Exact code locations and minimal,
fail-closed fixes were confirmed for each.

## Alternatives Considered

### Dead-end 1 — worktree branch-mismatch rebind
- Detection: `internal/state/worktree.go` `ValidateWorktreeAuthenticityReasons`
  appends `dedicated_worktree_branch_mismatch` (reason `internal/model/reason_code.go`).
- Trap: `internal/engine/progression/skill_resolution.go` `resolveS2Execute` and
  `internal/engine/progression/readiness.go` (~`DeriveWorktreeBlockers` fork) route
  to worktree-preflight ONLY when `WorktreePath == ""`. A bound-but-mismatched
  worktree falls to the `else` branch → permanent blocker, no rebind route.
- Hollow remediation: `internal/model/recovery.go` maps
  `dedicated_worktree_branch_mismatch` to `CommandTemplate: "slipway repair"`, but
  `internal/state/repair.go` `RepairBoundWorktreeScopeMetadata` never reads/writes
  `WorktreeBranch` and runs no git — repair cannot resolve it.
- **Option A (SELECTED):** make the existing worktree-preflight rebind primitive
  reachable for the branch-mismatch case (relax the `WorktreePath == ""`
  precondition) and retarget the recovery vocabulary to `slipway run`.
  worktree-preflight remains the sole writer of `WorktreeBranch`; `slipway run`
  re-enters it and reconciles the recorded branch to the worktree's actual HEAD.
- Option B (rejected): add a branch-rebind to `slipway repair`. Introduces a
  second writer of the branch authority, fighting the single-binder invariant;
  more code; keeps a non-`run` recovery path.

### Dead-end 2 — repair non-actionable findings
- Dual-active: `cmd/repair.go` emits "multiple active changes require operator
  intervention" with the generic default next-action — even though the
  conflicting slugs are already in hand (`activeChanges` / `unique` map). The
  sibling `cmd/common.go` `wrapResolutionError` already shows the actionable form
  (`--change <slug>` + `slipway status`).
- Generic drift: `repairDriftNextAction` default returns "inspect the named
  artifact and rerun the owning Slipway command after correction" — names no
  command, although `governanceDigestRunNextAction` already produces the
  actionable `slipway run` form for digest drift.
- **SELECTED:** dual-active names the slugs + `slipway status` /
  `slipway cancel --change` / `slipway done --change`; generic-drift default
  routes to `slipway run` (reopen the earliest affected authority). Reuses
  existing recovery vocabulary; `repairDriftFinding.NextAction` already carries
  executable commands.

### Dead-end 3 — abort→repair loop
- `InterruptedExecutionAt` is SET at `cmd/abort.go` and CLEARED ONLY at
  `cmd/run.go` (on a successful governed run, emitting a `resume.succeeded`
  event). `slipway repair` never touches it (grep: zero hits in repair paths).
- In the broken-execution branch, abort advises `slipway repair`; repair ignores
  the marker; `slipway status` still shows it → repair↔status loop. The common
  path already advises `run`/`run --resume` correctly via
  `projectStatusExecutionAction`.
- **Option (a) (SELECTED):** abort's `case "repair":` guidance also names
  `slipway run` as the step that clears the interrupted-execution marker. One
  string; keeps repair in the flow (it fixes the broken summary/checkpoint) but
  names the legitimate clearer.
- Option (b) (rejected): make repair clear the marker — wrong layer; the marker
  is runtime execution state coupled to a successful `run` (forging a "resumed"
  state without execution evidence violates evidence discipline).

### Dead-end 4 — S2 scope guidance (narrative parity, cosmetic)
- `internal/engine/progression/readiness.go` `scopeContractNeedsRecoveryGuidance`
  returns false for S2_EXECUTE (only S3/S4 get the prose guidance diagnostic).
- The executable next action at S2 is ALREADY adequate: per-blocker remediations
  (`internal/model/recovery.go` scope reason codes, including `slipway run` for
  changed-files-missing) plus the `scopeContractReopenTarget` advance gate from
  PR #102 (`advance_governed.go` / `stale_evidence_recovery.go`) that reopens to
  S2 on a scope-contract failure. The suppressed diagnostic is narrative only.
- **SELECTED:** one-line gate relaxation so the diagnostic is emitted at S2 too,
  for surface parity (the executable path is already covered, so this is
  explanatory consistency, not a functional dead-end fix).

## Unknowns
- None blocking. All four dead-ends were located in current code with confirmed
  fix shapes; the post-#99/#102 recovery model (`slipway run` as the mutating
  recovery action) is the consistent target for the retargeted vocabulary.

## Assumptions
- worktree-preflight re-derivation overwrites a stale `WorktreeBranch` from fresh
  preflight evidence (supported by `ApplyWorktreeMetadata` /
  `PersistScopeWorktreeMetadata` being the binding writers).
- `slipway cancel` and `slipway done` accept `--change <slug>` (confirmed:
  `addChangeSelectorFlags`), so the dual-active guidance names real commands.
- Public recovery JSON field shape (`primary_command`/`primary_action`/
  `recovery_class`/`steps[]`) is a stable external contract; only vocabulary
  values change.

## Canonical References
- `intent.md` (this bundle) — request and scope.
- Dead-end 1: `internal/state/worktree.go`, `internal/engine/progression/skill_resolution.go`,
  `internal/engine/progression/readiness.go`, `internal/engine/progression/validation.go`,
  `internal/engine/progression/advance_governed.go`, `internal/model/recovery.go`,
  `internal/model/reason_code.go`, `internal/state/repair.go`.
- Dead-end 2: `cmd/repair.go` (dual-active block; `repairDriftNextAction`;
  `governanceDigestRunNextAction`), `cmd/common.go` (`wrapResolutionError`).
- Dead-end 3: `cmd/abort.go`, `cmd/run.go`, `cmd/status_view_build.go`
  (`projectStatusExecutionAction`), `internal/state/health.go`.
- Dead-end 4: `internal/engine/progression/readiness.go`
  (`scopeContractNeedsRecoveryGuidance`), `internal/model/recovery.go` (scope
  reason codes), `internal/engine/progression/stale_evidence_recovery.go`
  (`scopeContractReopenTarget`, PR #102).
