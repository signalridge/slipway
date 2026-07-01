# Requirements

## Requirements

### Requirement: Slot-Based Config Surface
REQ-001: The system MUST support `subagents.default`, `subagents.plan_audit`,
`subagents.executor`, `subagents.review`, `subagents.fix`, and
`subagents.verify` as the only user-configurable subagent delegation slots.

#### Scenario: Configure review via skills hub
GIVEN `.slipway.yaml` defines `subagents.review.type: skills`,
`subagents.review.name: sliphub`, and `subagents.review.session_instructions`
WHEN Slipway validates the config and builds a review handoff
THEN the resolved review directive uses type `skills`, name `sliphub`, and the
configured session instructions.

#### Scenario: Reject review substep config
GIVEN `.slipway.yaml` defines `subagents.security_review`
WHEN Slipway loads configuration
THEN validation fails closed because review substeps are not configurable slots.

### Requirement: Session Instruction Semantics
REQ-002: The system MUST call the natural-language field
`session_instructions` and treat it as delegated-session guidance inherited from
`default`, not as provider or subagent profile inheritance.

#### Scenario: Default session instructions are inherited
GIVEN `.slipway.yaml` defines `subagents.default.session_instructions` and
`subagents.verify.name`
WHEN Slipway resolves the verify slot
THEN the verify directive includes the inherited session instructions.

#### Scenario: Slot session instructions override default
GIVEN `.slipway.yaml` defines both `subagents.default.session_instructions` and
`subagents.fix.session_instructions`
WHEN Slipway resolves the fix slot
THEN the fix directive includes the fix-specific session instructions.

### Requirement: Provider Routing Boundary
REQ-003: The system MUST restrict slot `type` to `native`, `mcp`, or `skills`,
default unset `type` to `native`, and use `name` as the provider target without
adding provider-specific typed arguments to Slipway config.

#### Scenario: Native default
GIVEN `.slipway.yaml` defines `subagents.plan_audit.name: auditor` and omits
`type`
WHEN Slipway resolves the plan-audit slot
THEN the resolved directive has type `native` and name `auditor`.

#### Scenario: Invalid provider type
GIVEN `.slipway.yaml` defines `subagents.executor.type: webhook`
WHEN Slipway validates the config
THEN validation fails with the allowed provider types.

### Requirement: Host Projection Coverage
REQ-004: The system MUST project resolved subagent directives to every current
host dispatch surface: plan-audit handoff, wave executor plan, review batch,
fix contract, and ship verification handoff.

#### Scenario: Plan-audit handoff receives directive
GIVEN `.slipway.yaml` defines `subagents.plan_audit.name: auditor`
WHEN `slipway next --json` returns the plan-audit handoff
THEN `next_skill.subagent.name` is `auditor`.

#### Scenario: Review batch receives one shared review directive
GIVEN `.slipway.yaml` defines `subagents.review.type: skills` and
`subagents.review.name: sliphub`
WHEN `slipway next --json` returns a review batch
THEN `review_batch.subagent` contains the configured review directive for the
batch rather than per-reviewer substep config.

### Requirement: Documentation and Generated Surface Alignment
REQ-005: The system MUST update docs and generated skill/template wording so
users and hosts see the slot schema, `session_instructions`, and generated
capability boundary consistently.

#### Scenario: Documentation explains allowed slots
GIVEN a user reads the subagent configuration reference
WHEN they inspect the examples and field list
THEN the docs show only `default`, `plan_audit`, `executor`, `review`, `fix`,
and `verify` slots and do not mention `subagent_provider_profiles`.

#### Scenario: Generated instructions use session terminology
GIVEN generated host skill templates mention configured subagent directives
WHEN templates are rendered in tests
THEN they describe `session_instructions` as delegated-session guidance and do
not call it provider prompt inheritance.
