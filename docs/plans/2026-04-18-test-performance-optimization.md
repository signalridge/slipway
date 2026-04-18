# Test Performance Optimization Plan

> Status: Wave 0 landed on current branch; Waves 1-3 planned, Wave 4 optional
> Author: @LuYixian
> Date: 2026-04-18

## Scope Summary

Speed up repository test execution, with `cmd/` as the primary target, without
weakening behavioral coverage or introducing test flakiness through unsafe
parallelism.

This plan is intentionally scoped to test harness and test-structure changes
first. Broad production refactors are deferred until the lower-risk test-only
optimizations are exhausted and measured.

## Evidence Summary

Historical clean baseline in this repo put the full suite near `~262s`, with
`cmd/` near `~186s` and dominating total runtime.

After the Wave 0 changes already landed on this branch, an isolated current-turn
rerun of `go test ./cmd -count=1` measured `~151.9s`. That confirms the branch
already captured real wins, but it also means any remaining target must be
anchored to the post-Wave 0 baseline rather than the older `~186s` figure.

Current-turn investigation reconfirmed that `cmd/` is still the main hotspot,
but also refined the root-cause model:

1. The main bottleneck is not simply "missing `t.Parallel()`".
2. The bigger problem is that many `cmd/` tests are built around
   `withWorkspace(...)`, which performs `os.Chdir(root)` and therefore couples
   tests to process-global CWD.
3. A second high-value bottleneck is repeated heavy fixture setup:
   `createGovernedRequest(...)` was used `234` times and previously created a
   change by running the real `new` command, then mutating state, then
   scaffolding the bundle again.

### Current Structural Signals

| File | Tests | `withWorkspace(...)` uses | `t.Parallel()` count |
|---|---:|---:|---:|
| `cmd/progression_next_test.go` | 63 | 54 | 2 |
| `cmd/new_test.go` | 51 | 46 | 5 |
| `cmd/lifecycle_commands_test.go` | 34 | 34 | 0 |
| `cmd/status_view_build_test.go` | 31 | 21 | 23 |
| `cmd/health_test.go` | 30 | 30 | 0 |
| `cmd/review_test.go` | 28 | 12 | 16 |
| `cmd/validate_artifact_gate_test.go` | 16 | 16 | 0 |
| `cmd/repair_test.go` | 16 | 15 | 0 |
| `cmd/cli_e2e_test.go` | 12 | 11 | 0 |

### Current Remaining Real `new` Setup Sites

After `createGovernedRequest(...)` was rebuilt on top of the direct fixture
helper, raw helper call counts no longer describe the remaining optimization
surface. The more useful question now is: which tests still create setup state
by routing through the real `new` command?

| File | direct `new` setup sites |
|---|---:|
| `cmd/error_contract_test.go` | 5 |
| `cmd/cli_e2e_test.go` | 2 |
| `cmd/status_context_repair_test.go` | 2 |
| `cmd/next_tool_path_test.go` | 1 |

These are review candidates, not automatic conversions. Some of these sites are
still valid command-truth coverage and should remain real-CLI.

## Root Cause

### Root Cause A: Process-global CWD suppresses safe parallelism

`withWorkspace(...)` calls `os.Chdir(root)`. Production helpers such as
`projectRootFromWD` and related path resolution functions read from the current
working directory. That design forces large portions of `cmd/` to remain
serial, because these tests are not isolated from one another at the process
level.

### Root Cause B: Many tests pay full CLI-creation cost when they only need a fixture

Many `cmd/` tests do not actually need to verify the `new` command path. They
only need:

- a valid governed `change.yaml`
- a scaffolded governed bundle
- a specific lifecycle state such as `S1_PLAN` or `S2_EXECUTE`

Before the current branch optimization, those tests often exercised the full
`new` path anyway, then mutated the saved change, then re-scaffolded the
artifact bundle. That repeated setup cost is pure test overhead, not useful
behavioral coverage.

### Root Cause C: Some heavy tests are integration-shaped but used as broad regression nets

