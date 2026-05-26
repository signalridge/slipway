# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack:
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
optimize test runtime and governance verification workflow
## Complexity Assessment
complex
Rationale: touches test behavior and governance workflow verification strategy, requiring profiling, coverage judgment, and workflow evidence without changing core governance semantics.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Measure current Go test runtime and identify the slowest or most repetitive test areas.
- Delete, merge, or narrow tests that are clearly redundant or low-value while preserving meaningful contract coverage.
- Improve efficiency of existing tests when the same coverage can be kept with less setup, less broad execution, or safer parallelism.
- Add `t.Parallel()` or equivalent parallel execution only where tests are isolated and deterministic.
- Optimize Slipway governance workflow verification so routine paths avoid unnecessary repeated full test runs while still widening verification before completion.

## Out of Scope
- Do not weaken core Slipway lifecycle, governance, handoff, or JSON contract semantics.
- Do not remove critical contract coverage merely to reduce runtime.
- Do not add a new workflow mode, preset, or user-facing configuration flag unless the existing codebase already has the needed surface.
- Do not perform unrelated cleanup outside the test/runtime and governance verification scope.

## Constraints
- Keep changes small and grounded in measured bottlenecks.
- Prefer repository-native Go tooling and existing Slipway workflow surfaces.
- Keep final verification deterministic and auditable.

## Acceptance Signals
- Baseline test runtime and post-change runtime are recorded well enough to explain the optimization.
- Targeted tests covering changed test/governance behavior pass.
- Final `go test ./...` passes.
- Final `go build ./...` passes.
- The change evidence explains which redundant test or workflow verification passes were removed or narrowed.

## Open Questions
<!-- No unresolved intake questions. Technical discovery will be recorded in research.md. -->

## Deferred Ideas
- Larger CI matrix redesign or cache strategy changes are deferred unless profiling proves they are required for this task.

## Approved Summary
<!-- Cleared by rescope pivot — re-confirm after amendment -->
