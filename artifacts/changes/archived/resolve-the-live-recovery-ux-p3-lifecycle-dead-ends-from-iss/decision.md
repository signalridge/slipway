# Decision
## Project Context
- Tech Stack: Go
- Conventions: engine packages under internal/engine (read-only over model); cmd thin orchestrators; model is a leaf; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Selected Approach
Make every live #86 dead-end name an executable next action by reusing existing
primitives and the post-#99 `slipway run` recovery model — no new command
surfaces, no new git mutation, no fail-open path.

1. **Worktree branch-mismatch rebind (REQ-001).** Reach the existing
   worktree-preflight rebind for the bound-but-mismatched case: relax the
   `WorktreePath == ""` precondition in `resolveS2Execute`, `DeriveWorktreeBlockers`,
   and the advance/readiness gates so a `dedicated_worktree_branch_mismatch`
   re-enters preflight. Retarget the recovery vocabulary
   (`internal/model/recovery.go`) for `dedicated_worktree_branch_mismatch` from
   `slipway repair` to `slipway run`. worktree-preflight stays the sole writer of
   `WorktreeBranch`; the rebind reconciles recorded metadata to the worktree's
   actual git HEAD (no `git checkout`).
2. **Repair actionability (REQ-002).** In `cmd/repair.go`: the dual-active finding
   names the conflicting slugs (already in `activeChanges`/`unique`) plus
   `slipway status` / `slipway cancel --change <slug>` / `slipway done --change <slug>`;
   the `repairDriftNextAction` default returns the `slipway run` reopen
   instruction (matching `governanceDigestRunNextAction`).
3. **Abort guidance (REQ-003).** In `cmd/abort.go`, the `case "repair":` arm also
   names `slipway run` as the interrupted-execution clearer.
4. **S2 scope guidance parity (REQ-004).** In
   `internal/engine/progression/readiness.go`, drop the
   `state != S3 && state != S4` early-return in `scopeContractNeedsRecoveryGuidance`
   so the diagnostic is emitted whenever there are scope blockers.

## Key Decisions
- Reuse worktree-preflight as the single branch-authority writer; do NOT add a
  branch-rebind to `repair` (rejected: second writer, non-`run` path). [research]
- Route both repair gaps and the abort marker to `slipway run`, the canonical
  post-#99 mutating recovery action. [research]
- Do NOT make `repair` clear `InterruptedExecutionAt` (rejected: wrong layer;
  would forge a resumed state without execution evidence). [research]
- S2 scope item is narrative parity only; the executable path is already covered
  by per-blocker remediation + the #102 `scopeContractReopenTarget` gate, so the
  fix is a one-line gate relaxation, not a new mechanism. [research]
- Drop #86 item 5 (restamp/recover/Tier docs): obsolete — PR #99 removed that
  surface. [intent]
- Guardrail domain `external_api_contracts` because public CLI/JSON recovery
  vocabulary changes; preserve recovery object field shape with contract tests.

## Rejected Alternatives
- Add `RepairBoundWorktreeBranchBinding` to repair (Option B for REQ-001): more
  code, second `WorktreeBranch` writer, keeps a non-`run` recovery path.
- Make `repair` clear `InterruptedExecutionAt` (Option b for REQ-003): couples a
  "resumed" state to repair with no execution evidence.
- A dedicated `slipway worktree rebind` subcommand: unnecessary new surface; the
  `slipway run` re-walk covers it.

## Interfaces and Data Flow
- `internal/engine/progression/skill_resolution.go` `resolveS2Execute`: route to
  worktree-preflight when `WorktreePath == ""` OR the bound worktree fails
  authenticity with `dedicated_worktree_branch_mismatch`.
- `internal/engine/progression/validation.go` `DeriveWorktreeBlockers` +
  `internal/engine/progression/readiness.go` + `advance_governed.go`: admit the
  branch-mismatch case into the re-derive path so fresh preflight evidence
  overwrites the stale `WorktreeBranch`.
- `internal/model/recovery.go`: `dedicated_worktree_branch_mismatch` →
  `CommandTemplate: "slipway run"`; generic repair drift → `slipway run`.
- `cmd/repair.go`: dual-active finding string + `repairDriftNextAction` cases.
- `cmd/abort.go`: `case "repair":` guidance string.
- No schema changes; recovery JSON object fields unchanged (only vocabulary
  values).

## Rollout and Rollback
- Rollout: behavior is the recovery vocabulary/routing itself; no flag. Verified
  by `go build/vet/test ./...`, `go test ./internal/toolgen/...`,
  `go run . init --refresh --tools all` + `git diff --check` (zero drift),
  contract/regression tests per dead-end, and `go run . validate --json`.
- Rollback: revert the branch. No data migration; the worktree rebind only
  reconciles recorded metadata, reversible by reverting. Public docs/generated
  surfaces revert with code (external contract change).

## Risk
- Relaxing the `WorktreePath == ""` precondition could re-enter preflight for an
  unintended case → gate strictly on the `dedicated_worktree_branch_mismatch`
  authenticity reason, and rely on preflight's existing fail-closed authenticity
  check (registered + dedicated worktree only); add a regression test.
- Vocabulary changes could break host tools parsing recovery JSON → preserve the
  object field shape; only values change; contract tests assert the new values
  and the stable shape.
- Dual-active guidance must name real commands → confirmed `slipway cancel`/`done`
  accept `--change`; test asserts the rendered command.
- S2 scope parity must not change the executable next action → it only adds the
  narrative diagnostic; per-blocker remediations are unchanged; test asserts the
  diagnostic presence without altering blockers.
