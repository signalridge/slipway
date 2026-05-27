# Intent

## Project Context
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: local-first Slipway governance, `change.yaml` as current-state authority, lifecycle events as append-only trace, host skills as procedural surfaces.

## Summary
Absorb the useful parts of local `spec-kitty` into Slipway without importing its lane orchestration or platform model. The selected absorption is a Slipway-native Scope Contract: planned `tasks.md target_files` become the file-boundary contract, execution evidence is reconciled against that contract, and review/verification surfaces are tightened around explicit contract evidence.

## Complexity Assessment
complex

This touches governed artifact semantics, execution evidence evaluation, CLI validation/status/review surfaces, host-skill templates, and tests. It is not sensitive-domain work and does not alter external API contracts.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Treat planned `tasks.md target_files` as the first version of a Scope Contract for allowed file boundaries.
- Add planned-vs-actual reconciliation between task plan targets and `execution_summary.tasks[].changed_files`.
- Surface scope drift as a deterministic blocker in validation/status/review readiness after execution evidence exists.
- Add a machine-readable scope drift report shape that can distinguish pass, missing contract, missing changed-files evidence, and out-of-scope changed files.
- Update host-skill/template guidance so spec/review/goal verification must check relevant contracts when present.
- Make updateable `artifacts/codebase` output prefer the active workspace/worktree path so a governed worktree can refresh its own codebase map without mutating the main checkout.
- Add focused Go tests for parser/reconciliation/gate behavior and template wording.

## Out of Scope
- No lane parallelism, cross-lane merge, or worktree-per-lane scheduler from `spec-kitty`.
- No dashboard, kanban, SaaS, orchestrator API, charter interview, or doctrine-pack platform.
- No adapter-matrix expansion beyond the current Slipway strategy.
- No event-sourcing rewrite: `change.yaml` remains current-state authority and `events/lifecycle.jsonl` remains append-only trace.
- No PreToolUse hook enforcement in this change; it is a follow-up after the reconciliation semantics are stable.
- No scope-based context-hydration pruning in this change; it is a follow-up after false-positive risk is understood.
- No living glossary implementation in this change; it remains an optional craft improvement.

## Constraints
- Keep the runtime narrow and use existing Slipway artifacts before adding new DSL.
- Do not add compact/lite/full modes or a second product model.
- Keep the first hard boundary file-based; operation-level permissions such as dependency additions or public API changes remain future work.
- Avoid broad refactors in `cmd` or governance state management.

## Acceptance Signals
- `go test ./internal/engine/... ./cmd` passes or any failure is clearly unrelated and documented.
- `go test ./...` passes before closeout if runtime allows.
- `go run . validate --json` on the active change reports the new artifact state without stale intake/research blockers after evidence is written.
- Added tests demonstrate that a changed file outside planned `target_files` is reported as scope drift.
- Host-skill/template tests demonstrate that review/goal verification guidance names contract evidence checks.
- `codebase-map` tests demonstrate that updateable `artifacts/codebase` files are written under the invocation worktree and reported as worktree-local paths.

## Open Questions
(none)

## Deferred Ideas
- Add Claude Code PreToolUse Scope Sentinel once the read-only reconciliation semantics are proven.
- Reuse Scope Contract globs for context assembly after scope matching proves low-noise.
- Add a living glossary for canonical governance terms.
- Add schema-version/migration metadata when `change.yaml` needs a larger schema evolution.
- Add operation-level allowances such as `add-deps`, `delete-tests`, and `change-public-api` only when there is a reliable detector and real demand.

## Approved Summary
Confirmed by user on 2026-05-26T17:41:14Z: implement the final selected spec-kitty absorption as Slipway-native Scope Contract support. Planned `tasks.md target_files` become the file-boundary contract, execution evidence is reconciled against `execution_summary.tasks[].changed_files`, and drift is surfaced in governance validation/review readiness. Keep `change.yaml` as authority and lifecycle events as trace. Exclude lane orchestration, dashboard/SaaS/doctrine/orchestrator layers, adapter expansion, hook enforcement, context-pruning, glossary, and schema-migration work from this change.

Amended by user on 2026-05-27: `artifacts/codebase` should prefer the active worktree because codebase maps are updateable artifacts. Include this path-authority correction in the same governed change.
