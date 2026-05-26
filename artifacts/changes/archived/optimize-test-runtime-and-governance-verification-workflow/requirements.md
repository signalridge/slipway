# Requirements

## Project Context
- Tech Stack: Go CLI with Cobra commands and filesystem-backed governance artifacts.
- Conventions: Use repo-native Go tooling, colocated `*_test.go` tests, Slipway governed evidence files, and focused tests before a final full-suite proof.
- Test Command: `go test ./...`
- Build Command: `go build ./...`
- Languages: Go

## Requirements

### Requirement: Measured Runtime Baseline
REQ-001: The change MUST record enough baseline and post-change timing evidence to explain which test areas were optimized.

#### Scenario: `cmd` package is identified as the primary bottleneck
GIVEN a fresh `go test -json -count=1 ./...` baseline has been captured
WHEN package elapsed times are summarized
THEN the evidence identifies the slowest packages and keeps the raw JSON log path available for audit.

#### Scenario: optimization result is measurable
GIVEN implementation has stabilized
WHEN the final verification run completes
THEN the evidence compares the final `cmd` and full-suite runtime against the baseline and describes the causes of any improvement or remaining cost.

### Requirement: Redundant Test Coverage Removed Or Narrowed
REQ-002: The change MUST delete, merge, or narrow only tests whose behavior is already covered by stronger retained tests.

#### Scenario: strict-subset command tests are removed
GIVEN a candidate test is redundant
WHEN it is removed
THEN a retained test still covers the same command behavior and at least one stronger assertion, such as blockers, state, JSON contract shape, or archived-owner accounting.

#### Scenario: core governance contracts remain covered
GIVEN tests are removed or narrowed
WHEN targeted command tests run
THEN `next`, `status`, worktree-preflight, stats, and template contract coverage still passes.

### Requirement: Existing Test Harness Efficiency Improved
REQ-003: The change MUST make existing tests cheaper where the same behavior can be preserved with less setup, fewer subprocesses, or reduced global state.

#### Scenario: repository fixture creation avoids unnecessary subprocesses
GIVEN a test only needs a valid repository identity
WHEN `ensureTestGitRepo` prepares the fixture
THEN it creates a minimal valid `.git` layout without running repeated `git init` and `git config` subprocesses.

#### Scenario: command tests can avoid process-wide cwd mutation
GIVEN a command is executed against a temp Slipway workspace
WHEN the command receives a test project-root override
THEN root resolution uses that override and does not require `os.Chdir` for commands that do not explicitly test cwd behavior.

### Requirement: Safe Parallelization
REQ-004: The change MUST add `t.Parallel()` only to tests that are isolated by temp roots and no longer depend on process-wide cwd or shared mutable globals.

#### Scenario: health and CLI workflow tests run in parallel
GIVEN each test uses its own temporary workspace and command-root override
WHEN the Go test runner schedules those tests concurrently
THEN they remain deterministic and pass repeatedly without relying on global cwd serialization.

#### Scenario: cwd-sensitive tests stay serialized
GIVEN a test intentionally verifies `projectRootFromWD`, nested worktree discovery, or other cwd-dependent behavior
WHEN test parallelism is evaluated
THEN the test remains serialized or continues to use the existing `withWorkspace` helper.

### Requirement: Governance Verification Workflow Avoids Redundant Full Runs
REQ-005: The change MUST refine Slipway workflow guidance so routine stages avoid repeated full-suite verification while still requiring fresh widened proof before closeout.

#### Scenario: worktree preflight uses bounded baseline verification
GIVEN a governed change enters worktree preflight
WHEN the preflight host asks for baseline verification
THEN it prefers the cheapest deterministic baseline command that proves the worktree is viable and reserves full-suite verification for cases where no narrower proof is adequate.

#### Scenario: closeout guidance widens verification once
GIVEN targeted tests have passed during implementation
WHEN the diff is stable for final closeout
THEN workflow guidance asks for one fresh full-suite proof plus build verification, while treating `go vet`, `staticcheck`, race, and extra checks as risk-triggered rather than every-iteration defaults.
