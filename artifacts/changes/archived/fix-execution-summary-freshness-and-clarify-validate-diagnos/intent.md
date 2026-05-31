# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
Fix the confirmed execution-summary freshness bug and close out related Lattice
diagnostic issues according to the user's final triage.
## Complexity Assessment
complex
Rationale: the change touches execution freshness semantics, regression tests,
documentation/assurance wording, and issue-tracker closeout for multiple linked
reports. The code change is bounded but the workflow contract needs precision.

## Guardrail Domains
<!-- none detected -->

## In Scope
- #28: remove the per-task freshness loop where `execution-summary.yaml`
  `captured_at` makes earlier task evidence stale, and add regression coverage.
- #29: clarify final-assurance/documentation wording so active `validate --json`
  is described as a pre-`done` freshness gate, not a post-archive replay
  promise.
- #32: add no-active/archived/missing-authority validate zero-write regression
  coverage where useful, then close or request reproduction because the reported
  write path is not present on current HEAD.
- GitHub closeout for #30, #32, and #34 when current evidence shows they are not
  must-fix implementation bugs.

## Out of Scope
- Do not change the active-only semantics of `validate --change <slug>` for
  archived changes.
- Do not implement #30's tracked archived runtime evidence defense in this
  change.
- Do not implement #34's orphan bundle diagnostic enhancement in this change.
- Do not redesign archived audit surfaces; that remains a future product
  surface if needed.

## Constraints
- Keep the diff surgical and aligned with the existing execution-summary
  freshness model.
- Treat `execution-summary.yaml` as downstream aggregate evidence; its own
  `captured_at` must not be an upstream per-task freshness baseline.
- Preserve non-empty orphan bundle behavior: diagnostic/reporting improvements
  may be future work, but automatic deletion is out of scope.
- Use fresh Go test/build verification before closeout.

## Acceptance Signals
- A regression test fails before the #28 fix and passes after it, proving
  summary `captured_at` alone does not stale older task evidence.
- Validate zero-write regression coverage exists for no-active/archived or
  missing-authority failure paths relevant to #32.
- Final-assurance wording no longer implies archived bundles can be revalidated
  through active `validate --json` after `done`.
- `go test -count=1 ./...` and `go build ./...` pass in the governed worktree.
- GitHub issues #30, #32, and #34 are closed with evidence-backed comments; #29
  is closed after the wording clarification; #28 is closed by the fix.

## Open Questions
None.

## Deferred Ideas
- Add a read-only archived audit surface such as `slipway audit --archived
  --change <slug>`.
- Add `done`/`repair` checks for tracked archived runtime evidence.
- Add a structured orphan-bundle diagnostic code for non-empty active bundle
  directories missing `change.yaml`.
- Ask reporters for #30/#32/#34 versions and reproduction details if those
  closed reports are later reopened as enhancements.

## Approved Summary
User-confirmed on 2026-05-31T05:49:11Z from the request's final triage:
implement #28, clarify #29 wording, add #32 zero-write regression coverage, and
close #30/#32/#34 when current evidence shows they are not must-fix bugs. The
change must not expand `validate --change` into archived validation, must not
implement #30's defensive archive check, and must not implement #34's diagnostic
enhancement in this delivery.