The heaviest current examples include:

- lock contention tests around `TestMutatingCommandsBlockOnStateLock`
- status / validate / next consistency tests
- `cmd/cli_e2e_test.go`

Some of these must remain integration-heavy, but several negative-path and
shared-readiness assertions can be narrowed or de-duplicated.

## What Landed In This Branch

The following low-risk changes are already applied on the current branch:

### L1. Direct governed-change fixture helper

`cmd/lifecycle_commands_test.go` now provides
`createGovernedChangeFixture(...)`, and both
`createGovernedRequest(...)` and `createActiveNonDiscoveryChange(...)` are
built on it.

All `cmd/*_test.go` files compile into the same `cmd` test package, so a helper
defined in one `_test.go` file is available to the others without extracting it
to a separate `testutil` package.

That change removes repeated "run `new` -> reload change -> mutate state ->
scaffold again" setup from a large slice of `cmd/` tests.

### L2. Safe parallelism for pure unit tests

Added `t.Parallel()` to pure, non-CWD-coupled tests in:

- `cmd/progression_test.go`
- `cmd/route_flags_test.go`
- `cmd/status_artifact_dag_test.go`

### L3. Initial targeted evidence

Targeted reruns showed real wins after the helper change. Example samples from
the same command family:

| Test | Earlier sample | Later sample |
|---|---:|---:|
| `TestCLIEndToEndGovernedLifecycleBlockersAndCancel` | `1.63s` | `1.34s` |
| `TestCLIEndToEndRunResumeResponseFlow` | `1.07s` | `0.84s` |
| `TestCLIEndToEndSuccessfulDoneArchive` | `0.96s` | `0.74s` |
| `TestCheckpointSetsActiveCheckpoint` | `0.49s` | `0.25s` |
| `TestAbortTextUsesRunResumeWhenResumableWaveStateExists` | `0.54s` | `0.34s` |

This is sufficient evidence that fixture de-CLIization is a valid first
optimization path.

## Expected Yield Envelope

These are rough planning estimates, not hard guarantees:

- Wave 1: likely `~5-10%` additional reduction from the current `~151.9s`
  baseline by converting the remaining setup-only real-`new` sites.
- Wave 2: likely `~5-15%` additional reduction if more pure or read-only tests
  can safely run in parallel.
- Wave 3: likely `~5-10%` additional reduction by shrinking heavyweight E2E,
  consistency, and lock-contention coverage.

That makes a primary post-Waves-1-to-3 target of `<=120s` plausible on the
current branch. Reaching `<=90s` should be treated as a stretch goal unless the
measured slope after Waves 1-2 stays strong enough to justify it, or Wave 4 is
explicitly taken.

## Strategy

Do not start with a repo-wide production refactor to remove CWD usage.

That work is real, but it is broad, command-wiring heavy, and riskier than the
test-only opportunities already proven to pay off. The order should be:

1. Remove obvious test-only waste first.
2. Add safe parallelism where no process-global state is involved.
3. Narrow or split integration-heavy tests that are over-covering.
4. Only then consider production seams for CWD independence if the remaining
   ceiling is still too low.

## Waves

### Wave 0: Landed Immediate Wins

**Outcome**: Remove repeated fixture waste and capture the first low-risk
parallelism wins.

**Files**:

- `cmd/lifecycle_commands_test.go`
- `cmd/progression_test.go`
- `cmd/route_flags_test.go`
- `cmd/status_artifact_dag_test.go`

**Evidence shape**:

- targeted `go test ./cmd -run ... -count=1`
- `git diff --check`

**Acceptance**:

- targeted fixture-heavy tests get faster
- `cmd` package remains green

### Wave 1: Expand Direct Fixture Construction Across Remaining Setup-Only `cmd/` Tests

**Priority**: P0

**Goal**: Stop paying real-command setup cost in tests that only need a
governed change fixture.

**Do not change production code in this wave.**

#### 1a. Review the remaining direct `new` setup sites first

