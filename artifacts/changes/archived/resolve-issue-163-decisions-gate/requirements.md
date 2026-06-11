# Requirements

## Requirements

### Requirement: Parse Decision Status
REQ-001: The system MUST parse `decision.md` into a structured decision contract
that exposes selected decision text and an explicit lifecycle status when a
status section is present.

#### Scenario: Status and selected approach are parsed together
GIVEN `decision.md` contains a `## Status` section and a substantive
`## Selected Approach`
WHEN the artifact parser reads the decision
THEN the parsed result includes the normalized status and selected decision
items from the same source.

#### Scenario: Existing status-free decisions remain compatible
GIVEN `decision.md` has all required substantive sections but no status section
WHEN the artifact parser reads the decision
THEN the parsed result treats the status as unspecified and preserves selected
decision items.

### Requirement: Reject Dead Decision Statuses
REQ-002: The system MUST fail closed when `decision.md` has an explicit
superseded, deprecated, or rejected status and a governed stage would otherwise
use that decision as planning authority.

#### Scenario: Superseded decision blocks planning readiness
GIVEN `decision.md` has all required substantive sections and `## Status`
contains `Superseded`
WHEN planning readiness validates the governed bundle at plan-audit or later
THEN readiness reports a blocking decision-status diagnostic instead of
approving the decision contract.

#### Scenario: Deprecated decision is not surfaced to host skills
GIVEN `decision.md` contains a substantive selected approach and `## Status`
contains `Deprecated`
WHEN next-skill constraints are built
THEN the selected approach is not exposed as a pending or locked decision.

### Requirement: Reject Unknown Explicit Statuses
REQ-003: The system MUST reject an explicit unrecognized decision status rather
than treating it as accepted, while preserving compatibility for missing status.

#### Scenario: Unknown explicit status blocks
GIVEN `decision.md` contains `## Status` with `Retired-ish`
WHEN the decision contract is evaluated at plan-audit or later
THEN readiness reports an unknown-status decision diagnostic.

#### Scenario: Missing status does not block
GIVEN `decision.md` has all required substantive sections and no status section
WHEN the decision contract is evaluated
THEN no status-related blocker is produced.

### Requirement: Prove Parser and Gate Coverage
REQ-004: The implementation MUST include unit and property-style coverage for
decision status parsing, status rejection, readiness blocking, and host
constraint behavior.

#### Scenario: Normalization variants stay rejected
GIVEN generated or table-driven status variants differ by case, whitespace,
punctuation, or heading alias
WHEN `ShouldRejectDecisionStatus` evaluates superseded and deprecated variants
THEN they remain rejected.

#### Scenario: Existing decision contract regressions keep passing
GIVEN existing tests cover template-only, missing, unreadable, and
pending-vs-locked decision behavior
WHEN the full test suite runs
THEN those existing behaviors still pass alongside the new status tests.
