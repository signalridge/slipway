# Requirements

## Requirements

### Requirement: Selected Review Pass Evidence Fails Before Persistence
REQ-001: The system MUST reject `slipway evidence skill --verdict pass` for a
selected S3 review skill before writing verification YAML unless the submitted
references contain exactly one valid `context_origin:stage=review=<handle>`
token.

#### Scenario: Missing context-origin does not write YAML
GIVEN an active governed change in `S3_REVIEW` with `independent-review`
selected
WHEN `slipway evidence skill --skill independent-review --verdict pass` omits
`context_origin:stage=review=<handle>`
THEN the command fails with an actionable error and no
`verification/independent-review.yaml` is written.

#### Scenario: Malformed or duplicate review context-origin does not write YAML
GIVEN an active selected S3 reviewer
WHEN pass evidence includes a malformed review context-origin token or more
than one review-stage context-origin token
THEN the command rejects the record before persistence.

### Requirement: Non-Completion Evidence Paths Stay Usable
REQ-002: The system MUST keep non-pass reviewer evidence, non-selected review
evidence handling, task evidence, wave evidence, and ordinary task/wave
Markdown proof behavior outside the selected-review pass hard blocker.

#### Scenario: Fail verdict remains recordable
GIVEN an active selected S3 reviewer
WHEN `slipway evidence skill` records `--verdict fail` with blockers and no
review context-origin pass handle
THEN the fail record remains valid command-boundary evidence.

### Requirement: Agent-Facing Examples Teach Structured Review Evidence
REQ-003: Generated command surfaces, review skill templates, capability
remediation, and command reference docs MUST show passing selected-review
evidence with `context_origin:stage=review=<handle>` and
`verification/<skill>-notes.md`, and MUST describe degraded fallback mode as an
additional structured reference rather than a substitute for the review handle.

#### Scenario: Generated examples are safe to follow
GIVEN an agent reads generated evidence or review skill instructions
WHEN the instructions show a passing selected-review evidence command
THEN the command includes a review context-origin reference and the `*-notes.md`
notes-file convention.

### Requirement: Regression Coverage Pins The Boundary
REQ-004: The change MUST add focused tests that prove missing, malformed, and
duplicate selected-review context-origin references fail before persistence,
while a valid selected-review pass still records successfully.

#### Scenario: Focused tests cover issue #394
GIVEN the command-boundary and parser tests
WHEN the focused test suite runs
THEN it proves the selected-review fail-fast behavior and preserves existing
defense-in-depth readiness behavior.
