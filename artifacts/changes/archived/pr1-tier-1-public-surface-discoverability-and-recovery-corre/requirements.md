# Requirements

## Requirements

### Requirement: Config key listing
REQ-001: The `slipway config list` command MUST enumerate every `.slipway.yaml`
key, and for each key MUST print its name, type, default value, allowed values
(when constrained), and scope. The command MUST support `--json` output. Running
`slipway config` with no subcommand MUST behave as `slipway config list`.

#### Scenario: Human-readable listing
GIVEN a repository with a Slipway config struct in `internal/model/config.go`
WHEN the user runs `slipway config list`
THEN every strict-decoded config key is printed with name, type, default,
allowed-values, and scope.

#### Scenario: JSON listing
GIVEN the same repository
WHEN the user runs `slipway config list --json`
THEN the output is valid JSON containing one entry per config key with the same
fields, suitable for machine consumption.

### Requirement: Catalog derives from the config struct and is contract-tested
REQ-002: The config key catalog MUST be derived from the
`internal/model/config.go` configuration struct rather than a hand-maintained
list, and a contract test MUST assert that every struct field has a catalog
entry so that adding a field without a catalog entry fails CI.

#### Scenario: New struct field without a catalog entry fails CI
GIVEN a new strict-decoded field is added to the config struct
AND no corresponding catalog entry is provided
WHEN the contract test runs
THEN the test fails, identifying the field that lacks a catalog entry.

#### Scenario: Catalog and struct in parity passes
GIVEN every config struct field has a catalog entry
WHEN the contract test runs
THEN the test passes.

### Requirement: Resolved effective value lookup
REQ-003: The `slipway config get <key>` command MUST print the resolved
effective value for a key — the value from `.slipway.yaml` merged over built-in
defaults — and MUST support `--json` output. An unknown key MUST be rejected
with a clear error and a non-zero exit code.

#### Scenario: Get a defaulted key
GIVEN a `.slipway.yaml` that does not set `execution.auto`
WHEN the user runs `slipway config get execution.auto`
THEN the command prints the built-in default effective value.

#### Scenario: Get an unknown key
GIVEN any repository
WHEN the user runs `slipway config get not.a.real.key`
THEN the command prints a clear error naming the unknown key and exits non-zero.

### Requirement: Validated config mutation
REQ-004: The `slipway config set <key> <value>` command MUST validate the
key/value using the same strict decode contract as config loading
(`KnownFields(true)`), MUST persist a valid value to `.slipway.yaml`, and MUST
reject an invalid key or value with a clear error, a non-zero exit code, and no
corruption or loss of existing `.slipway.yaml` content.

#### Scenario: Set a valid value round-trips
GIVEN any repository
WHEN the user runs `slipway config set execution.auto true`
THEN `.slipway.yaml` is written with that value
AND a subsequent `slipway config get execution.auto` returns `true`.

#### Scenario: Reject an invalid value without corrupting the file
GIVEN a `.slipway.yaml` with existing valid content
WHEN the user runs `slipway config set execution.auto nope`
THEN the command exits non-zero with a clear validation error
AND `.slipway.yaml` is left unchanged (no partial write, no dropped keys).

### Requirement: Environment variables are discoverable from help
REQ-005: The behavior-affecting environment variables `SLIPWAY_GITHUB_API_URL`,
`SLIPWAY_CONTEXT_WINDOW_TOKENS`, and `SLIPWAY_SESSION_OWNER` MUST each be
documented in the `--help` output of a relevant command so they are discoverable
without reading source.

#### Scenario: Env vars surfaced in help
GIVEN the built CLI
WHEN a user inspects the `--help` of the commands those variables affect
THEN each of the three environment variables is named with a short description
of its effect.

### Requirement: Run help points to the config surface
REQ-006: The `slipway run --help` output MUST reference the `slipway config`
surface so a user discovering `run` can find the configuration command.

#### Scenario: Back-pointer present
GIVEN the built CLI
WHEN the user runs `slipway run --help`
THEN the help text references `slipway config`.

### Requirement: Unknown top-level config keys are surfaced, not swallowed
REQ-007: Loading a `.slipway.yaml` that contains an unknown top-level key MUST
emit a warning that names the unknown key, rather than silently discarding it.
The warning MUST NOT abort loading of the otherwise-valid config.

#### Scenario: Unknown top-level key warns
GIVEN a `.slipway.yaml` containing an unrecognized top-level key
WHEN any command loads the config
THEN a warning naming the unknown key is emitted
AND the valid remainder of the config still loads.

### Requirement: S2 stale-wave recovery recommends a state-valid command
REQ-008: When wave-orchestration evidence is stale in `S2_IMPLEMENT`, the
recovery surfaced by `slipway run`/`status`/`next` MUST recommend a command that
is runnable in `S2_IMPLEMENT` and MUST NOT recommend the S3-only `slipway fix`
command. The fix MUST remain fail-closed: it MUST correct only the recommended
command and MUST NOT weaken the stale-evidence gate or bypass S3 review.

#### Scenario: Stale S2 wave evidence routes to a runnable command
GIVEN a change in `S2_IMPLEMENT` with stale wave-orchestration evidence
WHEN `slipway run --json` (or `status`/`next`) reports recovery
THEN `recovery.primary_command` is a command runnable in `S2_IMPLEMENT`
AND it is not `slipway fix`.

#### Scenario: Regression test guards the routing
GIVEN the recovery routing for stale S2 wave evidence
WHEN the regression test runs
THEN it asserts the recommended command is state-valid for `S2_IMPLEMENT` and is
not `slipway fix`.

### Requirement: Change ships clean under the repo quality bar
REQ-009: The change MUST keep the full Go test suite green and satisfy the
repository's lint/format gate (`gofmt -s` clean and golangci-lint clean),
including the new `config` command, the catalog contract test, and the #324
regression test.

#### Scenario: Suite and lint are green
GIVEN the completed change
WHEN `go test ./...` and the repository lint/format checks run
THEN the test suite passes and the format/lint checks report no findings.
