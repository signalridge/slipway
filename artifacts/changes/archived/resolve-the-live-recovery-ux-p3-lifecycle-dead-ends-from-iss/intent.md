# Intent

## Summary
Resolve the live Recovery UX P3 lifecycle dead-ends from issue #86 (rescoped) so
every blocked governance state names an executable next action. Three confirmed
user-facing dead-ends are fixed plus one narrative-parity gap, and two obsolete
items are dropped:

1. **Worktree branch mismatch has no rebind path.** A change bound to a worktree
   whose actual git branch no longer matches the recorded `WorktreeBranch` emits
   `dedicated_worktree_branch_mismatch`, whose remediation points to
   `slipway repair` — but repair has no branch-rebind code, so the remediation is
   hollow and the change is stranded. Fix: make the existing worktree-preflight
   rebind primitive reachable for the branch-mismatch case and retarget the
   recovery vocabulary to `slipway run` (which re-enters preflight and rebinds the
   recorded branch to the worktree's actual branch).
2. **`slipway repair` non-actionable findings.** The dual-active finding
   ("multiple active changes require operator intervention") and the generic
   drift fallback ("inspect the named artifact and rerun the owning Slipway
   command after correction") name no executable command. Fix: dual-active names
   the conflicting slugs plus `slipway status` / `slipway cancel --change` /
   `slipway done --change`; the generic-drift default routes to `slipway run`
   (reopen the earliest affected authority), consistent with the post-#99
   recovery model.
3. **abort → repair loop.** `slipway abort` sets `InterruptedExecutionAt`, and in
   the broken-execution-state branch advises `slipway repair`, but only
   `slipway run` clears that marker and repair never touches it — so
   repair↔status can loop. Fix: abort's repair branch also names `slipway run` as
   the step that clears the interrupted-execution marker and continues.
4. **S2 scope guidance narrative parity (cosmetic).** The scope-contract recovery
   guidance diagnostic is suppressed at S2_EXECUTE (shown only at S3/S4). The
   executable next action at S2 is already adequate (per-blocker remediation +
   the `scopeContractReopenTarget` advance gate from #102), so this is narrative
   only: enable the same explanation at S2 for surface parity.

## Complexity Assessment
complex
<!-- Rationale -->
Touches lifecycle recovery routing and the public recovery vocabulary across
several surfaces (worktree-preflight routing, repair findings, abort guidance,
scope guidance). Each fix keeps blocked states fail-closed while making the next
action executable; the worktree rebind reuses an existing primitive rather than
adding a new git mutation. No new schema or data migration. Not critical because
there is no fail-open risk: every change either reuses an existing fail-closed
primitive or only rewrites a remediation string into an executable command.

## Guardrail Domains
`external_api_contracts` — intent-based classification. The change alters
externally consumed Slipway CLI/JSON recovery behavior: the
`dedicated_worktree_branch_mismatch` recovery `CommandTemplate`/remediation, the
`slipway repair` dual-active and generic-drift finding strings + next actions,
the `slipway abort` guidance text, and the S2 scope guidance surface. Public
recovery JSON field shape (`primary_command`, `primary_action`,
`recovery_class`, `steps[]`) MUST stay stable; intentional vocabulary changes
need contract tests and docs/generated-surface updates. The worktree rebind only
reconciles Slipway's recorded `WorktreeBranch` to the worktree's actual git HEAD
(no `git checkout`/HEAD move, no working-tree mutation), and the preflight
authenticity check still fail-closes against unregistered/non-dedicated
worktrees, so it is not an irreversible/sensitive git operation.

## In Scope
- Worktree branch-mismatch rebind path: route the bound-but-mismatched worktree
  through the existing worktree-preflight rebind (relax the `WorktreePath == ""`
  precondition for the branch-mismatch case in `resolveS2Execute`,
  `DeriveWorktreeBlockers`, and the advance/readiness gates), and retarget the
  `dedicated_worktree_branch_mismatch` recovery vocabulary from `slipway repair`
  to `slipway run`.
- `slipway repair` actionability: dual-active finding names the conflicting slugs
  and executable resolution commands; generic-drift `repairDriftNextAction`
  default routes to `slipway run`.
- `slipway abort` repair-branch guidance also names `slipway run` as the
  interrupted-execution clearer.
- S2 scope guidance narrative parity: emit the scope-contract recovery guidance
  diagnostic at S2_EXECUTE as well (one-line gate relaxation).
- Contract tests for each changed public surface; docs/generated-surface
  alignment; regression replays for each dead-end.

## Out of Scope
- Issue #86 item 5 (reconcile docs with the restamp/recover/Tier surface):
  obsolete — PR #99 removed that surface entirely; there is no such
  reconciliation left to do.
- Adding a `git checkout`/branch-switch operation (the rebind only reconciles
  recorded metadata to the actual HEAD).
- Issues #95 (premature execution-completeness advance), #92 (health/validate
  assurance disagreement), #91 (mechanical seed substance gate), #88
  (safety_baseline satisfy path / repair --focus sast), #80 (codebase-map
  staleness), #75 (force-close) — separate efforts.
- Dual-active *prevention* (stopping two changes from becoming active); this
  change only makes the existing dual-active repair finding actionable.

## Constraints
- Public CLI/JSON recovery contract: preserve documented recovery object fields;
  intentional vocabulary changes require contract tests + docs updates.
- Worktree rebind reuses worktree-preflight as the sole writer of
  `WorktreeBranch`; do not add a second branch-authority writer.
- Blocked states stay fail-closed: every fix either reuses an existing
  fail-closed primitive or only rewrites a remediation string to an executable
  command; no force/bypass path is added.
- Import layering: `progression` may import `state`/`model`; `model` stays a
  leaf.
- Verify against the repo's own loop: `go build/vet/test ./...`,
  `go test ./internal/toolgen/...`, and current-worktree
  `go run . init --refresh --tools all` with zero project-visible drift.
- Self-dogfood through the current worktree binary; any node that requires
  guessing or source-reading to proceed is a defect to fix in-repo.

## Acceptance Signals
- `go build ./...`, `go vet ./...`, `go test ./...` green; toolgen self-loop has
  zero drift; `go run . validate --json` green from the current worktree.
- A change bound to a worktree on a mismatched branch surfaces a recovery whose
  `primary_command` is `slipway run`, and `slipway run` rebinds the recorded
  branch to the worktree's actual branch and continues (no `slipway repair`
  dead-end).
