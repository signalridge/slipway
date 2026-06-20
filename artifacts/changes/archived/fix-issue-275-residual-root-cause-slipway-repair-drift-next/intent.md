# Intent

## Summary
Fix issue #275 residual root cause: slipway repair drift next_action falls through to a misleading "run slipway run" suggestion for tasks.md parse-failure drift (unknown metadata key / wave_plan_load_failed). Add a dedicated repairDriftNextAction case so such drift routes to fix-tasks.md guidance aligned with validate/run remediation, plus a regression test.

## Complexity Assessment
simple
<!-- Rationale: single focused logic fix (one new case in repairDriftNextAction) + three-surface remediation wording consistency review + one regression test. Root cause precisely located via live reproduction; no unknowns. -->

## In Scope
- `cmd/repair.go` `repairDriftNextAction` (~lines 600-616): add a dedicated case for tasks.md parse-failure drift (reason containing "unknown metadata key" / "uses unknown metadata key" / wave-plan derivation failure) whose next_action points to "edit tasks.md to fix/remove the unsupported metadata key, then re-run `slipway repair` / `slipway validate`", instead of falling through to the default "run `slipway run`".
- Cross-check remediation wording on the three already-correct surfaces for this drift class and align them as one product surface: `cmd/common.go:766` and `cmd/common.go:862` (wave_plan_load_failed → "Update tasks.md ..."), and validate's `tasks_checklist_invalid_format` fix_scope remediation ("Fix the tasks.md checklist format before continuing.").
- Regression test: assert `slipway repair --json` on an S2 change whose tasks.md carries an unknown metadata key returns an `unrepaired_drift[].next_action` that routes to fixing tasks.md and does NOT contain "slipway run"; assert the four surfaces (repair/validate/run/next) stay mutually consistent for this drift.

## Out of Scope
- Changing the unknown-metadata-key validation itself (issue is explicit: Slipway should NOT accept the key).
- Letting repair auto-rewrite/delete content in the user's governed tasks.md (fail-closed: do not mutate governed artifacts).
- Changing run's early wave_plan_load_failed interception behavior (already correct, already carries a precise remediation).

## Constraints
- Fail-closed: repair must not gain bypass/force or auto-rewrite-of-governed-artifact powers.
- Public CLI/JSON contract change (`repair --json` `unrepaired_drift[].next_action`) must be treated as an external contract.

## Acceptance Signals
- For an S2 change whose tasks.md has an unknown metadata key, `slipway repair --json` `unrepaired_drift[].next_action` routes to fixing tasks.md and contains no "run `slipway run`".
- `validate` / `run` / `next` remediation for the same drift is consistent with the repair guidance (all point to fixing tasks.md).
- New regression test passes; `go test ./...` is green.

## Open Questions
None.

## Approved Summary
**What**: Add a dedicated case to `repairDriftNextAction` (`cmd/repair.go`) so tasks.md
parse-failure drift — unknown/unsupported metadata key and wave-plan derivation failure
(`wave_plan_load_failed`) — routes its `unrepaired_drift[].next_action` to "edit tasks.md
to fix/remove the unsupported metadata key, then re-run `slipway repair` / `slipway validate`",
instead of falling through to the misleading default "run `slipway run`". Align the same-drift
remediation wording across the repair / validate / run / next surfaces so all four are mutually
consistent. Add a regression test.

**Scope bounds** — In: `cmd/repair.go` `repairDriftNextAction` logic + cross-surface wording
alignment (`cmd/common.go` `wave_plan_load_failed`, validate `tasks_checklist_invalid_format`)
+ a new `cmd/` regression test. Out: the unknown-metadata-key validation itself stays (Slipway
must keep rejecting the key); repair gains NO auto-rewrite/force/bypass over governed tasks.md
(fail-closed); run's existing early `wave_plan_load_failed` interception is unchanged.

**Primary acceptance**: For an S2 change whose tasks.md carries an unknown metadata key,
`slipway repair --json` `unrepaired_drift[].next_action` routes to fixing tasks.md and contains
no "run `slipway run`"; repair/validate/run/next stay mutually consistent; the new regression
test passes and `go test ./...` is green.

Confirmed by user: 2026-06-20.
