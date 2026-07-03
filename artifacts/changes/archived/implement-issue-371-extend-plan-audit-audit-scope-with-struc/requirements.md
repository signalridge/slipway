# Requirements

## Requirements

### Requirement: Structured plan dimension attestation contract
REQ-001: The system MUST parse plan-dimension attestation references from
`VerificationRecord.References` using the grammar
`dim:<name>=<verdict>:<evidence-ref>` for the supported dimensions
`decision_soundness` and `consistency`, and it MUST fail closed for malformed
tokens, unsupported names, unsupported verdicts, conflicting verdicts for the
same dimension, placeholder evidence, absolute paths, parent traversal, or
unresolvable workspace-relative evidence references.

#### Scenario: Valid attestation is accepted
GIVEN a passing verification record with references
`dim:decision_soundness=pass:internal/engine/progression/advance_governed.go:1305`
and `dim:consistency=pass:artifacts/changes/example/requirements.md#requirements`
WHEN the plan-dimension contract is evaluated
THEN the system recognizes both dimensions as passing attestations.

#### Scenario: Conflicting attestation fails closed
GIVEN a verification record with both `dim:consistency=pass:<ref>` and
`dim:consistency=fail:<ref>`
WHEN the plan-dimension contract is evaluated
THEN the system reports `plan_dimension_attestation_conflict`.

### Requirement: S1 plan-audit gate enforces required dimensions without re-walk
REQ-002: The S1 plan gate MUST require a passing `plan-audit` record to carry
passing `decision_soundness` and `consistency` attestations for standard and
strict presets while the change is still in `S1_PLAN`, MUST keep light preset
attestation gaps advisory, and MUST NOT retroactively block already-past-S1
changes for tokenless S1 plan-audit evidence.

#### Scenario: Tokenless S1 plan-audit is blocked
GIVEN a standard-preset change in `S1_PLAN` with a passing `plan-audit` record
that has no required `dim:` references
WHEN the plan gate is evaluated
THEN the gate reports `plan_dimension_decision_soundness_unattested` and
`plan_dimension_consistency_unattested`.

#### Scenario: Past-S1 change is not sent back to S1
GIVEN a standard-preset change already in `S3_REVIEW` with old passing
`plan-audit` evidence that predates required `dim:` tokens
WHEN the plan gate is evaluated as part of readiness
THEN the S1 gate does not force a plan-audit rerun for the old record.

### Requirement: S3 spec-compliance review owns late plan dimension attestations
REQ-003: The S3 review authority MUST require selected
`spec-compliance-review` evidence to be passing, to carry a valid
`context_origin:stage=review=<handle>` reference, and to carry passing
`decision_soundness` and `consistency` attestations owned by the S3 review
context rather than by the S1 audit origin.

#### Scenario: Selected review without dimensions is blocked
GIVEN `spec-compliance-review` is selected for a standard-preset S3 change
WHEN its passing verification record lacks the required `dim:` references
THEN review authority reports the missing dimension attestation blockers.

#### Scenario: Selected review with review context and dimensions passes
GIVEN `spec-compliance-review` carries a valid review context-origin handle and
passing `decision_soundness` and `consistency` references
WHEN review authority evaluates the selected review set
THEN the plan-dimension portion of review authority passes.

### Requirement: Evidence CLI rejects incomplete passing dimension evidence early
REQ-004: `slipway evidence skill` MUST reject a passing `plan-audit` record and
a passing selected S3 `spec-compliance-review` record before writing evidence
when the required `dim:` references are missing, malformed, conflicting,
failing, or invalid; failed verdict records MAY carry failed dimension
attestations when accompanied by blockers.

#### Scenario: Passing plan-audit without dimensions is rejected
GIVEN an operator runs `slipway evidence skill --skill plan-audit --verdict pass`
without required `dim:` references
WHEN the CLI validates the candidate record
THEN it rejects the write with a plan-dimension reason code.

#### Scenario: Failed review can record failed dimension
GIVEN an operator records `--skill spec-compliance-review --verdict fail` with
`dim:decision_soundness=fail:<ref>` and a blocker
WHEN the CLI validates the candidate record
THEN it allows the failed evidence record to be written.

### Requirement: Reason and recovery contracts are complete
REQ-005: Every new plan-dimension blocker reason MUST be canonical, covered by
the reason-code contract, and mapped to actionable recovery that reruns the
owning skill or repairs the owning stage artifact in place without bypass,
force-close, automatic migration, or lifecycle re-walk.

#### Scenario: Missing S1 dimension points to plan-audit rerun
GIVEN readiness reports `plan_dimension_consistency_unattested` in S1
WHEN recovery is built
THEN recovery directs the operator to rerun or refresh the owning
`plan-audit` evidence.

#### Scenario: Unsound decision points to in-place repair
GIVEN readiness reports `plan_dimension_decision_unsound`
WHEN recovery is built
THEN recovery directs the operator to repair the current plan artifacts in
place and rerun the owning audit, not to bypass or force-close.

### Requirement: Deterministic consistency catches unknown prose REQ references
REQ-006: The deterministic plan validation layer MUST report
`plan_dimension_consistency_unknown_requirement_ref` when governed artifact
prose outside task `covers` metadata references a `REQ-*` identifier that is
not declared in `requirements.md`, and it MUST NOT attempt broader semantic
truth checks that the engine cannot prove.

#### Scenario: Unknown prose requirement reference is blocked
GIVEN `requirements.md` declares one requirement and a governed artifact prose
section mentions a different requirement identifier that has not been declared
WHEN plan validation runs
THEN the system reports
`plan_dimension_consistency_unknown_requirement_ref` for the undeclared
requirement identifier.

### Requirement: Generated skill surfaces require grounded dimension evidence
REQ-007: The plan-audit and spec-compliance-review skill templates MUST instruct
auditors to produce grounded `dim:decision_soundness=pass:<ref>` and
`dim:consistency=pass:<ref>` references, MUST describe failed-dimension blocker
behavior, and MUST prevent `decision_soundness` evidence from relying on
`artifacts/` self-confirmation.

#### Scenario: Plan-audit host records required references
GIVEN the plan-audit host reaches a passing audit conclusion
WHEN it records evidence
THEN its documented command includes both required `dim:` references and a
codebase-grounded decision-soundness reference outside `artifacts/`.

### Requirement: Verification covers parser, gates, CLI, templates, and generation
REQ-008: The implementation MUST include focused tests for the parser, S1 gate,
S3 review authority, evidence CLI validation, deterministic unknown-REQ scan,
reason/recovery contracts, and generated skill/template surfaces, and it MUST
run or report the repo-level verification commands attempted for the change.

#### Scenario: Test suite constrains the new contract
GIVEN the implementation is complete
WHEN the focused and repo-level verification commands run
THEN regressions in missing dimension tokens, invalid references, past-S1
re-walk behavior, S3 review ownership, and generated instructions are detected.
