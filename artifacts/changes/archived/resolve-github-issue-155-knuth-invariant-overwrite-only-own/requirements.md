# Requirements

## Requirements

### Requirement: Prose scaffold edits are non-material
REQ-001: The system MUST compute planning and research skill input digests from
the material view of governed prose artifacts, so engine-owned comments,
empty scaffold sections, and narrow known default prose do not stale evidence.

#### Scenario: scaffold-only prose edit keeps planning evidence fresh
GIVEN `plan-audit` evidence was stamped for a governed change with authored
planning artifacts
WHEN an artifact edit changes only HTML guidance comments, empty scaffold
sections, or a recognized engine-owned default in `intent.md`,
`requirements.md`, `research.md`, or `decision.md`
THEN the named prose artifact input digest remains unchanged.

### Requirement: Human prose remains fail-closed material
REQ-002: The system MUST include any unknown non-empty or human-authored prose
artifact content in skill input digests, preserving stale evidence recovery for
material edits.

#### Scenario: authored prose edit stales planning evidence
GIVEN `plan-audit` evidence was stamped for a governed change with authored
planning artifacts
WHEN a user changes human-authored prose in `requirements.md`, `research.md`, or
`decision.md`
THEN `EvaluateRequiredSkillsForChange` reports the corresponding
`required_skill_stale:<skill>:<artifact>` blocker.

#### Scenario: research artifact material edit stales research evidence
GIVEN `research-orchestration` evidence was stamped for a discovery-required
change
WHEN a user changes human-authored prose in `research.md`
THEN the research skill input digest changes for `research.md`.
