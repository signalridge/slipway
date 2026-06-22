# Testing

- Question: What proof is needed for Workstream A after exported command
  deletion and checkpoint-state removal?
- Black-box surface checks must cover root help and command help because the
  issue is about the actual exported surface, not only Cobra hidden fields.
  Current `go run . --help` still lists `checkpoint`, `learn`, and `stats`;
  current `go run . run --help` still lists `--resume-response`.
- Existing resume tests prove non-checkpoint recovery paths that must survive:
  `TestRunRequiresExplicitResumeAfterAbortWithWaveBackedState`,
  `TestRunRejectsResumeWhenWaveRunsAreIncomplete`, and
  `TestRunResumeUnavailableExplainsLifecycleBoundary` pass today. Evidence:
  command transcript `go test ./cmd -run 'TestRunRequiresExplicitResumeAfterAbortWithWaveBackedState|TestRunRejectsResumeWhenWaveRunsAreIncomplete|TestRunResumeUnavailableExplainsLifecycleBoundary'`.
- Deletion tests to remove or replace: checkpoint behavior tests under
  `cmd/checkpoint_test.go`, checkpoint-specific branches in
  `cmd/progression_next_test.go`, model tests for `ActiveCheckpoint`, and
  state health tests for checkpoint findings. These are no longer product
  behavior once A1 lands.
- Learn/stats command tests are command-surface tests. If reusable internals
  stay, move coverage to the retained owner, e.g. `internal/state` or
  `status --stats`, instead of keeping deleted command behavior alive.
- Toolgen/docs verification is required because command exposure is generated
  in multiple places. Minimum targeted packages for A are `./cmd`,
  `./internal/model`, `./internal/state`, `./internal/engine/wave`,
  `./internal/engine/progression`, `./internal/tmpl`, and `./internal/toolgen`.
- Required black-box outcomes for A: root help omits `checkpoint`, `learn`, and
  `stats`; `run` and `implement` help omit `--resume-response`; `run` still
  shows `--resume`; `status --stats` remains available if repo diagnostics are
  preserved there.
- Search verification should reject live product/template/docs references to
  `ActiveCheckpoint`, `resume-response`, `checkpoint_type`,
  `resume_checkpoint`, checkpoint reason codes, `$slipway-learn`, and
  `$slipway-stats`, with any remaining hits justified as historical changelog or
  unrelated prose.
