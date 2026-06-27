# Intent

## Summary
Add a repeatable built-binary performance baseline for state-read-heavy
lifecycle commands.

## Complexity Assessment
simple
The change should add benchmark/fixture tooling and a documented baseline around
existing public commands. It should not alter lifecycle routing semantics or add
new caching behavior.

## In Scope
- Add a repo-native, repeatable way to construct or reuse a synthetic state-read
  fixture with at least 25 worktrees, 300 `change.yaml` files, and 100
  verification records.
- Measure built `slipway` binary timings for root `status --json`, bound
  `status --json`, bound `next --json --diagnostics`, bound `validate --json`,
  and explicit `--change <slug>` scenarios.
- Persist a human-readable baseline artifact that records `real/user/sys`,
  worktree count, `change.yaml` count, verification record count, command
  version/commit, and the local repeat command.
- Add a lightweight regression threshold path, with a default 30% threshold for
  the measured core commands.
- Add targeted tests for the fixture/baseline parser or threshold logic.

## Out of Scope
- No lifecycle route/freshness/action-contract semantic changes.
- No persistent cross-command index or cache.
- No release workflow, GitHub settings, third-party action pinning, or API token
  hardening work.
- No compatibility layer for retired benchmark formats.

## Constraints
- Measurements must use a built binary, not `go run`.
- Tooling must be deterministic enough for local repeat use and CI-adjacent
  validation, but it should not make every PR run a slow 25-worktree benchmark
  unless explicitly invoked.
- Existing ignored runtime evidence/events must stay uncommitted.

## Acceptance Signals
- A documented command can create or refresh the required fixture and record a
  baseline artifact.
- The baseline artifact includes command timings and fixture dimensions for all
  required scenarios.
- Threshold checking reports which command regressed beyond the allowed 30%.
- Targeted tests cover parser/threshold behavior, and final verification runs
  `go test ./... -count=1`.

## Open Questions
None.

## Approved Summary
Approved under standing user auto-authorization on 2026-06-27: implement a
focused state-read performance baseline and regression threshold path for the
already-built lifecycle command surfaces. Keep semantic changes, persistent
indexes, release hardening, and compatibility layers out of scope.
