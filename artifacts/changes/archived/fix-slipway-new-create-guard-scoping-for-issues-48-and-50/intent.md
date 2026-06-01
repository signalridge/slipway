# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack:
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
Fix `slipway new` create-guard behavior for GitHub issues #48 and #50 after
confirming each reported problem against the live code and targeted
reproductions.
## Complexity Assessment
complex
Rationale: the change touches governed lifecycle creation, worktree authority,
and cross-worktree active-change discovery. Regressions could block normal
governed work or accidentally allow true same-workspace collisions, so the fix
needs issue-specific reproduction coverage plus full workflow verification.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Confirm #48: an unbound intake-stage active change can make `slipway new`
  fail globally and the current remediation can point to a non-unblocking path.
- Confirm #50: a bound active change in a sibling dedicated worktree can make
  `slipway new` fail from the repo root or another worktree even when normal
  reads are worktree-scoped.
- Fix the create guard and related user-facing diagnostics/remediation so
  unrelated worktree-scoped governed changes can be started while preserving
  same-workspace collision protection.
- Add regression tests covering the confirmed #48 and #50 scenarios and any
  changed diagnostic/remediation contract.
- Update governed artifacts and verification evidence required by the Slipway
  lifecycle for this change.

## Out of Scope
- Reworking the full runtime active-change storage model proposed by #46 unless
  live code inspection proves it is strictly required for #48/#50.
- Implementing a new long-lived `worktree bind`, `park`, or global scheduler
  command unless the existing guard cannot be corrected safely without it.
- Changing non-governed `wt-*` worktree behavior or unrelated issue scopes such
  as #44/#46/#47 except as reproduction context.
- Weakening protections that prevent two active governed changes from sharing
  the same workspace authority.

## Constraints
- Follow the Slipway governed lifecycle in the generated worktree.
- Treat GitHub issue bodies and current worktree code as authoritative; do not
  rely on stale memory for the bug classification.
- Keep edits minimal and aligned with existing lifecycle/state-store patterns.
- Prefer repo-native Go tests and fresh `go test -count=1 ./...` verification
  before closeout.

## Acceptance Signals
- A targeted test reproduces and then passes for #48's unbound intake-stage
  create-guard behavior.
- A targeted test reproduces and then passes for #50's bound sibling-worktree
  create-guard behavior.
- Existing same-workspace active-change collision tests still pass or are
  updated to the intended contract.
- `go test -count=1 ./...` and Slipway validation/closeout gates pass from the
  governed worktree.

## Open Questions
<!-- No unresolved intake clarification questions. -->

## Deferred Ideas
- A dedicated `slipway worktree bind` / `park` UX for operators who explicitly
  want to isolate an intake-stage change without advancing it.
- A configurable repo-wide serialization policy if future governance decides
  one active governed change across all worktrees is the desired default.

## Approved Summary
Confirmed from the user objective: solve GitHub issues #48 and #50 through the
Slipway lifecycle, confirm each report against current evidence, and fully fix
only problems that truly exist. The implementation will focus on create-guard
scoping/remediation for unbound and bound active changes, preserve real
same-workspace collision protection, exclude broader #46 storage migration and
new command surfaces unless required, and verify with targeted regressions plus
fresh full Go and Slipway closeout checks. Confirmation timestamp:
2026-06-01T15:06:41Z.
