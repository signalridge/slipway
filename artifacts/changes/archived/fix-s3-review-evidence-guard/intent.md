# Intent

## Summary
fix S3 review evidence guard
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
<!-- none detected -->

## In Scope
- Add record-time validation to `slipway evidence skill` so a passing selected
  S3 review skill cannot persist without exactly one valid
  `context_origin:stage=review=<handle>` reference.
- Cover all selected S3 review skills, including `spec-compliance-review`,
  `code-quality-review`, `independent-review`, and `security-review` when
  selected for the active change.
- Add focused command-boundary tests for missing, malformed, and duplicate
  review context-origin references.
- Keep existing ship/readiness validation as defense in depth.
- Update generated review skill or recovery examples so passing selected-review
  evidence uses `*-notes.md` and includes the context-origin reference.

## Out of Scope
- Do not implement or rely on the #384 refresh/restamp path.
- Do not ban ordinary task, wave, or targeted verification Markdown under
  `verification/`.
- Do not redesign S3 review selection or subagent dispatch.
- Do not clean unrelated worktrees, active changes, or root checkout dirt.

## Constraints
- Reuse the existing context-origin parsing/readiness contract instead of
  ad hoc string checks.
- Fail before writing YAML evidence when the selected-review pass record is
  structurally invalid.
- Preserve archived and non-selected evidence behavior unless it conflicts with
  the fail-closed selected-review contract.

## Acceptance Signals
- A `slipway evidence skill --skill independent-review --verdict pass` attempt
  for an active selected S3 reviewer fails before writing YAML when the
  context-origin reference is missing.
- The same boundary rejects malformed and duplicate review-stage context-origin
  references for selected S3 review skills.
- Passing selected-review evidence with exactly one valid review-stage
  context-origin reference still records successfully.
- Focused tests and the relevant Go package tests pass in the governed
  worktree.
- `slipway status --json`, `slipway validate`, and lifecycle readiness agree
  that this change is ready for review/ship when implementation is complete.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->

- [x] Locate the evidence command write path, the selected-review skill
  selection source, and the reusable context-origin parser.
- [x] Identify which generated skill/recovery text surfaces produce passing
  selected-review evidence examples.

## Deferred Ideas
<!-- Identified but postponed ideas -->
- Warning-level health/validate detection for visually misleading
  `verification/<review-skill>.md` sibling files may be added only if it is
  small and does not distract from the fail-closed write boundary.

## Approved Summary
- Approved 2026-06-30T16:25:00Z by user instruction to proceed without further
  questions if the fix is straightforward. This change will fail closed at
  `slipway evidence skill` record time for passing selected S3 reviewer
  evidence that lacks exactly one valid `context_origin:stage=review=<handle>`
  reference, add focused tests, and tighten generated/recovery examples to use
  `*-notes.md` plus the structured context-origin token. It explicitly excludes
  #384 refresh/restamp behavior, broad S3 review redesign, and any ban on
  ordinary task or wave proof Markdown.
