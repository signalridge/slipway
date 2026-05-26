# Requirements

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Measured Runtime Baseline
REQ-001: The change MUST use fresh timing data to target the current slowest
test paths instead of speculative edits.

#### Scenario: Baseline identifies the hotspot
GIVEN the current test suite is measured with `go test -json -count=1 ./...`
WHEN optimization targets are chosen
THEN the selected targets are justified by package-level or test-level timing
evidence.

### Requirement: Aggressive Coverage-Preserving Test Reduction
REQ-002: The change MUST aggressively delete, consolidate, parallelize, or
de-wait tests where equivalent coverage remains.

#### Scenario: Redundant test cost is removed
GIVEN two or more tests cover the same command-layer invariant
WHEN the coverage can be preserved by a narrower or existing test
THEN redundant execution is removed or consolidated and the rationale is
recorded.

### Requirement: Command Hotspot Reduction
REQ-003: The change MUST focus first on the current `cmd` package runtime
hotspot.

#### Scenario: Command package runtime drops
GIVEN `cmd` dominates full-suite wall-clock
WHEN the optimization is complete
THEN `go test ./cmd -count=1` and the full-suite timing show reduced command
test cost or document the remaining irreducible integration cost.

### Requirement: Unique Governance Coverage Retained
REQ-004: The change MUST retain at least one focused test for each unique
lifecycle, governance-readiness, artifact-authority, worktree, and CLI JSON
contract being protected.

#### Scenario: Aggressive pruning preserves unique contracts
GIVEN a test is deleted or consolidated
WHEN the final suite is reviewed
THEN equivalent unique contract coverage is still present in a surviving test
or a narrower replacement.

### Requirement: Verification Proof
REQ-005: The change MUST produce targeted and full verification evidence before
closeout.

#### Scenario: Optimized suite remains green
GIVEN the test suite has been optimized
WHEN verification runs complete
THEN targeted hotspot tests, `go test ./...`, `go build ./...`, and Slipway
validation pass with fresh evidence.
