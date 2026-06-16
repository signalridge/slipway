# Assurance

## Scope Summary
This change fixes issue #238 by narrowing only the
`intake-clarification` freshness input for `intent.md`. Open Questions
checklist state and resolution notes are treated as research-owned routing
material for the intake digest, while substantive intake sections remain owned
by intake clarification.

The implementation is limited to `internal/engine/progression` digest behavior
and regression tests. It does not change CLI arguments, verification YAML
schemas, lifecycle stage names, or the meaning of substantive `intent.md`
sections.

## Verification Verdict
Current governed execution evidence is passing for the implemented waves.
Focused regression evidence demonstrates that resolving an Open Question no
longer reopens S0 intake recovery, while changing `## Summary` still produces
`required_skill_stale:intake-clarification:intent.md`.

Package-level integration evidence for `internal/engine/progression` is passing.
S3 review and S4 goal-verification evidence are passing. Final full-suite proof
was refreshed with `go test -parallel=1 ./...`, which ran the repository test
suite with test parallelism disabled and returned `ok` for all tested packages.
Final closeout remains responsible for stamping the active validate and ship-gate
attestations before archive.

## Evidence Index
- `verification/execution-summary.yaml`: run_summary_version 1, tasks `t-01`
  and `t-02` completed with verdict `pass`.
- `verification/wave-orchestration.yaml`: wave execution recorded as `pass`.
- `verification/wave-orchestration-notes.md`: records the RED/GREEN path,
  focused issue #238 tests, and `go test ./internal/engine/progression -count=1`.
- `verification/spec-compliance-review.yaml`: R0 review passed with
  `scope_contract:pass` and `negative_path:pass`.
- `verification/code-quality-review.yaml`: IR1 review passed with a distinct
  review-origin handle from spec compliance.
- `verification/goal-verification.yaml`: acceptance verification passed with
  fresh focused, package, and coverage command references.
- `internal/engine/progression/evidence_digests.go`: routes only
  `intake-clarification` through the filtered `intent.md` digest helper.
- `internal/engine/progression/stale_evidence_recovery_test.go`: verifies the
  stale recovery behavior for Open Questions resolution and substantive Summary
  edits.
- `internal/engine/progression/evidence_digests_test.go`: verifies the digest
  boundary for intake, research, and plan-audit.

## Requirement Coverage
- REQ-001 is covered by the intake-specific digest helper in
  `evidence_digests.go` and by tests asserting that Open Questions resolution
  does not stale intake evidence.
- REQ-002 is covered by negative tests that edit `## Summary` and assert S0
  stale recovery for `required_skill_stale:intake-clarification:intent.md`.
- REQ-003 is covered by the unchanged research and plan-audit full-file digest
  paths and by tests asserting their `intent.md` digests still change when Open
  Questions content changes.

## Residual Risks and Exceptions
No accepted scope exceptions remain.

One rollout risk remains: existing active changes whose intake evidence was
stamped with the old full-file digest may need one public recovery and
re-stamp. That is expected because historical digest values cannot be
reinterpreted without bypassing engine-owned freshness state.

## Rollback Readiness
Rollback is a normal revert of the helper and regression tests. The minimum
rollback verification is `go test ./internal/engine/progression -count=1`.
Rollback would intentionally reintroduce issue #238, so it should only be used
if the filtered intake digest causes a broader lifecycle regression.

## Archive Decision
Archive is not approved until final closeout records active `validate --json`
freshness/readiness proof in the current worktree and the ship gate accepts the
closeout references.

At the time this assurance artifact is refreshed, the change is in S4
verification and has not been archived. Active validation must be run after this
refresh and before `done`; any failing freshness, scope-contract, review,
goal-verification, final-closeout, or ship-gate check blocks archive.
