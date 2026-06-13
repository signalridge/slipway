# Architecture

Re-authored for change
`resolve-github-issues-195-and-196-make-status-expose-done-re`
(GitHub issues #195 and #196).

Question: how should `slipway status` expose terminal readiness and archived
records without weakening active lifecycle authority?

## Affected Seams

- `cmd/status.go:202` through `cmd/status.go:219` is the explicit
  `status --change <slug>` route. It currently calls `loadChangeBySlug`, whose
  active-only loader returns an error before archived records can be surfaced.
- `cmd/common.go:347` through `cmd/common.go:367` already contains the archived
  fallback pattern for active-only commands. `resolveExplicitChange` maps an
  archived slug to `archived_change_not_validatable` with archive metadata,
  proving that archived lookup is already a supported read concern.
- `cmd/status_view_build.go:21` through `cmd/status_view_build.go:155` builds
  the governed status projection. It evaluates governance readiness and gate
  state, but does not convert an approved S4 ship gate into a top-level
  done-ready signal for status.
- `cmd/next.go:516` through `cmd/next.go:535` contains the existing read-only
  done-ready projection used by `next`: S4 plus approved ship authority becomes
  `advanced.action=done_ready` with `run_slipway_done_to_finalize`.
- `internal/state/lifecycle.go:23` through `internal/state/lifecycle.go:32`
  loads archived changes from `artifacts/changes/archived/<slug>/change.yaml`.
  `internal/state/store.go:223` through `internal/state/store.go:228`
  searches archived records across workspace roots.

## Dependency Flow

`status --change` resolves a slug, loads active state, builds a status view,
then renders JSON/text. Archived records are not active lifecycle authorities,
but they are valid query targets because `state.LoadArchivedChange` already
loads them for read surfaces.

Done-ready status is a projection, not a persisted lifecycle state. The change
must keep `change.status=active` until `slipway done` archives the record, while
making the readiness visible through status view fields and narrative.

## Constraints And Invariants

- Active `change.yaml` remains the only authority for active lifecycle
  mutation.
- Archived records are terminal query records and must not be routed through
  active lifecycle mutation or repair guidance.
- Done-ready must not be reported as `done` before `slipway done` runs.
- The status surface should reuse existing reason code
  `run_slipway_done_to_finalize` instead of inventing a second finalization
  blocker taxonomy.
