# Requirements

## Requirements

### Requirement: CI emits kernel-scoped coverage
REQ-001: CI MUST produce a coverage profile for the governance-kernel packages (`internal/engine/gate`, `internal/engine/governance`, `internal/engine/progression`) on every push and pull request, measured deterministically on a single OS (ubuntu).

#### Scenario: Coverage profile produced on CI run
GIVEN the CI pipeline runs on a push or pull request
WHEN the coverage job executes the test suite with `-coverpkg` scoped to the three kernel packages
THEN a coverage profile covering those packages is produced and consumed by the coverage gate.

### Requirement: Fail closed on kernel coverage regression
REQ-002: The coverage gate MUST fail the CI job (non-zero exit) when any governance-kernel package's measured coverage is below its committed baseline, and MUST pass when every kernel package is at or above its baseline.

#### Scenario: A coverage drop fails CI
GIVEN a committed per-kernel-package coverage baseline
WHEN a change reduces a kernel package's covered statements below its baseline value
THEN the coverage checker exits non-zero and the CI coverage job fails.

#### Scenario: No regression passes
GIVEN a committed per-kernel-package coverage baseline
WHEN every kernel package's measured coverage is greater than or equal to its baseline
THEN the coverage checker exits zero and the CI coverage job passes.

### Requirement: Correct union coverage attribution
REQ-003: The checker MUST compute per-package coverage using union semantics — when `-coverpkg` causes the same code block to appear once per test binary, each block is counted once and treated as covered if any occurrence was executed.

#### Scenario: Duplicate coverpkg blocks counted once
GIVEN a coverage profile in which the same kernel code block appears multiple times because `-coverpkg` instrumented it in several test binaries
WHEN the checker computes a kernel package's coverage
THEN each unique block contributes its statement count exactly once and is covered if any occurrence was hit, so the per-package percentage matches `go tool cover`'s aggregate.

### Requirement: Baseline integrity with no bypass
REQ-004: The baseline MUST only change through an explicit committed edit produced by the checker's write mode, and the gate MUST NOT provide any environment variable, flag, or other path that skips, forces, or soft-passes a regression failure.

#### Scenario: Baseline changes only via reviewed commit
GIVEN the coverage gate and its committed baseline file
WHEN a contributor needs to change the baseline
THEN the change is produced by running the checker in write mode and appears as a reviewable diff in the pull request, and no bypass mechanism lets a below-baseline run pass.

### Requirement: Explicit documented exclusion list
REQ-005: The checker MUST support an explicit exclusion list so that non-kernel, generated, or test-only packages present in the profile are not gated, and that list MUST be documented.

#### Scenario: Excluded package is not gated
GIVEN a documented exclusion list applied by the checker
WHEN a package on the exclusion list appears in the coverage profile
THEN the checker does not enforce a baseline for that package.

### Requirement: Documented gate and ratchet workflow
REQ-006: The repository documentation MUST describe the coverage gate, the governance-kernel package set, and the ratchet-update workflow for raising the baseline.

#### Scenario: Contributor can follow the gate documentation
GIVEN the operator/contributing documentation
WHEN a contributor reads it
THEN they find the coverage gate's purpose, the kernel package set, how a regression is reported, and how to ratchet the baseline up via the write command.
