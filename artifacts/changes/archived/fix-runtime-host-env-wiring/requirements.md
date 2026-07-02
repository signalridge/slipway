# Requirements

## Requirements

### Requirement: Structured Environment Wiring Catalog

REQ-001: The system MUST expose environment-variable wiring contracts in
`EnvCatalog()` for variables whose values have accepted tokens, format
constraints, examples, fallback behavior, or other non-obvious runtime effects.

#### Scenario: host capability tokens are discoverable

GIVEN a host integrator needs to run review or verification skills that require
subagent capability
WHEN they inspect `SLIPWAY_HOST_CAPABILITIES` through the env catalog
THEN the catalog states that `subagent` and `delegation` satisfy the subagent
capability, `none` and `unavailable` declare it unavailable, and unset or empty
values remain unknown.

#### Scenario: future env vars cannot hide contracts

GIVEN a production code path reads a public env var with accepted values or
fallback semantics
WHEN the env catalog entry omits structured contract detail
THEN tests MUST fail before the change can pass verification.

### Requirement: Host-Facing Config Output

REQ-002: `slipway config list --env` and `slipway config list --env --json`
SHALL project the structured env wiring contract without removing or renaming
existing public output fields.

#### Scenario: JSON consumers receive machine-readable wiring

GIVEN a caller runs `slipway config list --env --json`
WHEN the response includes `SLIPWAY_HOST_CAPABILITIES`
THEN the entry includes machine-readable accepted values, examples, and unset
behavior while retaining existing fields such as `name`, `scope`,
`description`, and `secret`.

#### Scenario: text output closes the host-integration loop

GIVEN a human runs `slipway config list --env`
WHEN they inspect runtime-host entries
THEN the text output includes enough wiring detail to set the environment
without reading Go source.

### Requirement: Public Host Integration Documentation

REQ-003: The documentation MUST reconcile workflow skill preconditions such as
"host declared subagent capability" with concrete environment knobs and keep
host manuals out of per-stage workflow skill bodies.

#### Scenario: public docs explain subagent capability declaration

GIVEN a host integrator reads the public docs
WHEN they need to declare subagent capability
THEN the docs show `SLIPWAY_HOST_CAPABILITIES=subagent`, explain the
`delegation` alias, explain explicit unavailable declarations, and point to
`slipway config list --env` as the current catalog authority.

#### Scenario: skill boundary remains focused

GIVEN generated review, audit, or verification skills mention host capability
requirements
WHEN this change is complete
THEN those skills remain workflow evidence instructions rather than duplicated
host environment manuals.

### Requirement: Verification Coverage

REQ-004: The change MUST include tests that prove catalog completeness,
structured output, and host capability wiring documentation for the guarded
external API contract surface.

#### Scenario: focused tests cover the repair

GIVEN the implementation is complete
WHEN focused tests for `internal/model`, `cmd`, and capability resolution run
THEN they pass and prove the public env contract matches existing runtime
semantics.

#### Scenario: governed verification can close

GIVEN the governed change reaches review
WHEN validation, selected reviews, and final verification run
THEN they use fresh evidence and do not rely on stale or hand-edited readiness
state.
