# Requirements

## Requirements

### Requirement: Done-ready status projection
REQ-001: The system MUST expose a top-level done-ready readiness signal in
`slipway status --json` when an active governed change is in `S4_VERIFY` and
the ship gate is approved, without changing the persisted lifecycle status to
done before finalization.

#### Scenario: Status shows done-ready handoff
GIVEN an active governed change in `S4_VERIFY` with approved ship readiness
WHEN an operator runs `slipway status --json --change <slug>`
THEN the JSON output includes `done_ready: true`
AND the output includes the `run_slipway_done_to_finalize` reason
AND the narrative tells the operator to run `slipway done` to finalize.

### Requirement: Archived change status lookup
REQ-002: The system MUST make `slipway status --json --change <slug>` report an
archived terminal record when the active bundle is absent or missing authority
but `artifacts/changes/archived/<slug>/change.yaml` exists.

#### Scenario: Status reports archived terminal record
GIVEN a governed change that has been successfully finalized and archived
WHEN an operator runs `slipway status --json --change <slug>`
THEN the command exits successfully
AND the JSON output includes `archived: true`
AND the lifecycle status reflects the archived record status
AND the output does not report `change_state_load_failed`.
