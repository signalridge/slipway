# Assurance

## Scope Summary
This change delivers the approved targeted authoring-contract adoption for
Slipway governed guidance. It adds the runtime session handoff writing contract
for `.git/slipway/runtime/handoff.md`, keeps that handoff advisory and
CLI-revalidated, adds scoped generated skill-template quality guidance, adds
decision supersession guidance, and repairs the S1/audit stale-intake
supersession path discovered while advancing this change.

The final implementation also includes two S3 review repairs:

- SCR-002: `internal/tmpl/templates_test.go` now pins the generated
  skill-template checklist clauses for familiar action words, reliable context
  pointers, checkable criteria, no-op pruning, and preservation of
  `next_skill.name`, `verification_dir`, reason codes, command names, and
  evidence paths.
- REQ-006 repair: `freshPlanAuditSupersedesIntakeDrift` now requires a passing
  `plan-audit` record plus an existing stored `plan-audit` digest that is fresh
  against current certified planning inputs before historical intake drift is
  suppressed.

## Verification Verdict
Verification verdict: pass for the current S3 review evidence set.

Fresh S3 peer evidence is recorded for `spec-compliance-review`,
`code-quality-review`, `independent-review`, `goal-verification`, and
`security-review`, all at `run_version: 1`. The active `validate --json` check
after stamping selected reviewer evidence reports `evidence_freshness: fresh`,
all five selected S3 skills as `pass`, and `scope_contract.status: pass`.

The current full-suite proof is
`artifacts/changes/add-handoff-authoring-guidance/verification/goal-verification-proof.md`
with SHA-256 digest
`05f8a877357d607c31c6e2a0917b38e4c3f7a90644171fdd6f682132baf877ca`.
`suite-result.yaml` records the same digest for `run_summary_version: 1`.
The proof records fresh targeted test runs for `./internal/tmpl/...`,
`./internal/toolgen/...`, `./cmd/...`, `./internal/engine/progression/...`,
then `go clean -testcache` and exact `go test ./...`, all exiting 0.

## Evidence Index
- `verification/execution-summary.yaml`: run summary version 1 with tasks
  `t-00` through `t-05` completed and passing.
- `verification/wave-orchestration.yaml`: S2 wave orchestration evidence with
  executor contexts and task evidence refs for `t-00` through `t-05`.
- `verification/suite-result.yaml`: S3 suite-result keystone for
  `run_summary_version: 1`, digest
  `05f8a877357d607c31c6e2a0917b38e4c3f7a90644171fdd6f682132baf877ca`.
- `verification/goal-verification-proof.md`: fresh command proof for the final
  post-repair verification window.
- `verification/goal-verification.yaml`: passing goal-verification evidence with
  `fresh:command_ref`, `scope_contract:pass`, and
  `context_origin:stage=review=codex-goal-verification-post-repair-2`.
- `verification/spec-compliance-review.yaml`: passing R0 spec-compliance
  evidence with `layer:R0=pass`, `scope_contract:pass`,
  `negative_path:pass`, `repair_batch_id:s3-review-repair:add-handoff-authoring-guidance`,
  and `context_origin:stage=review=codex-spec-compliance-final`.
- `verification/code-quality-review.yaml`: passing IR1 code-quality evidence
  with `toolchain_compat:pass` and
  `context_origin:stage=review=codex-code-quality-final`.
- `verification/independent-review.yaml`: passing independent-review evidence
  with `context_origin:stage=review=codex-independent-final`.
- `verification/security-review.yaml`: passing security-review evidence with
  `context_origin:stage=review=codex-security-final`.
- `verification/s3-review-repair-notes.md`: repair evidence for SCR-002 and the
  REQ-006 plan-audit digest-fresh correction.

## Requirement Coverage
REQ-001 is covered by workflow, run-command, and context-pressure guidance plus
template/runtime tests that assert the handoff contract includes current
position, work completed, next-session focus, path references, suggested next
skills from fresh `slipway next --json`, and redaction guidance.

REQ-002 is covered by workflow and run-command guidance plus negative tests that
reject handoff-as-authority wording. Fresh session routing remains owned by
`slipway status --json`, `slipway next --json`, gates, freshness, and evidence.

REQ-003 is covered by `checklist-quality.md` and the repaired
`TestRequirementsQualityChecklistSidecarExistsAndIsReferenced`, which now pins
all required skill-template quality clauses and contract-token preservation.

REQ-004 is covered by `internal/tmpl/templates/artifacts/decision.md` and
`TestDecisionTemplatePinsSupersessionGuidance`, which keep replaced decisions
reviewable and marked as superseded by concrete replacement guidance without
adding a new artifact type.

REQ-005 is covered by template, toolgen, context-pressure, and progression
regression tests. The final goal-verification proof records fresh targeted
tests and full `go test ./...` after both S3 repairs.

REQ-006 is covered by `freshPlanAuditSupersedesIntakeDrift` and progression
regressions. The positive path proves digest-fresh passing `plan-audit`
supersedes historical intake drift at S1/audit. The negative path proves stale
intake remains actionable when `plan-audit` is absent, non-passing, missing a
stored digest, or carrying a stale stored digest.

## Residual Risks and Exceptions
No accepted correctness, security, spec-compliance, or code-quality exceptions
remain in the S3 review notes.

Residual operational risk is limited to ordinary governance-surface behavior:
future generated guidance edits can still drift if tests are bypassed or if new
adapters are added without extending generation coverage. The current change
mitigates that risk through template tests, toolgen tests, targeted progression
tests, selected peer review evidence, and a fresh full-suite proof.

## Rollback Readiness
Rollback is a normal git revert of the changed template, hook, progression, and
test files plus the governed artifacts for this change. No data migration,
runtime state conversion, external API migration, credential rotation, or
irreversible operation is involved.

If rollback is needed after merge, revert the source/test changes and discard
the governed bundle for this change. The advisory handoff file remains runtime
context only and is not a persisted lifecycle authority or evidence source.

## Archive Decision
Archive decision: ready for final-closeout review, subject to final-closeout
recording fresh evidence, reviewer-independence, and assurance attestations.

Before `done`, capture active `validate --json` freshness/readiness proof from
the bound worktree and rely on that active validation result for ship readiness.
Archived bundles are frozen records of the completed change and are not treated
as active validation surfaces after `done`.
