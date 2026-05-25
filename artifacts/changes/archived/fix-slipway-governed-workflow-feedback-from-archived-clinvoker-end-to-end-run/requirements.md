# Requirements

## Project Context
- Tech Stack: Go CLI governance engine
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, Markdown, YAML

## Requirements

### Requirement: Feedback Disposition Coverage
REQ-001: Every item in the archived `workflow-feedback.md` MUST have a current-state disposition: fixed by code/tests, fixed by documentation/policy, already fixed with regression evidence, or explicitly deferred with rationale.

#### Scenario: Completion audit
GIVEN the archived workflow feedback file
WHEN final verification runs
THEN each symptom has a recorded disposition and evidence path.

### Requirement: Handoff Contract Clarity
REQ-002: `next` and `run` diagnostics MUST avoid misleading host/action combinations for S0 research and S1 bundle/audit handoffs.

#### Scenario: S0 intake research
GIVEN an active change in `S0_INTAKE/research`
WHEN `next --json --diagnostics` is run
THEN `next_skill.name` remains `intake-clarification` and required actions do not imply an S0 `research.md` host artifact.

#### Scenario: S1 bundle/audit
GIVEN an active change in `S1_PLAN/bundle` with bundle artifacts present
WHEN `next --json --diagnostics` is run
THEN the response gives an actionable plan-audit handoff or an explicit bundle action contract instead of only `no_skill_required:S1_PLAN`.

### Requirement: Codebase Map Truthfulness
REQ-003: `slipway codebase-map`, stats, health, and next technique hints MUST distinguish populated codebase maps from scaffold-only placeholder files.

#### Scenario: scaffold-only map
GIVEN `slipway codebase-map` creates placeholder documents
WHEN stats or next evaluates the durable codebase map
THEN scaffold-only files are reported as incomplete/advisory and do not satisfy populated-context expectations.

### Requirement: Artifact Schema and Skill Guidance Alignment
REQ-004: Generated host guidance MUST align with runtime artifact schemas and evidence schemas.

#### Scenario: research format
GIVEN an agent follows `research-orchestration`
WHEN it writes `research.md`
THEN the required top-level headings `## Alternatives Considered`, `## Unknowns`, `## Assumptions`, and `## Canonical References` are directly included.

#### Scenario: verification references
GIVEN an agent writes verification evidence
WHEN it follows generated guidance
THEN `references` are documented as a YAML sequence of strings, not structured maps.

### Requirement: Plan Task Contract Alignment
REQ-005: The task parser and plan-audit guidance MUST accept the metadata naturally required by the plan-audit checklist without causing validation failures.

#### Scenario: evidence and acceptance metadata
GIVEN `tasks.md` includes task metadata keys `evidence` and `acceptance`
WHEN wave parsing and validation run
THEN the plan parses successfully and task semantic hashes include those fields.

#### Scenario: future lifecycle evidence
GIVEN a task acceptance criterion requires S3/S4 review or closeout evidence before S2 execution can complete
WHEN plan-audit runs
THEN guidance treats that as a blocker or requires the criterion to be moved to review/closeout.

### Requirement: Worktree, Locking, and Archive Path Friction
REQ-006: Worktree, locking, and archive behavior from the archived run MUST be fixed or documented in a way that prevents agent misinterpretation.

#### Scenario: existing worktree deadlock
GIVEN S2 worktree-preflight evidence exists for an unbound discovery change
WHEN `next` or `run` evaluates the change
THEN worktree metadata is consumed before required-action blockers deadlock the preflight path.

#### Scenario: read command locking
GIVEN a user considers parallel `status` and `next`
WHEN reading agent-facing documentation
THEN it explicitly states that state-locking commands are not safe to parallelize unless shared-read locks are later implemented.

#### Scenario: archive metadata
GIVEN a worktree-bound change is archived at the project-root archive location
WHEN archived `change.yaml` is read
THEN frozen artifact paths point at archive-local files rather than stale active worktree bundle paths.

### Requirement: Verification
REQ-007: The completed change MUST be verified through targeted tests, full Go tests with explicit timeout, build, `validate --json`, `next --json --diagnostics`, and `run --json --diagnostics`.

#### Scenario: final verification
GIVEN all implementation tasks are complete
WHEN closeout runs
THEN fresh command output proves the requirements and no required work remains.
