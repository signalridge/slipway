# Requirements

## Requirements

### Requirement: Explicit current review evidence refresh
REQ-001: The system MUST provide a documented `slipway evidence skill` option that lets an operator intentionally refresh already-current passing evidence for a selected S3 review skill.

#### Scenario: Refresh a selected current reviewer
GIVEN an active governed change in S3 review with a selected review skill that already has passing evidence for the current review set
WHEN an operator records a new passing result for that same skill using the explicit refresh option
THEN Slipway SHALL replace the skill verification record and stamp the current evidence digest instead of returning the existing duplicate-evidence dead end.

### Requirement: Duplicate review evidence remains fail-closed by default
REQ-002: The system MUST reject ordinary duplicate recording of already-current passing selected review evidence unless the operator supplied the explicit refresh option or an existing narrow repair condition applies.

#### Scenario: Duplicate reviewer without refresh opt-in
GIVEN an active governed change in S3 review with current passing selected review evidence
WHEN an operator records a new passing result for the same skill without the explicit refresh option
THEN Slipway SHALL reject the command with a clear invalid-usage error and preserve the existing verification record.

### Requirement: Refresh path is discoverable
REQ-003: The system MUST document the explicit refresh option in the CLI help-facing command surface and generated command metadata.

#### Scenario: Agent discovers refresh workflow
GIVEN an agent or operator reads the evidence command surface
WHEN they need to record an intentional rerun for already-current passing review evidence
THEN the command documentation SHALL name the explicit refresh option and its selected-review scope.
