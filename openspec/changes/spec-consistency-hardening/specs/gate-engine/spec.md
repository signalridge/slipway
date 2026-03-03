## MODIFIED Requirements

### Requirement: G_plan Semantics (L2/L3)
`G_plan` approval SHALL require parseable canonical governed `tasks.md` structure.

#### Scenario: Malformed tasks structure blocks G_plan
- **WHEN** governed `tasks.md` structure is not parseable for wave planning
- **THEN** `G_plan` SHALL be `blocked` until structure is repaired
