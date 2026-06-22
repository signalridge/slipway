# Concerns

- Stale context risk: prior codebase-map content described an adapter expansion
  change, so this map was re-authored for issue #297 Workstream A before being
  used as planning context.
- Compatibility risk: removing `ActiveCheckpoint` breaks old `change.yaml`
  files that contain `active_checkpoint`. Issue #297 intentionally asks to
  delete the concept, so the plan should prefer a clear fail/repair story or a
  bounded cleanup over preserving retired behavior.
- Resume dead-end risk: the replacement semantics rely on normal task verdicts
  `blocked`/`incomplete` plus `run --resume`. Current resume entry validation
  only treats S2 execution with a ready execution summary and `ResumeWaveIndex >
  0` as resumable. Evidence: `cmd/common.go:930-950`, `cmd/run.go:270-289`.
- Checkpoint deletion risk: auto mode, confirmation requirements, next JSON,
  status actions, health/repair, lifecycle events, and templates all encode
  checkpoint-specific branches. Removing only `cmd/checkpoint.go` would leave
  product-surface remnants and likely compile failures.
- Learn deletion risk: `learn` consumes repo stats and lifecycle history. If
  any analysis helper remains useful, it should become internal-only or move to
  a retained diagnostic owner; the unsupported apply path should disappear with
  the command.
- Stats deletion risk: `cmd/stats.go` duplicates diagnostics that are still
  useful through `status --stats` and `health`. Preserve `state.CollectRepoStats`
  if still used by retained commands, but remove the standalone `stats`
  command, docs, generated skills, and manifest rows.
- Generated-surface risk: checked-in docs and adapter inventories can keep a
  deleted command visible even after Cobra registration is removed. Toolgen,
  templates, docs, and `docs/SURFACE-MANIFEST.json` must be refreshed together.
- Reversibility: most deletions are local to command/state/template surfaces,
  but model-state deletion is not trivially reversible for active changes with
  checkpoints. This is acceptable for A only because issue #297 explicitly
  removes checkpoint as a product concept.
