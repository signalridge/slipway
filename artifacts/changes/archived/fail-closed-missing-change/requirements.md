# Requirements

## Requirements

### Requirement: Explicit missing validate target fails closed
REQ-001: The system MUST return a typed fail-closed precondition error when
`validate --change <slug>` names a change slug that does not exist, and it MUST
preserve existing explicit archived and unscoped no-active validation semantics.

#### Scenario: Missing explicit validate target
GIVEN a Slipway workspace with no active or archived change named
`definitely-not-a-change`
WHEN an operator runs `slipway validate --change definitely-not-a-change --json`
THEN the command exits with code 3
AND the JSON error has `error_code: change_not_found`
AND the JSON error has `exit_code: 3`
AND the remediation tells the operator how to check or choose a valid change

#### Scenario: Archived explicit validate target remains fail closed
GIVEN a governed change has been archived as done
WHEN an operator runs `slipway validate --change <archived-slug> --json`
THEN the command exits with code 3
AND the JSON error remains `archived_change_not_validatable`
AND the remediation points at archived evidence or choosing an active change

#### Scenario: Unscoped no-active validate remains diagnostic
GIVEN a Slipway workspace has no active governed change
WHEN an operator runs `slipway validate --json` without `--change`
THEN the command emits the existing diagnostics view
AND the command does not write governed state
