# Requirements

## Project Context
- Tech Stack: Go CLI
- Conventions: command behavior under `cmd/`, state/worktree authority under
  `internal/state/`, command regressions in `cmd/*_test.go`
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: Unbound intake change must not globally block unrelated discovery work
REQ-001: When an existing active change is unbound because it is still an
intake-stage/non-discovery root-owned change, `slipway new` MUST allow a new
discovery-scoped change that will be isolated into its own default worktree.

#### Scenario: root-owned unbound intake plus discovery follow-up
GIVEN an active governed change with empty `WorktreePath` owns the current root
AND the follow-up `slipway new` classification has `needs_discovery=true`
WHEN the follow-up change is created
THEN the create guard MUST not fail on the existing root-owned unbound change
AND the follow-up change MUST bind to a separate default worktree.

### Requirement: Same-workspace unbound collision remains fail-closed
REQ-002: When an existing active unbound change and a requested new change would
both own the same workspace, `slipway new` MUST fail with
`error_code=active_change_exists`.

#### Scenario: root-owned unbound intake plus root-owned follow-up
GIVEN an active unbound governed change owns the current workspace
AND the follow-up `slipway new` classification has `needs_discovery=false`
WHEN the follow-up change is created
THEN the command MUST fail before creating state
AND remediation MUST NOT say `slipway next` will bind the existing change.

### Requirement: Bound sibling worktree must not globally block unrelated new work
REQ-003: When an existing active change is already bound to a different sibling
worktree, `slipway new` from the repo root or another non-colliding workspace
MUST allow a new change whose target workspace is different.

#### Scenario: hidden sibling bound authority plus root discovery follow-up
GIVEN an active governed change is bound to a sibling worktree
AND normal workspace markers may make that sibling hidden from root-scoped reads
WHEN a root-scoped discovery follow-up is created
THEN hidden-authority discovery MAY still see the sibling
BUT the create guard MUST not reject unless the new target workspace collides.

### Requirement: Create guard must compare current/prospective workspace authority
REQ-004: The `slipway new` create guard MUST evaluate active-change conflicts
against the current invocation workspace and the prospective target workspace
for the new change, not against every active change repo-wide.

#### Scenario: prospective target is known before worktree creation
GIVEN `slipway new` has resolved classification and generated a slug
WHEN it checks active-change conflicts
THEN it MUST compute whether the new change would bind to the default worktree
or remain in the current workspace
AND reject only active changes that own the current bound worktree or the
prospective target workspace.

### Requirement: Regression and lifecycle evidence must prove the issue fixes
REQ-005: The change MUST include automated regression coverage for #48, #50,
same-workspace collision preservation, and governed lifecycle artifacts/evidence
through Slipway closeout.

#### Scenario: verification
GIVEN the implementation is complete
WHEN targeted and full verification commands are run
THEN the #48/#50 regressions, existing same-workspace protection, and Slipway
validation/closeout gates MUST pass with fresh evidence.
