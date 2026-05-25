# Requirements

## Project Context
- Tech Stack: Go CLI, generated Codex skills
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Remove agent-facing catalog entrypoints
REQ-001: Generated adapter skill output MUST NOT include a top-level
`using-slipway-catalog.md` file or a workflow reference tree rooted at
`slipway/references/catalog`.

#### Scenario: Generated tree excludes retired catalog surfaces
GIVEN `slipway init --tools codex --refresh` generates skill artifacts
WHEN the generated skill tree is inspected
THEN no `using-slipway-catalog.md` file exists
AND no file path under `slipway/references/catalog` exists.

### Requirement: Preserve workflow-owned indexing
REQ-002: Generated workflow skill output MUST preserve a compact skill index at
`slipway/references/skill-index.md`.

#### Scenario: Workflow reference index exists
GIVEN adapter skill output is generated
WHEN the workflow skill references are inspected
THEN `slipway/references/skill-index.md` exists
AND the index is informational, deterministic, and not an execution authority.

### Requirement: Route directly to exported host skills
REQ-003: The generated workflow skill and skill index MUST direct agents to
real exported host skill paths such as `slipway-security-review/SKILL.md`, not
to catalog artifact paths.

#### Scenario: Direct host handoff is documented
GIVEN a governed next step returns `next_skill.name`
WHEN the generated workflow skill and index are read
THEN they describe deriving `.codex/skills/slipway-<name>/SKILL.md`
AND do not instruct agents to follow catalog artifact paths.

### Requirement: Keep procedure support under real skills
REQ-004: Procedure, checklist, overlay, and script support files MUST be emitted
only under their corresponding exported `slipway-<name>/` skill directories
when the skill is exported.

#### Scenario: Support files are not duplicated through workflow catalog paths
GIVEN generated adapter skill output contains support files for an exported
skill
WHEN the skill tree is inspected
THEN support files live under the exported skill's own `references/` or
`scripts/` tree
AND no duplicate support files are emitted under `slipway/references/catalog`.

### Requirement: Refresh removes stale retired catalog files
REQ-005: `slipway init --refresh` MUST clean up generated stale
`using-slipway-catalog.md` and `slipway/references/catalog/**` artifacts from
previous Slipway versions.

#### Scenario: Refresh cleans old generated catalog artifacts
GIVEN an existing generated adapter tree contains the old catalog files
WHEN generation runs in refresh mode
THEN the old generated catalog files are removed
AND the new `slipway/references/skill-index.md` file is present.

### Requirement: external_api_contracts guardrail compliance
REQ-006: Tests MUST cover the external generated-skill contract change so the
old catalog route layer cannot be reintroduced accidentally.

#### Scenario: Contract tests protect new layout
GIVEN the focused test suites run
WHEN capability export, toolgen, and next-skill hint tests execute
THEN they assert the new index/direct-handoff layout and reject old catalog
paths.
