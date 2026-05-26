# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: 
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: 

## Summary
Remove obsolete backward-compatibility paths and redundant legacy code from Slipway
## Complexity Assessment
complex
<!-- Rationale: provide justification for the assessed complexity level -->

## Guardrail Domains
external_api_contracts

## In Scope
- Identify and remove obsolete internal backward-compatibility paths, redundant helper code, dead aliases, duplicate routing logic, stale generated-surface glue, and unused compatibility shims in the Slipway Go codebase.
- Update tests, fixtures, docs, and governed artifacts when they directly describe or depend on removed internals.
- Treat Slipway as an initial-version project: remove historical migration and backward-compatibility paths rather than preserving old workspace, old generated-surface, or old agent compatibility.
- Keep the current first-version product coherent after cleanup: command surfaces, JSON output, docs, and tests should describe the new current behavior exactly.

## Out of Scope
- Do not preserve behavior solely because an older Slipway version or old generated workspace may rely on it.
- Do not change unrelated governance policy semantics, workflow presets, or lifecycle state transitions unless required to remove legacy compatibility.
- Do not perform broad style-only refactors, dependency upgrades, release work, or repository-wide formatting churn.

## Constraints
- Treat `external_api_contracts` as active because current command, JSON, and agent handoff surfaces must remain internally consistent after compatibility removal.
- Use the governed worktree for implementation and evidence.
- Prefer repo-native verification: `go test ./...` and `go build ./...`.
- Deletion must be evidence-backed by code search, tests, docs, or generated-surface references rather than keyword matching alone.
- Historical compatibility tests and docs should be removed or rewritten when they only protect old-version behavior.

## Acceptance Signals
- Research artifact lists candidate legacy/compatibility/redundant areas and classifies each as remove, migrate, or retain.
- Removed code has no remaining call sites, stale tests, or docs claiming the old internal behavior.
- User/agent-facing CLI and JSON contracts remain covered by tests or are explicitly updated with documented migration rationale.
- `go test ./...` passes.
- `go build ./...` passes.

## Intake Research Notes
- User clarified on 2026-05-26T02:52:50Z that no backward compatibility is required because this is an initial-version project.
- `runtime-state.yaml` migration/repair compatibility can be removed if current `change.yaml` behavior stays coherent.
- Generated-surface cleanup for old Slipway versions can be removed when it only supports upgrading stale generated workspaces.
- Current command/JSON surfaces still need to be coherent after cleanup; do not remove fields or commands without updating docs and tests to the new first-version contract.

## Open Questions
<!-- No unresolved intake questions. S1 research owns candidate-level classification and approach selection. -->

## Deferred Ideas
- Full public deprecation policy or versioned CLI contract design if research finds externally used legacy behavior that should eventually be removed.
- Packaging, release, or distribution changes for a future version after the cleanup lands.

## Approved Summary
Confirmed 2026-05-26T02:36:33Z. Superseded by user clarification on 2026-05-26T02:52:50Z.

Remove obsolete backward-compatibility paths and redundant legacy code from Slipway without preserving historical compatibility for older Slipway versions, old generated workspaces, or stale agent/tool surfaces. Because Slipway is still an initial-version project, compatibility-only migration paths should be deleted when the current product contract can be made coherent through matching code, docs, and tests. The work excludes unrelated governance semantics, broad style-only refactors, dependency upgrades, and release work.
