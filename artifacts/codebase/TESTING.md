# Testing

Re-authored for change `resolve-issue-163-decisions-gate` (GitHub issue #163).

- Existing coverage:
  - `internal/engine/artifact/decision_contract_test.go` covers required
    decision sections, placeholder rejection, missing file behavior, unreadable
    file behavior, and authored decision acceptance.
  - `internal/engine/progression/validation_test.go:417` through
    `internal/engine/progression/validation_test.go:522` covers
    `DecisionContractBlockers` at pre-audit, plan-audit, and post-plan states.
  - `cmd/next_skill_constraints_test.go:52` through
    `cmd/next_skill_constraints_test.go:348` covers parsing selected approach
    text and routing it to pending or locked constraints depending on `G_plan`.
  - `internal/model/reason_code_contract_test.go` freezes canonical reason-code
    names and severities.
- Gaps for issue #163:
  - No parser test extracts lifecycle status from `decision.md`.
  - No test proves `superseded` or `deprecated` decisions fail closed in
    planning readiness.
  - No test proves dead decisions are not surfaced as pending or locked skill
    constraints.
  - No property or fuzz-style test proves status normalization is stable across
    casing, spacing, punctuation, and heading aliases.
  - No reason-code snapshot entry exists for a dead/superseded decision blocker.
- Planned verification:
  - Add artifact-level tests for parsed decision status, selected decisions, live
    statuses, dead statuses, unknown statuses, and malformed status sections.
  - Add a fuzz or property-style unit test around `ShouldRejectDecisionStatus`
    normalization, including `superseded` and `deprecated` variants.
  - Extend progression validation tests with a superseded/deprecated authored
    decision that otherwise satisfies all required sections.
  - Extend next-skill constraint tests to prove dead decisions produce no
    pending or locked decision text.
  - Run targeted tests for `internal/engine/artifact`, `internal/engine/progression`,
    `cmd`, and `internal/model`, then `go test -count=1 ./...`.
