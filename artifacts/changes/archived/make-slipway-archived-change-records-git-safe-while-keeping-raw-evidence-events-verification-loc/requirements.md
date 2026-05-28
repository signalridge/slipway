# Requirements

## ADDED Requirements

### Requirement: Git-safe governed archive records
REQ-001: When Slipway archives a governed change, the persisted archived `change.yaml` MUST be safe to commit to Git by omitting machine-local absolute workspace paths and by storing artifact paths as archive-local relative paths.

#### Scenario: Worktree archive is portable
GIVEN a worktree-bound governed change with artifact paths under an absolute worktree path
WHEN the change is archived as done or cancelled
THEN the archived `change.yaml` omits `worktree_path`
AND each archived artifact path is relative to the archived bundle
AND the archived bundle is written to the owning workspace archive directory
AND the archived record can still be loaded and listed through normal archived-change lookup.

### Requirement: Raw proof directories remain local-only
REQ-002: Slipway-managed Git ignore rules MUST keep raw runtime proof local by default for `artifacts/changes/**/evidence/`, `artifacts/changes/**/events/`, `artifacts/changes/**/verification/`, `artifacts/codebase/`, and `.worktrees/`.

#### Scenario: Top-level records remain trackable
GIVEN Slipway has ensured local-state ignore rules
WHEN Git evaluates paths inside a governed change archive
THEN top-level `change.yaml`, `intent.md`, `research.md`, `requirements.md`, `decision.md`, `tasks.md`, and `assurance.md` are not ignored
AND the raw proof subdirectories are ignored.

### Requirement: Ignore management is idempotent and entry-point owned
REQ-003: `slipway init`, `slipway new`, and `slipway codebase-map` MUST ensure the Slipway local-state `.gitignore` block before or while creating local governed state.

#### Scenario: Existing ignore content is preserved
GIVEN a repository with user-authored `.gitignore` entries
WHEN any supported Slipway entry point ensures local-state ignore rules
THEN the user-authored entries remain unchanged
AND rerunning the entry point does not duplicate the Slipway block.

### Requirement: Local proof omission does not create false integrity failures
REQ-004: Read-only learning or diagnostics MUST not treat missing archived lifecycle event logs as an integrity failure for archived changes whose raw `events/` directory is intentionally local-only.

#### Scenario: Archived record without events is tolerated
GIVEN an archived governed change has a Git-safe `change.yaml` and top-level artifacts but no `events/` directory
WHEN `slipway learn --preview` analyzes the repository
THEN the archived change remains loadable
AND missing lifecycle events for that archived change do not produce a missing-log signal.

### Requirement: Documentation reflects the Git boundary
REQ-005: User-facing runtime-file documentation MUST distinguish Git-managed governed records from local-only raw proof directories.

#### Scenario: Operator can predict what to commit
GIVEN an operator reads the README or operator guide
WHEN they inspect Slipway runtime files
THEN the docs identify top-level governed archive records as Git-manageable
AND identify `evidence/`, `events/`, `verification/`, `artifacts/codebase/`, and `.worktrees/` as local-only by default.

## MODIFIED Requirements

None.

## REMOVED Requirements

None.

## NON-GOALS

- Do not move the governed archive tree to a new top-level directory in this change.
- Do not upload, centralize, or Git-manage raw evidence, events, or verification bodies by default.
- Do not add backward-compatible schema shims for older archived `change.yaml` variants.

## DECISIONS

- DEC-001: Keep the existing `artifacts/changes/` layout but make its subdirectory ignore policy precise.
- DEC-002: Sanitize archived `change.yaml` at archive time rather than changing active change runtime authority.
- DEC-003: Treat missing archived lifecycle events as acceptable when events are local-only.

## ROLLBACK

Rollback by reverting the archive-sanitization and ignore-management changes. Existing active changes remain readable because active `change.yaml` schema is not narrowed.
