# Decision

## Project Context
- Tech Stack: Go CLI, generated Codex skills
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Keep catalog artifacts and only delete support copies
- Tradeoff: small diff and less generator churn.
- Rejected because it preserves `using-slipway-catalog.md` and
  `slipway/references/catalog/*.md` as a second apparent routing layer.

### Remove all skill indexing
- Tradeoff: eliminates ambiguity and minimizes generated files.
- Rejected because the user wants indexing retained and a workflow-owned
  reference is useful for audit/navigation.

### Selected: workflow-owned skill index with direct host paths
- Tradeoff: changes generated path contracts and tests, but removes the
  misleading catalog route layer while preserving index value.
- Selected because it matches the confirmed model: `slipway/SKILL.md` is the
  entry skill, `slipway/references/skill-index.md` is informational, and
  real `slipway-<name>/SKILL.md` files are the only procedure authority.

## Selected Approach
Remove generation of top-level `using-slipway-catalog.md`, stop emitting
`slipway/references/catalog/**`, and generate a deterministic workflow-owned
`references/skill-index.md` instead. Render index rows with direct host skill
paths for exported skills. Update workflow skill wording and runtime hint
labels to remove catalog-path terminology. Add refresh cleanup for the retired
generated paths.

## Interfaces and Data Flow
- `slipway init` still generates adapter skill artifacts through
  `internal/toolgen`.
- `capability.BuildSkillIndexWithPaths` renders the skill index and receives
  direct `SkillPath(cfg, id)` values from toolgen.
- `renderStandaloneWorkflowSkill` passes `SkillIndexPath` into the workflow
  template instead of `CatalogManifestPath`.
- `cmd/next_skill_view.go` support hints no longer expose catalog artifact
  paths; they use exported skill labels.
- No runtime lifecycle state, command syntax, or JSON field contract changes
  are intended.

## Rollout and Rollback
- Rollout: update generator, tests, and golden inventory; verify with focused
  tests, broad Go tests, and build.
- Rollback: revert the generator/test changes and rerun the same verification
  commands. No data migration is involved because only generated skill files
  and docs/tests change.

## Risk
- Stale generated files could survive refresh if cleanup does not explicitly
  remove old paths. Mitigation: add cleanup and tests.
- Non-exported capability metadata may no longer be visible in agent-facing
  skill trees. Mitigation: index only actionable exported skills; keep internal
  registry metadata unchanged.
- Generated wording could imply alternate routing authority. Mitigation:
  workflow template states that `next_skill.name` and real host skill paths are
  the handoff authority.
