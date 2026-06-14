# Requirements

## Requirements

### Requirement: Domain Review Attribution

REQ-001: The system MUST expose structured attribution when the `domain-review` governance action is satisfied by another skill evidence record.

#### Scenario: Spec compliance satisfies domain review

GIVEN a guarded change whose active controls include blocking `domain-review`
AND the latest execution summary is present
AND only current passing `spec-compliance-review` evidence exists for domain review
WHEN Slipway resolves runtime required actions
THEN the `domain-review` action is satisfied
AND the action identifies `spec-compliance-review` as the satisfying evidence source.

### Requirement: CLI Surface Consistency

REQ-002: The system MUST preserve required-action attribution in the shared command JSON surfaces used by `status`, `validate`, and `next --json --diagnostics`.

#### Scenario: Read-only command surfaces explain satisfied domain review

GIVEN a guarded change whose `domain-review` action is satisfied by current `spec-compliance-review` evidence
WHEN the status, validate, and next JSON views are built
THEN each view includes the same structured satisfied-by mapping on the `domain-review` required action.

### Requirement: Fail-Closed Review Readiness

REQ-003: The system MUST NOT report satisfied-by attribution for stale, failing, missing, or execution-summary-unbound `spec-compliance-review` evidence.

#### Scenario: Stale spec compliance remains unsatisfied

GIVEN a guarded change with a latest execution summary version
AND `spec-compliance-review` evidence recorded against an older run version
WHEN Slipway resolves runtime required actions
THEN the `domain-review` action remains unsatisfied
AND its blocker diagnostics continue to explain the readiness failure.
