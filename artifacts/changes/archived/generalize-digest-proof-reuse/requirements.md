# Requirements

## Requirements

### Requirement: Internal proof-reuse edge validator
REQ-001: The system MUST provide an internal proof-reuse validation mechanism
that can describe a source skill, a consumer skill, the required run version,
execution-summary freshness, and digest-freshness checks without hardcoding the
validator to final-closeout.

#### Scenario: reusable validation accepts a fresh edge
GIVEN goal-verification and final-closeout records for the same current
run_summary_version
AND the execution-summary is ready and fresh
AND the recorded digest inputs for both skills still match current inputs
WHEN the closeout -> goal-verification proof-reuse edge is evaluated
THEN the validator reports no blockers.

#### Scenario: reusable validation rejects stale proof
GIVEN a proof-reuse edge whose source or consumer digest inputs no longer match
the current content state
WHEN the edge is evaluated
THEN the validator returns a fail-closed blocker naming the changed input.

### Requirement: Existing closeout public contract remains stable
REQ-002: The existing final-closeout reuse contract MUST remain backward
compatible at the public evidence boundary: final-closeout still records
`closeout:goal_verification_reuse=pass` and
`closeout:goal_verification_reuse_run_version=<run_version>`, and invalid reuse
still surfaces `closeout_goal_verification_reuse_invalid`.

#### Scenario: missing reuse run version blocks ship
GIVEN final-closeout records `closeout:goal_verification_reuse=pass`
WITHOUT `closeout:goal_verification_reuse_run_version=<run_version>`
WHEN ship authority evaluates the change
THEN `closeout_goal_verification_reuse_invalid` blocks verification and is
surfaced through G_ship readiness.

#### Scenario: run-version mismatch blocks ship
GIVEN final-closeout, goal-verification, and execution-summary do not agree on
the same reuse run version
WHEN ship authority evaluates the change
THEN `closeout_goal_verification_reuse_invalid` blocks the change.

### Requirement: Guardrail proof reuse remains fail-closed
REQ-003: The system MUST NOT reuse full-suite or SAST proof when the
`suite-result.yaml` proof is missing, malformed, tied to the wrong run version,
or missing a required guardrail SAST digest for the active safety baseline.

#### Scenario: missing suite-result prevents reuse
GIVEN goal-verification or selected review proof depends on suite-result inputs
AND `verification/suite-result.yaml` is absent or invalid
WHEN proof freshness is evaluated
THEN the system reports stale or unavailable proof rather than accepting reuse.

#### Scenario: missing SAST digest prevents guardrail reuse
GIVEN a guardrail-domain change requires `<domain>.safety_baseline`
AND the suite-result lacks the matching SAST digest
WHEN the proof-reuse edge is evaluated for that domain
THEN reuse is rejected and the workflow requires fresh SAST proof.

### Requirement: Host guidance prefers validated reuse without skipping proof
REQ-004: Generated goal-verification and final-closeout guidance SHALL direct
agents to reuse proof only when the engine-validated conditions hold, and SHALL
direct agents to rerun full-suite or SAST proof when validation fails.

#### Scenario: final-closeout reuse-first wording is bounded
GIVEN goal-verification proof is fresh for the current run version and content
state
WHEN final-closeout guidance is read
THEN it presents validated reuse as the preferred path and records the required
reuse references.

#### Scenario: goal-verification producer role is preserved
GIVEN goal-verification is responsible for producing `suite-result.yaml`
WHEN goal-verification guidance is read
THEN it does not tell agents to unconditionally reuse S2 execution proof as a
replacement for producing or refreshing suite-result proof.

### Requirement: Focused verification proves reuse safety
REQ-005: The change MUST include focused tests proving valid reuse and
fail-closed invalidation before completion is claimed.

#### Scenario: focused tests pass
GIVEN the implementation is complete
WHEN `go test ./internal/engine/wave ./internal/engine/progression -count=1`
runs
THEN the command exits successfully and covers the proof-reuse safety cases.
