# Intent

## Project Context
- Tech Stack: Go CLI
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
INT-001: Resolve open GitHub issues #59, #61, and #62 by making governance health diagnostics actionable, making handoff confirmation output distinguish non-checkpoint actions from checkpoint resume, and making goal-verification placeholder scans portable on macOS.

## Complexity Assessment
critical

## Guardrail Domains
external_api_contracts

## In Scope
- INT-001A: Resolve GitHub issue #59:
  - prevent `slipway health --governance` from serving stale governance-control or traceability signals after lifecycle/control/artifact changes, or otherwise provide an explicit refresh/recompute path if caching remains intentional.
  - align `validate`, `run`, and `health --governance` diagnostics enough that operators can identify the authoritative next action when the surfaces differ.
  - expose actionable traceability/governance gap identities in JSON diagnostics or observation output so operators do not need to infer missing refs by manual bisection.
- INT-001B: Resolve GitHub issue #61:
  - make hard-stop confirmation output actionable by either creating a resumable checkpoint accepted by `--resume-response` or changing diagnostics to name the correct non-checkpoint next action.
  - cover the behavior with regression tests for the reported review-handoff case.
- INT-001C: Resolve GitHub issue #62:
  - remove GNU-only `grep -P` from generated goal-verification placeholder-scan instructions, or include a macOS-compatible fallback command.
  - update tests/goldens for the generated skill/template content so this does not regress.
- INT-001D: Keep the implementation scoped to Slipway CLI/governance-health/progression/template behavior needed by those issues.
- INT-001E: Resolve GitHub issue #59 item 4 for the wave-execution gate by ensuring runtime task evidence freshness (`captured_at`) before a `wave-orchestration` record can satisfy the gate. The planning-gate (plan-audit) half is deferred to #66: mtime-based source-freshness false-positives on Slipway's own `tasks.md` checkbox writeback, so it is replaced by a content-digest approach there rather than shipped here.
- INT-001F: Resolve GitHub issue #59 item 2/item 3 by documenting and testing the authority boundary between active readiness (`validate`), mutating transition result (`run`), and diagnostic health feedback (`health --governance`).

## Out of Scope
- Do not redesign the whole governance kernel, workflow state model, or evidence schema unless a minimal fix proves impossible.
- Do not address unrelated open issues, previous issue #53 tier work, or Lattice product-code behavior.
- Do not close/archive the governed change; stop at the Slipway close-ready/done-ready boundary before final close.
- Do not close the GitHub issues from this change unless the user later asks for issue-management follow-through.

## Constraints
- Preserve existing JSON contracts where possible; this change is classified as `external_api_contracts`, so any changed fields must be deliberate, documented by tests, and backward-compatible when practical.
- Favor source-backed diagnostics over cosmetic wording changes: issue #59 requires the health/validate/run surfaces to reflect current authority.
- The default agent environment includes macOS/BSD userland, so verification instructions must not require GNU-only flags without a fallback.
- Preserve unrelated local worktrees and ignored runtime state.

## Acceptance Signals
- Issue #59: tests prove governance-health data is invalidated or recomputed after relevant lifecycle/control/artifact changes, JSON diagnostics include actionable traceability/gap identities, stale planning/runtime evidence fails closed, and command surfaces document the validate/run/health authority boundary.
- Issue #61: tests prove a hard-stop confirmation boundary no longer emits a non-actionable `--resume-response` path without an active checkpoint.
- Issue #62: generated goal-verification instructions no longer require GNU `grep -P` on macOS, and template/golden tests cover the portable placeholder scan.
- Full verification before close-ready includes `go test ./...`, `go build ./...`, and Slipway `validate --json` for this change.

## Open Questions
(none)

## Resolved Ownership Notes
- health ownership: `cmd/health.go` recomputes governance snapshots and `internal/engine/governance/health.go` renders health checks.
- confirmation ownership: `cmd/next.go` derives `confirmation_requirement`; `cmd/next_context_build.go` handles actual checkpoint resume.
- placeholder scan source: `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`.

## Deferred Ideas
- Broader governance UX consolidation across all command surfaces.
- Large-scale health/validate/run schema redesign.
- Closing or commenting on the GitHub issues after implementation.

## Approved Summary
Approved from the user's standing goal instruction on 2026-06-03T15:22:26Z: implement one governed change for open issues #59, #61, and #62, make best-effort autonomous decisions if blockers appear, and continue until the change is ready to close but before final close/archive. The change is limited to Slipway governance-health freshness/diagnostics, hard-stop confirmation actionability, and macOS-portable goal-verification placeholder-scan instructions. Out of scope are unrelated issues, broad governance redesign, Lattice product-code changes, and final close/archive.
