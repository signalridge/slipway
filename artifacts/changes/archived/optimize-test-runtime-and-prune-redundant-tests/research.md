# Research

## Research Findings

### Architecture
- Affected modules:
  - `cmd/*_test.go`: CLI contract, lifecycle, governance-readiness, worktree,
    lock, and route-surface tests.
  - `cmd/locks.go` / `cmd/lock_helpers.go`: command-level state-lock acquire
    path used by mutating commands.
  - `cmd/process_unix.go`: cancel/abort preemption wait path used by process
    lifecycle tests.
  - `internal/tmpl/*_test.go`, `internal/fsutil/*_test.go`,
    `internal/state/*_test.go`, and `internal/engine/progression/*_test.go`:
    secondary slow packages, but current wall-clock is still dominated by
    `cmd`.
- Dependency chains:
  - CLI commands in `cmd/` resolve project/change context, acquire locks, and
    delegate durable state to `internal/state` and workflow decisions to
    `internal/engine/progression`.
  - Command tests seed governed bundles through test helpers, then assert JSON
    surfaces or state transitions.
- Blast radius: test suite and narrow testability seams only; production
  behavior should remain unchanged.
- Constraints:
  - `change.yaml` remains current-state authority and `lifecycle.jsonl` remains
    append-only trace.
  - Tests protecting lifecycle, governance, artifact, worktree, and CLI JSON
    contracts must not be deleted unless equivalent coverage remains.

### Patterns
- Existing conventions:
  - `commandForRoot(t, root, makeXCmd())` runs commands against per-test temp
    roots without mutating process cwd.
  - `ensureTestGitRepo(t, root)` provides cheap git identity where a real git
    subprocess is unnecessary.
  - Independent command and engine tests use `t.Parallel()`.
  - Full verification uses `go test -timeout=20m ./... -count=1`, with focused
    regression tests during iteration.
- Reusable abstractions:
  - `withCommandWorkspace` plus `commandForRoot` can replace some remaining
    `withWorkspace`/`os.Chdir` tests.
  - Existing lock helpers centralize command lock acquisition, making a narrow
    test-only duration seam possible without changing command behavior.
- Convention deviations:
  - Some older tests still rely on `withWorkspace` and cannot run in parallel
    even when they pass explicit roots.
  - Several lock/preemption tests intentionally wait at least one second because
    config values are second-granularity and normalized away from zero.

### Risks
- Technical risks:
  - High: deleting unique governance/lifecycle contract tests would weaken the
    safety net around state transitions and CLI JSON surfaces.
  - Medium: global test-only timing seams can introduce races if tests mutate
    them per test instead of setting them once for the package.
  - Medium: replacing real git worktree tests with fake fixtures can miss
    worktree integration bugs.
  - Low: adding `t.Parallel()` to tests that already use isolated temp roots
    and no cwd/global mutation.
- Guardrail domains: none. This change does not modify auth, credentials, PII,
  finance, schema migrations, irreversible operations, or external API
  contracts.
- Reversibility: high. Test-only changes and scoped helper seams can be
  reverted without data migration or user-facing behavior changes.

### Test Strategy
- Existing coverage:
  - Fresh baseline command:
    `/usr/bin/time -p go test -json -timeout=20m -count=1 ./... >
    /tmp/slipway-test-runtime-baseline-20260526.json`
  - Baseline wall-clock: `47.01s`.
  - Package hotspot: `github.com/signalridge/slipway/cmd` at `45.77s`.
  - Next packages by package elapsed: `internal/state` `11.886s`,
    `internal/fsutil` `8.424s`, `internal/toolgen` `7.091s`,
    `internal/engine/progression` `6.429s`, `internal/tmpl` `6.331s`.
  - Slowest observed tests include command lock/wait tests, real git worktree
    tests, and governance surface consistency tests.
