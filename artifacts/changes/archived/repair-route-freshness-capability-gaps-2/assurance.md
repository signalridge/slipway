# Assurance

## Scope Summary

This change deep-confirmed the remaining route/freshness/capability reports
against current `main` and narrowed the repair to the two confirmed public
lifecycle-surface gaps:

- default compact `run --json` needed explicit regression coverage for
  selected host capability requirements, capability blockers, blocked freshness,
  recovery, confirmation, and no-advance behavior when independent review
  subagent capability is unavailable;
- `status --json` needed to expose the same actionable selected review skill
  that `next`, `next --diagnostics`, `validate`, and `run` already expose for
  blocker-driven review-alignment handoffs.

The production repair is intentionally small. `statusView` now includes
`actionable_next_skill`, and `buildStatusViewWithReadContext` populates it from
the existing shared `buildActionableNextSkillView` helper. The rest of the work
is focused regression coverage in `cmd/progression_next_test.go`.

Out-of-scope reported items were not changed: StateReadContext performance, CI
scheduling, release/ruleset policy, supply-chain workflow hardening, and the
stale claim that `run` bypasses the shared next-view capability gate.

## Verification Verdict

Pre-ship verification is passing for the implemented scope. The peer review set
is complete and fresh for `run_summary_version=1`:

- `spec-compliance-review`: pass, `layer:R0=pass`,
  `scope_contract:pass`, `negative_path:pass`,
  `context_origin:stage=review=spec-review-20260628-r2`;
- `code-quality-review`: pass, `layer:IR1=pass`,
  `context_origin:stage=review=quality-review-20260628-r2`;
- `independent-review`: pass, `layer:IR1=pass`,
  `context_origin:stage=review=independent-review-20260628-r2`.

Fresh local verification also passed:

- `go test -count=1 ./cmd -run 'TestStalePlanningReviewAlignmentActionContractStaysConsistentAcrossSurfaces|TestReviewStateActionableNextSkillConsistentAcrossCommandSurfaces|TestReviewBatchHostCapabilityUnavailableFailsClosedUnlessFallbackSelected'`
- `go test -count=1 ./cmd`
- `golangci-lint run ./...`
- `git diff --check`
- `SLIPWAY_HOST_CAPABILITIES=subagent go run . validate --json`

The active validation captured after peer evidence and with the explicit
subagent host capability signal reported `skills_ready` pass for all selected
peer reviews, `execution_evidence_freshness=fresh`, and
`scope_contract.status=pass`. The only remaining blockers at that point were
the expected terminal closeout blockers for this assurance artifact and
ship-verification evidence/attestations.

## Evidence Index

- `verification/execution-summary.yaml`: S2 task evidence for `t-01`, `t-02`,
  and `t-03`, all passing for `run_summary_version=1`.
- `verification/wave-orchestration.yaml`: wave orchestration pass for the three
  planned implementation tasks.
- `verification/spec-compliance-review.yaml`: S3 spec-compliance pass after the
  r2 repair.
- `verification/code-quality-review.yaml`: S3 code-quality pass after the r2
  production projection change.
- `verification/independent-review.yaml`: S3 independent-review pass after the
  r2 repair.
- Focused regression tests:
  `TestReviewBatchHostCapabilityUnavailableFailsClosedUnlessFallbackSelected`,
  `TestStalePlanningReviewAlignmentActionContractStaysConsistentAcrossSurfaces`,
  and `TestReviewStateActionableNextSkillConsistentAcrossCommandSurfaces`.
- Command-package suite: `go test -count=1 ./cmd`.
- Lint and diff hygiene: `golangci-lint run ./...` and `git diff --check`.

## Requirement Coverage

- REQ-001 is covered by
  `TestReviewBatchHostCapabilityUnavailableFailsClosedUnlessFallbackSelected`.
  The test asserts compact `run --json` includes host capability details,
  `host_capability_unavailable`, `blocked_by_governance`, blocker-resolution
  confirmation, blocked freshness, recovery guidance, compact handoff
  `S3_REVIEW`, and a persisted reload proving `CurrentState == S3_REVIEW` and
  `PlanSubStepNone`.
- REQ-002 is covered by
  `TestStalePlanningReviewAlignmentActionContractStaysConsistentAcrossSurfaces`
  and `TestReviewStateActionableNextSkillConsistentAcrossCommandSurfaces`.
  The tests prove `status --json`, `validate --json`, `next --json`,
  `next --json --diagnostics`, and `run --json` report the same current action
  class and the same actionable selected review skill.

## Residual Risks and Exceptions

- The active S3 surface still carries the expected
  `s3_task_plan_amendment_review_required` diagnostic because `tasks.md` was
  amended during S3 review to include the newly touched status projection files.
  `validate --json` treats this as a review-required diagnostic, not an
  execution freshness failure; the r2 peer reviews covered the amended scope.
- `SLIPWAY_HOST_CAPABILITIES=subagent` is required for validation in this host
  when independent-review subagent capability has actually been provided. With
  the signal omitted, Slipway correctly fails closed with
  `host_capability_unavailable`.
- Terminal ship-verification is intentionally still pending at this assurance
  authoring point. It must run last and record the required
  `closeout:assurance_complete=pass` and
  `closeout:reviewer_independence=pass` attestations before the change can
  advance to done-ready.

## Rollback Readiness

Rollback is simple because the production change is additive status JSON
projection plus tests. Reverting `cmd/status.go`, `cmd/status_view_build.go`,
and the added regression blocks in `cmd/progression_next_test.go` restores the
previous public status shape and test suite. No data migration, external API
write, irreversible operation, dependency update, or configuration mutation was
introduced.

## Archive Decision

Not archive-ready yet. The implementation and peer review scope are complete,
but the active change must still pass terminal ship-verification and then active
`validate --json` before `done`. Archived bundles must be treated as frozen
records only; active readiness must come from the current governed worktree
before finalization.
