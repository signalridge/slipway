# Assurance

## Scope Summary
Within-wave parallel execution is now the forced default in Slipway's
wave-orchestration. The engine marks a multi-task wave `parallel` at
materialization (already guaranteed dependency-free and file-disjoint), surfaces
the per-wave `parallel` signal in `slipway next --json`, records the wave
`dispatch_mode`, and the generated wave-orchestration skill instructs hosts to
dispatch a wave concurrently by default. `execution.parallelization: off` opts a
project out. Delivered across 7 tasks / 3 waves; no engine-side executor was
added (Slipway stays host-driven).

## Verification Verdict
Pass. `go build ./...`, `go vet ./...`, and the changed-package test suites
(`internal/model`, `internal/state`, `cmd`, `internal/toolgen`) are green,
including new tests for every requirement. Final full-suite verification is
captured at S4 goal-verification.

## Evidence Index
- Task evidence: `t-01`..`t-07` recorded via `slipway evidence task` (run_version 1).
- `verification/wave-orchestration.yaml` (execution), `verification/plan-audit.yaml`,
  `verification/research-orchestration.yaml`, `verification/intake-clarification.yaml`.
- `verification/execution-summary.yaml` (engine-generated).
- Tests: `TestWavePlanWaveParallelRequiresMultipleTasks`, `TestWaveRunValidateDispatchMode`,
  `TestConfig*Parallelization*`, `TestMaterializeWavePlan*Parallel*`,
  `TestBuildWaveRunsDerivesParallelDispatchMode`, `TestWavePlanViewFromModelSurfacesParallel`,
  `TestWaveOrchestrationSkillForcesParallelByDefault`.

## Requirement Coverage
- REQ-001 (per-wave parallel signal) — `t-01`, `t-03`; materialize + model tests.
- REQ-002 (next --json surfaces it) — `t-04`; `cmd` view test.
- REQ-003 (skill parallel-by-default) — `t-05`, `t-06`, `t-07`; toolgen contract test.
- REQ-004 (dispatch_mode recorded + validated) — `t-01`, `t-03`; model + BuildWaveRuns tests.
- REQ-005 (`parallelization` off-switch) — `t-02`, `t-03`, `t-05`; config + materialize tests.
- REQ-006 (signal excluded from freshness hashes) — `t-01`, `t-03`; hash-stability test.

## Residual Risks and Exceptions
- Host degradation is recorded as verification prose, not a structured per-wave
  field; the engine derives `dispatch_mode: parallel` for parallel waves. A
  structured host-recorded dispatch mode (evidence `--dispatch-mode` flag) is an
  accepted deferred follow-up (noted in intent.md Deferred Ideas).
- Same-Go-package tasks in one wave are file-disjoint but not safe to compile
  concurrently; this is a known limitation of the single-tree, file-disjoint
  choice (surfaced honestly in the wave-orchestration evidence for Wave 1).

## Rollback Readiness
Additive change: new fields are `omitempty`, `parallelization` defaults to
forced, and the skill wording is template-only. Rollback = revert the branch; no
data migration. Verification command: `go build ./... && go vet ./... && go test ./... -count=1`.

## Archive Decision
Not yet archivable. Archive readiness requires fresh S4 goal-verification and a
final `validate --json` freshness/readiness proof captured on the active change
before `done`; this assurance record is authored at S3_REVIEW and does not claim
post-archive revalidation.