- Infrastructure needs:
  - Keep current fixture helpers.
  - Add at most a narrow package-level testability seam for lock/preemption
    wait durations, set once in `cmd` tests.
  - Convert safe remaining cwd-based tests to root-injected command execution.
- Verification approach:
  - Run targeted `go test ./cmd -count=1` after command test changes.
  - Run targeted package tests for any touched internals.
  - Run final `/usr/bin/time -p go test -json -timeout=20m -count=1 ./...`
    and compare wall-clock/package timing to baseline.
  - Run `go build ./...`, `go run . validate --json`, and Slipway closeout.

## Alternatives Considered

### Option A: Aggressive test deletion/consolidation
Delete or merge the slowest command tests first, especially tests that exercise
multiple surfaces (`status`/`validate`/`next`) or real worktree paths.

Tradeoffs:
- Fastest possible reduction if many tests are removed.
- Highest regression risk because these tests protect governance surfaces where
  subtle drift has historically mattered.
- Does not solve repeated lock/preemption waits that remain after deletion.

### Option B: Targeted runtime optimization while preserving contracts
Keep unique lifecycle/governance coverage, then reduce runtime by adding a
narrow test-only timing seam for command lock/preemption waits, converting safe
cwd-based tests to `commandForRoot`/`t.Parallel()`, and only pruning tests that
are demonstrably duplicate after inspection.

Tradeoffs:
- Best balance of runtime reduction and safety.
- Requires small production-file testability seam, but no user-visible behavior
  change.
- May reduce less than aggressive deletion, but keeps the contract surface
  trustworthy.

### Option C: No test-code changes; only workflow guidance
Leave tests untouched and rely on targeted test commands during development,
with full-suite verification only at final closeout.

Tradeoffs:
- Lowest implementation risk.
- Does not address the user's request to make tests themselves faster.
- Current full-suite wall-clock remains around `47s`.

Selected: Option A, with a safety floor. User confirmed on
2026-05-26T06:32:02Z: "我想激进优化". Implementation should aggressively
delete or consolidate redundant coverage and compress artificial wait paths.
The safety floor is that unique lifecycle, governance, artifact-authority,
worktree, and externally consumed CLI JSON contract coverage must remain
represented by at least one focused test.


## Unknowns
- Resolved: current full-suite baseline -> `47.01s` wall-clock.
- Resolved: current primary package hotspot -> `cmd` at `45.77s`.
- Resolved: selected approach -> aggressive pruning/consolidation with a
  coverage-preserving safety floor.
- Remaining: none.


## Assumptions
- The earlier test-runtime optimization is already present in this checkout.
  Evidence: `cmd/common.go` exposes `setCommandProjectRoot`,
  `cmd/common_test.go` exposes `commandForRoot`, and tests already use
  `ensureTestGitRepo` and widespread `t.Parallel()`.
- `cmd` remains the right first target. Evidence:
  `/tmp/slipway-test-runtime-baseline-20260526.json` package pass events show
  `github.com/signalridge/slipway/cmd` at `45.77s`.
- Real worktree tests should be retained unless a narrower unit-level invariant
  already covers the same behavior. Evidence: codebase concerns identify
  worktree binding and artifact roots as brittle authority areas.


## Canonical References
- `artifacts/changes/optimize-test-runtime-and-prune-redundant-tests/intent.md` for the original request and intake context.
- `artifacts/codebase/ARCHITECTURE.md` for module boundaries.
- `artifacts/codebase/CONCERNS.md` for worktree/artifact authority risks.
- `artifacts/codebase/CONVENTIONS.md` for command/test organization.
- `artifacts/codebase/TESTING.md` for verification command conventions.
- `cmd/common_test.go` for `commandForRoot`, `withCommandWorkspace`, and
  `ensureTestGitRepo`.
- `cmd/locks.go` and `cmd/lock_helpers.go` for command state-lock paths.
- `cmd/process_unix.go` for cancel/abort preemption waits.
- `/tmp/slipway-test-runtime-baseline-20260526.json` for fresh baseline data.
