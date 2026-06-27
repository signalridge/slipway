# Requirements

## Requirements

### Requirement: State Has No Engine Production Dependency
REQ-001: The system MUST prevent production code under `internal/state` from
importing `github.com/signalridge/slipway/internal/engine` or any of its
subpackages.

#### Scenario: production import is rejected
GIVEN the repository contains production Go files under `internal/state`
WHEN `go test ./internal/architecture -count=1` runs
THEN any production import from `internal/state` to `internal/engine` fails the
architecture test with a file-level violation.

#### Scenario: tests remain free to exercise integrations
GIVEN a `_test.go` file needs a black-box or fixture import
WHEN the architecture test scans package imports
THEN test files are not treated as production dependency violations.

### Requirement: State Remains A Persistence Layer
REQ-002: The system MUST keep `internal/state` responsible for governed artifact
paths, strict load/save validation, and runtime evidence I/O without depending
on engine context or engine wave packages.

#### Scenario: execution summary freshness still works
GIVEN an execution summary and current change metadata
WHEN state freshness diagnostics are computed
THEN the public status values remain `fresh`, `stale`, or `unknown` without an
`internal/engine/context` import from `internal/state`.

#### Scenario: wave plan persistence still works
GIVEN a materialized wave plan or runtime wave evidence exists
WHEN state load/save helpers read or write those persisted artifacts
THEN the serialized `wave-plan.yaml` and wave evidence model remain compatible.

### Requirement: Lifecycle Behavior Is Preserved
REQ-003: The system MUST preserve existing lifecycle, repair, wave execution,
status, next, evidence, health, and implementation behavior while correcting the
dependency direction.

#### Scenario: targeted behavior remains green
GIVEN the dependency boundary refactor is complete
WHEN targeted state, engine, architecture, and command tests run
THEN the tests pass without relaxing assertions or removing coverage.

#### Scenario: full repository verification remains green
GIVEN the change is ready for review
WHEN `go test ./... -count=1` runs
THEN the full test suite passes.
