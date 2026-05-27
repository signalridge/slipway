# Requirements

## Requirements

### Requirement: Scope Contract evaluator

REQ-001: Slipway MUST define a Scope Contract evaluation over planned `tasks.md target_files` and execution `changed_files` without adding a new work-package/frontmatter authority.

Acceptance:
- The evaluator reads planned targets from the existing task plan model.
- The evaluator reads actual files from execution task summaries or task runs.
- The evaluator returns a stable pass/fail/warning report shape.

### Requirement: Scope drift detection

REQ-002: Scope Contract evaluation MUST report changed files that are outside the planned target file/glob set as scope drift.

Acceptance:
- Exact file targets match their identical changed file.
- Directory/glob targets cover nested changed files when explicitly declared.
- Files outside all targets are listed deterministically.

### Requirement: Conservative missing evidence handling

REQ-003: Missing or incomplete scope evidence MUST be visible and conservative after execution.

Acceptance:
- Missing planned targets after execution are reported as an invalid or missing contract.
- Missing changed-files evidence after execution is reported instead of silently passing.
- Pre-execution changes do not block merely because there is not yet execution evidence.

### Requirement: Governance surfacing

REQ-004: Validation/status/review readiness MUST surface Scope Contract failures as blockers after execution evidence exists.

Acceptance:
- `validate --json` includes scope drift status or blockers for an active governed change.
- Review entry or readiness refuses to pass when drift is unresolved.
- Existing blast-radius derivation remains separate from Scope Contract verdicts.

### Requirement: Contract evidence host guidance

REQ-005: Host-skill guidance MUST require contract evidence checks when relevant.

Acceptance:
- Spec-compliance/review guidance names forward/reverse contract checks.
- Goal-verification guidance requires final evidence to mention in-scope contract pass/fail when contracts exist.
- Template tests lock the wording or generated surface.

### Requirement: Authority boundaries

REQ-006: The implementation MUST preserve Slipway authority boundaries and avoid spec-kitty platform imports.

Acceptance:
- `change.yaml` remains current-state authority.
- Lifecycle event logs remain append-only trace.
- No lane scheduler, dashboard, SaaS/doctrine/orchestrator layer, or adapter expansion is introduced.

### Requirement: Worktree-local codebase map updates

REQ-007: Updateable `artifacts/codebase` output MUST prefer the active workspace/worktree instead of the main checkout when commands are invoked from a bound worktree.

Acceptance:
- `codebase-map` writes generated map files under the invocation worktree's `artifacts/codebase` directory.
- `next --json` and related context surfaces report worktree-local `artifacts/codebase` paths for a bound worktree.
- Running `codebase-map` from a worktree does not create or update root-checkout `artifacts/codebase` files.

## Non-Functional Requirements
- Keep the implementation deterministic and testable with local Go tests.
- Keep file/glob matching intentionally small and documented by tests.
- Keep changes scoped to governance evaluation, CLI surfacing, host guidance, and their tests.
