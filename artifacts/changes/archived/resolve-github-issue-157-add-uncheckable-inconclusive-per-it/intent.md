# Intent

## Summary
Resolve GitHub issue #157: add uncheckable/inconclusive per-item status and coverage accounting for spec verification.
## Complexity Assessment
complex
Rationale: this changes an AI-facing verification contract in a guardrail
domain. The implementation surface is narrow, but the output semantics affect
how governed review records uncertain traceability.

## Guardrail Domains
external_api_contracts

## In Scope
- Update the authored spec-trace skill contract under
  `internal/tmpl/templates/skills/spec-trace/` so per-mapping coverage can
  record `covered`, `skipped`, `drift`, `ambiguous`, and `uncheckable`.
- Require `ambiguous` and `uncheckable` rows to carry a reason and to be counted
  as coverage gaps rather than silently treated as pass evidence.
- Update the spec-compliance-review authored template only as needed so review
  guidance treats "could not check" as an auditable coverage gap and blocks
  pass claims when ambiguity is unresolved.
- Add or update template tests that lock the Issue #157 contract in the
  generated skill text.
- Drive the governed change through fresh validation evidence until Slipway
  reports `done_ready`.

## Out of Scope
- Do not rework the lifecycle engine to infer semantic uncertainty from prose.
- Do not re-open or re-solve Issue #104; it is only a related caution about
  explicit parseable contracts.
- Do not copy GSD ADR-22 wholesale or introduce checker-authority machinery
  beyond the scope needed by this skill-output contract.
- Do not run `slipway done`; stop at `done_ready` unless the user explicitly
  asks for finalization later.

## Constraints
- Source of truth for generated skill content is
  `internal/tmpl/templates/skills/`, not exported/generated copies.
- Keep the change contract-based: engine emits/hosts the checklist surface and
  the skill judges the trace, avoiding new engine prose heuristics.
- Sensitive or external-contract governance must fail closed: unresolved
  `ambiguous` or `uncheckable` rows cannot falsely bless a pass verdict.

## Acceptance Signals
- Rendered spec-trace guidance exposes a mandatory coverage matrix with
  per-item `ambiguous` and `uncheckable` statuses, reason fields, and coverage
  gap accounting.
- Spec-compliance-review guidance requires recording uncertain trace mappings as
  coverage gaps and disallows pass claims when unresolved ambiguity remains.
- Targeted template tests fail on the old `covered | skipped | drift`-only
  contract and pass after the new contract is present.
- Relevant Go tests, formatting/static checks, `go run . validate --json`, and
  `go run . run --json --diagnostics` prove the governed change is `done_ready`.

## Open Questions
None.

## Deferred Ideas
- A future change can add structured machine validation for trace matrix rows if
  the project decides that skill-output prose is no longer enough.
- A future change can model checker-authority tiers in engine-owned evidence if
  the workflow needs hard-block severity to vary by checker capability.

## Approved Summary
Confirmed by user delegation on 2026-06-10 after selecting the `standard`
workflow preset and authorizing future choices to be made by best evidence. This
change narrows Issue #157 to the remaining real gap: spec verification must be
able to record per-item `ambiguous`/`uncheckable` coverage rows with reasons and
coverage-gap accounting, so "could not check" is auditable and never silently
passes. The work is limited to authored skill/template contract updates,
regressions, and governed readiness evidence, stopping at `done_ready`.
