# Requirements

## Requirements

### Requirement: Pure context-origin vocabulary and R2 review wire token

REQ-001: The system MUST keep the generalized context attestation grammar in
`internal/model` as the pure source of truth for cross-stage independence:
`context_origin:stage=<stage>=<handle>`, `plan_origin:<handle>`,
`audit_origin:<handle>`, executor handle-set extraction from existing
`executor_agent:` references, and `CrossStageContextCollisions`. The retired
`review_origin` vocabulary MUST remain absent from active code, templates, docs,
and tests. The review-SET rescope MUST add only the wire constant
`StageContextReview = "review"` to `internal/model`; it MUST NOT add per-reviewer
model stage constants for spec, code, independent, or security, and it MUST NOT
change `CrossStageContextCollisions`, `participantsCollide`, or
`ContextParticipant`.

#### Scenario: Review records share one wire token without model key collapse

GIVEN passing `spec-compliance-review`, `code-quality-review`, and
`independent-review` records each carry `context_origin:stage=review=<handle>`
WHEN the engine parses those records
THEN the parser returns a `review` handle for each record, and the authority
layer keys lattice participants by recording skill name rather than by the
literal `review` stage string.

#### Scenario: The pure model helper remains unchanged

GIVEN the review-SET implementation is complete
WHEN `internal/model/context_attestation.go` is inspected
THEN it contains the new `StageContextReview` wire-token constant, retains
stdlib-only imports, and does not import `cmd`, `internal/tmpl`,
`internal/toolgen`, or `internal/engine`.

#### Scenario: Retired review_origin stays retired

GIVEN the shipped tree
WHEN source, generated templates, docs, and non-archived tests are searched for
the literal `review_origin`
THEN no active compatibility shim, parser, emitted token, or reason code remains
outside intentional retirement assertions or archived governed artifacts.

### Requirement: Plan-audit author and auditor remain distinct

REQ-002: The system MUST continue to enforce, in `EvaluatePlanGate`, that a
passing plan-audit record on standard or strict carries both `plan_origin` and
`audit_origin`, and that the two handles are distinct. Missing, malformed, or
equal plan-audit handles MUST fail closed with `plan_audit_origin_invalid`.
Light preset MUST stay advisory and non-blocking for this facet. The plan gate
MUST own only this local plan-audit edge and MUST NOT evaluate downstream
executor or review-set participants at S1.

#### Scenario: Distinct plan-audit handles pass

GIVEN a strict change whose plan-audit evidence references
`plan_origin:plan-author` and `audit_origin:plan-auditor`
WHEN `EvaluatePlanGate` runs
THEN no `plan_audit_origin_invalid` blocker is raised.

#### Scenario: Plan-audit self-stamp fails closed

GIVEN a standard change whose plan-audit evidence references the same handle for
`plan_origin` and `audit_origin`
WHEN `EvaluatePlanGate` runs
THEN it emits `plan_audit_origin_invalid` with recovery that names re-running
plan-audit with distinct native subagents.

### Requirement: S3 review dispatch is a variable concurrent review set

REQ-003: At `S3_REVIEW`, the system MUST dispatch a concurrent review set with
three mandatory reviewers, `spec-compliance-review`, `code-quality-review`, and
`independent-review`, and MUST add `security-review` only when the engine-derived
security-selection decision is true. The set size MUST be minimum 3 and maximum
4. The returned next-skill contract, status/validate/evidence views, required
skill evaluation, freshness/digest checks, and recovery surfaces MUST all use
the same selected review set. No S3 reviewer MUST be ordered after another S3
reviewer.

#### Scenario: Baseline S3 dispatch returns the mandatory trio

GIVEN a code-profile change in `S3_REVIEW` whose security-review selector is
false
WHEN `ResolveNextSkill` and the required-skills evaluator compute the S3 review
set
THEN both surfaces name exactly `spec-compliance-review`,
`code-quality-review`, and `independent-review`, and no
`required_skill_missing:security-review` blocker is emitted.

#### Scenario: Security-selected S3 dispatch returns four reviewers

GIVEN a code-profile change in `S3_REVIEW` whose security-review selector is
true
WHEN the next-skill and required-skills surfaces compute the review set
THEN both include `security-review` alongside the mandatory trio, and missing
security evidence blocks with `required_skill_missing:security-review`.

#### Scenario: Review routing has no spec-before-code fallback

GIVEN spec-compliance-review evidence exists but code-quality-review evidence is
missing
WHEN `slipway next`, `slipway validate`, or evidence recovery computes the next
action
THEN it still treats the selected S3 reviewers as unordered peers and does not
encode "code-quality after spec" as a lifecycle prerequisite.

