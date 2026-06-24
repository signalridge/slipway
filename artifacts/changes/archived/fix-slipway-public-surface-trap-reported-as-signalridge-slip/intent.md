# Intent

## Summary
Fix Slipway public-surface trap reported as signalridge/slipway#325: `slipway next --json --diagnostics` exposes `input_context.wave_plan`, a diagnostic-only projection (cmd/next.go wavePlanView) that carries view-only fields `wave_count` and `advisories`. Its shape diverges from the persistable engine-owned cache wave-plan.yaml (model.WavePlan), which has neither field. When an agent mirrors the visible projection into wave-plan.yaml, the strict KnownFields loader (internal/state/wave_execution.go) rejects the extra fields with wave_plan_load_failed, and the remediation misleadingly tells the user to update tasks.md. Make the public surface non-misleading: distinguish the diagnostic projection from the persistable cache, and make the wave_plan_load_failed remediation point at the engine-owned wave-plan cache (regenerate via the public flow / never hand-edit) instead of tasks.md. wave-plan.yaml must stay an engine-owned, non-hand-editable cache; view-only fields stay out of it.

## Complexity Assessment
simple
<!-- Rationale: a focused public-surface correctness fix — accurate remediation/diagnosis for one reason code plus a non-breaking diagnostic-only annotation. No new lifecycle behavior, no schema growth, bounded test surface. -->

## In Scope
- **Accurate `wave_plan_load_failed` remediation** (`internal/model/reason_code.go`, `internal/model/recovery.go`): rewrite the message/remediation so it points at the engine-owned wave-plan cache and the public regenerate path (`slipway repair` then `slipway run` to refresh affected execution evidence; never hand-edit), instead of "update tasks.md".
- **Precise diagnosis on unknown/unsupported fields**: where the strict `KnownFields` loader rejects `wave-plan.yaml` (`internal/state/wave_execution.go` `loadWavePlanFromPath`, and the error-wrap / remediation-mapping site `cmd/common.go` ~L1097), surface an actionable error that names the engine-owned cache shape / unsupported-field cause rather than implying a `tasks.md` authoring problem.
- **Read-side diagnostic-only annotation** (`cmd/next.go` `wavePlanView` + the surrounding `input_context.wave_plan` surface and relevant generated command-surface / handoff docs): mark the projection as diagnostic-only and non-persistable, in a NON-breaking way (semantic doc/comment; no change to existing JSON field names or structure).
- **Tests**: (1) a `wave-plan.yaml` carrying view-only / unknown fields yields the new accurate remediation (cache shape + regenerate, NOT tasks.md); (2) regression asserting view-only fields (`wave_count`, `advisories`) stay excluded from the persisted cache; (3) coverage for the diagnostic-only annotation.

## Out of Scope
- Direction B: expanding `model.WavePlan` to accept `wave_count` / `advisories` (explicitly rejected — would pollute the cache and freshness hashes with derived/view-only data).
- Changing existing `input_context.wave_plan` JSON field names or structure (no breaking public-contract change; plan-audit and contract tests must keep parsing it).
- Relaxing the strict `KnownFields(true)` parsing of `wave-plan.yaml` (the cache stays fail-closed; we improve the *diagnosis*, not the schema's tolerance).
- Issue #324 (S2 stale-evidence recovery recommending a state-invalid `slipway fix`) — distinct issue.
- Any change to wave-planning logic, freshness hashing, or advisory computation semantics.

## Constraints
- `wave-plan.yaml` remains engine-owned and non-hand-editable; fail-closed semantics preserved.
- No breaking change to the public JSON contract consumed by plan-audit or guarded by contract tests.
- Follow repo conventions; full `go test ./...` and golangci-lint (gofmt simplify) must be green.

## Acceptance Signals
- A `wave-plan.yaml` containing `wave_count`/`advisories` (or any unsupported field) produces an error whose remediation names the engine-owned wave-plan cache and the public regenerate path and does NOT instruct the user to edit `tasks.md`. (unit test)
- `input_context.wave_plan` is annotated/documented as a diagnostic-only, non-persistable projection; a regression test confirms view-only fields are never written into `wave-plan.yaml`. (new + existing `TestNarrowingAdvisoriesAreViewOnlyAndExcludedFromPersistedPlan` stay green)
- `go test ./...` and golangci-lint run clean.

## Open Questions
None.

## Approved Summary
Resolve signalridge/slipway#325 (Direction A + read-side hardening). The bug: `input_context.wave_plan` is a diagnostic-only projection carrying view-only fields (`wave_count`, `advisories`) whose shape diverges from the persistable, engine-owned cache `wave-plan.yaml` (`model.WavePlan`); mirroring the projection into the cache trips the strict `KnownFields` loader with `wave_plan_load_failed`, whose remediation misleadingly points at `tasks.md`.

The fix: (1) rewrite the `wave_plan_load_failed` message/remediation to point at the engine-owned wave-plan cache and the public regenerate path (`slipway repair` then `slipway run`; never hand-edit) instead of `tasks.md`; (2) on strict-parse rejection of unsupported/unknown fields, surface an actionable error that names the cache shape / unsupported-field cause; (3) annotate `input_context.wave_plan` as a diagnostic-only, non-persistable projection in a non-breaking way; (4) add tests.

Key boundaries: do NOT expand `model.WavePlan` to accept the view-only fields (Direction B rejected), do NOT change existing `wave_plan` JSON field structure, do NOT relax `KnownFields(true)` strictness, and leave #324 untouched.

Primary acceptance signal: a `wave-plan.yaml` containing `wave_count`/`advisories` produces a remediation that names the engine-owned cache + public regenerate path and does NOT instruct editing `tasks.md`; view-only fields stay excluded from the persisted cache; `go test ./...` and golangci-lint green.

Confirmed by user: 2026-06-24T12:21:39Z
