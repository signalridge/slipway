# Decision

## Decision
Implement a Slipway-native Scope Contract evaluator and governance surfacing path. The contract source is the existing `tasks.md target_files`; the actual source is execution changed-file evidence. Drift is reported as a blocker after execution evidence exists.

## Selected Approach
- Add a small internal evaluator for planned-vs-actual file reconciliation.
- Treat task target files as allowed exact files, directories, or simple glob patterns.
- Keep scope drift separate from blast-radius controls.
- Surface the result through validation/status/review readiness where governed blockers are already reported.
- Update host-skill templates so final review and goal verification explicitly check contract evidence.
- Resolve updateable `artifacts/codebase` output against the active workspace/worktree so a bound worktree owns its generated codebase-map refresh.

## Interfaces
- Input:
  - `wave.TaskPlan` / `TaskNode.TargetFiles`
  - `model.ExecutionSummary` or `map[string]model.TaskRun`
- Output:
  - deterministic report with verdict, planned targets, changed files, out-of-scope files, warnings, and blocker reason codes
- CLI surface:
  - validate/status/review blockers and JSON detail where existing structures allow
- Codebase-map path surface:
  - `codebase-map` writes to active workspace/worktree `artifacts/codebase`
  - `next --json` reports bound worktree-local codebase map paths

## Rollback Strategy
- Remove the new evaluator and its surfacing call sites.
- Keep existing `target_files`, execution summary, and blast-radius behavior unchanged.
- Revert host-skill wording if template assertions fail or prove too noisy.

## Risks
- Existing tests may create execution summaries without changed-files evidence; scope checks must distinguish fixture gaps from real pass/fail.
- Broad glob semantics can surprise users; start with exact, directory prefix, and simple `*`/`**` matching only.
- Scope checking should not block pre-execution planning before task evidence exists.

## Alternatives Considered

- Add spec-kitty `owned_files` frontmatter.
  - Decision: rejected.
  - Tradeoff: stronger named concept, but duplicate authority in Slipway.

- Add PreToolUse hook enforcement first.
  - Decision: deferred.
  - Tradeoff: earlier feedback, but higher false-positive cost before reconciliation semantics are proven.

- Keep scope checking only in review prose.
  - Decision: rejected.
  - Tradeoff: cheaper, but does not provide a deterministic governance gate.

## Consequences
- Slipway gains a direct AI scope-drift detector without taking on spec-kitty's lane/platform model.
- Tasks must accurately declare generated or artifact files they intend to touch.
- Dedicated worktrees can refresh their own `artifacts/codebase` files without mutating the main checkout.
- Future hook/context-hydration work can reuse the same contract instead of inventing a second boundary model.