### Requirement: Security-review selection is engine-derived and fail closed

REQ-004: The system MUST derive the optional security-review selection from one
engine-side decision, exposed as `ControlSecurityReview`, rather than from
template prose, raw disk evidence, or duplicated call-site logic. The selector
MUST activate on security-relevant guardrail domains (`auth_authz`,
`security_credentials`, `privacy_pii`, and `external_api_contracts`), on high
governance blast radius, and on strict preset with medium-or-higher governance
blast radius. Ambiguous or unavailable blast-radius data MUST degrade to at
least medium through the existing control derivation path, preserving a
fail-closed default. On light preset, security-review may be advisory; on
standard or strict, selected security-review MUST be blocking.

#### Scenario: SAST-domain changes select security-review

GIVEN a change whose guardrail domain is `auth_authz`,
`security_credentials`, `privacy_pii`, or `external_api_contracts`
WHEN controls are derived for review
THEN `ControlSecurityReview` is active and the S3 review set includes
`security-review`.

#### Scenario: High blast radius selects security-review without a guardrail domain

GIVEN a change with no guardrail domain and a planned or executed blast radius
above the high threshold
WHEN controls are derived for review
THEN `ControlSecurityReview` is active and the S3 review set includes
`security-review`.

#### Scenario: Strict medium blast radius selects security-review

GIVEN a strict change with medium governance blast radius and no SAST-domain
guardrail
WHEN controls are derived for review
THEN `ControlSecurityReview` is active; a standard medium-blast change without a
SAST-domain guardrail does not select security-review.

### Requirement: One selected-review-skill set feeds routing, requiredness, and lattice ownership

REQ-005: The system MUST compute a single selected-review-skill slice and use it
for all S3 review-set consumers: `ResolveNextSkill`, the governance registry
required-skill filter, review-authority participants, review-authority
`ownedStages`, ship-authority base participants, and closeout chain-order checks.
The selected slice MUST always contain the mandatory trio and MUST append
`security-review` only when the shared security-selection decision is true.
Any divergence between routing and requiredness MUST fail tests.

#### Scenario: Routing and requiredness cannot disagree

GIVEN a security-selected change
WHEN `ResolveNextSkill`, `RequiredSkillsForStateWithRegistry`, and
`EvaluateRequiredSkillsForChange` are evaluated for `S3_REVIEW`
THEN all three surfaces name the same four selected reviewers.

#### Scenario: Unselected security remains silent

GIVEN `security-review` is not selected but an old passing
`verification/security-review.yaml` exists on disk
WHEN review authority builds the lattice participants
THEN it does not add security-review as a participant and does not emit
`required_skill_missing:security-review`.

### Requirement: Review-authority lattice is keyed by recording skill and sourced from passingSkills

REQ-006: The review-authority lattice MUST preserve the literal
`context_origin:stage=review=<handle>` token for every reviewer while keying each
review participant by its recording skill name. The review `ownedStages` set and
the review participant loop MUST be built from the same selected-review-skill
slice. Review participants MUST be sourced from the `passingSkills` map returned
by `EvaluateRequiredSkillsForChange`, not from the raw-disk
`loadPresentPassingVerification` path. A selected and passing reviewer that lacks
a well-formed `stage=review` handle MUST fail closed with
`context_origin_handle_invalid`; any two selected reviewers sharing the same
handle, or any selected reviewer sharing a handle with executor or
`audit_origin`, MUST fail closed with `cross_stage_context_not_distinct` on an
owned edge.

#### Scenario: Same review handle fails closed on an owned edge

GIVEN spec-compliance-review and independent-review are both selected, passing,
and both record `context_origin:stage=review=same-handle`
WHEN review authority evaluates cross-stage context distinctness
THEN it emits `cross_stage_context_not_distinct` for the skill-name pair and does
not skip the edge because the wire token stage is shared.

#### Scenario: Missing selected review handle fails closed

GIVEN code-quality-review is selected and passing but records no well-formed
`context_origin:stage=review=` handle
WHEN review authority evaluates cross-stage context distinctness
THEN it emits `context_origin_handle_invalid` for code-quality-review and stops
collision evaluation for that malformed participant set.

#### Scenario: Raw disk evidence cannot resurrect an unselected reviewer

GIVEN a passing security-review file exists on disk from an older run but the
current selected-review-skill set excludes security-review
WHEN review authority builds participants
THEN security-review is absent from participants because the source is
`passingSkills`, not raw disk.

### Requirement: Ship authority and closeout ordering use the same selected review set