The large helper-driven files were already migrated by Wave 0. The remaining
review candidates are the tests that still call the real `new` command outside
the obvious truth suites:

- `cmd/cli_e2e_test.go`
- `cmd/error_contract_test.go`
- `cmd/status_context_repair_test.go`
- `cmd/next_tool_path_test.go`

**Rule**:

If a test is not asserting behavior of `new`, `preset`, or initial intake
classification itself, it should not create state by running `makeNewCmd()` or
`runRootCommand([]string{"new", ...})` in setup.

Use direct fixture helpers where `new` is only a precondition. Keep the real
command path where the test is asserting intake behavior, CLI envelope shape,
or true `S0_INTAKE` semantics.

#### 1b. Preserve command-truth suites where they matter

Keep real-command creation flows in:

- `cmd/new_test.go`
- `cmd/preset_test.go`
- lifecycle tests that explicitly verify creation, preset confirmation, or
  intake behavior
- any test that is specifically asserting the `new` command's output contract or
  the immediate post-creation state

That prevents helper drift from silently redefining product behavior.

**Expected speedup**:

- material reduction in `cmd/` runtime
- especially in files with many repeated governed-change setups

**Evidence shape**:

- targeted before/after timing for each converted file
- green targeted suites

#### 1c. Audit helper correctness once after migration

Because more tests will depend on the shared direct fixture path, add one
focused helper audit after the remaining conversions:

- add or refresh invariant-style checks around fixture-created state and bundle
  scaffolding
- run one intentional local mutation check to confirm that breaking a required
  fixture invariant causes dependent tests to fail instead of vacuously passing

### Wave 2: Selective Parallelism, Not Blanket Parallelism

**Priority**: P0

**Goal**: Increase concurrency only where shared process state is absent.

#### 2a. Parallelize pure command-logic files

Continue the pattern used for:

- `cmd/progression_test.go`
- `cmd/route_flags_test.go`
- `cmd/status_artifact_dag_test.go`

Candidates:

- files with no `withWorkspace(...)`
- subtests that do not depend on env vars, CWD, or lock-holder helpers

#### 2b. Parallelize subtests inside already-isolated fixture parents

Where a parent test creates a single read-only fixture and child subtests only
assert projections or formatting, use `t.Run(...); t.Parallel()` at the child
level.

#### 2c. Explicit exclusions

Do not parallelize by default:

- tests using `withWorkspace(...)`
- tests that call `os.Chdir(...)`
- tests that use shared env/process state
- lock-holder subprocess tests
- tests using real git worktree coordination

**Evidence shape**:

- package stays green under `go test ./cmd -count=1`
- no flaky reruns under repeated targeted runs

### Wave 3: Narrow or Split Heavy Integration Tests

**Priority**: P1

**Goal**: Keep integration truth, but stop using the heaviest tests as generic
regression buckets.

#### 3a. Audit `cmd/cli_e2e_test.go`

Keep:

- a small number of positive-path end-to-end smoke tests
- one or two representative governed-lifecycle flows

Move or shrink:

- negative-path assertions already covered in `abort`, `checkpoint`, `review`,
  `done`, `validate`, or `next` focused test files

#### 3b. Audit consistency tests that recompute the same readiness envelope

The current hot set includes:

- status / validate / next consistency tests
- status view builder tests

Where one test verifies envelope consistency and another only needs mapping /
formatting, split the logic so the mapping tests stop recreating full governed
state unless necessary.

#### 3c. Shorten lock-contention test cost

`TestMutatingCommandsBlockOnStateLock` is still among the heaviest tests.

Do not add a second production seam here unless the existing configuration
surface proves insufficient. The test suite already has
`Execution.LockWaitTimeoutSeconds`; Wave 3 should first reuse that existing knob
and then reduce cost by splitting the lock matrix, trimming duplicated cases, or
narrowing assertions before considering broader product wiring changes.

**Evidence shape**:

- targeted timing drop in `cli_e2e`, consistency, and lock-heavy suites
- no loss of coverage intent in test names / assertion inventory

### Wave 4: Optional CWD Decoupling in Production Code

