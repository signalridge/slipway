# Requirements

## Requirements

### Requirement: Cross-worktree handoff is informational
REQ-001: The generated SessionStart hook MUST render `change_bound_to_other_worktree` from its read-only `slipway next --json` handoff as informational context rather than a failed hook diagnostic.

#### Scenario: Active change bound elsewhere
GIVEN a SessionStart hook runs from a worktree with no locally bound active governed change
AND `slipway next --json` exits non-zero with `error_code` equal to `change_bound_to_other_worktree`
WHEN the hook renders its handoff payload
THEN the output contains an informational line naming the other active change and its bound worktree
AND the output does not contain `hook_diagnostic: slipway next --json failed:`.

### Requirement: Explicit lifecycle commands remain fail-closed
REQ-002: Explicit `slipway next` and `slipway run` invocations from the wrong worktree SHALL continue to fail closed with `change_bound_to_other_worktree` and exit 3 semantics.

#### Scenario: Wrong-worktree command invocation
GIVEN an active governed change is bound to another worktree
WHEN an explicit lifecycle command resolves the active change from the current worktree without `--change`
THEN the command fails with `change_bound_to_other_worktree`
AND the remediation includes the change slug, bound worktree, and `--change` guidance.

### Requirement: Real hook failures remain diagnostics
REQ-003: The generated SessionStart hook MUST continue to surface genuine failures as `hook_diagnostic` lines.

#### Scenario: Broken next contract
GIVEN `slipway next --json` fails with an unrelated error
WHEN the SessionStart hook renders its payload
THEN the output includes `hook_diagnostic: slipway next --json failed:`.

#### Scenario: Root resolution failure
GIVEN `slipway root` fails
WHEN the SessionStart hook renders its payload
THEN the output includes `hook_diagnostic: slipway root failed:`.

### Requirement: Generated host parity
REQ-004: The behavior SHALL be implemented through the shared generated hook template so claude, cursor, gemini, and opencode receive identical SessionStart handoff semantics.

#### Scenario: Shared template generation
GIVEN the tool generation registry maps host SessionStart hooks to the shared template
WHEN the hook template is rendered
THEN the cross-worktree informational classifier is present in the rendered template
AND no per-host generated hook copy is required to implement the behavior.
