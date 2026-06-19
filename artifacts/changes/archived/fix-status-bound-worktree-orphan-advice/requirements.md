# Requirements

## Requirements

### Requirement: Prefer Current Bound Worktree Status
REQ-001: The system MUST make unscoped `slipway status --json` invoked from an
active change's bound git worktree report that active change instead of routing
the same slug to `orphaned_change_bundle` delete recovery when a stale root
bundle directory without `change.yaml` also exists.

#### Scenario: Same slug has valid bound authority and stale root residue
GIVEN an active governed change is bound to the current git worktree
AND that worktree contains the authoritative `artifacts/changes/<slug>/change.yaml`
AND the canonical root contains `artifacts/changes/<slug>/` without `change.yaml`
WHEN the operator runs `slipway status --json` without `--change` from inside the
bound worktree
THEN the response reports the active governed change with its slug
AND the response does not present `orphaned_change_bundle` or
`slipway delete --change <slug>` as recovery for that active slug.

### Requirement: Preserve Legitimate Delete Recovery
REQ-002: The system MUST preserve existing status delete-recovery behavior for
genuinely abandoned or partially deleted bundles when the current invocation is
not inside a valid active bound worktree for that slug.

#### Scenario: Partial delete still routes to delete recovery
GIVEN a governed bundle has been partially deleted so its active bundle directory
still contains files but no `change.yaml`
AND no valid active bound worktree authority is selected by the current
invocation
WHEN the operator runs `slipway status --json`
THEN the response remains diagnostics-mode
AND the response includes delete recovery for the orphaned bundle.
