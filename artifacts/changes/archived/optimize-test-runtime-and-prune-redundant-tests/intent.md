# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: 

## Summary
optimize test runtime and prune redundant tests

## Complexity Assessment
complex
Rationale: test runtime optimization requires current baseline measurement,
coverage-preserving pruning decisions, safe test parallelization, and final
verification that the suite still protects the changed behavior.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Measure the current Go test runtime and identify package/test-level hotspots.
- Optimize the test suite itself where coverage can be preserved: remove or
  consolidate redundant tests, reduce repeated fixture/setup cost, and add safe
  `t.Parallel()` coverage where tests are independent.
- Focus first on the current runtime hotspot packages, especially command-layer
  tests if the fresh baseline still shows `cmd` dominates wall time.
- Keep workflow guidance aligned with efficient verification: targeted tests
  during iteration, full `go test ./...` only for final proof.

## Out of Scope
- No production behavior changes except narrow testability seams required to
  make existing tests faster without weakening behavior coverage.
- No deletion of tests that provide unique coverage for lifecycle, governance,
  artifact, or CLI contract behavior.
- No broad framework rewrite, new test runner, dependency manager changes, or
  unrelated formatting/refactoring.

## Constraints
- Use repo-native Go tooling and Slipway governance surfaces.
- Keep changes small and auditable; every deleted or consolidated test must have
  a coverage-preserving rationale.
- Prefer deterministic local fixtures over subprocess-heavy setup when behavior
  under test does not require a real external command.

## Acceptance Signals
- Fresh baseline and post-change timings show measurable test runtime reduction
  or a documented reason why current tests are already near the safe floor.
- Targeted hotspot tests pass after changes.
- `go test ./...` passes before closeout.
- `go build ./...` passes before closeout.
- Slipway validation passes and the governed change reaches done-ready/done.

## Open Questions
<!-- none -->

## Deferred Ideas
<!-- Identified but postponed ideas -->

## Approved Summary
User confirmed on 2026-05-26T06:24:47Z: optimize Slipway's Go test runtime by
measuring the current suite, then safely pruning/consolidating redundant tests,
reducing repeated fixture/setup cost, and parallelizing independent tests where
coverage is preserved. Keep lifecycle, governance, artifact, and CLI contract
coverage intact; avoid production behavior changes except narrow testability
seams needed for faster tests. Completion requires fresh before/after timing
evidence, targeted hotspot verification, full `go test ./...`, `go build
./...`, and successful Slipway closeout.
