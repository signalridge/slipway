# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
Fix the four workflow feedback items tracked in GitHub issue #24.

## Complexity Assessment
complex
Rationale: the change spans CLI recovery behavior, diagnostic JSON, generated
host-skill guidance, review checklist expectations, and regression tests.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Make stale execution evidence recoverable through a supported command path,
  or expose an exact safe route when automatic repair is not possible.
- Make run-summary-missing and missing task evidence blockers actionable by
  naming runtime task evidence paths and the required JSON envelope fields.
- Surface a clear done/status-time warning when a worktree-bound archived change
  still has source changes only in dirty or untracked Git state.
- Tighten governed review guidance so requirement-named negative paths and
  dependency/toolchain compatibility, including Rust MSRV drift, are explicit
  review evidence expectations.

## Out of Scope
- Changing Lattice implementation code.
- Requiring every `slipway done` invocation to commit or push Git changes.
- Reworking Slipway's full workflow preset or worktree provisioning policy.

## Constraints
- Preserve existing governance gates and fail-closed behavior.
- Keep generated templates and exported `.codex` / `.claude` skills in sync
  where the repository already tracks both.
- Keep changes scoped to issue #24 feedback rather than broad workflow redesign.

## Acceptance Signals
- Focused Go tests cover stale execution summary repair from runtime task
  evidence, unrepaired stale planning/invalid task evidence, actionable missing
  task evidence diagnostics, dirty worktree archive warnings, and review
  template content.
- `go test ./...` passes.
- `go run . validate --json` reports the governed change ready for its current
  stage after evidence is written.

## Open Questions
<!-- none -->

## Deferred Ideas
<!-- Identified but postponed ideas -->

## Approved Summary
Confirmed by user request on 2026-05-30T17:54:33Z: fix all issue #24 feedback
items in Slipway. The change will add a supported stale execution evidence
repair path, improve run-summary/task-evidence diagnostics, warn about dirty
worktree implementation state at done/status surfaces, and strengthen governed
review guidance for negative-path and dependency/toolchain checks.
