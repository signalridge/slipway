# Requirements

## Requirements

### Requirement: Freeze Canonical Reason Codes
REQ-001: The system MUST treat the canonical reason-code taxonomy as an explicit, reviewed contract rather than an implicit mutable map.

#### Scenario: Taxonomy changes are explicit
GIVEN the canonical reason-code definitions used by `model.NewReasonCode`
WHEN a developer adds, removes, or renames a canonical code
THEN a snapshot-style regression MUST fail until the expected taxonomy is intentionally updated.

### Requirement: Unknown Codes Are Not Silently Humanized
REQ-002: The system MUST NOT silently present an unrecognized reason code as if it were a canonical machine contract.

#### Scenario: Unknown reason code construction fails closed
GIVEN code constructs or loads a `ReasonCode` with an unrecognized code
WHEN the reason code is normalized for output or recovery routing
THEN the resulting code MUST make the unknown-code condition explicit and MUST preserve the original token as detail for diagnosis.

### Requirement: Tests Assert Stable Contracts Instead Of Message Prose
REQ-003: The test suite MUST reject direct text-matching assertions against unstable reason/error `Message` prose when stable fields are available.

#### Scenario: Message prose matching is linted
GIVEN a Go test asserts `Contains`, regex, or error-prose matching against a reason/error payload `Message`
WHEN `go test ./...` runs
THEN a repo-local lint regression MUST fail and tell the developer to assert stable fields such as `Code`, `Detail`, `ErrorCode`, `Category`, `ExitCode`, or structured `Details`.

#### Scenario: Existing tests use stable fields
GIVEN current tests that validate reason-code or CLI error behavior
WHEN they need to prove the contract
THEN they MUST assert machine-stable fields instead of coupling to presentation message fragments.
