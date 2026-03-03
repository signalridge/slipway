## MODIFIED Requirements

### Requirement: Filesystem Persistence (No DB)
Governance evidence persistence path SHALL be request-scoped.

#### Scenario: Governance evidence uses request partition
- **WHEN** persistence writes governance evidence
- **THEN** target path SHALL be `.spln/evidence/skills/<request_id>/<evidence-file>.json`

### Requirement: Evidence Schema Persistence
Governance evidence core fields SHALL include `request_id`.

#### Scenario: Missing request_id evidence rejected
- **WHEN** governance evidence lacks `request_id`
- **THEN** evidence validation SHALL fail for readiness checks
