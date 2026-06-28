# Requirements

## Requirements

### Requirement: Compact run JSON preserves governance blockers
REQ-001: The system MUST expose selected host capability requirements,
capability blocker state, freshness fields, recovery guidance, and confirmation
requirements in default compact `run --json` output whenever the diagnostic
`run --json --diagnostics` view would stop on unavailable required host
capability.

#### Scenario: Review host capability unavailable in compact run handoff
GIVEN a governed change in `S3_REVIEW` with selected review skills that require
subagent capability and the host reports no available subagent capability
WHEN an operator runs `slipway run --json --change <slug>` without
`--diagnostics`
THEN the JSON response includes the selected host capability requirement,
`host_capability_unavailable` blocker, blocked freshness state, recovery
guidance, and a `blocked_by_governance` confirmation requirement without
advancing past the blocked review handoff.

### Requirement: Review-alignment action contracts stay cross-surface consistent
REQ-002: The system MUST keep blocker-driven review-alignment handoff decisions
consistent across `status --json`, `validate --json`, `next --json`,
`next --json --diagnostics`, and `run --json` so operators see the same current
action class and the same actionable review skill on every public lifecycle
surface.

#### Scenario: Stale upstream evidence maps to a selected review skill
GIVEN a governed change in `S3_REVIEW` where selected review evidence appears
passing but stale upstream evidence requires rerunning an aligned selected
review skill
WHEN an operator queries status, validate, next, diagnostic next, and compact
run JSON surfaces
THEN each surface reports the aligned review skill as the actionable handoff and
uses a governance-blocked action contract rather than splitting into unrelated
current action kinds or commands.
