# Assurance

## Scope Summary
Direction B merge of the two S3 verification surfaces into a single terminal
pre-ship gate. `goal-verification` and `final-closeout` — the only two S3 skills
that ran tests and both of which verified completion — were a single verification
concern artificially split into two review peers held together by reuse glue. This
change collapses them into one always-required `ship-verification` skill carrying
the sole hard `G_ship` gate, and removes the machinery that existed only to wire the
two halves together.

Delivered across 7 package-coherent tasks (each leaving a compilable package):
- t-01 model + gate emit-site: deleted dead reason codes
  (`closeout_goal_verification_reuse_invalid`, `closeout_chain_order_invalid`),
  re-homed the attestation/ordering codes onto ship-verification
  (`ship_verification_assurance_attestation_missing`,
  `ship_verification_reviewer_independence_missing`,
  `ship_verification_evidence_missing`), and removed the `SuiteResult` keystone type.
- t-02 skill registry: dropped `goal-verification`/`final-closeout` entries and the
  `reviewSkillGoal` constant; added the always-required `ship-verification`
  definition (S3, `G_ship`, `run_tests`, NOT a review skill); the selected review
  set is now spec/code/independent (+security).
- t-03 progression: single `SkillShipVerification` constant; rebuilt
  `buildShipAuthorityFromReadiness` as the single terminal gate; deleted
  `proofReuseEdge`/`closeoutGoalVerificationReuseBlockers`/`closeoutChainOrder`
  machinery; kept one ordering invariant (ship-verification ≥ every selected peer).
- t-04 capability + state: re-pointed the coverage-analysis host binding to
  ship-verification; removed the suite-result persistence path.
- t-05 CLI surfaces: removed the `slipway evidence suite-result` subcommand;
  repointed constant references; kept `slipway done` failing closed on `G_ship`.
- t-06 templates: deleted the goal/closeout template trees, added the merged
  ship-verification template, dropped the suite-result keystone from the review and
  command-partial templates.
- t-07 toolgen + docs: regenerated the golden inventory and SURFACE-MANIFEST;
  updated workflow/design/commands/explanation/how-to/README to the
  single-terminal-gate model.

This is a breaking change to the governance evidence contract, CLI JSON, generated
skills, and docs, by design — no backward-compatibility shim is provided and
in-flight S3 changes must re-run S3.

## Verification Verdict
All four selected S3 review peers passed with fresh, independent, run-version-matched
evidence (run_version 1), each from a distinct context-origin handle:
- spec-compliance-review: pass — `context_origin:stage=review=af34d091ac7d30b57`, `layer:R0=pass`
- code-quality-review: pass — `context_origin:stage=review=a825963dee633d68e`, `layer:IR1=pass`
- independent-review: pass — `context_origin:stage=review=ad93c563f24c7afeb`, `layer:IR1=pass`
- security-review: pass — `context_origin:stage=review=ab8a381d4c6a29224`, `layer:IR1=pass`

The selected review set excludes the verification gate (REQ-001 acceptance met:
`selected_review_skills` = spec/code/independent/security, no goal-verification).
The terminal `ship-verification` gate is recorded last, after the peers, and owns the
one authoritative full-suite run. `slipway validate --json` reports
`evidence_freshness: fresh` and `scope_contract.status: pass` with no unexpected files.

## Evidence Index
- Selected-peer verdicts: `verification/{spec-compliance-review,code-quality-review,independent-review,security-review}.yaml`
- S2 execution evidence: `verification/execution-summary.yaml`, `verification/wave-orchestration.yaml`, `verification/wave-plan.yaml`
- Plan + intake gates: `verification/plan-audit.yaml`, `verification/intake-clarification.yaml`
- Authoritative full-suite proof: `verification/logs/ship-suite.txt` (fresh `go test ./...`)
- Terminal gate verdict: `verification/ship-verification.yaml`
- Freshness/digest state: `verification/evidence-digests.yaml`
- Per-task code evidence: `.git/slipway/runtime` task records for t-01..t-07

## Requirement Coverage
- REQ-001 (single terminal ship-verification gate): t-02, t-03 — `selected_review_skills`
  excludes the gate; ship-verification is the sole `G_ship` gate, timestamped at/after
  every peer. Verified by the spec-compliance peer and this gate's freshness recheck.
- REQ-002 (retire goal-verification/final-closeout): t-01, t-02, t-06, t-07 — registry
  entries, templates, generated dirs, and command surfaces removed; clean-break grep
  confirms no live `goal-verification`/`final-closeout`/`reviewSkillGoal`/
  `SkillGoalVerification`/`SkillFinalCloseout` identifiers outside historical bundles.
- REQ-003 (cancel suite-result keystone): t-01, t-03, t-04, t-05, t-06 — `SuiteResult`
  type, persistence, the `evidence suite-result` subcommand, and review-template
  keystone references removed; the authoritative suite runs once, inside this gate.
- REQ-004 (remove proof-reuse machinery + dead codes): t-01, t-03 — `proofReuseEdge`,
  `closeoutGoalVerificationReuseBlockers`, the chain-order invariant, and the two dead
  reason codes deleted from source and the catalog.
- REQ-005 (ship-verification owns merged responsibilities): t-01, t-03, t-04, t-06 —
  one suite run + acceptance 3-level proof + freshness recheck + assurance and
  reviewer-independence attestations recorded in one evidence pass; guardrail SAST
  routing preserved (not exercised here: `guardrail_domain` is empty).
- REQ-006 (aligned surfaces + fail-closed): t-05, t-06, t-07 — code/skills/commands/docs
  aligned; `go test ./...` green; `slipway done` fails closed on `G_ship` with no
  bypass, force-close, or private-attestation path.

## Residual Risks and Exceptions
- Scope: t-05 touched `cmd/cli_e2e_test.go` and `cmd/done_bulk_reason_codes_test.go`
  (legitimate blast radius from consolidated test helpers); both are now in t-05
  `target_files` and the scope contract reports `pass` with no unexpected files.
- Breaking change: in-flight S3 changes on the prior two-surface model must re-run S3.
  Accepted — clean break was a locked intake decision; no compat shim by design.
- `guardrail_domain` is empty for this change, so the guardrail `high_risk_check`
  SAST path is not exercised here. Its routing onto ship-verification is covered by
  unit tests (REQ-005 scenario) rather than a live SAST run in this closeout.

## Rollback Readiness
The change ships as a single direct PR to `main` (slipway-self-change convention);
rollback is a clean PR revert. There is no data migration, persisted-state format
change requiring downgrade handling, or external contract beyond the governance
evidence schema itself. A reverted build returns to the prior two-surface model;
any change recorded against the new ship-verification contract in the interim would
need to re-run S3 on the reverted build (same re-run cost the forward break imposes).

## Archive Decision
Ready to archive on `slipway done`. Active-change freshness/readiness was proven
through the public `slipway validate --json` gate on the current worktree
(`evidence_freshness: fresh`, `scope_contract.status: pass`) before `done`; this is
a live validate proof on the active change, not a re-description of an archived
bundle. The terminal `ship-verification` gate records the authoritative full-suite
pass plus the assurance-complete and reviewer-independence attestations, after which
`G_ship` opens and the change is finalized.
