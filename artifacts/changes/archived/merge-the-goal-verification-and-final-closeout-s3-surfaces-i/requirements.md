# Requirements

## Requirements

### Requirement: Single terminal ship-verification gate
REQ-001: The system MUST expose a single S3 governance skill named `ship-verification` that is always-required, carries the hard `G_ship` gate, is NOT a member of the selected review set, and is timestamped at or after every selected review peer. The system SHALL NOT resolve `goal-verification` or `final-closeout` as skill names in the registry, templates, generated adapters, or docs.

#### Scenario: Selected review set excludes the verification gate
GIVEN a governed change at S3_REVIEW
WHEN `slipway status --json` reports `selected_review_skills`
THEN the list contains spec-compliance-review, code-quality-review, independent-review (plus security-review when policy selects it)
AND it does NOT contain goal-verification or ship-verification.

#### Scenario: ship-verification is the sole terminal G_ship gate
GIVEN all selected S3 review peers have passing evidence
WHEN `ship-verification` evidence is recorded with a verdict pass at or after every review peer timestamp
THEN G_ship evaluates ready
AND no `goal-verification` or `final-closeout` evidence is required anywhere in the flow.

### Requirement: Retire goal-verification and final-closeout surfaces
REQ-002: The system MUST remove the `goal-verification` and `final-closeout` skill definitions, templates, generated adapter skill directories, and command surfaces. The build MUST NOT contain `reviewSkillGoal`, `SkillGoalVerification`, or `SkillFinalCloseout` identifiers after the change.

#### Scenario: No dangling retired-surface references
GIVEN the change is implemented
WHEN the repository is searched for `goal-verification`, `final-closeout`, `reviewSkillGoal`, `SkillGoalVerification`, and `SkillFinalCloseout`
THEN no source, template, generated adapter, or doc references remain except historical artifacts under `artifacts/changes/`.

### Requirement: Cancel the shared suite-result keystone
REQ-003: The system MUST NOT produce or consume a shared `suite-result.yaml` keystone as an S3 review input. Review peers SHALL read code, artifacts, and S2 execution evidence; the authoritative full test suite MUST run exactly once, inside `ship-verification`, before ship.

#### Scenario: Reviewers do not depend on a produced suite keystone
GIVEN a governed change at S3_REVIEW
WHEN spec-compliance-review, code-quality-review, and independent-review record evidence
THEN none of their evidence contracts require a `suite-result.yaml` produced by another skill
AND the authoritative suite run is recorded only in the ship-verification evidence.

### Requirement: Remove proof-reuse-edge machinery and dead reason codes
REQ-004: The system MUST delete the `proofReuseEdge` / `proofReuseEdgeBlockers` / `closeoutGoalVerificationReuseBlockers` machinery and the goal↔closeout chain-order invariant, and MUST remove the reason codes `closeout_goal_verification_reuse_invalid` and `closeout_chain_order_invalid` from the catalog.

#### Scenario: Dead machinery and codes are gone
GIVEN the change is implemented
WHEN the repository is searched for `proofReuseEdge`, `closeoutGoalVerificationReuseBlockers`, `closeout_goal_verification_reuse_invalid`, and `closeout_chain_order_invalid`
THEN no definitions or references remain in source or the reason-code catalog.

### Requirement: ship-verification owns the merged verification responsibilities
REQ-005: The `ship-verification` skill MUST own and record, in one evidence pass: one authoritative full-suite run, the guardrail `high_risk_check:<domain>.safety_baseline` SAST proof when a guardrail domain is set, acceptance-criteria 3-level (Exists/Substantive/Wired) proof, evidence-freshness recheck, the `assurance.md` completeness attestation (standard/strict), and the reviewer-independence attestation (standard/strict).

#### Scenario: Guardrail high-risk proof routes to ship-verification
GIVEN a governed change whose guardrail_domain is non-empty
WHEN `slipway next --json` surfaces the required high-risk reference tokens
THEN those tokens are attached to the `ship-verification` skill
AND G_ship blocks until ship-verification records the matching `high_risk_check:<domain>.safety_baseline=pass`.

#### Scenario: Assurance and independence attestations gate ship on strict
GIVEN a change at the strict preset
WHEN ship-verification evidence omits the assurance-complete or reviewer-independence attestation
THEN G_ship reports the corresponding actionable blocker and does not open.

### Requirement: Aligned product surfaces and preserved fail-closed contract
REQ-006: The system MUST keep code, generated adapter skills, command surfaces, and docs (`docs/workflow.md`, `docs/design.md`) aligned to the ship-verification model, and MUST preserve fail-closed behavior with no bypass, force-close, or private attestation path and no backward-compatibility shim for the retired surfaces.

#### Scenario: Full suite is green and docs match behavior
GIVEN the change is implemented
WHEN `go test ./...` runs
THEN every package passes
AND docs/workflow.md and docs/design.md describe ship-verification as the single terminal S3 gate with no goal-verification/final-closeout references.

#### Scenario: No bypass path is introduced
GIVEN the ship-verification gate is unsatisfied
WHEN `slipway done` is attempted
THEN it fails closed with a ship-gate blocker and provides no force-close or private-attestation override.
