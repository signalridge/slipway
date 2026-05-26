# Research

Question: What can be cleaned up without breaking documented Slipway CLI, JSON, lifecycle, migration, or generated-surface contracts?

## Research Findings

### Architecture
- Affected modules:
  - `cmd/` owns CLI surfaces and JSON response structs; `cmd/next.go:16` defines the wide `nextView` output shape, and `cmd/run.go:163` reuses `buildNextView` rather than owning a separate decision implementation.
  - `internal/state/` owns current-state persistence and legacy runtime sidecar migration; `internal/state/change_runtime.go:16` names `runtime-state.yaml`, and `internal/state/store.go:510` transparently merges the legacy sidecar on load.
  - `internal/toolgen/` owns generated skill, command, prompt, hook, and stale-artifact cleanup; `internal/toolgen/toolgen.go:761` runs stale generated-artifact cleanup during refresh.
- Dependency chains:
  - CLI `next`/`run` -> `buildNextView` -> progression/readiness/gate projections -> JSON output shape.
  - state load/save -> `mergeLegacyRuntimeState` / `deleteLegacyRuntimeState` -> health and repair diagnostics.
  - `init` / tool generation refresh -> `cleanupStaleGeneratedArtifacts` -> stale skill, command, agent, prompt, hook, and support-file cleanup.
- Blast radius:
  - High for removing `nextView` fields, `runtime-state.yaml` load support, or generated-surface stale cleanup, because these are externally visible upgrade or agent-consumption surfaces.
  - Medium for consolidating internal helper duplication if no public command, JSON, or disk shape changes.
- Constraints:
  - `README.md:221` documents `runtime-state.yaml` migration/repair compatibility.
  - `docs/plans/2026-05-24-governance-kernel-runtime-framework.md:40` documents `nextView` as a compatibility JSON output shape, not duplicated business logic.
  - `docs/plans/2026-05-24-governance-kernel-runtime-framework.md:82` says these constraints change only through explicit ADR or command-contract update with tests and migration notes.

### Patterns
- Existing conventions:
  - Public command behavior is locked by command tests, route tests, and generated-surface tests.
  - Legacy cleanup is generally exercised as upgrade hygiene: remove stale generated files, but preserve user-managed files.
  - Runtime compatibility migrates recognized legacy sidecar fields into `change.yaml` and removes old sidecars on save/repair.
- Reusable abstractions:
  - `cleanupPrefixedEntries` and `cleanupUnexpectedEntries` are existing reusable cleanup primitives for toolgen stale artifacts.
  - `ChangeRuntimeStateLoadError` lets health/diagnostic callers distinguish unreadable legacy sidecars from unreadable `change.yaml`.
  - `ReasonCode` and structured CLI errors are the existing contract for user-visible failures.
- Convention deviations:
  - Any removal of `runtime-state.yaml`, retired generated-surface cleanup, or command JSON fields would need explicit contract migration rather than ordinary refactoring.

### Risks
- High: deleting `runtime-state.yaml` support will intentionally stop loading old active changes that still depend on the sidecar.
- High: deleting `nextView` fields or changing JSON shape could break current agent callers using `next` or `run`; do this only for fields proven legacy-only.
- Medium: deleting toolgen stale cleanup will intentionally stop cleaning old generated `slipway-sync`, old command prompts, old hook registrations, or legacy agent config blocks in existing user workspaces.
- Low: consolidating internal helper logic is reversible if tests prove public behavior unchanged, but it may not satisfy the user's goal of removing real compatibility paths.
- Reversibility:
  - Internal refactors are revertible.
  - Removing migration or JSON compatibility is harder to roll back after release because users may observe broken upgrade paths.

### Test Strategy
- Existing coverage:
  - Runtime sidecar migration and health/repair are covered in `internal/state/*_test.go`, `cmd/health_test.go`, and `cmd/repair_test.go`.
  - Retired generated-surface cleanup is covered by `internal/toolgen/toolgen_test.go:797`, `internal/toolgen/toolgen_test.go:829`, `internal/toolgen/toolgen_test.go:939`, and `internal/toolgen/support_files_test.go:60`.
  - Removed route surfaces such as legacy `--mode` are covered by `cmd/route_surface_command_test.go:47`.
- Infrastructure needs:
  - If a compatibility path is removed, add or update a focused test proving the new initial-version contract and remove obsolete compatibility tests in the same patch.
  - For contract-preserving cleanup, run focused package tests before full `go test ./...` and `go build ./...`.