**Priority**: P2, only if Waves 1-3 leave too much runtime on the table

**Goal**: Remove the process-global CWD requirement from command path
resolution, so more `withWorkspace(...)` tests can become parallel in a safe
way.

**Candidate files**:

- `cmd/common.go`
- command entrypoints that currently depend on `projectRootFromWD`
- any helper that infers root/worktree from `os.Getwd()`

**Important**:

This is a real product refactor, not just a test cleanup. It should be taken
only after the lower-risk test-only wins are measured, because it expands the
blast radius into runtime command wiring.

**Evidence shape**:

- compile-clean removal of old CWD-only helpers
- green full `cmd` suite
- explicit regression coverage for root/worktree resolution

## Acceptance Criteria

This plan is done when:

1. Remaining setup-only uses of the real `new` command are either converted to
   direct fixtures or explicitly justified as command-truth coverage.
2. Pure unit-style `cmd` tests use `t.Parallel()` wherever safe.
3. `cmd/cli_e2e_test.go` is reduced to smoke coverage rather than mixed
   negative-path duplication.
4. `cmd/` package runtime drops materially from the current post-Wave 0
   `~151.9s` baseline, with `<=120s` as the primary Waves-1-to-3 target.
5. `<=90s` is treated as a stretch target, not a mandatory closeout condition,
   unless measured progress after Waves 1-2 still makes it credible or Wave 4
   is explicitly accepted.
6. A clean, isolated benchmark run is captured after each wave that materially
   changes runtime, and again after Waves 1-3.
7. The existing CI verify job remains green; CI wall-clock can be recorded as a
   secondary trend signal, but it is not the primary optimization benchmark.
8. Only if the clean benchmark still shows `cmd/` dominating for structural
   reasons should Wave 4 proceed.

## Measurement Protocol

Do not trust full-package timing collected while multiple profiling commands are
running in parallel.

The authoritative benchmark for this plan is a local isolated `cmd` package run
because `.github/workflows/ci.yml` currently executes full-suite `go test ./...`
and `go test ./... -race -count=1`, not a dedicated per-package timing job on a
controlled runner. CI should remain green throughout, but its wall-clock time is
only a regression signal unless and until a stable per-package timing step is
published.

For acceptance and comparison, use isolated runs such as:

```bash
go test ./cmd -count=1
go test ./cmd -run 'TestCheckpoint|TestAbort|TestCLIEndToEnd' -count=1
go test ./cmd -run 'TestHealth|TestStats|TestReview|TestValidateArtifactGate' -count=1
```

Capture the command in the PR or closeout note next to the timing numbers.

For this branch, record the current post-Wave 0 baseline explicitly:

```bash
go test ./cmd -count=1   # current-turn isolated sample: ~151.9s
```

## Risks

- **Helper drift risk**: direct fixture helpers may stop reflecting real `new`
  behavior. Mitigation: keep `new_test.go` and `preset_test.go` as the command
  truth suite.
- **Fixture correctness risk**: a bug in the shared fixture helper could let a
  broad slice of tests pass for the wrong reason. Mitigation: keep explicit
  helper invariants under test and perform a one-time mutation audit after Wave
  1 migration.
- **Unsafe parallelism risk**: blanket `t.Parallel()` on CWD-dependent tests
  will create flakiness. Mitigation: treat `withWorkspace(...)` and
  `os.Chdir(...)` as default no-parallel markers.
- **Over-pruning risk**: narrowing E2E tests can accidentally remove a unique
  behavioral check. Mitigation: before deleting an E2E assertion, identify the
  focused test that covers the same contract.
- **Measurement noise risk**: comparing numbers from concurrent profiling runs
  gives false confidence. Mitigation: require isolated reruns for benchmark
  claims.

## Non-Goals

- moving all tests into a shared `testutil` package
- mocking away filesystem or git behavior that is correctly exercised with real
  temp directories
- CI sharding or `go test -parallel` tuning before local test structure is
  improved
- broad production refactors as the first optimization step