REQ-007: The ship-authority lattice and closeout chain-order gate MUST use the
same selected-review-skill set as S3 review authority. Goal-verification and
final-closeout MUST remain the ship-owned lattice endpoints. Review-owned edges
MUST NOT re-fire at ship, while every selected reviewer, including
independent-review and selected security-review, MUST be ordered no later than
goal-verification and final-closeout. Missing optional security-review when not
selected MUST remain silent.

#### Scenario: Independent-review is ordered before goal-verification

GIVEN independent-review and goal-verification records are both present and
passing
WHEN the independent-review timestamp is newer than goal-verification
THEN `closeout_chain_order_invalid` blocks on standard or strict with recovery
that routes to re-running the lagging verification stage.

#### Scenario: Security-review participates only when selected

GIVEN security-review is selected and has a passing record
WHEN ship authority evaluates lattice and chain order
THEN security-review participates in both checks; when security-review is not
selected, an old security-review file contributes to neither check.

### Requirement: Independent-review and security-review are standalone workflow-owned hosts

REQ-008: The system MUST promote `independent-review` and `security-review` to
standalone `StateS3Review` governance host definitions and workflow-owned
templated skill surfaces. `independent-review` MUST be mandatory;
`security-review` MUST be conditional on the shared security selector. The
embedded base-reader bindings MUST be removed from spec-compliance-review and
code-quality-review so the mandatory reviewers are distinct dimensions:
spec-trace, engineering quality, and cold independent correctness/safety.
Security-review MUST be a distinct boundary/security review when selected. Each
standalone review host MUST instruct native-subagent dispatch on the shared
worktree and record `context_origin:stage=review=<handle>` through
`slipway evidence skill`.

#### Scenario: Independent-review has one standalone home

GIVEN generated skills are rendered
WHEN the independent-review surface is inspected
THEN it is a workflow-owned S3 host template that records a `stage=review`
handle, and spec/code review templates no longer embed the independent
base-reader procedure.

#### Scenario: Security-review is conditional but first-class

GIVEN generated skills are rendered
WHEN the security-review surface is inspected
THEN it is a workflow-owned S3 host template with the same evidence shape and
`stage=review` token as the other S3 reviewers, while its requiredness still
depends on the selector.

### Requirement: Public CLI and generated surfaces expose the variable review set coherently

REQ-009: The public CLI surfaces that display, validate, record, recover, or
export governed review skills MUST handle a multi-skill S3 handoff. This
includes `next`, `status`, `validate`, `evidence`, `review`, stale-evidence
recovery, generated skill indexes, toolgen host export tests, and dogfood
fixtures. User-facing recovery MUST name all currently selected missing
reviewers and MUST not collapse the variable set to a single primary reviewer
except in call sites that explicitly need a conventionally-primary skill for
ordering.

#### Scenario: Next/validate report all missing selected reviewers

GIVEN a security-selected change enters S3 with no review evidence
WHEN `slipway next --json` and `slipway validate --json` run
THEN their action/blocker surfaces identify the four selected review skills
rather than only spec-compliance-review.

#### Scenario: Evidence recording rejects unselected security as required evidence

GIVEN security-review is not selected for the current change
WHEN stale security-review evidence exists or a caller asks why S3 is blocked
THEN the CLI does not treat security-review as required, while still allowing a
selected security-review run to satisfy the required set when the selector is
true.

### Requirement: Recovery vocabulary, documentation, and verification prove the fail-closed design

REQ-010: The system MUST keep the existing canonical reason-code contract
complete for every blocker used by the review-SET rescope. The R2 design SHOULD
reuse `required_skill_missing`, `context_origin_handle_invalid`,
`cross_stage_context_not_distinct`, `plan_audit_origin_invalid`, and
`closeout_chain_order_invalid`; if implementation introduces any new reason code
or retires any additional code, it MUST update the four-file reason/recovery
contract and tests. Documentation and templates MUST describe the variable
review set, security-selection rules, R2 `stage=review` token, fail-closed
recovery, and the honest residual that host-emitted handles are structural
attestation, not cryptographic proof. Verification MUST include unit, command,
template/toolgen, docs, and strict dogfood coverage for both the mandatory trio
and selected-security quartet.

#### Scenario: No unknown reason code is produced

GIVEN a selected reviewer is missing, malformed, colliding, or out of order
WHEN the corresponding gate emits blockers
THEN each blocker resolves through the canonical reason/recovery tables and none
downgrades to `unknown_reason_code`.

#### Scenario: Documentation matches generated behavior

GIVEN generated S3 host skills and docs are inspected
WHEN they describe S3 review dispatch
THEN they consistently name the mandatory trio, optional security-review,
single fan-out, `context_origin:stage=review=<handle>` token, and structural
trust tier.
