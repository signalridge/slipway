# Decision

## Alternatives Considered

### Approach A: Documentation and archive-only cleanup
- Removes stale docs examples and local upstream path references.
- Leaves runtime quick mode, task evidence leniency, and artifact version metadata in place.
- Rejected because it does not address the clearest unnecessary runtime layers.

### Approach B: Bounded one-pass cleanup
- Removes the current `--quick` surface and quick-mode progression option.
- Tightens task evidence parsing to the current flat JSON shape.
- Removes unused `ManifestVersion` and `ArtifactState.Version` metadata.
- Cleans docs and tracked archived research references that depend on external upstream comparison tables, machine-local `ghq` paths, or stale OpenCode flat/nested questions.
- Preserves lifecycle-log compatibility, worktree fallback, marker-gated OpenCode cleanup, compact handoff/status projections, and run-version/schema-version fields used for freshness or persisted schema validation.
- Selected because it matches the confirmed preference: second confirmation, then as much approved cleanup as practical in one governed pass without taking on archive path-generation semantics.

### Approach C: Broad cleanup including archive path persistence and config-policy strictness
- Adds relative archived artifact path persistence, `.slipway.yaml` unknown-key fail-closed behavior, and stricter policy-pack key handling.
- Rejected for this pass because it mixes persisted archive semantics and external config contract changes into an already cross-cutting cleanup.

## Selected Approach

Implement Approach B. Remove the current `--quick` instead of redesigning an override surface in this batch. If a future expert escape hatch is needed, it should be a separate, explicit, audited advisory-control override with a reason and lifecycle trace.

## Interfaces and Data Flow

- CLI:
  - Remove `--quick` from `next` and `run` command flags.
  - Remove `quickMode` plumbing from `buildNextView`, `buildNextHandoffSourceView`, `advanceIfReady`, and `runGovernedLoop`.
- Progression:
  - Remove `AdvanceOptions.QuickMode` and the quick disabled-control injection block.
  - Keep `SkipAutoPass` unchanged.
- Task evidence:
  - Keep the flat `TaskEvidencePayload` fields.
  - Remove nested `TaskRun` support.
  - Return explicit errors for missing or invalid required fields instead of deriving from filename, expected version, default task kind/verdict, relative evidence ref, or file mtime.
- Artifact metadata:
  - Remove `ArtifactState.Version`.
  - Remove `ManifestVersion` from artifact template data.
  - Update tests and tracked archived `change.yaml` fixtures that contain artifact `version: 1`.
- Docs/archive references:
  - Replace stale OpenCode flat/nested open-question examples with a generic example.
  - Compress product docs' external system comparison section into Slipway-owned design principles.
  - Normalize tracked archived research/intent local upstream paths to project/source labels.

## Rollout and Rollback

- Rollout:
  1. Update tests to describe the target behavior.
  2. Remove runtime quick-mode code.
  3. Tighten task evidence parsing.
  4. Remove unused artifact version metadata.
  5. Refresh docs and tracked archived references.
  6. Run focused tests, full tests/build/docs, and Slipway validation.
- Rollback:
  - Revert this governed change. The implementation is source-only except for tracked governed artifacts and archived fixture text.
  - No data migration is required because archive path-generation semantics are intentionally out of scope.

## Risk

- Removing `--quick` may break private scripts that discovered the flag. This is accepted because the flag is a governance bypass and is not root-help documented.
- Tightening task evidence may break untracked manually authored evidence. This is accepted for current tracked evidence because archived task evidence uses the flat explicit shape.
- Removing artifact `version: 1` changes serialized `change.yaml` output and requires fixture/test updates.
- Archive path-generation semantics are explicitly deferred to avoid widening persistence risk.
