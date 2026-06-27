# Intent

## Summary
Enforce the `opt.md` 3.1 architecture boundary: `internal/state` remains the
low-level load/save/path/YAML/JSONL/local-runtime layer and no longer imports
`internal/engine` production packages.

## Complexity Assessment
complex
This is complex because it moves ownership of wave/context-specific behavior
across package boundaries while preserving lifecycle evidence and wave-plan
behavior.

## Guardrail Domains
None.

## In Scope
- Remove production imports from `internal/state` to `internal/engine/context`
  and `internal/engine/wave`.
- Move or rehome engine-semantic helpers for execution-summary freshness
  projection, task-plan parsing, and wave planning into an engine-owned or
  lower-level package that does not make `internal/state` understand engine
  lifecycle semantics.
- Update `internal/architecture/dependency_direction_test.go` so
  `internal/state` importing `internal/engine` fails the architecture test.
- Preserve existing public behavior for status, next, evidence, health, repair,
  wave execution, and execution-summary freshness.

## Out of Scope
- Do not implement `opt.md` 3.2 coverage-gate expansion in this change.
- Do not implement `opt.md` section 4 state-read performance caching or
  benchmarks in this change.
- Do not revisit already-landed release/supply-chain hardening or public
  lifecycle route/freshness behavior unless a direct compile/test break forces a
  narrow adjustment.

## Constraints
- Keep the change package-boundary focused; no broad lifecycle rewrite.
- Preserve existing serialized artifacts such as `execution-summary.yaml` and
  `wave-plan.yaml`.
- Keep `internal/state` responsible for persistence/path resolution, not wave
  planning or engine context semantics.

## Acceptance Signals
- `rg -n 'github.com/signalridge/slipway/internal/engine|internal/engine/' internal/state`
  finds no production imports.
- `go test ./internal/architecture -count=1` fails on a future
  `internal/state -> internal/engine` import regression and passes after this
  change.
- Targeted package tests for affected state/engine/cmd behavior pass.
- `go test ./... -count=1` passes before opening the PR.

## Open Questions
None.

## Deferred Ideas
- Add the broader public-surface coverage gate from `opt.md` 3.2 as a separate
  governed change.
- Add the state-read performance benchmark/context cache work from `opt.md`
  section 4 as later governed changes.

## Approved Summary
Confirmed by user on 2026-06-27.

This change only completes `opt.md` 3.1: remove production reverse dependencies
from `internal/state` to `internal/engine`, and add an
`internal/architecture` regression gate that fails if `internal/state` imports
`internal/engine` again. Acceptance is proven by no production
`internal/state -> internal/engine` imports, passing targeted
state/engine/cmd/architecture tests, and a final `go test ./... -count=1`.
`opt.md` 3.2 coverage-gate expansion and section 4 state-read performance work
remain separate governed changes.
