# Requirements

## Requirements

### Requirement: Checkpoint Surface Deletion
REQ-001: The system MUST remove `checkpoint` as a product and agent-facing
command surface, including the active checkpoint lifecycle state, the
`--resume-response` protocol, `resume_checkpoint` handoff data,
`checkpoint_type` task metadata, checkpoint reason codes, and live generated
skill/docs guidance.

#### Scenario: Root help no longer advertises checkpoint
GIVEN the Slipway CLI is built from this worktree
WHEN an operator runs `go run . --help`
THEN the output MUST NOT list `checkpoint` in any command group.

#### Scenario: Run surfaces no checkpoint response protocol
GIVEN the Slipway CLI is built from this worktree
WHEN an operator runs `go run . run --help` or `go run . implement --help`
THEN the output MUST NOT include `--resume-response`
AND `go run . run --help` MUST still include `--resume`.

#### Scenario: Checkpoint state is not a live lifecycle concept
GIVEN governed lifecycle state is loaded
WHEN status, next, run, repair, health, and abort paths inspect the change
THEN they MUST NOT branch on `ActiveCheckpoint`
AND they MUST NOT emit `resume_checkpoint`, checkpoint-resolution side effects,
or checkpoint-specific remediation.

### Requirement: Learn And Stats Command Deletion
REQ-002: The system MUST remove `learn` and `stats` as standalone command
surfaces and generated agent commands while preserving useful retained
diagnostics through supported commands such as `status --stats` and `health`.

#### Scenario: Root help no longer advertises learn or stats
GIVEN the Slipway CLI is built from this worktree
WHEN an operator runs `go run . --help`
THEN the output MUST NOT list `learn`
AND the output MUST NOT list standalone `stats`.

#### Scenario: Learn unsupported apply path is gone
GIVEN `learn` is deleted as a command
WHEN the command registry, tests, docs, and generated skill inventory are
searched
THEN no live product path MUST expose `learn_apply_unsupported`.

#### Scenario: Retained diagnostics remain reachable
GIVEN standalone `stats` is deleted
WHEN an operator runs retained diagnostic commands
THEN repo statistics that remain load-bearing MUST be reachable through
`status --stats`, `health`, or internal helpers used by those commands.

### Requirement: Generated Surface Alignment
REQ-003: The system MUST keep CLI help, command registry metadata, generated
skills, docs, toolgen install profiles, and `docs/SURFACE-MANIFEST.json`
aligned with the deleted Workstream A surfaces.

#### Scenario: Manifest and docs do not advertise deleted commands
GIVEN generated surfaces have been refreshed or checked
WHEN docs and manifest files are searched
THEN live references to `slipway checkpoint`, `$slipway-checkpoint`,
`slipway learn`, `$slipway-learn`, `slipway stats`, and `$slipway-stats`
MUST be absent except for clearly historical text, if any.

#### Scenario: Generated command guidance does not teach checkpoint resume
GIVEN generated skill templates are rendered
WHEN command or wave-orchestration guidance is inspected
THEN it MUST NOT instruct agents to use checkpoint resume,
`--resume-response`, `resume_checkpoint`, or `checkpoint_type`.

### Requirement: Resume And Evidence Invariants
REQ-004: The system MUST preserve governed evidence invariants, task verdict
blockers, interrupted-wave recovery, and ledger-backed `run --resume` behavior
after checkpoint deletion.

#### Scenario: Interrupted execution can still resume
GIVEN an S2 governed change has interrupted or incomplete wave execution state
WHEN an operator runs `slipway run --resume`
THEN Slipway MUST continue from the latest incomplete wave or fail closed with
a clear existing repair/remediation reason.

#### Scenario: Blocked or incomplete task evidence is not a dead end
GIVEN a task records a `blocked` or `incomplete` verdict with blocker detail
WHEN the blocking input is resolved and the operator resumes governed execution
THEN Slipway MUST provide a viable `run --resume` or rerun path that does not
require the deleted checkpoint protocol.

#### Scenario: Safety gates remain fail closed
GIVEN execution evidence is stale, incomplete, malformed, or out of scope
WHEN `status`, `next`, `validate`, or `review` evaluates readiness
THEN the existing evidence, freshness, scope, and parallel-overlap blockers
MUST remain fail-closed.

### Requirement: Verification Coverage
REQ-005: The change MUST include focused tests and black-box checks proving the
deleted surfaces are absent and retained behavior still works.

#### Scenario: Targeted packages pass
GIVEN implementation is complete
WHEN targeted Go packages for command, model, state, wave, progression,
template, and toolgen changes are run
THEN they MUST pass before broad suite verification.

#### Scenario: Full suite and surface checks pass
GIVEN targeted tests pass
WHEN `go run ./internal/toolgen/cmd/gen-surface-manifest --check`,
black-box help checks, search checks, and `go test ./...` are run
THEN the checks MUST pass or any unrelated environmental failure MUST be
documented with evidence.
