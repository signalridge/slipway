# Decision

## Alternatives Considered

- Expand `coverage-baseline.json` to include kernel and public-surface packages.
  This is mechanically small, but it mixes two risk tiers in one review artifact
  and makes kernel baseline preservation harder to see.
- Add changed-line coverage for every touched file. This is stronger long-term,
  but it is a larger implementation with more false-positive risk and is not
  necessary because opt.md 3.2 permits a tiered gate.
- Add a named `covergate` target and a separate
  `coverage-public-surface-baseline.json`. This keeps the kernel baseline
  intact, lets CI enforce both targets explicitly, and supports target-specific
  package/file/surface diagnostics.

## Selected Approach

Use a named target model in `covergate`:

- `kernel` continues to enforce `coverage-baseline.json`.
- `public-surface` enforces `coverage-public-surface-baseline.json`.
- Both targets reuse `internal/coverage` profile parsing, package selection,
  baseline marshaling, and regression checking.
- The public-surface target carries static package-to-surface metadata so failure
  output can name the affected package, surface, and source files.

This approach satisfies opt.md 3.2 while preserving the existing governance
kernel gate and avoiding any compatibility layer or soft-pass behavior.

## Interfaces and Data Flow

- CLI:
  - Add `-target <kernel|public-surface>` to
    `internal/coverage/cmd/covergate`.
  - Keep `-check`, `-write`, `-profile`, `-baseline`, and `-exclude` semantics
    fail-closed.
  - Default target is `kernel`, which is the product behavior of the existing
    command, not a compatibility shim.
- Data:
  - Existing `coverage-baseline.json` remains the kernel baseline.
  - New `coverage-public-surface-baseline.json` records public-surface package
    floors and surface metadata.
  - `coverage.Baseline` gains optional surface metadata used by diagnostics.
- CI/local:
  - CI measures the union of kernel and public-surface cover packages once, then
    checks both targets against their committed baselines.
  - `just coverage-gate` and `just coverage-baseline` mirror CI behavior.

## Rollout and Rollback

- Rollout:
  - Commit the new target implementation, tests, baseline, CI recipe, local
    recipe, and docs in one PR.
  - Let GitHub CI prove both coverage checks pass before merge.
- Rollback:
  - Revert the PR. This removes the public-surface target and baseline while
    restoring the previous kernel-only CI recipe.
  - Verification command after rollback: `go test ./internal/coverage... -count=1`
    and `go test ./... -count=1`.

## Risk

- Coverage measurement cost can increase because the gated cover package set now
  includes `cmd` and `internal/state`; mitigate by measuring the union once
  instead of running two full coverage test passes.
- Package-level coverage cannot prove each individual command path is exercised;
  mitigate with surface/file diagnostics and targeted behavior tests for the
  gate itself.
- Baseline floors can ratchet down if regenerated without review; mitigate by
  keeping baseline changes as explicit PR diffs and documenting that tests are
  the usual fix for regressions.
