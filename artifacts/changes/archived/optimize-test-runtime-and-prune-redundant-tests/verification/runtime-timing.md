# Runtime Timing

## Baseline

Command:

```bash
/usr/bin/time -p go test -json -timeout=20m -count=1 ./... > /tmp/slipway-test-runtime-baseline-20260526.json
```

Result:

- full suite: `real 47.01`
- `cmd` package elapsed: `45.77s`
- hotspot: `cmd` dominated suite wall-clock, so optimization stayed focused on
  command tests and command-test fixtures.

## Final Measurements

Commands:

```bash
/usr/bin/time -p go test ./cmd -count=1
/usr/bin/time -p go test -json -timeout=20m -count=1 ./... > /tmp/slipway-test-runtime-post-20260526.json
go test ./cmd -run 'TestMutatingCommandsBlockOnStateLock|TestCancelPreemptsInFlightTasks|TestCurrentWorktreeRootPropagatesGitErrors' -count=1
GOOS=windows go test -c ./cmd -o /tmp/slipway-cmd-windows.test.exe
go build ./...
git diff --check
```

Results:

| Run | Result |
| --- | --- |
| `go test ./cmd -count=1` | pass, package `28.866s`, `real 29.17` |
| `go test -json -timeout=20m -count=1 ./...` | pass, `real 37.78` |
| focused changed-path `cmd` tests | pass, package `0.517s` |
| `GOOS=windows go test -c ./cmd` | pass |
| `go build ./...` | pass |
| `git diff --check` | pass |

Package-level final hotspots from `/tmp/slipway-test-runtime-post-20260526.json`:

| Package | Elapsed |
| --- | ---: |
| `github.com/signalridge/slipway/cmd` | `36.564s` |
| `github.com/signalridge/slipway/internal/state` | `12.480s` |
| `github.com/signalridge/slipway/internal/toolgen` | `7.915s` |
| `github.com/signalridge/slipway/internal/fsutil` | `7.774s` |
| `github.com/signalridge/slipway/internal/tmpl` | `6.742s` |
| `github.com/signalridge/slipway/internal/engine/progression` | `4.823s` |

Runtime delta:

- full-suite wall-clock: `47.01s -> 37.78s`, down `9.23s` (`19.6%`);
- `cmd` package elapsed in full-suite JSON: `45.77s -> 36.564s`, down
  `9.206s` (`20.1%`);
- standalone `cmd` package cold run now completes in `29.17s` real.

## Changes Applied

- Added package-private test timing seams for command lock waits, command cancel
  grace waits, and process preemption polling. Production defaults remain
  second/100ms based; `cmd` tests set faster package-wide units once in
  `TestMain`. The preemption polling default lives in a build-neutral file so
  package tests continue to compile cross-platform.
- Made command test workspace initialization skip generated adapter scaffolding
  unless a test explicitly needs it.
- Removed redundant command-surface tests after mapping each deleted assertion
  to stronger surviving coverage.
- Replaced expensive governed bundle scaffolding in lifecycle command fixtures
  with a minimal valid bundle writer for tests that only need lifecycle state.
- Changed worktree-preflight git fixture setup to use an empty initial commit.
- Replaced an expensive active-change/worktree error propagation test with a
  narrower `currentWorktreeRoot` git-error test.

## Deleted Or Consolidated Test Coverage

| Removed or narrowed test | Surviving coverage |
| --- | --- |
| `TestBuildGovernedStatusViewIncludesAssuranceContractBlockersAtReview` | `TestNextPreviewIncludesAssuranceContractBlockersAtReview` plus progression readiness tests still cover review blockers. |
| `TestBuildGovernedStatusViewIncludesSelectedArchivedDependencyContext` | `TestBuildNextContextIncludesSelectedArchivedDependencyContext` preserves selected archived dependency context coverage on the host handoff path. |
| `TestDoneAllReadySkipsShipGateBlockedChanges` | `TestDoneAllReadyPreservesShipGateReasonCodes`, `TestDoneAllReadyPreservesSpecificReadinessArtifactBlockers`, and archive-ready lifecycle tests preserve bulk done blockers and ready-change behavior. |
| `TestResolveActiveChangeRefPropagatesWorktreeResolutionErrors` | `TestCurrentWorktreeRootPropagatesGitErrors` keeps the git-error propagation invariant without constructing a full governed fixture. |

## Residual Hotspots

The largest remaining costs are integration-style command flows and internal
packages that intentionally exercise real filesystem, template, or progression
behavior. Further gains should target fixture reuse and narrower assertions in
`internal/state`, `internal/toolgen`, `internal/fsutil`, and `internal/tmpl`
after reviewing their individual contract overlap.
