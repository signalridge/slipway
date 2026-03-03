## MODIFIED Requirements

### Requirement: Mitigation Mapping Consistency
`mitigation_target` SHALL remain optional metadata, and writers SHOULD omit it by default in MVP to reduce denormalized drift.

#### Scenario: Mitigation target omitted by default
- **WHEN** governance evidence is produced in default MVP mode
- **THEN** writer SHALL be allowed to omit `mitigation_target` and consumers SHALL derive mapping from `skill_name`

### Requirement: Evidence Output Contract
Governance evidence SHALL be request-scoped and include `request_id` as a required field.

#### Scenario: Request-scoped evidence path and key
- **WHEN** governance evidence is persisted
- **THEN** path SHALL resolve under `.spln/evidence/skills/<request_id>/` and evidence missing `request_id` SHALL be invalid for readiness
