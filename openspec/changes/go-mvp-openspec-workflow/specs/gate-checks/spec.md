## ADDED Requirements

### Requirement: Terminology Boundary
MVP SHALL use `check` terminology for gate decisions.

- `check` means a concrete gate input item
- checks are not tool "skills" and SHALL NOT use `skill_name` naming
- gate outcomes SHALL NOT depend on `evidence/skills` style files

#### Scenario: Skill term is not used for gate input
- **WHEN** gate input metadata is persisted
- **THEN** fields SHALL use `check_id` terminology rather than `skill_name`

### Requirement: Check Types (MVP)
MVP gate checks SHALL have only two types:
- `command_check`: command + exit code + output snippet
- `human_confirmation`: explicit `y|n` decision prompt

Optional advisory notes MAY exist, but SHALL NOT become a third blocking type.

Persistence contract:
- `command_check` results are stored in run-record `checks[]`
- `human_confirmation` results are stored in run-record `human_confirmations[]`

#### Scenario: Command check shape
- **WHEN** a command check runs
- **THEN** result SHALL include `check_id`, `command`, `exit_code`, `pass`

### Requirement: Request-Scoped Run Record Storage
All check results SHALL be persisted in request-scoped run record:
- `.speclane/runs/<request_id>.yaml`

MVP SHALL NOT require:
- policy registry
- policy snapshot
- multi-layer override policy objects
- separate evidence directories

#### Scenario: Check persistence path
- **WHEN** any gate check finishes
- **THEN** check result SHALL be appended/updated in `.speclane/runs/<request_id>.yaml`

### Requirement: Gate-Check Catalog (Built-in Baseline)
System SHALL provide stable built-in check IDs for MVP gate evaluation of `G_scope`, `G_plan`, and `G_ship`.

Built-in baseline checks are:

- `scope_sections_valid` (`command_check`)
- `worktree_authentic` (`command_check`)
- `scope_confirmed` (`human_confirmation`)
- `plan_artifacts_ready` (`command_check`)
- `openspec_validate_pass` (`command_check`)
- `execute_ready` (`human_confirmation`)
- `tests_pass` (`command_check`)
- `lint_pass` (`command_check`)
- `tasks_all_checked` (`command_check`)
- `review_done` (`human_confirmation`)
- `ship_ready` (`human_confirmation`)

`G_pivot` is rule-based in MVP and SHALL NOT require catalog check IDs.

#### Scenario: Built-in check IDs are stable
- **WHEN** status/context renders gate checks
- **THEN** built-in check IDs SHALL remain stable across runs

### Requirement: Command-Check Mapping Contract
Each built-in command check SHALL map to one canonical command/probe descriptor.

MVP mapping:
- `scope_sections_valid` -> `builtin:scope_sections_valid` (required explore heading/content validator)
- `worktree_authentic` -> `builtin:worktree_authentic` (path/worktree/branch authenticity probe)
- `tests_pass` -> `go test ./...` (applies when code delta exists)
- `lint_pass` -> `golangci-lint run` (applies when code delta exists)
- `tasks_all_checked` -> `grep -n "^- \\[ \\]" tasks.md` (pass when no matches)
- `openspec_validate_pass` -> `openspec validate <change>`
- `plan_artifacts_ready` -> `builtin:plan_artifacts_ready` (deterministic engine probe)

#### Scenario: Command check descriptor resolves by check ID
- **WHEN** runtime evaluates a built-in command check
- **THEN** it SHALL resolve command/probe descriptor from this mapping by `check_id`

### Requirement: Gate-to-Check Mapping Contract
Gate engine SHALL resolve gate requirements from this mapping contract (single source of truth).

MVP gate mapping:
- `G_scope` -> command checks: `scope_sections_valid`, `worktree_authentic`; human confirmation: `scope_confirmed`
- `G_plan` -> command checks: `plan_artifacts_ready`, `openspec_validate_pass`; human confirmation: `execute_ready`
- `G_ship` -> command checks: `tests_pass`, `lint_pass`, `tasks_all_checked`; human confirmations: `review_done`, `ship_ready`
- `G_pivot` -> rule gate (no catalog check IDs in MVP)

#### Scenario: Gate engine consumes mapping contract
- **WHEN** a gate is evaluated
- **THEN** required check IDs SHALL be loaded from this mapping contract rather than duplicated in gate-engine definitions

### Requirement: Failure Handling Is User-Driven
Failure handling SHALL default to block and MAY continue only with explicit user override.

When a required `command_check` fails:
- gate status becomes `blocked`
- CLI SHALL show failing check results
- operator MAY choose explicit override confirmation to continue

Override in MVP is interactive and minimal:
- no role model
- no dual approval
- no policy object
- only explicit user confirmation + optional reason note in run record

#### Scenario: User overrides failed command check
- **WHEN** required command check fails and operator confirms override
- **THEN** workflow MAY continue and run record SHALL persist override trace on the overridden command check (`override=true`, optional `override_note`, `override_at`)

### Requirement: Confirmation Prompts Are Explicit
Human confirmations SHALL use explicit prompt text and persist prompt+answer.

Prompt language contract:
- canonical prompt templates in spec SHALL be English
- runtime/AI layer MAY localize prompts to user language
- localization SHALL NOT change check identity (`check_id`) or decision semantics

Baseline prompts:
- `scope_confirmed`: `Is scope confirmed? [y/n]`
- `execute_ready`: `Is execution ready? [y/n]`
- `review_done`: `Is review complete? [y/n]`
- `ship_ready`: `Ready to ship? [y/n]`

#### Scenario: Prompt text in run record
- **WHEN** confirmation is collected
- **THEN** run record SHALL contain prompt text and answer for traceability
