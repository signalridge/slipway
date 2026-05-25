# Requirements

## Project Context
- Tech Stack: Go CLI
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Early canonical worktree binding
REQ-001: Discovery-required changes created in a Git repository with a usable
HEAD MUST bind the default repo-local worktree before governed bundle artifacts
are scaffolded.

#### Scenario: New discovery change binds before intent artifact
GIVEN `slipway new --json` creates a discovery-required change in a Git repo
WHEN the command persists the change and creates `intent.md`
THEN the change records `.worktrees/<slug>` and `feat/<slug>` metadata before
the intent artifact is written, and JSON/text output reports the binding.

### Requirement: Deterministic codebase-map baseline population
REQ-002: `slipway codebase-map` MUST populate missing or scaffold-only
`artifacts/codebase/*.md` files with deterministic repository facts rather than
treating placeholders as complete context.

#### Scenario: Missing codebase map becomes populated
GIVEN no durable codebase map exists
WHEN `slipway codebase-map --json` runs
THEN all canonical docs are reported as `populated` and downstream stats/health
do not see scaffold-only placeholder docs.

### Requirement: Remediation archive relationship
REQ-003: A change that remediates feedback from an archived bundle MUST persist
and report explicit source archive relationships when finalized.

#### Scenario: Done reports remediation archive source
GIVEN a governed change references `artifacts/changes/archived/<source-slug>`
WHEN `slipway done --json` archives the remediation change
THEN output includes `archive_kind=remediation`, `archive_path`, and
`remediation_sources`, and archived `change.yaml` persists the same source.

### Requirement: Targeted stale-evidence classification
REQ-004: Post-execution freshness checks MUST distinguish planning drift,
execution drift, and assurance-only verification edits.

#### Scenario: Planning edits do not masquerade as execution drift
GIVEN execution evidence exists for a change
WHEN planning artifacts change after evidence capture
THEN validation/review blockers report `stale_planning_evidence`.

#### Scenario: Assurance-only edits stay out of execution freshness
GIVEN execution evidence exists for a change
WHEN only `assurance.md` changes after evidence capture
THEN execution evidence remains fresh and closeout/ship checks own the
assurance validation path.

### Requirement: Thin root Slipway catalog artifacts
REQ-005: Root `slipway` catalog artifacts MUST stay metadata/dispatch records
and MUST NOT duplicate full dedicated skill procedure text.

#### Scenario: Generated catalog artifact points at instruction authority
GIVEN tool artifacts are generated with refresh mode
WHEN catalog artifacts are emitted under `.codex` or `.claude`
THEN they contain catalog metadata and `## Instruction Authority`, and they do
not contain `## Full Instructions`.

### Requirement: Archived feedback disposition closure
REQ-006: The target `workflow-feedback.md` MUST record explicit dispositions and
evidence pointers for every currently actionable item.

#### Scenario: Feedback table has no unresolved actionable items
GIVEN the remediation implementation and verification are complete
WHEN the feedback file is reviewed
THEN every actionable item is marked fixed with code, test, or governance
evidence, and no non-deferred item lacks an evidence pointer.

### Requirement: Full governed workflow verification
REQ-007: This remediation change MUST progress through Slipway governed gates to
done-ready/done with required evidence files present.

#### Scenario: Lifecycle evidence is complete
GIVEN implementation and verification are complete
WHEN `slipway run`, `slipway status`, and `slipway done` are used
THEN the change reaches terminal archive state without bypassing governed
controls.
