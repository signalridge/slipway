## MODIFIED Requirements

### Requirement: Governed `tasks.md` Minimum Structure (MVP)
Governed `tasks.md` SHALL include deterministic task-node structure (`## Task Nodes` + fenced YAML root `tasks`) with required node fields and valid dependencies.

#### Scenario: Missing canonical task-node block fails readiness
- **WHEN** governed `tasks.md` lacks required heading or YAML task-node block
- **THEN** governed planning readiness SHALL fail with remediation
