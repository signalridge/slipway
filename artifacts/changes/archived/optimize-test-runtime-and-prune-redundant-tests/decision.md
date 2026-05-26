# Decision

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Option A: Aggressive test deletion and consolidation
Delete or consolidate redundant command-layer tests first, compress artificial
waits, and parallelize remaining isolated tests. Keep one focused test for each
unique governance contract.

Tradeoffs:
- Largest expected runtime reduction.
- Higher review burden because deleted tests must have a coverage-preserving
  rationale.
- Risk is acceptable because scope is limited to tests and narrow testability
  seams.

### Option B: Conservative runtime tuning only
Keep all tests, add timing seams and more `t.Parallel()` where obviously safe.

Tradeoffs:
- Lower regression risk.
- Smaller runtime gain and does not fully satisfy the user's request for
  aggressive pruning.

### Option C: Workflow-only optimization
Keep the suite unchanged and document targeted test commands for iteration.

Tradeoffs:
- Lowest code risk.
- Does not make tests themselves faster and leaves full-suite wall-clock near
  the measured baseline.

## Selected Approach
Option A is selected. The user explicitly requested aggressive optimization on
2026-05-26 after seeing the measured baseline and alternatives.

Implementation will:
- compress command lock/preemption wait paths in tests via narrow timing seams;
- remove or consolidate redundant command-surface consistency tests where a
  representative test preserves the invariant;
- convert safe remaining cwd-based command tests to root-injected,
  parallelizable execution;
- keep at least one focused test for lifecycle, governance readiness, artifact
  authority, worktree binding, and CLI JSON contracts.

## Interfaces and Data Flow
No user-facing interface changes are planned.

Potential internal testability seams:
- command lock acquisition may gain a package-private duration override used
  only by tests;
- process preemption polling may gain a package-private interval override used
  only by tests.

Production command flow remains unchanged: CLI command -> root/change
resolution -> state lock -> `internal/state` / `internal/engine` operations.

## Rollout and Rollback
Rollout:
- apply scoped test/runtime-helper edits;
- run `go test ./cmd -count=1` and any touched internal packages;
- run `/usr/bin/time -p go test -json -timeout=20m -count=1 ./...` and compare
  with the `47.01s` baseline;
- run `go build ./...` and `go run . validate --json`.

Rollback:
- revert the test/helper edits in this branch;
- rerun `go test ./cmd -count=1` and `go test ./...`.

## Risk
- High: deleting unique governance contract coverage could hide workflow drift.
  Mitigation: only delete/consolidate after identifying surviving equivalent
  coverage.
- Medium: test-only timing seams can be racy if mutated per test. Mitigation:
  package-level test setup should set stable test defaults once.
- Medium: replacing real git worktree tests with fakes can miss integration
  bugs. Mitigation: retain at least one real worktree-path test.
- Low: adding `t.Parallel()` to isolated temp-root tests.
