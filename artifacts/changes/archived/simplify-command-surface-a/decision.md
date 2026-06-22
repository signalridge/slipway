# Decision

## Alternatives Considered

- Hide-only deletion: remove command rows from root help or set Cobra
  `Hidden: true`. Rejected because `stats` is already hidden in Cobra but still
  exported through custom root help, toolgen, docs, and manifest, and because it
  leaves checkpoint state and resume protocol alive.
- Direct Workstream A deletion: remove `checkpoint`, `learn`, and `stats` from
  CLI/product/generated surfaces; delete checkpoint lifecycle state and
  protocol; preserve `run --resume`, task verdict blockers, `status --stats`,
  `health`, and internal stats helpers that retained commands still need.
  Selected.
- Expand scope to Workstream B first: add `evidence task --result-file` and
  engine-owned run-version behavior before deleting A surfaces. Rejected for
  this governed change because issue #297's full one-PR plan is broader than
  the requested A slice and would materially expand the implementation and
  review surface.

## Selected Approach

Implement direct Workstream A deletion.

The implementation will remove the `checkpoint`, `learn`, and `stats` command
surfaces from Cobra registration, custom root help, toolgen command metadata,
install profiles, command/template guidance, docs, generated skill inventories,
and surface manifest rows. It will also remove checkpoint-specific lifecycle
state and protocol branches: `ActiveCheckpoint`, `--resume-response`,
`resume_checkpoint`, checkpoint confirmation action kinds, checkpoint lifecycle
events, checkpoint health/repair handling, checkpoint task metadata, and
checkpoint reason codes.

The implementation will preserve retained governance behavior: `run --resume`,
interrupted-wave recovery, blocked/incomplete task verdict blockers,
`status --stats`, `health`, evidence freshness, scope checks, parallel overlap
checks, and sensitive-domain fail-closed behavior.

## Interfaces and Data Flow

- Removed CLI interfaces: `slipway checkpoint`, `slipway learn`, standalone
  `slipway stats`, `run --resume-response`, and `implement --resume-response`.
- Removed JSON/handoff fields: `input_context.resume_checkpoint` and
  checkpoint-specific confirmation action kind/support fields where they exist
  only to support the deleted protocol.
- Removed state/data fields: `model.Change.ActiveCheckpoint`, checkpoint reason
  codes, `checkpoint_type` task metadata, and lifecycle event side effects
  dedicated to resolving active checkpoints.
- Retained interfaces: `slipway run --resume`, `slipway status --stats`,
  `slipway health`, `slipway evidence task`, `slipway evidence skill`, and
  `slipway evidence suite-result`.
- Retained data flow: task evidence and wave summaries remain the authority for
  S2 execution progress, freshness, scope, and resume decisions.

## Rollout and Rollback

Rollout is local to this branch and governed change. The implementation will be
verified with targeted package tests, generated-surface checks, black-box help
checks, search checks, and `go test ./...` before S3 closeout.

Rollback path: revert this branch's code/doc/template/artifact changes and
rerun `go test ./cmd ./internal/model ./internal/state ./internal/engine/wave
./internal/engine/progression ./internal/tmpl ./internal/toolgen` plus
`go test ./...`. Because `ActiveCheckpoint` is intentionally removed, rollback
would restore compatibility with active checkpoint state only by reverting the
model and command deletions together.

## Risk

- Incomplete deletion risk: checkpoint is coupled across command entry,
  handoff, status, health/repair, model, wave metadata, templates, docs, and
  tests. Mitigation: search checks and targeted package tests.
- Resume regression risk: deleting checkpoint must not regress
  interrupted-wave `run --resume`. Mitigation: preserve and update focused
  resume tests, then add blocked/incomplete coverage if current behavior is
  insufficient.
- Generated-surface drift risk: docs or generated skill inventories can expose
  deleted commands after code compiles. Mitigation: update toolgen registry and
  run surface manifest checks.
- State compatibility risk: active changes with persisted `active_checkpoint`
  are intentionally no longer supported as a live protocol. Mitigation: keep
  any failure explicit and bounded rather than silently reviving checkpoint.
