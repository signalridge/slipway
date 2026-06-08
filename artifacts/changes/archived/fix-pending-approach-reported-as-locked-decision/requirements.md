# Requirements

## Requirements

### Requirement: Pending decisions are never reported as locked
REQ-001: `slipway next --json` MUST NOT include a recommended-but-unconfirmed
decision (a `decision.md` "Selected Approach"/"Selected Direction" that the
lifecycle has not yet locked) in `skill_constraints.locked_decisions`.

#### Scenario: Unconfirmed approach excluded from locked_decisions
GIVEN a governed change whose `decision.md` records a recommended "Selected
Approach" while the `G_plan` gate is not approved
WHEN `slipway next --json` is run
THEN the recommended approach does NOT appear in
`skill_constraints.locked_decisions`.

### Requirement: Pending decisions are surfaced explicitly
REQ-002: The system MUST surface the recommended-but-unconfirmed decision under
an explicit `skill_constraints.pending_decisions` field so downstream hosts can
see the recommendation while knowing it is unconfirmed.

#### Scenario: Unconfirmed approach present in pending_decisions
GIVEN the same change with a recommended "Selected Approach" and `G_plan` not
approved
WHEN `slipway next --json` is run
THEN the recommended approach appears in `skill_constraints.pending_decisions`.

### Requirement: Confirmed decisions are reported as locked
REQ-003: Once the lifecycle has locked the plan (the `G_plan` gate is
`approved`), the system MUST report the decision under
`skill_constraints.locked_decisions` and MUST NOT report it under
`pending_decisions`.

#### Scenario: Locked decision moves to locked_decisions after G_plan approval
GIVEN a governed change whose `G_plan` gate is `approved`
WHEN `slipway next --json` is run
THEN the decision appears in `skill_constraints.locked_decisions`
AND it does NOT appear in `skill_constraints.pending_decisions`.

### Requirement: Lock state is derived from the lifecycle gate, not text matching
REQ-004: The locked-vs-pending determination MUST be derived from the governed
lifecycle gate state (`G_plan`) and MUST NOT rely on expanding the decision-text
placeholder matcher to detect pending wording.

#### Scenario: Non-placeholder pending prose is still excluded while unlocked
GIVEN a `decision.md` "Selected Approach" containing concrete, author-written
prose (not template placeholder text) while `G_plan` is not approved
WHEN `slipway next --json` is run
THEN the approach is still excluded from `locked_decisions` and surfaced under
`pending_decisions`, with no change to placeholder-phrase matching.

### Requirement: Pending field is preserved in handoff payloads
REQ-005: The `pending_decisions` field MUST be preserved when
`skill_constraints` is cloned for handoff payloads.

#### Scenario: Cloned skill constraints retain pending_decisions
GIVEN a `skill_constraints` value carrying `pending_decisions`
WHEN it is cloned for a handoff payload (`cloneSkillConstraints`)
THEN the clone carries the identical `pending_decisions` contents.

### Requirement: Consumer guidance treats pending as advisory
REQ-006: The generated `spec-compliance-review` surface MUST direct hosts to
treat `pending_decisions` as advisory (do NOT enforce decision-fidelity
violations against an unconfirmed decision), and MUST be regenerated from its
template source rather than hand-edited.

#### Scenario: spec-compliance-review distinguishes pending from locked
GIVEN the regenerated `spec-compliance-review` skill surface
WHEN a host reads its Decision Fidelity guidance
THEN it enforces fidelity only against `locked_decisions`
AND it is instructed not to flag fidelity violations for `pending_decisions`.
