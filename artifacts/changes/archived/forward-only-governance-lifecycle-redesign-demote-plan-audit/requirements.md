# Requirements

## Requirements

### Requirement: Explicit Primary Lifecycle Commands

REQ-001: Slipway MUST expose explicit primary lifecycle commands for governed
work: `intake`, `plan`, `implement`, `review`, and `done`.

#### Scenario: Command registration is complete

GIVEN generated command surfaces and root help are produced
WHEN the command inventory is inspected
THEN the primary lifecycle commands MUST be registered, documented, generated
for supported AI adapters, and covered by command-description and flag-contract
tests.

#### Scenario: Stage command rejects the wrong state

GIVEN an active change is not in the state owned by a primary stage command
WHEN that stage command runs
THEN it MUST fail closed with a governance-blocked error, report the current
state, and point to the primary command for the current state.

### Requirement: Run Is A Shortcut Driver

REQ-002: `slipway run` MUST remain available as an auto-driver shortcut, but it
MUST NOT be the only lifecycle entry point.

#### Scenario: Run reports delegation

GIVEN an active change in any governed state
WHEN `slipway run --json` is invoked
THEN the JSON response MUST include `command: "run"` and `delegated_to` naming
the primary command for the starting state.

#### Scenario: Stage commands own lifecycle meaning

GIVEN the current state is known
WHEN an agent chooses an action
THEN it SHOULD use the primary stage command for that state; `run` is reserved
for shortcut driving or automation.

### Requirement: Current-Change Amendments

REQ-003: Same-intent scope changes MUST be handled as current-change
amendments, not as a separate lifecycle command.

#### Scenario: Implementation discovers a scope gap

GIVEN an implementation executor needs a file outside declared task scope
WHEN the file is required to satisfy the existing intent
THEN the executor MUST propose or block for a scope amendment; the coordinator
MUST update the current artifacts and evidence before continuing.

#### Scenario: Objective changed

GIVEN the requested work no longer belongs to the approved intent
WHEN review or implementation detects that conflict
THEN Slipway MUST surface `new_change_required:intent_conflict` and the work
MUST continue in a new governed change.

### Requirement: Review Owns Gates And Repair Convergence

REQ-004: Review MUST own plan/code/evidence gates. Ordinary plan, scope, and
code alignment gaps before review MUST NOT become lifecycle-regression commands.
`slipway fix` MUST be the explicit S3 repair-dispatch surface for review
findings.

#### Scenario: Review feedback is repaired independently

GIVEN a selected reviewer records a failing finding
WHEN the finding is actionable inside the current intent
THEN `slipway fix` MUST surface the finding as part of a consolidated S3 repair
batch, a separate fresh-context repair subagent MUST perform the repair, the
affected reviewer MUST record rereview evidence with
`context_origin:stage=review=<handle>` and
`context_origin:stage=fix=<handle>`, and `slipway review` MUST close the
finding.

#### Scenario: Review findings are collected before repair

GIVEN the selected S3 review batch is running or has multiple findings
WHEN an agent asks for the repair path
THEN `slipway fix` MUST instruct the agent to collect all selected-reviewer
findings first, consolidate them by root cause into one repair brief, and avoid
partial one-finding repair while the selected review batch is still collecting.

#### Scenario: Fix is not local integrity repair

GIVEN an agent needs to repair S3 review findings
WHEN it asks for the repair path
THEN Slipway MUST route to `slipway fix`, not `slipway repair`; `repair`
remains bounded local integrity and layout repair.

#### Scenario: Ship gate remains hard

GIVEN review has not converged or final ship authorization is not satisfied
WHEN finalization is attempted
THEN the ship gate MUST fail closed.

#### Scenario: S3 task amendments stay in review

GIVEN the change is already in S3 review
WHEN `tasks.md` changes inside the current intent
THEN Slipway MUST NOT require S2 implementation replay solely because the task
plan changed; status, validate, next, and final closeout checks MUST keep the
change in S3 and route convergence through the selected S3 review/fix loop.

### Requirement: Continuous Wave Implementation

