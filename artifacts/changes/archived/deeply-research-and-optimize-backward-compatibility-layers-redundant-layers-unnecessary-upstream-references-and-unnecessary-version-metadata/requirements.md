# Requirements

## Requirements

### Requirement: Remove the current quick-mode bypass surface.
REQ-001: `slipway next` and `slipway run` MUST no longer expose `--quick`, and progression MUST no longer support an invocation-only quick mode that disables advisory controls.

#### Scenario: Quick flag removed from command surface
GIVEN a user inspects `slipway next --help` or `slipway run --help`
WHEN command help is rendered
THEN `--quick` is absent.

#### Scenario: Progression has no quick bypass option
GIVEN governed progression advances a change
WHEN `AdvanceOptions` is constructed by command code
THEN there is no `QuickMode` option and no quick-mode disabled-control injection path.

### Requirement: Tighten task evidence parsing to the current flat JSON shape.
REQ-002: Task evidence JSON MUST require explicit flat fields for `task_id`, `run_summary_version`, `task_kind`, `verdict`, `evidence_ref`, and `captured_at`; nested `task_run` and best-effort defaulting MUST not be accepted.

#### Scenario: Strict evidence shape accepted
GIVEN task evidence contains explicit flat fields matching the expected run summary version
WHEN Slipway parses the task evidence
THEN the execution task summary is built from the explicit fields.

#### Scenario: Missing task evidence fields rejected
GIVEN task evidence omits `task_id`, `run_summary_version`, `task_kind`, `verdict`, `evidence_ref`, or `captured_at`
WHEN Slipway parses the task evidence
THEN parsing fails with an explicit validation error instead of deriving values from filename, defaults, or file metadata.

#### Scenario: Nested task_run rejected
GIVEN task evidence contains a nested `task_run` object
WHEN Slipway parses the task evidence
THEN parsing fails because only the current flat evidence shape is supported.

### Requirement: Remove unused artifact version metadata.
REQ-003: Slipway MUST remove artifact-level version metadata that has no behavioral consumer, including `ManifestVersion` template data and `ArtifactState.Version`.

#### Scenario: Artifact state serialization omits version
GIVEN a governed bundle is scaffolded or reconciled
WHEN `change.yaml` is written
THEN artifact entries include path/state/hash/timestamp data as applicable but do not write `version: 1`.

#### Scenario: Template data omits unused manifest version
GIVEN artifact templates are rendered
WHEN template data is built
THEN no unused `ManifestVersion` field is populated.

### Requirement: Clean stale external-reference and version wording without changing required release metadata.
REQ-004: Product docs and tracked archived research artifacts MUST stop depending on machine-local upstream paths or stale OpenCode flat/nested examples; release/tool schema versions and freshness-bound run versions MUST remain intact.

#### Scenario: Product docs avoid stale upstream comparison dependency
GIVEN a reader opens product design/workflow docs
WHEN they inspect design comparisons and open-question examples
THEN the docs describe Slipway-owned behavior without requiring external upstream comparison tables or obsolete OpenCode flat/nested questions.

#### Scenario: Archived research keeps intent without machine-local paths
GIVEN tracked archived research/intent artifacts mention local `ghq` upstream paths
WHEN the artifacts are cleaned
THEN the historical reference intent remains but local absolute path details are replaced with project/source labels.

### Requirement: Preserve intentional compatibility and authority boundaries.
REQ-005: The cleanup MUST preserve lifecycle-log compatibility, unbound active-change fallback, marker-gated OpenCode cleanup, narrow JSON handoff/status projections, and run-version/schema-version metadata used for freshness or persisted schema validation.

#### Scenario: Intentional compatibility remains
GIVEN existing tests cover lifecycle events, worktree fallback, generated cleanup, and run-version-bound evidence
WHEN the cleanup is implemented
THEN those tests continue to pass or are updated only to reflect the approved removal of unnecessary behavior.

### Requirement: Verify the one-pass cleanup with focused and full evidence.
REQ-006: The implementation MUST include focused regression tests for touched behavior and final full verification evidence.

#### Scenario: Verification is complete
GIVEN the cleanup is implemented
WHEN verification runs
THEN focused tests for CLI flags, task evidence, artifact metadata, docs, and archive cleanup pass, followed by `go test ./...`, `go build ./...`, `mkdocs build --strict`, and `go run . validate --json`.
