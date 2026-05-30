# Requirements
## Project Context
- Tech Stack: Go
- Conventions: Cobra CLI; cmd/ surfaces, internal/state durable state, internal/engine workflow logic; --json surfaces are consumed contracts
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Language-agnostic codebase-map fact detection
REQ-001: `slipway codebase-map` MUST detect repository facts (languages, build/test
commands, key dependencies) from the actual repository via manifest files
(`go.mod`, `Cargo.toml`, `package.json`/`tsconfig.json`,
`pyproject.toml`/`setup.py`/`requirements.txt`, `pom.xml`, `build.gradle[.kts]`,
`Gemfile`, `composer.json`, `*.csproj`/`*.sln`) and a bounded source-file
extension scan, and MUST NOT assume Go or any single stack.

#### Scenario: Rust/Cargo repository
GIVEN a repository whose only manifest is a `Cargo.toml`
WHEN `slipway codebase-map --json` runs
THEN STACK lists `Rust` and cargo build/test commands
AND it never reports `Languages: Go` or `go build ./...`.

#### Scenario: No recognizable manifest
GIVEN a repository with no recognizable language manifest or source files
WHEN `slipway codebase-map --json` runs
THEN no language is fabricated (no `Languages: Go`)
AND the docs equal the blank scaffold templates.

### Requirement: No fabricated semantic prose in baseline docs
REQ-002: Baseline generation MUST populate STACK/STRUCTURE/TESTING only with
detected facts (undetected fields remain blank scaffold lines; zero detections
yield the blank template), and MUST emit INTEGRATIONS/ARCHITECTURE/CONVENTIONS/
CONCERNS as blank scaffold. The CLI MUST NOT emit hardcoded Slipway/Go prose
(for example `cmd/ owns CLI surfaces`, `internal/state`) for arbitrary
repositories.

#### Scenario: Non-Slipway repository
GIVEN any repository that is not Slipway
WHEN `slipway codebase-map` generates ARCHITECTURE.md / STRUCTURE.md
THEN they contain no Slipway-specific `cmd/`/`internal/` prose.

### Requirement: Refresh legacy buggy generated codebase maps
REQ-003: `slipway codebase-map` MUST recognize and refresh the old deterministic
Go/Slipway baseline documents that prior Slipway versions generated for
non-Go/non-Slipway repositories. This is a targeted migration of Slipway-authored
buggy baseline content only; hand-authored substantive analysis MUST NOT be
overwritten merely because repository facts later changed.

#### Scenario: Rust repository already polluted by old Go/Slipway docs
GIVEN a Rust/Cargo repository whose `artifacts/codebase` files already contain
old Slipway-generated phrases such as `Languages: Go`, `go build ./...`, and
`cmd/ owns CLI surfaces`
WHEN `slipway codebase-map --json` runs again
THEN those old generated phrases are removed or replaced by the current
Rust/scaffold baseline
AND the command does not report the old generated content as authored
`populated` analysis.

#### Scenario: User-authored map is preserved
GIVEN a repository whose codebase-map docs contain project-specific authored
analysis that does not match the old deterministic Go/Slipway baseline shape
WHEN `slipway codebase-map --json` runs
THEN that authored content is preserved.

### Requirement: Distinguish CLI baseline from authored content
REQ-004: The `codebase-map` assessment MUST classify a doc whose content equals
the freshly regenerated CLI baseline (and is non-blank) as a new `baseline`
status rather than `populated`, expose an aggregate `baseline` status and a
`baseline_docs` list on the `--json` and text surfaces.

#### Scenario: Freshly mapped repository
GIVEN a repository freshly processed by `slipway codebase-map`
WHEN `slipway codebase-map --json` reports status
THEN the aggregate status is `baseline` (not `populated`)
AND CLI-detected docs appear under `baseline_docs`.

### Requirement: Self-explanatory from-root active-change diagnostic
REQ-005: When `slipway next`/`run` are invoked from a directory matching no bound
worktree while >=1 active change is bound to another worktree, the CLI MUST
return a self-explanatory error that names the change slug(s) and bound worktree
path(s) with remediation (use `--change <slug>` or cd into the worktree),
instead of the misleading `no_active_change`.

#### Scenario: Bare invocation from the repo root
GIVEN an active change bound to `.worktrees/<slug>`
WHEN `slipway next --json` runs from the repo root
THEN the error code is `change_bound_to_other_worktree`
AND it names the slug and the bound worktree path with remediation.

### Requirement: --change resolves the bound worktree from any directory
REQ-006: `slipway next`/`run --change <slug>` MUST resolve and operate against the
change's bound worktree (located via git's worktree registry + `change.yaml`
`WorktreePath`) from any directory, including the repo root. No new config field
or hand-maintained registry is introduced.

#### Scenario: --change from the repo root
GIVEN an active change bound to `.worktrees/<slug>`
WHEN `slipway next --json --change <slug>` runs from the repo root
THEN it succeeds and the `input_context.workspace_root` points to the worktree.

