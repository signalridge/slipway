# Decision

## Alternatives Considered
- Minimal route-completion patch: extend the existing `invocationRouteView` projection to `done` and `evidence`, switch their target resolution to the current per-command read context where useful, and add focused command tests.
- New lifecycle contract package: move route, action, freshness, and capability projections out of `cmd` into a shared package and make every command consume it.
- Generic lifecycle JSON envelope: wrap all public lifecycle command responses in one common top-level envelope.

The selected direction is the minimal route-completion patch. The other options increase blast radius without improving the concrete missing behavior found during research.

## Selected Approach
Use the route/action/freshness/capability foundations already present on main and close the remaining gap in mutating command surfaces:

- Add `InvocationRoute *invocationRouteView` to `doneView`, `evidenceSkillView`, `evidenceTaskView`, and `evidenceTaskBatchView`.
- Use `newStateReadContext(root)` in `done`, `evidence skill`, and `evidence task` target resolution paths so target resolution and loaded change facts share the current command's read context.
- Compute route information from the active target change before any mutating operation that can archive or rewrite authority, especially `done`.
- Populate `invocation_route` on successful JSON outputs for single-change `done` and evidence commands.
- Add black-box command tests proving route output and explicit missing slug fail-closed behavior.

This avoids a compatibility layer and does not preserve the old missing-route contract. The public JSON response directly gains the correct route field.

## Interfaces and Data Flow
- Public JSON additions:
  - `done --json`: add `invocation_route`.
  - `evidence skill --json`: add `invocation_route`.
  - `evidence task --json`: add `invocation_route`.
  - evidence task batch result-file JSON: add `invocation_route` at the batch level if it records against one target change.
- Internal flow:
  - Command root resolution creates one `stateReadContext`.
  - Active target slug is resolved via `resolveActiveChangeRefWithReadContext`.
  - The active `model.Change` used for command validation also feeds `buildInvocationRouteView` through an apply helper.
  - Mutating command logic continues to own archiving, evidence stamping, lifecycle event append, and execution summary synchronization.

## Rollout and Rollback
- Rollout is a normal code/test PR. Existing callers that ignore unknown JSON fields continue unaffected, but the implementation is not a compatibility shim; it directly changes the public contract by adding route data.
- Rollback is reverting the commit and rerunning:
  - `go test ./cmd -run 'Test.*(InvocationRoute|ChangeFlag|HostCapability|ReviewBatch|Freshness|Evidence|Done)' -count=1`
  - `go test ./... -count=1`

## Risk
- `done` archives the change, so route projection must use the pre-archive active change. If route projection reloads after archive, it can incorrectly report archived or missing active authority.
- Evidence commands validate stage/actionability before writing records. Route projection must not bypass `validateEvidenceSkillActionable`, `validateEvidenceTaskRunSummaryVersion`, or execution-summary synchronization.
- The route helper's `NextCommand` may describe the pre-mutation next command; for successful `done`, that is execution context rather than post-archive next action. Tests should assert the route fields needed by `opt.md` without treating `NextCommand` as a post-mutation recommendation.
- Existing split freshness, review batch action, and host capability behavior must not be rewritten as part of this change. Reuse the current tested code.
