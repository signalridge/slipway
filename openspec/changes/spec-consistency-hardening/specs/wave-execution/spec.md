## MODIFIED Requirements

### Requirement: Wave Planning Input Contract
Governed wave planning input SHALL be parsed from canonical `tasks.md` task-node payload, not free-form artifact presence.

#### Scenario: Malformed governed tasks block wave planning
- **WHEN** governed `tasks.md` canonical node payload is missing or malformed
- **THEN** planner SHALL fail before DAG construction with deterministic remediation
