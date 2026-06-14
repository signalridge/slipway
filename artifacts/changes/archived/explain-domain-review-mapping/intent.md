# Intent

## Summary
Resolve GitHub issue #203 by making Slipway's user-facing governance surfaces explain when the `domain-review` control is satisfied by `spec-compliance-review` evidence.
## Complexity Assessment
complex
Rationale: this changes machine-readable CLI surfaces for a guardrail-triggered workflow. The code blast radius is expected to be small, but the JSON contract and governance traceability behavior require regression coverage.

## Guardrail Domains
external_api_contracts

## In Scope
- Inspect how `domain-review` required actions are satisfied from recorded skill evidence.
- Update `validate --json` and/or `next --json --diagnostics` user-facing output so a satisfied `domain-review` action explicitly names the satisfying evidence or skill when `spec-compliance-review` is the evidence source.
- Preserve the intended policy behavior if `spec-compliance-review` is the correct domain-aware review evidence.
- Add focused regression coverage for the issue #203 scenario.

## Out of Scope
- Do not require a separate `domain-review.yaml` evidence file unless current code proves that the existing policy behavior is incorrect.
- Do not redesign the governance policy model or guardrail-domain selection.
- Do not change unrelated lifecycle stages, prompts, or generated skill semantics.

## Constraints
- Treat current CLI output and current worktree behavior as authority.
- Keep JSON additions backward-compatible for existing consumers where practical.
- Do not hand-edit engine-owned verification freshness state.

## Acceptance Signals
- A reproduction-shaped test shows that after only `spec-compliance-review` evidence is recorded, `domain-review` can be reported satisfied and the JSON/diagnostic surface includes an explicit satisfied-by mapping.
- Relevant Go tests pass for the touched packages.
- Governed evidence reaches the required ready state for this change.

## Open Questions
None

## Deferred Ideas
- A later change could add a broader governance-control-to-evidence explanation table in docs or help text if more implicit mappings exist.

## Approved Summary
User confirmed continuation after the guardrail scope prompt on 2026-06-14. The approved scope is to fix issue #203 by making the `domain-review` satisfaction path explicit in Slipway CLI JSON/diagnostic output when `spec-compliance-review` evidence is the satisfying record. The change will preserve current policy semantics unless implementation evidence shows the mapping is a bug, and it will exclude broader governance-policy redesign.
