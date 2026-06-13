# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `cmd/status.go`: explicit `status --change` route and `statusView` JSON
    shape.
  - `cmd/status_view_build.go`: governed status readiness projection and
    narrative.
  - `cmd/status_render.go`: text rendering and "What's Next" hint.
  - `cmd/common.go`: shared active-change loading and existing archived fallback
    precedent.
  - `internal/state/lifecycle.go` and `internal/state/store.go`: archived
    change loading and archive path discovery.
- Dependency chains:
  - `status --change` -> `loadChangeBySlug` -> `state.LoadChange` -> status
    view rendering.
  - governed status view -> `progression.EvaluateGovernanceReadiness` ->
    status blockers/recovery/narrative.
  - archived read path -> `state.LoadArchivedChange` ->
    `artifacts/changes/archived/<slug>/change.yaml`.
- Blast radius: low to medium. The public `status` surface changes, but the
  lifecycle mutation path, archive mechanics, and state persistence remain
  unchanged.
- Constraints:
  - Active lifecycle state must remain `active` until `slipway done` archives
    the change.
  - Archived records are terminal read targets, not active lifecycle targets.
  - Existing repair/delete diagnostics must remain available when no archived
    authority exists.

### Patterns
- Existing conventions:
  - `cmd/next.go` projects done-ready as a read-only query result rather than
    persisting a new state.
  - `cmd/common.go` already falls back to archived records for active-only
    command resolution and returns archive metadata instead of raw load failure.
  - `cmd/status_view_build.go` centralizes status narrative and blocker
    projection.
- Reusable abstractions:
  - `appendReasonCodes` for deduplicating `run_slipway_done_to_finalize`.
  - `model.BuildRecovery` for canonical finalization recovery.
  - `state.LoadArchivedChange` and `state.ArchivedChangeFilePathForRead` for
    terminal record reads.
- Convention deviations:
  - Additive `statusView` fields are required because `lifecycle_status` already
    means persisted change status.

### Risks
- Technical risks:
  - Medium: reporting done-ready as `done` would incorrectly imply the archive
    already happened.
  - Medium: an archive fallback could hide real active-bundle corruption if
    applied before checking a valid active change.
  - Low: additive JSON fields may require text rendering updates to avoid
    operator drift.
- Guardrail domains: none of the sensitive domains apply. This is local CLI
  state projection.
- Reversibility: high. The change is confined to status view construction,
  explicit status routing, and tests.

### Test Strategy
- Existing coverage:
  - `cmd/progression_next_test.go` verifies done-ready read-only projection for
    `next`.
  - `cmd/cli_e2e_test.go` verifies `done` archives a ready change.
  - `cmd/status_view_build_test.go` covers status view projection.
- Infrastructure needs:
  - Reuse existing command fixtures: `createGovernedRequest`,
    `markChangeReadyForDone`, `writePassingFinalCloseoutEvidence`, and
    `state.ArchiveChange`.
- Verification approach:
  - Add a focused status-view test asserting `done_ready`, finalization blocker,
    and narrative for an S4 ship-ready change.
  - Add a command-level archived-status test asserting
    `status --json --change <slug>` returns archived/done metadata after
    finalization.
  - Run focused `go test ./cmd`, then full `go test ./...` and Slipway
    validation.

### Options
- Option A: Change `lifecycle_status` from `active` to `done_ready` when S4 ship
  readiness is approved. Tradeoff: very visible, but it overloads a persisted
  status field and risks consumers treating the change as terminal before
  `slipway done`.
- Option B: Add explicit top-level status projection fields such as
  `done_ready` and `archived`, keep persisted `lifecycle_status`, and update the
  narrative/text hint. Tradeoff: additive and compatible, but consumers must
  read the new fields for the richer state.
- Option C: Keep JSON shape unchanged and only rewrite narrative. Tradeoff:
  lowest schema impact, but weaker machine-readable contract and less useful
  for agents.
- Selected: Option B. It preserves lifecycle semantics while adding the
  machine-readable and human-readable signals requested by #195 and #196.

## Unknowns
- Resolved: whether archived fallback already exists elsewhere -> yes,
  `resolveExplicitChange` uses `state.LoadArchivedChange` to avoid active-state
  false positives.
- Resolved: whether done-ready can be projected without new persistence -> yes,
  `next` already does this through S4 ship authority evaluation.
- Remaining: None.

## Assumptions
- `run_slipway_done_to_finalize` remains the canonical done-ready reason code.
  Evidence: existing `next` projection and recovery taxonomy use it.
- `lifecycle_status` should continue to reflect `model.Change.Status`.
  Evidence: `statusView` currently maps it directly from `change.Status`, and
  `done` is only set by archive finalization.
- Archived status should be read-only and not expose active next actions.
  Evidence: archived changes are loaded from `artifacts/changes/archived` and
  active commands treat them as outside active validation scope.

## Canonical References
- `cmd/status.go:15`
- `cmd/status.go:202`
- `cmd/status.go:381`
- `cmd/status_view_build.go:21`
- `cmd/status_view_build.go:318`
- `cmd/status_render.go:39`
- `cmd/common.go:347`
- `cmd/next.go:516`
- `internal/state/lifecycle.go:23`
- `internal/state/store.go:223`
- `cmd/progression_next_test.go:1727`
- `cmd/cli_e2e_test.go:448`
