# Intent

## Summary
Fix health --governance falsely reporting missing final assurance coverage verdicts as a blocking traceability incident during S2 execution (issue #92): make the assurance-coverage traceability gap stage-aware so it is non-blocking before review/verify and only fails closed at closeout, aligning health with validate/next readiness surfaces.
## Complexity Assessment
simple
<!-- Rationale: provide justification for the assessed complexity level -->
Single, localized classification fix in the governance traceability evaluator plus its one call site, guarded by unit tests. No new commands and no schema changes; the only additional surface is the doctor-synthesis rendering in `cmd/health.go`, which mirrors the same gap classification (an advisory traceability check raises no doctor incident). Root cause is already understood; the only design decision (how to classify the gap before S3) is resolved.

## In Scope
- `internal/engine/governance/traceability.go`: thread the change's lifecycle state/phase into `TraceabilityInput`, and make the assurance-coverage gaps (`requirement missing assurance coverage verdict` and `assurance verifies no requirement IDs`) **non-blocking (WARN-level)** while the change is before `S3_REVIEW`; keep them **blocking** at/after `S3_REVIEW`.
- `internal/engine/governance/health.go`: pass `change.CurrentState` into `TraceabilityInput` at the `deriveGovernanceControls` call site so the evaluator can apply the stage rule.
- `cmd/health.go`: in `governanceDoctorActions`, stop promoting a `traceability_coherence` check whose gaps are all non-blocking into a `governance_traceability_coherence` doctor action, so `--doctor` raises no non-repairable incident before review; a blocking (`FAIL`) check still surfaces unchanged.
- Unit tests proving: at `S2_EXECUTE` incomplete assurance coverage yields `traceability_coherence` = WARN (governance not forced unhealthy by this gap) and no doctor action; at `S3_REVIEW`/`S4_VERIFY`/`DONE` it yields FAIL (still fails closed) and the doctor action returns.

## Out of Scope
- No change to `validate` / `slipway next` behavior — they already correctly route to `wave-orchestration` at S2; this change makes `health` agree, not the reverse.
- No change to other traceability gap types (requirement→intent, task→requirement, decision→requirement, blocking open questions) — those remain blocking as today.
- Issue #91 (mechanical requirements/tasks seeding) — separate governed change.

## Constraints
- Must remain fail-closed at `S3_REVIEW`, `S4_VERIFY`, and `DONE`: a change cannot reach `done` with missing assurance coverage verdicts.
- Public health JSON shape stays stable (status values remain OK/WARN/FAIL; gap records unchanged in shape).
- Smallest clean design: no new lifecycle plumbing beyond passing the already-available `CurrentState`.

## Acceptance Signals
- A change at `S2_EXECUTE` with incomplete per-requirement assurance coverage: `slipway health --governance --json` reports `traceability_coherence` status `WARN` (not `FAIL`), `governance.healthy` is not driven false by this gap alone, and `--doctor` raises no non-repairable incident for it.
- The same change at `S3_REVIEW` with incomplete assurance coverage: `traceability_coherence` is `FAIL` (fails closed before closeout).
- `slipway validate --json` and `slipway next --json` outputs are unchanged for the S2 case; all three readiness surfaces agree the next action is `wave-orchestration`.
- New/updated Go unit tests pass and `go test ./...` is green.

## Approved Summary
Make the assurance-coverage traceability gap stage-aware. Before `S3_REVIEW`, a requirement lacking an assurance coverage verdict (or an assurance file that verifies no requirement IDs) is reported as a non-blocking WARN gap in `traceability_coherence`, so `health --governance` no longer raises a false blocking/incident state that contradicts `validate`/`next` (which correctly send the user to `wave-orchestration`). At and after `S3_REVIEW` the same gaps are blocking, preserving fail-closed behavior at closeout. Scope is limited to `traceability.go` + the `health.go` call site plus tests; `validate`/`next` and all other gap types are untouched; #91 is excluded. Primary acceptance signal: S2 → `traceability_coherence` WARN, S3+ → FAIL.

Confirmed by user 2026-06-06 (selected "S3 前降级为非阻塞 WARN (推荐)" approach, which also confirmed this scope).

Scope update 2026-06-07: review found the original scope (evaluator + health call site) did not fully satisfy the already-approved `--doctor` acceptance signal, because `cmd/health.go` `governanceDoctorActions` still promoted the now-advisory `traceability_coherence` WARN into a non-repairable doctor action — re-creating the #92 contradiction at a lower severity. Scope expanded to include the doctor-synthesis surface (`cmd/health.go` + `cmd/health_test.go`) and a first-class requirement (REQ-004). User authorized completing this through a full governed re-walk.