- `slipway repair` with multiple active changes names the conflicting slugs and
  an executable command (`slipway status` / `slipway cancel --change` /
  `slipway done --change`); a generic drift finding's next action is
  `slipway run`, never "inspect the named artifact and rerun".
- After `slipway abort` in the broken-execution branch, the printed guidance
  names `slipway run` as the step that clears the interrupted-execution marker;
  abort→run resolves without a repair↔status loop.
- The scope-contract recovery guidance diagnostic is present at S2_EXECUTE as
  well as S3/S4.
- Recovery JSON object field shape is unchanged; intentional vocabulary changes
  are covered by contract tests; docs/generated surfaces match.

## Open Questions
<!-- None: the four dead-ends were mapped against current main (release 0.10.0)
with exact code locations and minimal fixes confirmed; no blocking unknowns. -->

## Deferred Ideas
- Preventing dual-active states from arising (vs. making the repair finding
  actionable) — separate hardening.
- A dedicated `slipway worktree rebind` subcommand (the `slipway run` re-walk
  covers the case without a new command surface).

## Approved Summary
Deliver the rescoped issue #86 as an `external_api_contracts` governed change:
make every live lifecycle dead-end name an executable next action — worktree
branch-mismatch rebinds via the existing preflight primitive routed by
`slipway run`; `slipway repair` dual-active and generic-drift findings name
executable commands; `slipway abort` points to `slipway run` to clear the
interrupted-execution marker; and the S2 scope guidance gains narrative parity.
The obsolete restamp/recover/Tier docs item is dropped (the surface no longer
exists), and `git`-mutating rebind, dual-active prevention, and the other open
issues are out of scope. No fail-open or force/bypass path is introduced; public
recovery JSON shape is preserved with contract tests for the intentional
vocabulary changes.

Confirmed by user: 2026-06-06 (goal: resolve P3 issues #86/#80 to done-ready).
