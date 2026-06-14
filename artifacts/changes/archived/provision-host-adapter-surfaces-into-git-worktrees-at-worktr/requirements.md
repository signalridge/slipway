# Requirements

## Requirements

### Requirement: Provision host-adapter surfaces on worktree creation
REQ-001: When Slipway creates a new git worktree for a governed change, the system MUST provision every host-adapter surface that exists in the repository root into the new worktree, so that an agent (including an isolated subagent) working in that worktree has the same host-adapter skills, hooks, and settings as the main worktree.

#### Scenario: New worktree receives copied third-party skills and regenerated slipway surfaces
GIVEN the repository root contains `.claude/` with both third-party skill directories and `slipway-*` skill directories
WHEN Slipway creates a new worktree for a change
THEN the new worktree contains `.claude/skills/` with the third-party skill directories copied over AND the `slipway-*` skill directories present
AND the new worktree contains the `.claude` hooks and settings surfaces.

### Requirement: Generalize across all toolgen adapters
REQ-002: The provisioning MUST cover every toolgen adapter directory present in the repository root among `.claude`, `.cursor`, `.codex`, `.opencode`, and `.gemini`, REQUIRED to be driven by the toolgen adapter registry / detection rather than a single hardcoded `.claude` path.

#### Scenario: All detected adapters are provisioned
GIVEN the repository root contains `.claude/`, `.codex/`, and `.gemini/` adapter directories
WHEN Slipway creates a new worktree
THEN each of `.claude/`, `.codex/`, and `.gemini/` is provisioned into the worktree
AND an adapter directory absent from the repository root is not created in the worktree.

### Requirement: Exclude non-portable and self-referential paths
REQ-003: The copy step MUST exclude paths that would cause recursion, lock contention, or carry stale generation state: each `<tool>/worktrees/` subtree (it holds nested git worktrees) and lock files (e.g. `scheduled_tasks.lock`). The copy MUST NOT carry the source's generated `.adapter-generated` sentinel; Slipway regenerates a fresh sentinel via `toolgen.Generate`. The `.serena/` directory MUST NOT be provisioned because it is a self-reindexing MCP cache.

#### Scenario: Worktrees subtree and locks are not present in the provisioned worktree
GIVEN the repository root `.claude/` contains a `worktrees/` subtree and a `scheduled_tasks.lock` file
WHEN Slipway provisions the adapter into a new worktree
THEN the provisioned worktree's `.claude/` does not contain the `worktrees/` subtree or the lock file
AND the worktree contains no `.serena/` directory produced by provisioning.

### Requirement: Idempotent reuse provisioning without clobbering local edits
REQ-004: When `EnsureDefaultWorktreeForChange` binds a change onto a worktree path that is already a registered git worktree (the reuse branch), the system MUST provision idempotently: copy only the third-party adapter content that is missing at the destination, MUST NOT overwrite worktree-local third-party or manual edits, and MUST always regenerate the `slipway-*` surfaces. (A one-shot sweep that eagerly re-provisions all pre-existing worktrees is out of scope; reuse provisioning covers a change re-created onto an existing worktree.)

#### Scenario: Reuse backfills a missing surface but preserves a local edit
GIVEN a registered worktree is missing its `.claude/` adapter surface
AND it already contains a worktree-local edit inside one third-party skill file
WHEN `EnsureDefaultWorktreeForChange` binds a change onto that worktree (reuse branch)
THEN the worktree gains the missing `.claude/` third-party adapter content
AND the pre-existing worktree-local third-party edit is preserved unchanged
AND the worktree has freshly regenerated `slipway-*` surfaces.

### Requirement: Fail-closed on provisioning failure
REQ-005: If copying an adapter directory or regenerating slipway-owned surfaces fails during provisioning, the system MUST fail closed: worktree provisioning returns an error with actionable remediation, and MUST NOT leave a silently degraded worktree bound as if it were fully provisioned.

#### Scenario: Provisioning failure aborts with an error
GIVEN adapter provisioning will fail (e.g. the copy or regeneration step errors)
WHEN Slipway attempts to provision a worktree
THEN the provisioning operation returns a non-nil error describing the failure
AND the failure is surfaced to the caller rather than swallowed.

### Requirement: slipway-* surfaces reflect the worktree source
REQ-006: The provisioned `slipway-*` surfaces MUST be regenerated from the worktree's own source via `toolgen.Generate(..., refresh=true)`, so that a worktree whose source differs from main carries its own `slipway-*` surfaces rather than a stale copy of the main worktree's generated output.

#### Scenario: Regenerated slipway surface reflects worktree source
GIVEN a worktree whose toolgen skill source differs from the main worktree
WHEN Slipway provisions that worktree
THEN the worktree's `slipway-*` surfaces reflect the worktree's own source content
AND not a verbatim copy of the main worktree's generated output.

### Requirement: Provisioning respects the authority/surface dependency direction
REQ-007: The provisioning implementation MUST live in the surface-renderer layer (`internal/toolgen`) and be injected into the authority layer (`internal/state`) as a function value, so the authority package never imports a surface renderer. This preserves the enforced dependency direction (`internal/architecture`) that forbids `internal/state` and `internal/model` from importing `cmd`, `internal/tmpl`, or `internal/toolgen`. The composition root (`cmd`) wires the concrete provisioner into the worktree-binding call.

#### Scenario: Authority package does not import the surface renderer
GIVEN the worktree-binding logic in `internal/state` invokes host-surface provisioning
WHEN the architecture dependency-direction test inspects `internal/state` imports
THEN `internal/state` does not import `internal/toolgen`
AND the provisioner is supplied to the binding call by the composition root.
