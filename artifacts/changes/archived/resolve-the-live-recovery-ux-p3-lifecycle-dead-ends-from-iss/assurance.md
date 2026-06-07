# Assurance
## Project Context
- Tech Stack: Go
- Conventions: engine packages under internal/engine (read-only over model); cmd thin orchestrators; model is a leaf; one verdict-evidence YAML per skill under verification/.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Resolves the live Recovery UX P3 lifecycle dead-ends from issue #86 (rescoped) so
every blocked governance state names an executable next action: (1) a bound
worktree on a mismatched git branch now reconciles its recorded branch to the
worktree's actual HEAD via `slipway run` (recovery vocabulary retargeted from the
hollow `slipway repair`); (2) `slipway repair` dual-active findings name the
conflicting slugs plus `slipway status` / `slipway cancel --change` /
`slipway done --change`, and the generic-drift default routes to `slipway run`;
(3) `slipway abort`'s repair branch also names `slipway run` as the
interrupted-execution clearer; (4) the scope-contract recovery guidance
diagnostic gains surface parity at S2_EXECUTE. The obsolete #86 item 5
(restamp/recover/Tier docs) is dropped — PR #99 removed that surface. Classified
`external_api_contracts` because public CLI/JSON recovery vocabulary changes;
recovery JSON object field shape (`primary_command`/`primary_action`/
`recovery_class`/`steps[]`) is preserved.

## Verification Verdict
Pass for implementation, tests, and review up to the S4 goal/closeout handoff.
`go build ./...`, `go vet ./...`, `go test ./...` are green; `golangci-lint run`
reports 0 issues; `go run . init --refresh --tools all` + `git diff --check`
leaves zero project-visible drift (no generated surface references the changed
vocabulary). Each dead-end carries a focused regression test.

## Evidence Index
- Runtime execution: `verification/wave-plan.yaml` + `verification/execution-summary.yaml`
  record run_summary_version 1 with all five planned tasks passing.
- Worktree rebind (t-01): `internal/state/worktree.go` `ReconcileWorktreeBranchBinding`;
  tests `TestReconcileWorktreeBranchBindingRealignsBranchMismatch`,
  `TestReconcileWorktreeBranchBindingLeavesNonBranchMismatchAlone`,
  `TestBuildRecoveryWorktreeBranchMismatchRoutesToRun`.
- Repair actionability (t-02): `cmd/repair.go`; tests
  `TestBuildUnrepairedDriftFindingsKeepsActionableTargets`,
  `TestRepairDriftNextActionGenericAndDualActive`.
- Abort guidance (t-03): `cmd/abort.go`; test `TestAbortRepairBranchGuidanceNamesRun`.
- Scope parity (t-04): `internal/engine/progression/readiness.go`; test
  `TestScopeContractRecoveryGuidanceHasSurfaceParity`.
- Proof (t-05): build/vet/test, golangci-lint, init-refresh zero-drift.

## Requirement Coverage
- REQ-001: `internal/state/worktree.go` + `internal/engine/progression/advance_governed.go`
  (reconcile) + `internal/model/recovery.go` (vocab → slipway run); reconcile +
  recovery tests.
- REQ-002: `cmd/repair.go` dual-active + `repairDriftNextAction`; repair tests.
- REQ-003: `cmd/abort.go` repair-branch guidance; abort test.
- REQ-004: `internal/engine/progression/readiness.go` `scopeContractNeedsRecoveryGuidance`;
  scope parity test.
- REQ-005: recovery JSON shape preserved (only vocabulary values change), each
  change test-covered, init-refresh zero drift.

## Residual Risks and Exceptions
- The worktree rebind reconciles recorded metadata to the worktree's actual git
  HEAD via the canonical `PersistScopeWorktreeMetadata` setter (no `git checkout`,
  no second branch-authority writer); it fires only for a pure branch mismatch on
  an otherwise-valid dedicated worktree and fails closed for every other
  authenticity failure (unbound/invalid/unregistered/non-dedicated).
- The S2 scope guidance change is narrative parity only; per-blocker remediations
  and the scope-contract advance-reopen gate are unchanged, so the executable
  next action at S2 is unaffected.
- The codebase map remains advisory; source, governed artifacts, runtime
  evidence, and live `go run .` output are the closeout authorities.

## Rollback Readiness
Rollback is a branch revert. No data migration; the worktree rebind only
reconciles recorded metadata and is reversible. Public docs/generated surfaces
require no change (zero drift), so rollback restores prior behavior with code
alone.

## Archive Decision
Proceed to done-ready after S4 `goal-verification` and `final-closeout` pass,
including `check:external_api_contracts.safety_baseline=pass` and
`closeout:assurance_complete=pass`. Active `go run . validate --json` freshness/
readiness proof must be captured before `slipway done`; archived bundles are not
described as revalidated through the active validate gate.
