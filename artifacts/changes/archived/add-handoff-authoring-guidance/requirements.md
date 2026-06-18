# Requirements

## Requirements

### Requirement: Handoff Authoring Contract
REQ-001: The system MUST provide generated agent guidance for authoring
`.git/slipway/runtime/handoff.md` when context is tight or a fresh session needs
continuation context, and the guidance MUST make the runtime handoff useful
without turning it into governed evidence.

#### Scenario: Handoff guidance names content and boundaries
GIVEN an agent reads the generated Slipway workflow guidance
WHEN the agent needs to preserve continuation context
THEN the guidance tells the agent to write a concise narrative covering current
position, work completed in the session, next-session focus, and next action
AND the guidance tells the agent to reference intent, requirements, tasks,
decisions, diffs, and evidence by path instead of duplicating them
AND the guidance tells the agent to redact secrets, credentials, and personally
identifiable information.

#### Scenario: Suggested next skills come from the CLI
GIVEN an agent is writing a handoff for a governed Slipway change
WHEN it names suggested next skills or hosts
THEN it derives them from fresh `slipway next --json` output such as
`next_skill.name` and `next_skill.verification_dir`
AND it does not infer the next governed action from the handoff body.

### Requirement: CLI Authority Boundary
REQ-002: The system MUST keep `handoff.md` advisory and MUST NOT present it as
lifecycle authority, evidence, freshness input, or a gate.

#### Scenario: Fresh session routing remains CLI-owned
GIVEN a fresh agent sees handoff guidance or a handoff path
WHEN it needs the next governed action
THEN it uses `slipway status --json` and `slipway next --json` rather than
inferring lifecycle state from `handoff.md`.

#### Scenario: Handoff is not a governed host
GIVEN an agent reads generated Slipway guidance
WHEN it needs to preserve continuation context
THEN the guidance treats handoff authoring as advisory file-writing discipline
AND it does not introduce a standalone governed host skill, lifecycle command,
or gate for the current change.

### Requirement: Skill Template Quality Checks
REQ-003: The system MUST add compact quality guidance for Slipway
skill-template authors covering predictable skill writing: familiar leading
words, reliable context pointers, checkable completion criteria, and no-op
pruning.

#### Scenario: Quality checks preserve Slipway contract tokens
GIVEN a template author applies the shared quality checklist
WHEN they prune or rewrite skill prose
THEN they preserve concrete Slipway contract tokens such as `next_skill.name`,
`verification_dir`, reason codes, command names, and evidence paths.

#### Scenario: Quality checks avoid weak pointers and private jargon
GIVEN a template author applies the shared quality checklist
WHEN they add or move guidance behind a reference pointer
THEN the pointer says when the agent should read the referenced material
AND must-have contract material is not hidden behind an unreliable pointer
AND unnecessary Slipway-only jargon is replaced with familiar leading words
where a familiar word preserves the same contract.

#### Scenario: Completion criteria are checkable
GIVEN a generated skill describes a task, gate, review, or evidence step
WHEN the step has a completion criterion
THEN the criterion lets the agent distinguish done from not-done
AND the criterion is exhaustive where missing an item would weaken a gate,
review, or evidence contract.

### Requirement: Decision Supersession Discipline
REQ-004: The system MUST document how a replaced decision is marked superseded
without introducing a new governed artifact type.

#### Scenario: Replaced decision history remains reviewable
GIVEN a later plan decision replaces an earlier one
WHEN the author updates `decision.md`
THEN the old decision is marked as superseded by the replacement rather than
deleted or left as equally active guidance.

### Requirement: Regression Coverage and Scoped Cleanup
REQ-005: The system MUST pin the new authoring contracts with tests and MUST
limit prose cleanup to obvious high-ROI cleanup in touched surfaces.

#### Scenario: Tests reject governance bypass wording
GIVEN the generated templates are rendered in tests
WHEN handoff or quality guidance changes
THEN tests assert the new guidance exists and reject wording that lets
`handoff.md` bypass Slipway CLI gates, evidence, or freshness checks.

#### Scenario: Tests cover generated adapter surfaces
GIVEN Slipway templates are generated for supported host adapters
WHEN handoff, skill-quality, or decision guidance changes
THEN tests assert the generated workflow surface preserves the new contract
across adapters
AND tests reject stale phrases that make handoff a lifecycle authority or
remove required Slipway contract tokens.

### Requirement: Historical Intake Drift After Plan Audit
REQ-006: The system MUST avoid a forward-only dead end where stale historical
S0 intake evidence blocks S1/audit advancement after fresh passing plan-audit
evidence has certified the current planning inputs.

#### Scenario: Fresh plan audit owns current planning input freshness
GIVEN a change is in `S1_PLAN/audit`
AND `intake-clarification` evidence is stale because `intent.md` changed during
planning
AND `plan-audit` evidence is passing and digest-fresh for the current planning
inputs
WHEN stale-evidence repair targets are evaluated
THEN historical intake drift does not become the actionable repair target.

#### Scenario: Earlier intake drift remains fail-closed
GIVEN a change is still before fresh `plan-audit` evidence owns the current
planning inputs
WHEN substantive `intent.md` drift makes `intake-clarification` stale
THEN stale intake evidence remains visible as a blocker.