- Verification approach:
  - Compile and test touched packages.
  - Run full `go test ./...`.
  - Run `go build ./...`.
  - Inspect JSON behavior for any touched command surface.

### Candidate Classification
- Remove:
  - `runtime-state.yaml` load/repair/delete compatibility in `internal/state`, plus related docs and compatibility tests.
  - Toolgen stale generated-artifact cleanup that only supports upgrading older generated workspaces, including retired `slipway-sync`, old catalog manifest/routes, legacy Codex agent block cleanup, and old post-tool hook cleanup.
- Retain unless proven legacy-only:
  - Current `next` and `run` JSON fields that are still part of the first-version agent handoff contract.
  - Current stale cleanup needed for deterministic refresh of the current generated tree.
  - Tests that reject old command flags when those tests express the current first-version contract.

## Alternatives Considered

- Approach 1: Contract-preserving cleanup, recommended.
  - Keep documented migration and JSON compatibility.
  - Remove or consolidate only internal redundancy that code search and tests prove is not an external contract.
  - Tradeoff: safer and compatible, but will not delete the biggest legacy compatibility blocks in this wave.
- Approach 2: Generated-surface upgrade-window cutoff.
  - Remove selected toolgen cleanup paths for old generated artifacts such as retired `slipway-sync`, legacy catalog files, old Codex agent blocks, or old post-tool hooks.
  - Tradeoff: removes visible legacy cleanup code, but users refreshing older generated workspaces may keep stale artifacts.
- Approach 3: Breaking contract cleanup.
  - Remove `runtime-state.yaml` migration/repair support and narrow command JSON output shapes.
  - Tradeoff: most thorough deletion, but intentionally breaks old workspaces and agent consumers; requires explicit ADR/contract migration and broader test/doc updates.
- Selected: Approach 3, adjusted for initial-version cleanup. The user explicitly clarified that no backward compatibility is required because Slipway is still an initial-version project. This means old workspace migration, old generated-surface cleanup, and compatibility-only tests/docs should be removed when the current first-version contract remains coherent.

## Unknowns
- Resolved: whether `runtime-state.yaml` is purely internal dead code -> no; it is documented migration/repair compatibility.
- Resolved: whether `nextView` width alone is proof of duplication -> no; it is documented as compatibility output.
- Resolved: whether retired generated-surface cleanup is accidental dead code -> no; tests assert refresh cleanup behavior for old generated artifacts.
- Resolved: user selection among Approach 1, Approach 2, or Approach 3 -> Approach 3, with current first-version contract consistency preserved.
- Remaining: exact implementation cut for `nextView` fields; do not remove broad JSON fields unless implementation research proves a field is legacy-only.

## Assumptions
- The confirmed scope now means historical compatibility is not a blocker; current first-version contract consistency remains a blocker. Evidence: user clarification recorded in `intent.md`.
- Generated user workspaces may still contain old Slipway-generated files even when the current repository does not. Evidence: toolgen refresh tests seed stale files and assert cleanup.
- A full implementation should not start until the selected approach is recorded in `decision.md`. Evidence: `slipway-research-orchestration` hard gate.

## Canonical References
- `artifacts/changes/remove-obsolete-backward-compatibility-paths-and-redundant-legacy-code-from-slipway/intent.md`
- `README.md:221`
- `docs/plans/2026-05-24-governance-kernel-runtime-framework.md:40`
- `docs/plans/2026-05-24-governance-kernel-runtime-framework.md:82`
- `cmd/next.go:16`
- `cmd/run.go:163`
- `cmd/route_surface_command_test.go:47`
- `internal/state/change_runtime.go:16`
- `internal/state/change_runtime.go:96`
- `internal/state/store.go:510`
- `internal/state/health.go:428`
- `internal/state/execution_repair.go:41`
- `internal/toolgen/toolgen.go:761`
- `internal/toolgen/toolgen.go:997`
- `internal/toolgen/toolgen.go:1070`
- `internal/toolgen/toolgen.go:1124`
- `internal/toolgen/toolgen.go:1131`
- `internal/toolgen/toolgen.go:1656`
- `internal/toolgen/toolgen.go:1805`
- `internal/toolgen/toolgen_test.go:797`
- `internal/toolgen/toolgen_test.go:829`
- `internal/toolgen/toolgen_test.go:939`
- `internal/toolgen/support_files_test.go:60`
