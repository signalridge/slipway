# Intent

## Summary
cleanup stale code surfaces

## Complexity Assessment
complex

Rationale: this is a cross-package cleanup touching CLI view helpers, governance
progression internals, model/state definitions, config, tests, generated
surface metadata, and public command contracts. The change is intentionally not
batch-limited: every cleanup candidate from the pasted reports that remains
true under current-main fact confirmation is in scope.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Clean up current-main stale/dead/redundant code surfaces confirmed by a fresh
  read-only pass in this worktree.
- Complete the full pasted cleanup set in one governed change rather than a
  first-pass subset.
- Remove or inline production wrappers only kept alive by tests when the
  current production path has a newer direct helper.
- Fix low-risk lint-confirmed cleanup points from `unused --tests=false`,
  `unparam --tests=false`, and `staticcheck,ineffassign,wastedassign
  --tests=false`.
- Remove confirmed unused internal model/state/capability fields and methods
  where current production code has no writer or reader.
- Remove the inert `CloseoutConditional` required-skill filtering path and its
  dead closeout-required parameter wiring while preserving live
  `CloseoutRefreshRequired` and quality-mode behavior.
- Remove narrow retired reader/shim branches that only preserve old local
  state error wording or skip obsolete verification files.
- Remove reason-code catalog/remediation entries that are defined but no longer
  emitted by current gates, together with corresponding tests/snapshots.
- Consolidate low-risk duplicate command wiring where the behavior is already
  identical and tests cover the public output.
- Remove no-op/dead configuration and public surfaces confirmed by current
  callers, including no-op validation flags and write-only review drift
  counters.
- Retire confirmed old-state compatibility readers and public no-op command
  tokens, including retired workflow-state canonicalization and no-op
  `done/validate --json` flags, with generated docs/manifest updates.
- Consolidate the remaining confirmed redundancy items from the pasted reports,
  including S3 review template duplication, artifact contract helper
  duplication, verification test helpers, tiny command `findRepoRoot`
  duplication, command route/freshness wiring, GitHub helper duplication, stale
  evidence repair predicates, strict cache loaders, load-error wrappers, and
  reason/remediation table drift.

## Out of Scope
- Do not inspect or reuse old worktrees as source evidence. Only the new
  governed worktree and current `main`/`origin/main` facts are authoritative.
- Do not delete fail-closed defenses that reject retired inputs, including
  `StageContextGoal` and `StageContextCloseout`; those may be relabeled if their
  comments are misleading.
- Do not delete the current `defaults.artifact_schema: expanded` behavior,
  because this repository's `.slipway.yaml` actively uses it.
- Do not delete `custom_artifacts` or the `custom` artifact schema in this
  cleanup. Current worktree facts show the custom schema is decoded, validated,
  wired into change creation, and used by artifact/instructions/readiness paths.
  Retiring it would be a separate product decision, not stale-code cleanup.
- Do not touch ignored local agent surfaces or unrelated root/worktree dirt.

## Constraints
- No backward-compatibility shims should be preserved solely for historical
  bundle or local-runtime shapes, but fail-closed governance defenses stay.
- Public behavior may change where the confirmed cleanup explicitly removes a
  no-op compatibility token or dead reader, because the user confirmed no
  backward compatibility is required.
- Generated docs/manifests must be regenerated or verified if their source
  inputs change.
- Preserve unrelated untracked files and unrelated changes.

## Acceptance Signals
- `go test ./...` passes in the governed worktree.
- `golangci-lint run ./...` reports `0 issues`.
- Focused cleanup linters no longer report the targeted issues:
  `golangci-lint run --enable-only=unused --tests=false ./...`,
  `golangci-lint run --enable-only=unparam --tests=false ./...`, and
  `golangci-lint run --enable-only=staticcheck,ineffassign,wastedassign
  --tests=false ./...`.
- `go run ./internal/toolgen/cmd/gen-surface-manifest --check` passes if command
  surfaces or generated public metadata are in scope.
- `go run . validate --json` and `go run . next --json --diagnostics` show the
  governed change has reached the appropriate ready state for its lifecycle
  stage.

## Open Questions
<!-- Track real unknowns as a checklist. An unchecked `- [ ]` item is unresolved
     and routes intake to S0_INTAKE/research; mark `- [x]` once resolved. Leave the
     section empty (or write `None`) when there are none. Prose here is
     documentation, not a blocker — a genuine open question must be a `- [ ]`. -->
None

## Deferred Ideas
- None. Confirmed-live fail-closed defenses are retained because they are active
  safety behavior, not backward-compatibility support.

## Approved Summary
Confirmed by user on 2026-06-28: proceed with the full cleanup scope in one
governed change. The user clarified this is not a first batch and should include
all confirmed cleanup objects from the pasted reports, with no backward
compatibility preservation for stale/dead surfaces.

This change cleans current-main stale/dead/redundant Slipway code surfaces after
parallel read-only fact confirmation. It includes lint-confirmed production
wrappers and cleanup findings, inert internal wiring such as
`CloseoutConditional`, confirmed unused internal model/state/capability fields
and methods, narrow retired reader/shim branches, unused reason-code catalog
entries, no-op/dead config and public command tokens, retired workflow-state
compatibility, duplicate wiring consolidation, and the medium-priority
redundancy cleanup items from the pasted reports. It excludes old worktree
evidence, fail-closed retired-input defenses, live artifact-schema behavior
(`core`, `expanded`, and `custom`), ignored local agent surfaces, and unrelated
dirt.
