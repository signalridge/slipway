# Architecture

- Question: Which seams must change so every public lifecycle command reports
  the same invocation route, actionability, freshness/readiness, and host
  capability contract?
- Active command resolution currently centers on `resolveActiveChangeRef` in
  `cmd/common.go:339-390`. It resolves explicit `--change` first, then current
  git worktree binding, then a global active fallback. Most lifecycle commands
  (`next`, `run`, `done`, `evidence`) call this resolver before acting.
- Explicit slug resolution is in `resolveExplicitChange` at
  `cmd/common.go:405-483`. Missing active bundles currently become
  `no_active_change`, archived slugs become `archived_change_not_validatable`,
  and corrupted active bundles fail closed as `change_state_load_failed`.
- Bound-elsewhere error construction is in `wrapResolutionError` at
  `cmd/common.go:909-934`. It already carries `change_bound_to_other_worktree`,
  bound slug/path details, and executable remediation.
- `status` has a separate route path in `cmd/status.go:299-378`.
  `resolveStatusRouteForRoot` only consults `resolveActiveChangeRef` for
  multi-active cases, so a single active change bound to another worktree can be
  rendered as a normal governed status view from the root checkout.
- Archived-local precedence is explicitly protected by
  `statusArchivedChangeForCurrentWorktree` in `cmd/status.go:400-416` and by
  resolver archived fallback in `cmd/common.go:353-364` and `cmd/common.go:370-379`.
  Any shared route must preserve this #283 invariant.
- `statusView`, `validateView`, and `nextView` carry separate action/freshness
  shapes: `statusView` exposes `next_ready_actions` and `evidence_freshness`
  (`cmd/status.go:17-66`), `validateView` exposes `actionable_next_skill` and
  `evidence_freshness` (`cmd/validate.go:17-52`), and `nextView` exposes
  `confirmation_requirement` (`cmd/next.go:14-75`).
- `status` currently derives ready actions through
  `projectNextReadyActionsWithPrimary` (`cmd/common.go:994-1018`) and does not
  know whether the invocation workspace is locally executable. This is the root
  of the misleading root-status behavior.
- Execution freshness is projected by `projectFreshnessForExecMode` at
  `cmd/common.go:1020-1034`. The helper intentionally ignores non-freshness
  blockers such as `required_skill_missing`, so a broader readiness freshness
  field must be added instead of changing execution freshness semantics in place.
- Host capability data is currently advisory. `appendCatalogHints` resolves
  support hints from the capability registry (`cmd/next_skill_view.go:894-925`),
  and the registry resolver produces supports/hydrate references
  (`internal/engine/capability/resolver.go:48-70`). There is no CLI-visible
  field for required host capabilities, availability, or fail-closed fallback.
