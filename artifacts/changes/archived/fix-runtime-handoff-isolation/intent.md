# Intent

## Summary
fix runtime handoff isolation
## Complexity Assessment
complex
Rationale: the change touches Git-local runtime path contracts, session-start
handoff routing, generated skill/operator guidance, repair/health diagnostics,
and lock cleanup behavior. The lifecycle model should remain unchanged, but the
runtime hygiene boundary needs code, docs, and regression coverage.

## Guardrail Domains
None.

## In Scope
- Move runtime session handoff from the repo/scope-level
  `.git/slipway/runtime/handoff.md` path to a per-change runtime path such as
  `.git/slipway/runtime/changes/<slug>/handoff.md`.
- Update session-start handoff discovery, context-pressure guidance, generated
  workflow skill text, run-command guidance, and related tests so concurrent
  governed changes do not read or overwrite each other's handoff notes.
- Add health/repair visibility and cleanup for legacy runtime noise:
  repo-level `runtime/handoff*.md`, old `.git/slipway/changes/` runtime
  directories, and empty lock-anchor files without live metadata.
- Preserve the existing global create/repair lock design while improving stale
  or empty lock-anchor cleanup and operator diagnostics.

## Out of Scope
- Redesigning the Slipway lifecycle or stage model.
- Removing the workspace/scope-level `change-create.lock` and `repair.lock`
  semantics.
- Adding per-reviewer or per-executor git worktree isolation.
- Manually clearing the whole `.git/slipway` tree outside productized
  health/repair behavior.

## Constraints
- Runtime state that is consumed by gates must remain CLI-owned and
  per-change where it represents a governed change.
- Session handoff remains advisory continuation context, not lifecycle
  authority, governed evidence, freshness input, or a gate.
- Cleanup must fail safe: do not delete a lock that may be actively held, and
  do not silently discard ambiguous legacy handoff content.
- Keep the fix scoped to runtime hygiene and generated/user-facing guidance.

## Acceptance Signals
- Two active changes in separate worktrees can each have a session handoff
  without `session-start-hook` surfacing the wrong change's handoff.
- Legacy repo-level handoff files and legacy `.git/slipway/changes/` runtime
  directories are reported or repaired through product surfaces.
- Empty lock-anchor cleanup uses a safe non-blocking check and preserves active
  locks.
- Focused Go tests cover the new handoff path, legacy diagnostics/repair, and
  lock cleanup behavior.
- Relevant package tests and governed validation run successfully.

## Open Questions
None.

## Deferred Ideas
- A richer typed handoff registry with multiple named handoff documents per
  stage can be considered later if a real workflow needs it. This change should
  first restore one clear per-change session handoff contract.

## Approved Summary
Confirmed by user at 2026-06-19T17:31:41Z. Implement per-change runtime
handoff isolation, add productized cleanup/reporting for legacy runtime
handoff files, old runtime directories, and empty lock anchors, and keep global
create/repair lock semantics intact. Completion requires tests proving
handoff notes do not cross concurrent changes and cleanup does not remove live
lock state.
