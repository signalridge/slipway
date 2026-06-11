# Intent

## Summary
Resolve GitHub issue #158: add a generated surface inventory manifest and fail-closed sync checks so generated skills, CLI commands, JSON contracts, and docs stay represented together.
## Complexity Assessment
complex
Rationale: this crosses generated skill templates, CLI command surfaces, docs, JSON/API contract surfaces, committed generated artifacts, and CI-facing tests. It also needs a regenerable workflow that can fail closed without weakening existing README coverage.

## Guardrail Domains
None detected.

## In Scope
- Add a Slipway-owned generated surface inventory manifest covering at least generated skills, CLI commands, JSON/user-facing contracts, and documentation rows.
- Add a regenerating command or script path with explicit check and write modes so the committed manifest can be verified and updated deterministically.
- Add a Go test or equivalent CI-facing sync check that fails when a new generated command/skill/contract surface lacks the expected manifest/doc/skill representation.
- Add docs-row/README-facing checks where they are derived from stable manifest rows and do not weaken existing tests.
- Preserve and keep running the existing README command-token/contract checks.
- Keep implementation grounded in Slipway's own generated-surface authorities and tests.

## Out of Scope
- Do not replace Slipway's generated-surface architecture or migrate command/skill generation to an external script model.
- Do not remove or weaken existing README/token surface tests.
- Do not finalize or delete unrelated active work beyond the minimal governance cleanup needed to unblock this change.

## Constraints
- Use current worktree behavior and generated templates as authority.
- Keep the manifest additive and self-correcting: stale committed output should be fixed by a write/regeneration mode, not by hand-editing generated JSON.
- Keep planning artifacts Slipway-specific.

## Acceptance Signals
- Adding or exposing a generated command, skill, JSON contract, or doc-facing surface without updating the surface manifest/doc row causes a test failure.
- Running the regeneration path updates the committed manifest deterministically and leaves no diff when already synced.
- Existing README surface/token tests still pass.
- `go test ./...` and governed Slipway readiness reach done-ready for this change.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] Which existing Slipway files own generated skills, command metadata, JSON/user-facing contracts, and docs rows, and which should be manifest inputs?
- [x] What inventory-manifest mechanics are worth adapting without importing an external product model?
- [x] Where should the committed manifest live, and what generator/test entrypoint best matches existing Slipway conventions?

## Deferred Ideas
- README prose count contract tests beyond the minimum issue acceptance can be added only if they stay low-maintenance and align with existing docs.

## Approved Summary
Confirmed by the user objective on 2026-06-10 and refined on 2026-06-10: use the Slipway governed flow to resolve every issue in GitHub #158 until done-ready, make best choices when blocked, keep artifacts Slipway-specific, and fully implement the issue rather than a narrow subset. This change will add a regenerable generated-surface inventory manifest plus fail-closed sync coverage for commands, skills, JSON/contracts, docs rows, and stable README-facing coverage, while explicitly preserving existing README token checks and leaving unrelated worktrees/code outside scope.
