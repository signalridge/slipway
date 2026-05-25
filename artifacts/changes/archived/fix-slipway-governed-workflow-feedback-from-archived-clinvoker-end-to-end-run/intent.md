# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go CLI governance engine
- Languages: Go, Markdown, YAML
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Conventions: 

## Summary
fix Slipway governed workflow feedback from archived clinvoker end-to-end run
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
external_api_contracts

## In Scope
- Resolve every actionable Slipway workflow issue recorded in `artifacts/changes/archived/reference-users-yixianlu-projects-clinvoker-to-add-ci-cd-release-and-maintenance-capabilities-while-exercising-the-full-slipway-governed-workflow/workflow-feedback.md`.
- Exercise the current Slipway governed workflow end to end while fixing the issues, including `next`, `run`, `validate`, evidence records, review, verification, and finalization gates.
- Add or update focused Go tests, generated skill/template tests, and documentation where needed to make each workflow fix verifiable.
- Record any newly discovered workflow friction in the active change feedback before final closeout.

## Out of Scope
- Further feature work inside `/Users/yixianlu/Projects/clinvoker`; that reference run is evidence for Slipway fixes, not the target product in this change.
- Broad redesign of Slipway governance architecture beyond fixes needed by the listed feedback.
- Rewriting archived evidence from the completed reference run except as read-only input for this change.

## Constraints
- Follow Slipway state transitions instead of bypassing gates.
- Keep fixes scoped to the CLI/runtime/template surfaces needed by the feedback.
- Preserve compatibility for existing governed changes and JSON consumers where possible; this change touches external command contracts.

## Acceptance Signals
- Each item in `workflow-feedback.md` has a current-state disposition: fixed by code/test/docs, intentionally documented as policy, or explicitly deferred with rationale.
- `go test -timeout=20m ./... -count=1` passes after implementation.
- Targeted tests cover the changed behavior for handoff wording, schema/parser alignment, worktree binding, skill lookup, codebase-map placeholder handling, routing, and archive/path behavior as applicable.
- `slipway validate --json`, `slipway next --json --diagnostics`, and `slipway run --json --diagnostics` are exercised on this governed change without unhandled workflow deadlocks.

## Open Questions
<!-- None remain for intake. S1 research will classify each archived feedback item against current code and define the implementation plan. -->

## Deferred Ideas
<!-- Identified but postponed ideas -->
- Shared read locks for `status`/`next` may be larger than this pass if command locking architecture needs redesign; documenting exclusive state locks is acceptable if read-lock implementation is not scoped safely.
- Catalog deduplication may be resolved by generated-manifest/toolgen policy instead of deleting checked-in generated artifacts by hand.

## Approved Summary
User objective supplied in the active thread directs this change to fix all issues in the archived clinvoker workflow feedback while testing the full Slipway governed workflow. Scope is limited to Slipway workflow/runtime/template/documentation changes needed to resolve those feedback items, with clinvoker treated as reference evidence only. Primary acceptance is a requirement-by-requirement completion audit plus passing targeted and full Go verification.

Confirmation source: active thread objective, 2026-05-25T09:47:30Z.
