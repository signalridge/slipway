## MODIFIED Requirements

### Requirement: CLI Failure Contract
This CLI requirement SHALL be the canonical MVP source for exit-code taxonomy and JSON error envelope.

#### Scenario: CLI taxonomy source is local and stable
- **WHEN** CLI failure taxonomy is consumed by runtime/automation
- **THEN** values SHALL come from this command spec contract and SHALL NOT require design-doc lookup
