# Assurance

## Scope Summary

This change isolates advisory runtime session handoff notes by change slug,
updates hook and generated-agent guidance to use the per-change handoff
contract, and adds productized runtime hygiene for legacy handoff files,
retired runtime directories, and empty unheld lock anchors.

The implementation preserves the existing global `change-create.lock` and
`repair.lock` coordination model. Handoff content remains advisory continuation
context only; it is not lifecycle authority, governed evidence, freshness input,
or a gate.

## Verification Verdict

Implementation wave verification passed for all planned tasks. The active S2
validation after wave evidence reported fresh evidence, no blockers,
`can_advance: true`, and a passing scope contract.

During S3 review, independent and code-quality reviewers requested one repair
batch for legacy handoff filename coverage and stable runtime hygiene reason
codes. That repair was completed under fix context
`019ee328-5153-7c91-ab42-0d010692556e`, then all selected S3 peer reviews were
rerun under fresh review contexts and stamped passing evidence:
spec-compliance-review, code-quality-review, independent-review,
security-review, and goal-verification.

Current assurance verdict: implementation, repair, selected S3 review, security
review, and goal-verification evidence are complete for final closeout. Ship
readiness is still subject to final-closeout refreshing active validation and
the engine accepting G_ship before `done`.

## Evidence Index

- Plan audit: `artifacts/changes/fix-runtime-handoff-isolation/verification/plan-audit.yaml`
- Wave plan: `artifacts/changes/fix-runtime-handoff-isolation/verification/wave-plan.yaml`
- Wave orchestration: `artifacts/changes/fix-runtime-handoff-isolation/verification/wave-orchestration.yaml`
- Wave orchestration notes: `artifacts/changes/fix-runtime-handoff-isolation/verification/wave-orchestration-notes.md`
- Spec compliance review: `artifacts/changes/fix-runtime-handoff-isolation/verification/spec-compliance-review.yaml`
- Code quality review: `artifacts/changes/fix-runtime-handoff-isolation/verification/code-quality-review.yaml`
- Independent review: `artifacts/changes/fix-runtime-handoff-isolation/verification/independent-review.yaml`
- Security review: `artifacts/changes/fix-runtime-handoff-isolation/verification/security-review.yaml`
- Goal verification: `artifacts/changes/fix-runtime-handoff-isolation/verification/goal-verification.yaml`
- Full-suite proof: `artifacts/changes/fix-runtime-handoff-isolation/verification/full-suite-proof.md`
- Suite result: `artifacts/changes/fix-runtime-handoff-isolation/verification/suite-result.yaml`
- Task evidence: `.git/slipway/runtime/changes/fix-runtime-handoff-isolation/evidence/tasks/t-01.json`
- Task evidence: `.git/slipway/runtime/changes/fix-runtime-handoff-isolation/evidence/tasks/t-02.json`
- Task evidence: `.git/slipway/runtime/changes/fix-runtime-handoff-isolation/evidence/tasks/t-03.json`
- Task evidence: `.git/slipway/runtime/changes/fix-runtime-handoff-isolation/evidence/tasks/t-04.json`
- Task evidence: `.git/slipway/runtime/changes/fix-runtime-handoff-isolation/evidence/tasks/t-05.json`
- Task evidence: `.git/slipway/runtime/changes/fix-runtime-handoff-isolation/evidence/tasks/t-06.json`

Verification commands recorded during implementation:

- `go test ./internal/state -run 'TestGitScopedPaths|TestChangeHandoff'`
- `go test ./internal/fsutil -run Lock`
- `go test ./internal/state ./internal/fsutil`
- `go test ./cmd -run TestSessionStartHook`
- `go test ./cmd -run TestContextPressureHook`
- `go test ./internal/tmpl ./internal/toolgen`
- `go test ./internal/state -run Health`
- `go test ./cmd -run 'TestHealth|TestRepair'`
- `go test ./cmd ./internal/state ./internal/fsutil ./internal/tmpl ./internal/toolgen`
- `go run . validate --json`

## Requirement Coverage

- REQ-001: Covered by `t-01` and `t-02`. The state package exposes a
  per-change handoff path under the existing `runtime/changes/<slug>` root, and
  session-start tests cover two bound changes/worktrees without cross-reporting
  another change's handoff.
- REQ-002: Covered by `t-04`. Health and repair surfaces report legacy
  repo-level handoff files and the retired `.git/slipway/changes` runtime
  layout, with safe cleanup only for unambiguous empty retired directories.
- REQ-003: Covered by `t-05` and `t-04`. The lock cleanup primitive removes
  only unheld anchors without `.meta`; command-level repair wiring preserves
  global create and repair lock semantics.
- REQ-004: Covered by `t-03` and `t-06`. Context-pressure output, generated
  workflow/run guidance, template tests, generated-surface tests, and operator
  docs now describe the per-change advisory handoff contract.

## Residual Risks and Exceptions

- Legacy repo-level handoff files are intentionally not migrated or deleted
  automatically. Their ownership cannot be inferred safely, so health/repair
  reports them and points operators to the per-change replacement contract.
- Empty lock-anchor cleanup is conservative. If a lock has metadata or appears
  actively held, repair preserves it.
- Wave 1 and wave 2 were planned as parallel waves but executed sequentially in
  this session because implementation executor subagents were not explicitly
  authorized. The degraded execution mode and justification are recorded in
  `wave-orchestration.yaml`.
- Final closeout remains pending at the time this assurance artifact is updated.
  It must run after the selected S3 peer evidence and record the required
  assurance and reviewer-independence attestations before `done`.

## Rollback Readiness

Rollback is source-level revert of the affected code, template, test,
documentation, and change artifact files. No schema migration, irreversible
runtime rewrite, or destructive data cleanup is introduced.

Runtime cleanup behavior is opt-in through repair and is designed to fail safe:
ambiguous handoff content is not silently removed, and potentially active lock
state is preserved.

## Archive Decision

Archive readiness decision: ready for final closeout.

The latest active `go run . validate --json` proof after selected S3 peer
evidence showed requirements and tasks contracts valid, scope contract passing,
security-review and independent-review controls satisfied, and only
final-closeout evidence/attestations remaining. The bundle must not be archived
or treated as done until final closeout refreshes validation, records
`closeout:assurance_complete=pass` and
`closeout:reviewer_independence=pass`, and the engine accepts G_ship. After
archive, archived bundles are frozen records and must not be described as
revalidated through the active validation gate.
