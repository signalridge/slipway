# Intent

## Summary
Repair the P0 public lifecycle surface contracts described in `opt.md` section 1 so `status`, `validate`, `next`, `run`, `done`, and `evidence` report consistent route, action, freshness, and capability semantics for the same governed change.

## Complexity Assessment
complex
This touches multiple CLI public surfaces, lifecycle readiness/action reporting, governed evidence freshness semantics, and host capability fail-closed behavior. The work must be planned and verified as one coherent lifecycle surface repair because the `opt.md` acceptance criteria explicitly require the contracts to agree across commands.

## Guardrail Domains
Lifecycle governance, public CLI contract, and agent workflow safety. No credential, auth, schema migration, or financial domain is in scope.

## In Scope
- Implement a shared `InvocationRoute` or equivalent route model across `status`, `validate`, `next`, `done`, and `evidence`, including invocation workspace, change authority path, bound workspace path, route kind, local/effective lifecycle execution allowance, next command, and remediation.
- Make explicit `--change <slug>` missing behavior fail closed as `change_not_found` with exit code 3 for `validate`, aligned with `status`, `next`, `done`, and `evidence`.
- Split or supplement freshness reporting so execution evidence freshness, governance/skill evidence freshness, and overall readiness freshness cannot be confused.
- Align `status` and `validate` with `next` for current action kind, especially S3 review batch pending behavior.
- Expose host capability/delegation prerequisites for selected skills in CLI-visible output and fail closed or present explicit fallback when unavailable, covering the #339 dead-end class.
- Add black-box CLI and targeted tests for the route, missing explicit slug, freshness, action contract, and capability cases required by `opt.md` 1.1-1.5.

## Out of Scope
- Release and supply-chain hardening from `opt.md` section 2.
- Architecture boundary and coverage gate work from `opt.md` section 3, except when a minimal test is required to protect this change's touched surface.
- State-read performance work from `opt.md` section 4, already handled by prior performance changes.
- Backward compatibility layers for retired behavior. Per user instruction, implementations should replace incorrect contracts directly rather than preserve legacy compatibility shims.
- Broad docs polish, persistent indexes, lifecycle append rewriting, or unrelated CLI refactors.

## Constraints
- Current worktree CLI behavior is authority; do not infer lifecycle state from memory or root checkout state.
- Preserve unrelated dirty files in the root checkout, including `opt.md`, `.gemini/`, and `coverage.out`.
- Do not commit ignored runtime artifacts such as `.serena/`, lifecycle `events/`, or `verification/` contents unless they are explicitly non-ignored governed artifacts.
- No compatibility layer should be added for the retired or misleading behavior being removed.

## Acceptance Signals
- Root and bound worktree black-box `status`, `next`, and `validate` route/action contract behavior is consistent and tested.
- `validate --change definitely-not-a-change --json` exits 3 and returns stable `error_code: change_not_found`.
- Execution-fresh plus governance-stale cases report overall readiness as blocked/stale and do not present a single misleading freshness field.
- S3 review batch pending fixtures show `next`, `status`, and `validate` agree on the action kind.
- Host capability unavailable cases do not report prior authorization as sufficient when the selected skill cannot actually run; they fail closed or expose an explicit fallback.
- Targeted tests pass, then `go test ./... -count=1` passes before final review/ship.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
None

## Deferred Ideas
- Release workflow, branch protection, and supply-chain security hardening remain separate governed changes.
- Wider public-surface coverage gate expansion remains separate unless needed for this change's touched files.

## Approved Summary
Confirmed by the user's blanket approval on 2026-06-27 to continue the original task and allow subsequent governed actions. This change will repair `opt.md` section 1.1-1.5 as one coherent P0 lifecycle surface fix: shared route semantics, explicit missing `--change` fail-closed behavior, split freshness semantics, `status`/`validate` action alignment with `next`, and CLI-visible host capability/delegation fallback behavior. It excludes release security, architecture gate expansion, state-read performance, unrelated refactors, and compatibility layers for retired behavior.
