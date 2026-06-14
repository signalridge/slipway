# Intent

## Summary
Resolve GitHub issue #155: Knuth-invariant overwrite-only-own-defaults for prose artifact edit materiality and auto-reopen softening, referencing local gsd-core
## Complexity Assessment
complex
Rationale: this changes lifecycle freshness and auto-reopen behavior for
governed prose artifacts. It must preserve fail-closed evidence behavior while
distinguishing scaffold/default edits from human-authored material edits.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Resolve GitHub issue #155 for prose-artifact edit materiality, specifically
  intent.md, requirements.md, research.md, and decision.md digest/reopen
  behavior.
- Add a first-class, testable "engine-owned default/scaffold vs
  human-authored material content" decision for prose artifacts.
- Use local `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core` as a behavioral
  reference for the overwrite-only-own-defaults invariant, especially
  `KNOWN_TEMPLATE_DEFAULTS` and `stateReplaceFieldIfTemplate`.
- Preserve target_files structural hashing behavior that issue #155 already
  says is fixed/stale.
- Add focused tests proving default/scaffold-only structural edits avoid broad
  downstream reopen while material prose edits still reopen.

## Out of Scope
- Do not copy GSD implementation details or TypeScript structure directly.
- Do not weaken evidence-freshness, verification, or fail-closed reopen paths.
- Do not rework unrelated lifecycle stages, codebase-map scaffold detection, or
  target_files hashing beyond what the issue explicitly requires.
- Do not finalize with `slipway done`; the requested endpoint is done-ready.

## Constraints
- Work only in the Slipway-provisioned worktree for this change.
- Keep the implementation repo-native and minimal; GSD is a reference, not a
  dependency or source transplant.
- Default to material/reopen when artifact edit materiality is uncertain.
- Follow Slipway lifecycle gates and record evidence through the CLI, not by
  hand-editing verification state.

## Acceptance Signals
- `go test` coverage exercises scaffold/default prose edits, human-authored
  material prose edits, and the fail-closed unknown case.
- `go run . validate --json` reports the governed change can advance through
  the required gates.
- `go run . status --json` reaches done-ready and remains bound to
  `.worktrees/resolve-github-issue-155-knuth-invariant-overwrite-only-own`.
- The implementation demonstrably references the GSD behavior without adding a
  runtime dependency on GSD.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
- [x] Which current Slipway digest/reopen functions own prose artifact
      materiality for intent.md, requirements.md, research.md, and decision.md?
- [x] What exact scaffold/default values are engine-owned today for each prose
      artifact, and where should that table live?
- [x] Which tests currently cover auto-reopen on artifact edits, and what
      focused tests should be added for issue #155?
- [x] Does GSD's state-document default overwrite model imply any recovery or
      fail-closed edge case that Slipway lacks?

## Deferred Ideas
- Generalizing the known-default table to every governed artifact type.
- Adding a user-facing command to explain why a prose edit was classified as
  material or non-material.

## Approved Summary
User directive confirms this scope: use the Slipway governed flow to solve all
of GitHub issue #155, reference local gsd-core during implementation, make the
best available choice on blockers, and stop at done-ready. This change is
limited to prose-artifact materiality and auto-reopen softening; it must not
weaken evidence freshness or copy GSD code directly.
