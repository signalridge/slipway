# Architecture

- Question: What code seams must Workstream A change to remove `checkpoint`,
  `learn`, and `stats` from the product and agent-facing command surface while
  preserving ledger-backed governed recovery?
- Root command surface: the custom root help is grouped in `cmd/root.go`; it
  currently exposes `checkpoint` under Situational and `learn`/`stats` under
  Diagnostics. Command registration also still adds `makeLearnCmd`,
  `makeStatsCmd`, and `makeCheckpointCmd`. Evidence: `cmd/root.go:28-90`,
  `cmd/root.go:152-189`.
- Checkpoint lifecycle state: `internal/model.Change` persists
  `ActiveCheckpoint`, so deleting the concept is a model and state-schema
  change, not just a command removal. Evidence: `internal/model/change.go:62-64`.
- Checkpoint resume protocol: `run` and `implement` expose
  `--resume-response` beside `--resume`; entry validation gives active
  checkpoints priority over normal resumable wave execution. Evidence:
  `cmd/run.go:104-111`, `cmd/run.go:202-291`, `cmd/stage.go:41-117`.
- Next/status integration: `next` serializes `resume_checkpoint`, confirmation
  metadata, checkpoint consumption side effects, and action kinds; `status`
  surfaces `run --resume-response "<response>"` when `ActiveCheckpoint` exists.
  Evidence: `cmd/next.go:190-206`, `cmd/next.go:539-575`,
  `cmd/next.go:730-768`, `cmd/next.go:855-867`, `cmd/status_view_build.go:183-214`.
- Wave/task metadata: the task-plan parser and wave node include
  `checkpoint_type`, so A1 must remove that metadata from task planning,
  hashing, parsing, and generated guidance. Evidence:
  `internal/engine/wave/wave.go:12-19`, `internal/engine/wave/parse.go:103-127`,
  `internal/engine/wave/parse.go:137-147`.
- Surface generation: command metadata, workflow command groupings, namespace
  routers, adapter contracts, docs, and generated skill inventories are derived
  from `internal/toolgen`; removing A surfaces must update this registry rather
  than only hiding Cobra commands. Evidence: `internal/toolgen/toolgen.go:273-300`,
  `internal/toolgen/toolgen.go:310-352`, `internal/toolgen/toolgen.go:479-495`,
  `internal/toolgen/install_profiles.go:49-84`.
- Preserved recovery seam: non-checkpoint `run --resume` is already tied to
  S2 execution summary readiness and `state.ResumeWaveIndex`; that seam must
  remain after checkpoint deletion. Evidence: `cmd/common.go:930-950`,
  `cmd/run.go:270-289`.