### Requirement: RED tests precede guarded production changes
REQ-007: Because this change is in the `external_api_contracts` guardrail domain,
contract tests for changed behavior MUST be authored and captured as RED evidence
before production code changes. Same-commit test+implementation evidence is not
sufficient.

#### Scenario: codebase-map implementation starts
GIVEN codebase-map production files are about to be changed
WHEN wave execution begins
THEN failing tests for Rust baseline generation, legacy Go/Slipway map refresh,
no-manifest scaffold behavior, and `baseline` status already exist and fail
against the old implementation.

#### Scenario: active-change diagnostic implementation starts
GIVEN active-change resolution production files are about to be changed
WHEN wave execution begins
THEN failing tests for `change_bound_to_other_worktree` and root `--change`
resolution already exist and fail against the old implementation.

#### Scenario: archived explicit validate diagnostic implementation starts
GIVEN explicit change selector diagnostics are about to be changed
WHEN implementation begins
THEN a failing test already proves `validate --change <archived-slug>` returns
a concrete archived-change diagnostic instead of an empty-slug no-active view.

### Requirement: Automated coverage and green build
REQ-008: Automated tests MUST cover multi-language detection (Rust/Node/Python/
no-manifest), legacy generated Go/Slipway map refresh, the `baseline` status, the
from-root diagnostic, `--change` resolution from root, and stale empty active
bundle handling after archive, and explicit archived-slug validation diagnostics;
existing codebase-map command tests MUST be updated to the new contract; `go build ./...` and
`go test ./...` pass.

#### Scenario: Verification suite
GIVEN the implementation is complete
WHEN `go build ./...` and `go test ./...` run
THEN both succeed and the new/updated tests pass.

### Requirement: Documentation updated for the baseline contract
REQ-009: Project documentation and agent-facing skill guidance MUST describe the
new `baseline` status so AI host callers interpret CLI-detected facts as a
starting point awaiting authored verification, not as completed brownfield
analysis.

#### Scenario: Agent guidance describes baseline
GIVEN the new `baseline` status exists
WHEN a reader consults CLAUDE.md, command docs, codebase-map reference guidance,
or the codebase-mapping skill template
THEN `baseline` and its intended trust level are documented.

### Requirement: external_api_contracts guardrail compliance
REQ-010: The change MUST preserve the governed external-contract discipline:
explicit contract tests, TDD evidence for guarded production changes, domain
review after implementation, independent review before closeout, and rollback by
reverting the code change.

#### Scenario: Guardrail closeout
GIVEN implementation and tests are complete
WHEN review/verification runs
THEN TDD evidence, domain review evidence, independent review evidence, and full
verification evidence are present before closeout.

### Requirement: Ignore safe stale empty active bundles after archive
REQ-011: After a successful `slipway done` archives a worktree-bound change, any
stale root-checkout active bundle directory that lacks `change.yaml` and contains
only empty subdirectories MUST NOT make `slipway status`, `next`, or active-change
discovery fail with `change bundle missing authority file`.

#### Scenario: Empty root residue after worktree archive
GIVEN a worktree-bound change has been archived
AND the root checkout still has `artifacts/changes/<slug>/verification/` with no files
WHEN active-change discovery or status runs
THEN the empty stale directory is ignored rather than reported as authoritative
state corruption.

### Requirement: Repair safely removes empty orphan bundle directories
REQ-012: `slipway repair` MUST safely remove orphan active bundle directories
that contain no files, while preserving and reporting non-empty orphan bundle
directories as operator-reviewed integrity findings.

#### Scenario: Repair removes empty stale root bundle
GIVEN `artifacts/changes/<slug>/verification/` exists without files or `change.yaml`
WHEN `slipway repair --json` runs
THEN the empty stale bundle directory is removed and reported as an applied repair.

#### Scenario: Repair preserves non-empty orphan bundle
GIVEN an orphan active bundle directory contains any file
WHEN `slipway repair --json` runs
THEN the directory is preserved and reported as requiring operator intervention.

### Requirement: Archived explicit validate selector is self-explanatory
REQ-013: `slipway validate --change <slug>` MUST NOT return the generic empty
no-active diagnostic when `<slug>` names an archived/done change. It MUST either
validate the archived change or fail with a concrete diagnostic that names the
slug, terminal status, archived authority path, and supported next action. This
change selects the fail-explicitly behavior because validate currently operates
on active governance state.

#### Scenario: Explicit archived slug
GIVEN an archived change exists at `artifacts/changes/archived/<slug>/change.yaml`
WHEN `slipway validate --json --change <slug>` runs
THEN the error code is `archived_change_not_validatable`
AND the error details include `slug`, `status`, `archived: true`, and
`archive_path`
AND the command does not emit the empty-slug `no active change or ambiguous`
diagnostic.
