# Requirements

## Requirements

### Requirement: Record uncertain spec-trace rows
REQ-001: The spec-trace output contract MUST allow each coverage matrix row to
record `ambiguous` and `uncheckable` statuses in addition to existing
`covered`, `skipped`, and `drift` statuses.

#### Scenario: Reviewer cannot determine a mapping
GIVEN a reviewer is producing the spec-trace coverage matrix
WHEN a spec-to-code or code-to-spec mapping cannot be determined from available evidence
THEN the reviewer can record the row as `ambiguous` or `uncheckable` instead of omitting it or marking it `covered`

### Requirement: Account for uncertain rows as coverage gaps
REQ-002: The spec-trace contract MUST require every `ambiguous` or
`uncheckable` row to include a reason and MUST account for those rows as
coverage gaps rather than pass evidence.

#### Scenario: Coverage matrix includes an uncheckable row
GIVEN a coverage matrix row has status `uncheckable`
WHEN the reviewer writes the spec-trace report
THEN the report includes why the row could not be checked and lists it in coverage gap accounting

### Requirement: Fail closed in spec-compliance review
REQ-003: The spec-compliance-review guidance MUST prevent a `pass` verdict when
unresolved `ambiguous` or `uncheckable` trace rows remain.

#### Scenario: Stage 1 review sees unresolved ambiguity
GIVEN spec-compliance-review reads a spec-trace matrix with unresolved `ambiguous` or `uncheckable` rows
WHEN the reviewer determines the final verdict
THEN the review blocks or requests changes instead of treating the uncertainty as full bidirectional alignment

### Requirement: Pin the contract with tests
REQ-004: The implementation MUST include focused template tests that fail on
the old `covered | skipped | drift`-only contract and pass only when the new
uncertain-status and coverage-gap wording is present.

#### Scenario: Template contract regresses
GIVEN a future edit removes `ambiguous`, `uncheckable`, required reasons, or coverage-gap accounting from the templates
WHEN the focused template tests run
THEN the tests fail and identify the missing contract wording
