# Decision

## Alternatives Considered
- Approach 1: Contract-preserving cleanup. Keep historical migration and upgrade compatibility, remove only internal redundancy. Rejected because the user clarified that backward compatibility is not required for this initial-version project.
- Approach 2: Generated-surface upgrade-window cutoff. Remove selected old generated-surface cleanup while keeping runtime migration compatibility. Rejected as too partial for the clarified goal.
- Approach 3: Initial-version cleanup. Remove compatibility-only paths for old workspace state, old generated surfaces, stale agent/tool config, and related tests/docs while keeping the current first-version contract internally coherent.

## Selected Approach
Use Approach 3. Treat Slipway as an initial-version project and delete backward-compatibility paths that only serve older Slipway versions or stale generated workspaces. Keep current command, JSON, lifecycle, and generated-surface behavior coherent by updating docs and tests to describe the new current contract.

## Interfaces and Data Flow
- Remove legacy `runtime-state.yaml` load/repair/delete flow from `internal/state` and rely on `change.yaml` as the single active change authority.
- Remove old generated-surface cleanup code that only upgrades older generated workspace layouts, while preserving deterministic refresh for the current generated tree.
- Update CLI docs, command-contract docs, and tests so current behavior no longer advertises or asserts historical compatibility.

## Rollout and Rollback
- Rollout is a normal source change in the governed worktree.
- Rollback is a git revert of this cleanup if removed compatibility turns out to be needed.
- No runtime migration or deprecation window is required by scope.

## Risk
- Existing old workspaces with `runtime-state.yaml` or old generated Slipway surfaces may no longer upgrade cleanly.
- Current agent callers may break if broad JSON fields are removed without proof they are legacy-only; therefore JSON pruning must be targeted and tested.
- Test updates must distinguish compatibility-only assertions from current first-version product assertions.
