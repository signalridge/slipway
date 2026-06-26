# Requirements

## Requirements

### Requirement: Shared Invocation Route

REQ-001: The system MUST classify lifecycle command invocation through one
shared route/actionability contract for `status`, `validate`, `next`, `run`,
`done`, and `evidence`.

#### Scenario: Root invocation sees a bound active change elsewhere

GIVEN a single active governed change whose bound worktree is not the current
invocation workspace
WHEN an operator runs an unscoped lifecycle command from the root checkout
THEN the command MUST either fail with `change_bound_to_other_worktree`, exit 3,
or return a diagnostics/status view whose route fields say local lifecycle
execution is not allowed and whose remediation names `cd <bound-worktree>` or
`--change <slug>`.

#### Scenario: Bound worktree invocation is locally executable

GIVEN a governed change bound to the current worktree
WHEN an operator runs `status`, `next`, or `validate`
THEN each JSON view MUST identify the same route kind, invocation workspace,
change authority path, bound workspace path, local actionability, and effective
next remediation/action.

### Requirement: Stable Missing and Archived Explicit Changes

REQ-002: The system MUST use stable explicit `--change` error taxonomy across
public lifecycle commands.

#### Scenario: Missing explicit change

GIVEN no active or archived change exists for `definitely-not-a-change`
WHEN an operator runs `validate --change definitely-not-a-change --json`
THEN the command MUST fail closed with `error_code: change_not_found`, exit 3,
and executable remediation.

#### Scenario: Archived explicit change

GIVEN a change has been archived
WHEN an operator runs active lifecycle commands with `--change <archived-slug>`
THEN the commands MUST fail closed as archived/non-active active-command usage,
without writing new evidence or active lifecycle state.

### Requirement: Readiness-Safe Freshness

REQ-003: The system MUST distinguish execution-evidence freshness from
governance/skill evidence freshness and overall readiness freshness.

#### Scenario: Execution fresh but governance skill missing

GIVEN execution evidence is fresh but required skill evidence is missing or
stale
WHEN an operator reads `status --json` or `validate --json`
THEN the JSON and human-facing contract MUST NOT imply that the whole change is
fresh or completion-ready.

#### Scenario: Ship gate blocked

GIVEN `G_ship` is blocked
WHEN an operator reads status or validation output
THEN overall readiness freshness MUST be stale, blocked, or equivalent and MUST
not be collapsed into a single fresh evidence field.

### Requirement: Status and Validate Follow Next Action Contract

REQ-004: The system MUST align `status` and `validate` with `next` for the
current machine-action kind.

#### Scenario: Review batch pending

GIVEN an S3 review batch is pending
WHEN an operator compares `next --json --diagnostics`, `status --json`, and
`validate --json`
THEN all three surfaces MUST expose `review_batch` as the current action kind,
and recovery/remediation prose MUST NOT override it with a different primary
automation path.

### Requirement: Host Capability Fail-Closed Contract

REQ-005: The system MUST make selected-skill host capability prerequisites
visible in the CLI contract and fail closed when a required capability is
unavailable without an explicit supported fallback.

#### Scenario: Delegation capability unavailable

GIVEN a selected skill requires subagent/delegation or independent-review
capability
WHEN the current host capability is unavailable and no fallback has been
selected
THEN `run`, `next`, and `validate` MUST NOT report
`prior_authorization_sufficient=true` for that action and MUST surface a
fail-closed blocker or explicit remediation.

#### Scenario: Manual fallback selected

GIVEN delegation is unavailable but the selected skill supports manual
independent reviewer evidence
WHEN the operator explicitly chooses that fallback
THEN the CLI contract MUST name the degraded mode and the evidence requirement
needed to proceed.
