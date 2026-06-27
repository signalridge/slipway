# Requirements

## Requirements

### Requirement: Mutating lifecycle commands expose route contract
REQ-001: The system MUST expose the same invocation route contract on successful JSON output for single-change `done` and evidence recording commands that `status`, `next`, and `validate` already expose.

#### Scenario: done reports target route
GIVEN an active governed change in a bound worktree
WHEN `slipway done --json --change <slug>` finalizes that done-ready change from another workspace
THEN the JSON output includes `invocation_route` with the target slug, invocation workspace, bound workspace, authority path, route kind, lifecycle execution allowance, and remediation/next command semantics consistent with `status`, `next`, and `validate`.

#### Scenario: evidence reports target route
GIVEN an active governed change for which a skill or task evidence command is valid
WHEN `slipway evidence skill --json --change <slug>` or `slipway evidence task --json --change <slug>` records evidence
THEN the JSON output includes `invocation_route` for the target change using the same route fields as other lifecycle commands.

### Requirement: Existing fail-closed semantics remain intact
REQ-002: The system MUST preserve explicit missing slug, bound-elsewhere, archived, no-active, and wrong-state fail-closed semantics while adding route projection.

#### Scenario: explicit missing evidence target fails closed
GIVEN no active change exists for slug `definitely-not-a-change`
WHEN `slipway evidence skill --json --change definitely-not-a-change` is run
THEN the command exits with code 3 and reports `error_code: change_not_found` rather than falling back to generic diagnostics.

#### Scenario: done root bound-elsewhere remains actionable
GIVEN a single active change is bound to another worktree
WHEN unscoped `slipway done --json` is run from the root workspace
THEN the command fails closed with `change_bound_to_other_worktree` and remediation naming the bound worktree or an explicit `--change <slug>` command.

### Requirement: No compatibility layer for retired contracts
REQ-003: The implementation MUST replace misleading or incomplete lifecycle surface behavior directly and MUST NOT add an adapter or legacy compatibility layer that preserves the old missing-route output contract.

#### Scenario: consumers receive route data directly
GIVEN a successful JSON response from a touched mutating lifecycle command
WHEN a caller inspects the top-level response
THEN route data is present as the normal `invocation_route` field, not hidden behind a legacy or compatibility envelope.

### Requirement: Verification proves the coherent public surface
REQ-004: The change MUST include targeted tests proving route consistency for touched commands and MUST rerun existing freshness, action, and host capability tests that protect the rest of `opt.md` section 1.

#### Scenario: targeted tests cover touched command surfaces
GIVEN the implementation is complete
WHEN the targeted command tests run
THEN they verify `done`, `evidence skill`, and `evidence task` route output or fail-closed behavior without relying on source-only assertions.

#### Scenario: existing P0 contract tests continue to pass
GIVEN the route completion patch is applied
WHEN the existing route, freshness, action contract, and host capability tests run
THEN they continue to pass, proving the implementation did not regress the already repaired `status`, `next`, `validate`, and `run` surfaces.
