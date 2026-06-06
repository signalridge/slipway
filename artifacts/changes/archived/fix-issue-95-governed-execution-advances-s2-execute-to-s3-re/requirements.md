# Requirements
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; generated skills/docs via toolgen; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Execution completeness gates S2_EXECUTE → S3_REVIEW (#95)
REQ-001: At S2_EXECUTE, the wave-execution sync MUST block advancement to
S3_REVIEW unless every task declared in the materialized wave-plan has a passing
run at the active `run_summary_version`. A planned task with no recorded task
evidence MUST surface as `incomplete_execution_task:<taskID>` (one reason code
per missing task), in BOTH the mutating sync (`SyncGovernedWaveExecution`) and
the read-only preview (`PreviewGovernedWaveExecution`/`slipway validate`), and
MUST be suppressed only when plan-drift blockers already own the remediation. The
all-tasks-recorded-and-passing path MUST still advance unchanged.

#### Scenario: Planned task missing evidence blocks review
GIVEN a change at S2_EXECUTE whose wave-plan declares tasks t-01..t-03 and a
passing wave-orchestration record, but task evidence exists only for t-01 and t-02
WHEN `slipway run` evaluates wave execution
THEN advancement is blocked with `incomplete_execution_task:t-03` and the change
stays at S2_EXECUTE; `slipway validate` reports the same blocker.

#### Scenario: All planned tasks recorded still advances
GIVEN the same change with passing task evidence for t-01, t-02, and t-03
WHEN `slipway run` evaluates wave execution
THEN no `incomplete_execution_task` blocker is produced and the change advances
to S3_REVIEW.

### Requirement: Incomplete execution has a fail-closed, actionable recovery (#95)
REQ-002: `incomplete_execution_task` MUST be a canonical reason code carrying an
error message, and MUST resolve to a recovery remediation (refresh-execution
class) that directs the operator to execute and record the named task via
`slipway evidence task`, or to rescope `tasks.md` so the plan no longer claims it,
then re-run. No new defer/skip/bypass path is introduced.

#### Scenario: Recovery names the task and the next command
GIVEN an `incomplete_execution_task:t-03` blocker reaches a user surface
WHEN recovery guidance is rendered
THEN the remediation names task t-03 and instructs to record its evidence or
rescope the plan, with a non-empty next command and no leaked placeholders.

### Requirement: Wave-orchestration skill states the completeness contract (#95)
REQ-003: The generated wave-orchestration skill MUST state that every planned
task needs a passing task-evidence record before the change can advance to
review, and that an intentionally-dropped task is removed by rescoping `tasks.md`
(not by skipping evidence).

#### Scenario: Skill documents completeness before advance
GIVEN the wave-orchestration generated skill
WHEN an agent reads its task-evidence/advancement guidance
THEN it learns that leaving a planned task without passing evidence blocks
advancement and that rescoping is the way to intentionally drop a task.

### Requirement: Operator-visible, fail-closed safety_baseline satisfy-path (#88)
REQ-004: For a change with a guardrail domain, there MUST be a documented,
operator-visible, fail-closed path to satisfy the required
`<domain>.safety_baseline` high-risk check: the goal-verification generated skill
documents that the host records `high_risk_check:<domain>.safety_baseline=pass`
(or `=fail`, which blocks ship) in its verification References, derived from a
real SAST run / triage. No bypass, force-close, self-attestation, or CLI
stamp-it shortcut is introduced; the token MUST reflect a real goal-verification
verdict.

#### Scenario: Guardrail change can satisfy the ship gate without a bypass
GIVEN a change whose guardrail domain requires `<domain>.safety_baseline`
WHEN the host follows the goal-verification skill, runs SAST, and records
`high_risk_check:<domain>.safety_baseline=pass` in goal-verification References
THEN the G_ship high-risk check is satisfied and no bypass surface was used.

#### Scenario: No stamp-it shortcut exists
GIVEN the satisfy-path documentation and CLI surfaces
WHEN searched for a command that records the safety_baseline without a real
goal-verification verdict
THEN none exists; the only producer is the goal-verification verification record.

### Requirement: High-risk blockers carry the next action (#88)
REQ-005: The `high_risk_check_missing` and `high_risk_check_failed` recovery
remediations MUST name the exact token (`high_risk_check:<domain>.safety_baseline`),
the producing skill (goal-verification), and that the verdict must come from a
real SAST run; AND `slipway next` MUST surface the required
`<domain>.safety_baseline` token(s) in the goal-verification handoff when the
change has a guardrail domain.

#### Scenario: Missing high-risk check remediation is actionable
GIVEN a `high_risk_check_missing:<domain>.safety_baseline` blocker
WHEN recovery guidance is rendered
THEN the remediation names the token, names goal-verification, and references a
real SAST run, with a non-empty command and no leaked placeholders.

#### Scenario: next handoff lists the required token
GIVEN a change with a guardrail domain whose next skill is goal-verification
WHEN `slipway next --json` is read
THEN the goal-verification handoff lists the required `<domain>.safety_baseline`
token.

### Requirement: repair --focus sast stops making a false promise (#88)
REQ-006: `slipway repair --focus sast` MUST NOT advertise running SAST. The
`sast` explicit focus is removed from the `repair` command surface (retained on
`review` and `validate`, where it legitimately hydrates sast-orchestration
guidance). Selecting `--focus sast` on `repair` MUST fail with an error that
redirects to `slipway review --focus sast` / `slipway validate --focus sast`. No
SAST guidance capability is lost.

#### Scenario: repair rejects sast and redirects
GIVEN `slipway repair --focus sast`
WHEN the focus is validated
THEN it is rejected with an error naming the valid path
(`slipway review --focus sast` or `slipway validate --focus sast`).

#### Scenario: review/validate keep the sast focus
GIVEN `slipway review --focus sast` and `slipway validate --focus sast`
WHEN the focus is validated
THEN both resolve to the sast-orchestration backing as before.

### Requirement: Governed changes get a dedicated worktree at creation (worktree)
REQ-008: `slipway new` MUST provision a dedicated `.worktrees/<slug>` worktree on
branch `feat/<slug>` for EVERY governed change by default — not only discovery
changes — with the governed bundle created inside that worktree, so the main
checkout stays free for parallel work. This is governed by a new
`governance.auto_provision_worktree` config (default true); when disabled, or
when the environment cannot support a worktree (not a git repository / no HEAD),
creation MUST degrade gracefully with a clear `worktree_skipped_reason` and the
bundle in the project root. The single-active-change creation guard MUST stay
correct (a change isolated in its own worktree does not wrongly block creating
another), and `done`/archive MUST still strip the worktree path.

#### Scenario: Non-discovery change is provisioned a worktree
GIVEN a git repository with `governance.auto_provision_worktree` unset (default)
WHEN `slipway new` creates a governed change with `needs_discovery=false`
THEN the change is bound to `.worktrees/<slug>` on `feat/<slug>`, its bundle is
created under that worktree, and `worktree_skipped_reason` is not
`discovery_not_required`.

#### Scenario: Provisioning can be disabled
GIVEN `governance.auto_provision_worktree=false`
WHEN `slipway new` creates a non-discovery governed change
THEN no worktree is created, the bundle lives in the project root, and
`worktree_skipped_reason` reports the disabled/unsupported reason.

#### Scenario: Worktree-isolated changes do not block each other
GIVEN an active governed change bound to its own dedicated worktree
WHEN `slipway new` creates another governed change in its own worktree
THEN the single-active-change guard does not wrongly reject creation.

### Requirement: Generated surfaces stay aligned with zero drift (#88, #95)
REQ-007: After the source, skill-template, and surface changes, all generated
skills, command references, and docs MUST be regenerated so the toolgen
self-loop reports zero drift, and `go build ./... && go vet ./... &&
go test ./...` MUST pass.

#### Scenario: Toolgen self-loop is clean
GIVEN the change is implemented
WHEN toolgen regeneration and the drift check run
THEN there is zero drift and the build, vet, and test commands pass.
