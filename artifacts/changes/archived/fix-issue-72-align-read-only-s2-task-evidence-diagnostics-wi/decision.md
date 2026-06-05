# Decision

## Project Context
- Tech Stack: Go CLI
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Option A: Keep read-only `run_summary_missing`
This avoids code changes, but it leaves #72 unresolved. The operator can only
see the real task-evidence drift after running a mutating path, so the
read-only diagnosis remains misleading.

### Option B: Run wave sync from read-only surfaces
This reuses exact execution-path logic, but it would write derived runtime
state from `next`, `validate`, and `status`. That violates REQ-003 and the
documented query-only command contract.

### Option C: Add a non-mutating wave execution preview
Extract the common wave/task evidence diagnosis into a helper that can run
with or without writes. The mutating path keeps writing `wave-runs`,
`execution-summary.yaml`, and checklist changes. The read-only path only uses
the preview blockers to refine S2 readiness.

### Option D: Patch each command renderer
Each command could post-process `wave-orchestration:run_summary_missing`, but
that duplicates diagnosis logic across renderers and risks divergence from
`run`.

## Selected Approach
DEC-001: Use Option C. Add `PreviewGovernedWaveExecution` on top of the same
core evaluator as `SyncGovernedWaveExecution`. In S2 readiness, when the
execution summary is absent and the required-skill blocker is specifically
`wave-orchestration:run_summary_missing`, run the preview. If preview blockers
show present but stale/invalid/plan-drifted task evidence, remove the
misleading run-summary-missing blocker and append the specific blockers. If
task evidence is absent, preserve the existing missing-evidence path.

## Interfaces and Data Flow
- `internal/engine/progression/wave_sync.go` exposes
  `PreviewGovernedWaveExecution(root, change)` and keeps
  `SyncGovernedWaveExecution(root, change)` as the mutating surface.
- Both functions share one evaluator. The preview returns blockers after
  parsing evidence, stale checks, wave-plan loading, plan-drift checks, and
  non-pass task collection, but skips `state.SaveWaveRuns`,
  `state.SaveExecutionSummary`, and `syncCompletedTaskCheckboxes`.
- `internal/engine/progression/readiness.go` refines only S2
  run-summary-missing skill blockers for `wave-orchestration`, and only when
  no ready execution summary already exists.
- `cmd/progression_next_test.go` covers the JSON surfaces by exercising
  `next --json --diagnostics`, `validate --json`, and `status --json` on the
  same S2 fixture.

## Rollout and Rollback
Roll forward by adding the command-surface regression, extracting the preview
helper, and wiring readiness refinement. Roll back by reverting the touched
source/test files. No migration, persisted schema change, or runtime cleanup
is required.

## Risk
- Medium: read-only evaluation might accidentally write derived execution
  state. Mitigation: preview skips all write calls and regression asserts no
  `execution-summary.yaml` is created by read-only commands.
- Medium: absent task evidence could be reclassified incorrectly. Mitigation:
  refinement removes `run_summary_missing` only when preview blockers are more
  specific than `missing_task_evidence_for_run_summary`; existing missing
  evidence regression remains in place.
- Medium: JSON consumers may depend on the old blocker. Mitigation: the old
  blocker was misleading for present stale evidence; the replacement is more
  actionable and remains fail-closed.
