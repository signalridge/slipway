# Requirements

## Requirements

### Requirement: Auto Continues Routine Run Boundaries
REQ-001: The system MUST let `slipway run --auto` continue past routine
`run_slipway_run_to_advance` command boundaries when no real governance blocker
or executable skill handoff is present.

#### Scenario: auto crosses command-required pass-through
GIVEN a governed change has just advanced to a state or substep whose only
remaining blockers are `no_skill_required` and `run_slipway_run_to_advance`
WHEN `slipway run --auto` is executing the governed run loop
THEN the loop continues within the same invocation instead of requiring the user
to run `slipway run` again.

### Requirement: Manual Pacing Remains Unchanged
REQ-002: The system MUST preserve existing manual pacing when `execution.auto`
is false and `slipway run` is invoked without `--auto`.

#### Scenario: manual run stops at routine boundary
GIVEN a governed change reaches a routine `run_slipway_run_to_advance` boundary
WHEN `slipway run` runs without auto enabled
THEN the loop stops and returns the existing command-required confirmation
instead of continuing silently.

### Requirement: Auto Does Not Execute Skills
REQ-003: The system MUST NOT treat pure-pacing skill handoffs as executable by
the CLI; `run --auto` may soften the confirmation requirement but must still
stop for the host or agent to run the skill and record evidence.

#### Scenario: auto stops at skill handoff
GIVEN a governed change reaches `next_skill: wave-orchestration`,
`plan-audit`, or another required governance skill
WHEN `slipway run --auto` is executing
THEN the CLI returns the handoff and does not loop waiting for evidence it
cannot produce.

### Requirement: Hard Gates Stay Hard
REQ-004: The system MUST keep all non-delegable boundaries as stops under
`execution.auto`, including intake Approved Summary, security-review,
guardrail/sensitive confirmations, stale or unknown evidence, missing required
evidence, decision/human_action checkpoints, and done finalization.

#### Scenario: hard blockers are not auto-crossed
GIVEN a governed change has any non-pacing blocker or done-ready finalization
boundary
WHEN `slipway run --auto` is executing
THEN the loop stops and reports the same governance blocker or finalization
command required by the non-auto path.

### Requirement: Public Contract Matches Behavior
REQ-005: The system MUST update user-facing and generated command documentation
so `execution.auto` is described as bounded auto-to-next-real-gate behavior, not
as full automation or skill execution.

#### Scenario: docs describe bounded auto
GIVEN a user reads README or command reference docs
WHEN they inspect `execution.auto` or `slipway run --auto`
THEN the text states that auto can continue routine run boundaries and pure
pacing confirmations, but never executes skills, records evidence, or finalizes
done.
