# Intent

## Summary
Fix issue #384: expose an intentional refresh path for already-current passing review evidence
## Complexity Assessment
complex
Rationale: this changes a public evidence CLI workflow and must preserve existing fail-closed evidence semantics while adding an explicit operator-requested refresh path.

## Guardrail Domains
None detected.

## In Scope
- Update the `slipway evidence skill` public behavior for already-current passing review evidence.
- Add or adjust CLI help, remediation, and validation text so an explicit review rerun has an actionable path.
- Add focused regression tests covering refresh of already-current passing review evidence for selected review skills.
- Keep the fix scoped to issue #384 and the evidence recording surface.

## Out of Scope
- Subagent configuration redesign from #359/#360.
- `execution.auto` behavior changes from #361.
- Plan-audit semantic rigor and decision-soundness changes from #371.
- Release publishing or unrelated active governed changes.
- Broad evidence schema redesign unless a minimal local extension is required for #384.

## Constraints
- Preserve fail-closed evidence discipline: no silent restamp, bypass, or forged freshness state.
- Existing non-refresh evidence behavior must remain compatible unless explicitly invalid.
- The public CLI must make the supported operator action discoverable.
- Keep implementation and tests narrow enough for a high-ROI fix.

## Acceptance Signals
- A previously passing and current review skill can be intentionally refreshed or otherwise recorded through a documented public command path.
- The old dead-end `skill already has passing evidence for the current review set` behavior no longer blocks an explicit refresh workflow.
- Regression tests cover the already-current passing evidence case and prove ordinary duplicate recording still fails unless the explicit refresh path is requested.
- `go test ./...` passes or any failure is documented with root cause.

## Open Questions
None.

## Deferred Ideas
- A broader supplemental-review attachment model can be considered later if refresh semantics need richer audit history.

## Approved Summary
Confirmed by user on 2026-06-30T14:09:07Z.

Fix issue #384 by adding a supported public path for intentionally refreshing already-current passing review evidence. Scope includes the evidence CLI behavior, user-facing help/remediation, and focused regression tests for the already-current passing evidence case across review skills. Out of scope: the broader subagent configuration redesign in #359/#360, execution.auto behavior in #361, plan-audit semantic rigor in #371, and unrelated release publishing work. Primary acceptance signal: a previously passing/current review skill can be intentionally refreshed or otherwise recorded through a documented public command path, with tests proving the old dead-end error no longer blocks the explicit refresh workflow.
