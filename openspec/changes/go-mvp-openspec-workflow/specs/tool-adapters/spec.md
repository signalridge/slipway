## ADDED Requirements

### Requirement: Four tool targets
The system SHALL support generating skill and command files for four tools: Claude Code (claude), Cursor (cursor), Codex (codex), OpenCode (opencode). Each tool SHALL have a ToolConfig defining: skills_dir, commands_dir, trigger_prefix, trigger_style, auto_detect_paths.

#### Scenario: Tool registry
- **WHEN** the tool registry is queried
- **THEN** it SHALL contain exactly four entries: claude, cursor, codex, opencode

### Requirement: Core Workflow Independence from Tool Generation
Tool adapter generation is an auxiliary capability and SHALL NOT be a runtime dependency for core workflow execution.

- core lifecycle commands (`new/do/status/context/done/cancel/pivot/repair/analyze/review`) SHALL run correctly when no tool artifacts are generated
- `spln init --tools none` is a valid baseline bootstrap path for core governance workflow
- adapter-generation failures SHALL be reported in adapter context and SHALL NOT corrupt runtime state layout

#### Scenario: Core workflow runs without tool artifacts
- **WHEN** repository is initialized with `spln init --tools none`
- **THEN** core workflow commands SHALL remain fully functional without generated skill/command files

### Requirement: Trigger style mapping
Each tool SHALL use its defined trigger style: claude uses slash-colon (`/spln:`), cursor uses slash-hyphen (`/spln-`), codex uses dollar-mention (`$spln-`), opencode uses slash-hyphen (`/spln-`). Command meaning SHALL NOT change by tool.

#### Scenario: Claude trigger format
- **WHEN** a command file is generated for claude tool and command "new"
- **THEN** the trigger SHALL be `/spln:new`

#### Scenario: Codex trigger format
- **WHEN** a command file is generated for codex tool and command "new"
- **THEN** the trigger SHALL be `$spln-new`

### Requirement: Command skill file generation
The system SHALL generate one command skill file per CLI command per tool. Command skill IDs are: init, new, do, status, context, done, cancel, pivot, repair, analyze, review. Skill files SHALL follow the layout `<skills_dir>/spln-<skill-name>/SKILL.md`.

#### Scenario: Skill file path for claude
- **WHEN** skill "do" is generated for claude
- **THEN** the command skill file SHALL be at `.claude/skills/spln-do/SKILL.md`

### Requirement: Governance contract skill generation
The system SHALL generate governance contract skills per tool aligned to canonical states:
- intake-analysis
- scope-confirmation
- plan-audit
- wave-orchestration
- artifact-review
- goal-verification
- final-closeout

Governance contract skill files SHALL follow `<skills_dir>/spln-<skill-name>/SKILL.md` and SHALL include: execution protocol, step invariants, evidence contract, failure loop rules, context budget section, and embedded subagent role instructions where applicable.

#### Scenario: Governance contract skill path for claude
- **WHEN** governance contract skill "plan-audit" is generated for claude
- **THEN** the file SHALL be at `.claude/skills/spln-plan-audit/SKILL.md`

### Requirement: Helper skill trigger metadata
Generated skill files SHALL include trigger metadata for helper workflows:
- discovery/discussion trigger hints sourced from Superpowers + GSD + OPSX contracts
- worktree isolation trigger hints sourced from Superpowers + execution workflows
- review trigger hints sourced from review commands/skills
- pre-completion verification trigger hints sourced from verify/archive command contracts

This metadata SHALL be descriptive only and SHALL NOT change runtime gate decisions.

#### Scenario: Helper trigger metadata present
- **WHEN** `spln-do` command skill is generated
- **THEN** file content SHALL include source-grounded helper trigger guidance for discovery/worktree/review/verification usage

### Requirement: Command file generation
The system SHALL generate one command file per command per tool. Command files SHALL be < 50 lines, route to the corresponding command skill, and contain: invocation syntax, argument handoff, "when to invoke", handoff target. Command files SHALL NOT contain execution policy or governance logic.

#### Scenario: Command file for cursor
- **WHEN** command "new" is generated for cursor
- **THEN** the file SHALL be at `.cursor/commands/spln-new.md` and SHALL route to the `spln-new` command skill

### Requirement: Technique skill generation
The system SHALL generate technique skill files alongside command skills and governance contract skills. Three technique skills: spln-tdd, spln-systematic-debugging, spln-code-review-protocol. Each SHALL have `type: technique` frontmatter, < 500 words, anti-rationalization blocks, and CSO-optimized descriptions.

#### Scenario: Technique skill generation
- **WHEN** `spln init --tools claude` is run
- **THEN** technique skills SHALL be generated at `.claude/skills/spln-tdd/SKILL.md`, `.claude/skills/spln-systematic-debugging/SKILL.md`, `.claude/skills/spln-code-review-protocol/SKILL.md`

### Requirement: Deterministic output
Tool adapter generation SHALL be deterministic: same command ID + same tool config SHALL produce byte-identical output across runs. No timestamps, random values, or non-deterministic content in generated files.

#### Scenario: Byte-stable generation
- **WHEN** the same skill is generated twice with the same configuration
- **THEN** the output SHALL be byte-identical

### Requirement: Command-skill boundary
Command files SHALL route; skill files SHALL govern. Command files SHALL NOT duplicate skill policy tables. Command skills SHALL NOT redefine command syntax per tool. Governance pass/fail decisions SHALL always be produced by CLI/runtime state, never by command/skill text alone.

#### Scenario: Command does not contain policy
- **WHEN** a command file is generated
- **THEN** it SHALL NOT contain review layer definitions, gate logic, or evidence contracts

### Requirement: Init with tools flag
`spln init --tools <tool-list>` SHALL generate all skill and command files for the specified tools. `--tools all` generates for all 4 tools. `--tools none` generates no tool files. `spln init --refresh` SHALL regenerate all tool files.

#### Scenario: Init with multiple tools
- **WHEN** `spln init --tools claude,cursor` is run
- **THEN** skill and command files SHALL be generated for both claude and cursor tools
