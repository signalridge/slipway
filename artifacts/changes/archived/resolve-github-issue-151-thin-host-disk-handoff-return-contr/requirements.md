# Requirements

## Requirements

### Requirement: Remaining heavy stages expose a disk-handoff contract
REQ-001: The system MUST update the remaining heavy governed host stages from
issue #151 (`research-orchestration`, `plan-audit`, `intake-clarification`,
`spec-compliance-review`, and `code-quality-review`) so their authored skill
surfaces describe a thin-host disk-handoff contract for bulky artifacts.

#### Scenario: Heavy stage delegates bulky artifact authoring by path
GIVEN a governed host stage needs research notes, plan audit notes, intake
summary, or review findings that may be large
WHEN an AI agent follows the generated host skill surface
THEN the host is instructed to keep bulky content out of the coordinator
context and use path-based disk handoff under `artifacts/changes/<slug>/`.

### Requirement: Subagent confirmations are not evidence
REQ-002: The system MUST state that a subagent's short return is only a claim
and that evidence verdicts, run versions, timestamps, freshness inputs, and
stamping remain owned by Slipway CLI ingestion or verification flows.

#### Scenario: Forged or stale handoff cannot pass by prose
GIVEN a subagent writes an artifact and returns a short completion line
WHEN the host records governed evidence for that stage
THEN the host is required to use the supported Slipway evidence/review or
verification path, and must not treat the subagent line as a pass verdict.

#### Scenario: CLI stamps skill verification evidence
GIVEN a host has inspected a delegated artifact and needs to record a governance
skill verdict
WHEN it records the result through `slipway evidence skill`
THEN Slipway writes the verification record with CLI-owned timestamp and
`run_version` fields and refreshes the evidence digest for passing records.

### Requirement: Host context stays bounded through required-reading paths
REQ-003: The system MUST direct hosts to pass context as required-reading paths
or structured summaries instead of pasted artifact bodies wherever disk handoff
is available.

#### Scenario: Review host receives large diff or artifact context
GIVEN a review host needs to inspect a large diff, governed artifact bundle, or
command output
WHEN it prepares work for an isolated reviewer or subagent
THEN the handoff names the relevant files or commands to read and keeps the
coordinator response bounded to a short digest or confirmation.

### Requirement: Regression coverage pins the issue #151 contract
REQ-004: The system MUST include focused regression coverage that fails when
the remaining heavy stage templates omit the disk-handoff, short-confirmation,
or CLI-owned stamping boundary required by issue #151.

#### Scenario: Template contract drifts back to inline-heavy instructions
GIVEN a future edit removes disk-handoff guidance from one of the remaining
heavy stage templates
WHEN the focused template tests run
THEN the tests fail before generated surfaces can regress silently.

### Requirement: Governed readiness proves completion
REQ-005: The change MUST reach Slipway `done_ready` with fresh current-worktree
evidence and without running `slipway done`.

#### Scenario: Issue #151 implementation is ready but not finalized
GIVEN the implementation and regression tests are complete
WHEN `go test -count=1 ./...` and governed lifecycle checks are run from the
active worktree
THEN the repository tests pass and Slipway reports the active change as
`done_ready`.
