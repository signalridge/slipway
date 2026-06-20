# Requirements

## Requirements

### Requirement: Auditable Auto Checkpoint Resolution
REQ-001: The system MUST emit a distinguishable lifecycle event marker when
`execution.auto` auto-acknowledges an eligible non-guardrail `human_verify`
checkpoint, and MUST derive that marker from an explicit auto-acknowledgment
signal rather than from user-controlled response text alone.

#### Scenario: auto checkpoint resolution is distinguishable
GIVEN a governed change with a fresh non-guardrail `human_verify` checkpoint
WHEN `slipway run --auto` consumes that checkpoint
THEN the lifecycle event for `checkpoint.resolved` includes an auto-specific
reason or side effect that manual `--resume-response` events do not include.

#### Scenario: manual checkpoint resolution remains manual
GIVEN a governed change with a fresh non-guardrail `human_verify` checkpoint
WHEN `slipway run --resume-response "<text>"` consumes that checkpoint
THEN the lifecycle event remains attributable to manual response recording and
does not include the auto-specific checkpoint marker.

#### Scenario: manual sentinel text is not treated as auto
GIVEN a governed change with a fresh non-guardrail `human_verify` checkpoint
WHEN `slipway run --resume-response "auto-acknowledged"` consumes that checkpoint
THEN the lifecycle event remains manual because the auto-acknowledgment signal
was not set by the run entry path.

### Requirement: Learn Separates Manual And Auto Checkpoints
REQ-002: The system MUST keep `slipway learn` checkpoint resolution signals from
collapsing auto-acknowledged checkpoint resolutions into the same human
verification count as manual responses.

#### Scenario: learn counts auto checkpoint acknowledgments separately
GIVEN lifecycle events containing manual and auto checkpoint resolutions
WHEN `slipway learn --json` aggregates lifecycle signals
THEN manual checkpoint resolutions and auto-acknowledged checkpoint resolutions
are observable as separate signal counts.

### Requirement: Skill Auto Softening Is Explicitly Allowlisted
REQ-003: The system MUST only soften skill handoff boundaries under
`execution.auto` for skills explicitly classified as pure-pacing auto-safe.
The allowlist MUST preserve the current non-sensitive pacing behavior for these
known skills:
`intake-clarification`, `research-orchestration`, `plan-audit`,
`wave-orchestration`, `spec-compliance-review`, `code-quality-review`,
`independent-review`, `goal-verification`, and `final-closeout`.
`security-review`, `worktree-preflight`, and any unlisted or unknown skill MUST
remain hard stops under auto.

#### Scenario: pure pacing skill softens under auto
GIVEN a non-guardrail change whose next skill is one of the explicitly
allowlisted pure-pacing skills
WHEN the confirmation requirement is derived with auto enabled
THEN the boundary is an evidence continuation with prior authorization
sufficient.

#### Scenario: every current pure pacing skill remains softened
GIVEN a non-guardrail change for each currently allowlisted pure-pacing skill
WHEN the confirmation requirement is derived with auto enabled
THEN each listed skill handoff softens to an evidence continuation.

#### Scenario: unknown skill hard-stops under auto
GIVEN a non-guardrail change whose next skill is not in the pure-pacing allowlist
WHEN the confirmation requirement is derived with auto enabled
THEN the boundary remains a hard stop requiring fresh confirmation.

### Requirement: Auto Mode Safety Text Is Regression Pinned
REQ-004: The system MUST keep README, run command registry, and run prompt
surfaces pinned to the auto-mode red lines that auto never crosses sensitive or
guardrail confirmations, `security-review`, decision or human-action
checkpoints, stale checkpoints, or evidence gates.

#### Scenario: safety instruction surfaces retain red lines
GIVEN generated command surfaces and README are checked by tests
WHEN safety redline text for run auto mode is removed or weakened
THEN the relevant toolgen, template, or README regression test fails.

### Requirement: Governance Blockers Precede Handoff Pacing
REQ-005: The system MUST preserve blocker precedence so non-pacing governance
blockers outrank review-batch or skill-handoff pacing prompts even when auto is
disabled.

#### Scenario: auto-off non-pacing blocker wins over handoff
GIVEN a view with auto disabled, a non-pacing blocker, and a skill or review
handoff
WHEN the confirmation requirement is derived
THEN the result is `blocked_by_governance` rather than a handoff reason.
