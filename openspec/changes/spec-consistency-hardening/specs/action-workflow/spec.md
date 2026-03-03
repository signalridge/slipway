## MODIFIED Requirements

### Requirement: `S4_SPEC_BUNDLE` Execution Semantics (L2/L3)
`S4` SHALL validate canonical governed `tasks.md` task-node structure before entering `S5_PLAN_AUDIT`.

#### Scenario: Invalid tasks structure keeps workflow in S4
- **WHEN** `S4_SPEC_BUNDLE` detects malformed governed `tasks.md` structure
- **THEN** workflow SHALL remain in `S4` with deterministic remediation blockers
