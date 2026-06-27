# Requirements

## Requirements

### Requirement: Repeatable Built-Binary State-Read Baseline
REQ-001: The system MUST provide a repo-native way to measure state-read-heavy
lifecycle command performance with a built `slipway` binary, not `go run`, and
MUST cover the root `status --json`, bound `status --json`, bound
`next --json --diagnostics`, bound `validate --json`, and explicit
`--change <slug>` scenarios.

#### Scenario: Baseline fixture and command coverage
GIVEN a repository checkout with the state-read baseline tool available
WHEN an operator runs the documented baseline refresh command
THEN the tool builds or uses a `slipway` binary and records timings for every
required lifecycle command scenario.

### Requirement: Fixture Dimensions and Timing Evidence
REQ-002: The system MUST record enough fixture and timing metadata to make a
baseline result auditable, including `real/user/sys`, worktree count,
`change.yaml` count, verification record count, command version or commit, and
the command lines used.

#### Scenario: Baseline artifact is inspectable
GIVEN a refreshed state-read performance baseline
WHEN an operator opens the committed baseline artifact
THEN it shows the required fixture dimensions and timing values for each
measured command without requiring access to ignored runtime files.

### Requirement: Regression Threshold Checking
REQ-003: The system MUST provide a threshold check that compares a new
measurement to the committed baseline and reports each command whose `real`
duration regressed beyond the configured allowance, defaulting to 30%.

#### Scenario: A command exceeds the threshold
GIVEN a committed baseline and a new measurement where one command is more than
30% slower
WHEN the threshold check runs
THEN it fails and names the regressed command, baseline duration, current
duration, and allowed threshold.

#### Scenario: Measurements stay within threshold
GIVEN a committed baseline and a new measurement where all commands are within
the configured threshold
WHEN the threshold check runs
THEN it passes and reports that all measured state-read commands are within the
allowed regression budget.

### Requirement: Local Validation Path
REQ-004: The system MUST add targeted automated tests for the state-read
baseline parser, writer, or threshold logic, and final verification MUST include
`go test ./... -count=1`.

#### Scenario: Threshold logic is tested
GIVEN the state-read baseline package test suite
WHEN `go test` runs for that package
THEN it covers both passing and failing threshold comparisons.
