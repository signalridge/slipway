# Decision

## Alternatives Considered

### Option A: Reuse `lifecycle_status` for done-ready
Set `lifecycle_status` to `done_ready` when S4 ship readiness is approved.
This would make the handoff visible, but it would overload a field that already
maps to persisted `model.Change.Status`. Consumers could mistake readiness for a
finalized terminal state.

### Option B: Add projection fields and narrative
Keep `lifecycle_status` as the persisted change status and add optional
projection fields for `done_ready` and `archived`. Update the status narrative
and text hint to make the same facts human-visible. This is additive, keeps the
state machine semantics intact, and gives agents a machine-readable contract.

### Option C: Narrative-only repair
Only rewrite the narrative for S4 ship-ready and archived views. This avoids
schema additions but leaves agents and scripts without a clear top-level status
signal.

## Selected Approach

Select Option B. `status` will keep persisted lifecycle status intact, add
machine-readable projection fields for done-ready and archived states, and align
the text narrative with those fields. Explicit archived status reads will use
`state.LoadArchivedChange` only after active load fails, preserving active state
priority.

## Interfaces and Data Flow

- `statusView` gains optional `done_ready`, `archived`, and `archive_path`
  fields.
- Governed status construction sets `done_ready` when the current state is
  `S4_VERIFY` and `G_ship` is approved, then appends
  `run_slipway_done_to_finalize` and canonical recovery.
- `status --change <slug>` first attempts active `state.LoadChange`; if active
  load fails and `state.LoadArchivedChange` succeeds, it renders a read-only
  archived status view.
- Text rendering uses the same `done_ready` and `archived` fields for narrative
  and primary action hints.

## Rollout and Rollback

Rollout is a normal CLI release of additive status behavior. Rollback is a git
revert of the changes to `cmd/status.go`, `cmd/status_view_build.go`,
`cmd/status_render.go`, and focused tests. Verification commands:

- `go test ./cmd`
- `go test ./...`
- `git diff --check`
- `go run . validate --json`

## Risk

- Done-ready must not become a persisted terminal state. The implementation
  keeps `lifecycle_status=active` and adds `done_ready=true`.
- Archived fallback must not hide a valid active change. The implementation
  tries active load first and falls back only on active-load failure.
- Text and JSON status could drift. The implementation updates both the JSON
  view and the text hint/narrative path.
