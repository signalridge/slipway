# Intent

## Summary
Repair confirmed public route, freshness, and host-capability gaps from the opt.md fact-check.
## Complexity Assessment
complex
Rationale: changes touch multiple public CLI JSON/text surfaces, generated skill metadata, and governance capability fail-closed contracts.

## Guardrail Domains
external_api_contracts

## In Scope
- Fact-check the pasted opt.md findings against current code and live GitHub settings.
- Repair confirmed in-code public lifecycle surface gaps:
  - non-success invocation route kinds for no-active, multi-active, explicit-missing, and bound-elsewhere paths;
  - archived-local route kind for local archived worktree status;
  - `next` and `done` freshness fields aligned with `status` and `validate`;
  - human `status` freshness prose split into execution, governance, and overall readiness;
  - machine-readable host capability declarations for independent-review.
- Add focused tests for the public surfaces and registry/template drift gates.

## Out of Scope
- GitHub Live Settings changes: live checks show main protection, branch ruleset, release tag ruleset, and release-publish reviewer protection already configured.
- Large P2 performance architecture work such as WorkspaceIndex or internal/state semantic relocation.
- Release workflow hardening already verified as present in current code.

## Constraints
- Preserve current public JSON compatibility by adding fields rather than removing existing fields.
- Do not weaken host capability fail-closed behavior; fallback must be explicit.
- Keep changes limited to command surface, capability registry/template metadata, tests, and governed artifacts.

## Acceptance Signals
- `go test ./... -count=1` passes.
- `just coverage-gate` passes both kernel and public-surface gates.
- State-read performance baseline check passes within the committed regression budget.
- `go run . next --json --diagnostics` exposes split freshness fields.
- Focused tests cover non-success `invocation_route` and host capability registry metadata.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
None

## Deferred Ideas
- Implement WorkspaceIndex/state-read route caching as a separate performance change if profiling continues to justify it.
- Move remaining internal/state semantic wave/freshness logic only under a dedicated architecture-boundary change.

## Approved Summary
User requested fact confirmation followed by complete repair/optimization for the pasted findings. This change repairs the confirmed current code gaps in route/freshness/capability public surfaces, records false or already-fixed findings as out of scope, and verifies with tests, coverage gate, and performance baseline.