REQ-005: S2 implementation MUST continue wave-to-wave inside the
wave-orchestration host until a natural stop point.

#### Scenario: Wave boundary is not an operator command

GIVEN one implementation wave passes executor collection, changed-file safety,
task evidence recording, and post-wave integration
WHEN another ready wave exists
THEN wave-orchestration MUST continue to the next wave without requiring a
separate `run` call merely to cross the wave boundary.

#### Scenario: Dispatch safety remains hard

GIVEN a target-overlap preflight or post-result changed-file conflict makes
parallel dispatch unsafe
WHEN wave-orchestration detects the conflict
THEN it MUST stop for operator direction and MUST NOT silently serialize or
continue unsafe parallel writes.

### Requirement: Retired Command Surfaces Are Gone

REQ-006: Retired public repair commands MUST be removed from Cobra
registration, generated adapter surfaces, docs, surface manifest, command
partials, reason-code taxonomy, and tests. Slipway MUST NOT introduce a
replacement top-level planning command for same-intent amendments. `slipway fix`
is allowed only as the S3 review-finding repair-dispatch surface; it MUST NOT
become a lifecycle regression or local-integrity repair command.

#### Scenario: Surface inventory has no retired command rows

GIVEN the surface manifest is regenerated
WHEN command, JSON-contract, and skill rows are inspected
THEN no retired repair command row may be present, and the primary lifecycle
commands MUST be present.

### Requirement: Plan Audit Reviews The Plan, Not The Wave Cache

REQ-007: `plan-audit` MUST be the S1 review that permits S2 to start by
reviewing the governed plan artifacts themselves. It MUST NOT certify
`wave-plan.yaml` as a frozen planning authority.

#### Scenario: Plan audit permits S2 from reviewed artifacts

GIVEN the required plan artifacts are complete and coherent
WHEN `plan-audit` records a passing verdict
THEN Slipway MAY advance to S2 implementation based on those reviewed artifacts.

#### Scenario: S2 derives the wave projection from current tasks

GIVEN the change is in S2 implementation
WHEN Slipway needs a wave execution view
THEN it MUST derive the current wave projection from `tasks.md`, and stale or
missing persisted `wave-plan.yaml` MUST NOT block solely as S1 plan authority.

#### Scenario: Task evidence freshness follows task semantics

GIVEN the change is in S2 implementation and task evidence was recorded against
an earlier task-plan hash
WHEN `tasks.md` changes semantically
THEN Slipway MUST require affected task evidence to refresh without treating the
task update itself as a lifecycle regression.

### Requirement: Folded S3 Review Set

REQ-008: The live lifecycle MUST be S0/S1/S2/S3/DONE. `S4_VERIFY` MUST remain
retired, and verification work MUST be represented inside S3 review convergence.

#### Scenario: Goal verification is a peer reviewer

GIVEN the change is in S3 review
WHEN Slipway selects the review set
THEN `goal-verification` MUST be an unordered selected S3 peer that can run in
parallel with spec, code, independent, and selected security reviewers.

#### Scenario: Final closeout is last

GIVEN selected S3 peer evidence exists
WHEN final-closeout records ship evidence
THEN final-closeout MUST be at or after every selected S3 peer, but
goal-verification MUST NOT be serialized after the other reviewers.

### Requirement: Review Digest Keystone

REQ-009: S3 reviewer freshness MUST be anchored to the current execution cycle
using `suite-result.yaml` and shared reviewer input digests.

#### Scenario: Shared suite digest invalidates selected reviewers

GIVEN a selected reviewer was recorded against an earlier shared reviewer input
WHEN `suite-result:full_suite`, SAST digest, run-summary version, review summary,
or workspace diff input changes
THEN Slipway MUST stale the selected reviewer set conservatively, because the
current input model cannot prove a narrower file-scoped reviewer owner.

#### Scenario: Design narrative is honest

GIVEN a code repair changes shared reviewer inputs
WHEN readiness explains the required rereview
THEN public docs and generated skills MUST describe the full selected-set
rereview cost instead of promising file-scoped minimal reruns.
