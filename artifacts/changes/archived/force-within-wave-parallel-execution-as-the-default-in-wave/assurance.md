# Assurance

## Scope Summary
Within-wave parallel execution is now the forced default in Slipway's
wave-orchestration. The engine marks a multi-task wave `parallel` at
materialization (guaranteed dependency-free and file-disjoint, with same-wave
static conflicts hardened for path aliases, parent/child targets, and
case-only aliases), surfaces the per-wave `parallel` signal in `slipway
next --json`, records the wave `dispatch_mode`, and the template-generated
wave-orchestration skill instructs hosts to dispatch a wave concurrently by
default. `execution.parallelization: off` opts a project out. Delivered across
7 tasks / 3 waves; no engine-side executor was added (Slipway stays
host-driven).

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
  `TestWaveDispatchModesFromVerification`, `TestConfig*Parallelization*`,
  `TestMaterializeWavePlan*Parallel*`,
  `TestLoadWavePlanForChangePreservesMaterializedParallel`,
  `TestBuildWaveRuns*DispatchMode`, `TestBuildWaveRunsDropsStaleDispatchModes`,
  `TestPlanWavesRejectsStaticConflictsWithPathAliases`,
  `TestPlanWavesRejectsStaticConflictsWithParentChildTargets`,
  `TestPlanWavesRejectsStaticConflictsWithCaseAliases`,
  `TestWavePlanViewFromModelSurfacesParallel`,
  `TestAuthoritativeWavePlanViewReDerivesParallelFromCurrentConfig`,
  `TestSyncGovernedWaveExecutionRecordsDegradedDispatchMode`,
  `TestWaveOrchestrationSkillForcesParallelByDefault`.

## Requirement Coverage
- REQ-001 (per-wave parallel signal) — `t-01`, `t-03`; materialize + model tests.
- REQ-002 (next --json surfaces it) — `t-04`; `cmd` view test.
- REQ-003 (skill parallel-by-default) — `t-05`, `t-06`, `t-07`; committed template + toolgen contract test. Generated `.claude/` copies are ignored outputs, not tracked source.
- REQ-004 (dispatch_mode recorded + validated + fail-open recovery) — `t-01`, `t-03`; model, parser, BuildWaveRuns, and wave-sync tests.
- REQ-005 (`parallelization` off-switch) — `t-02`, `t-03`, `t-05`; config + materialize tests.
- REQ-006 (signal excluded from freshness hashes) — `t-01`, `t-03`; hash-stability test.
- REQ-007 (static conflict hardening for default parallel safety) — `t-03`; wave planning tests for path aliases, parent/child targets, and case-only aliases.

## Residual Risks and Exceptions
- Host degradation is recorded as a structured
  `dispatch_mode:wave=<wave_index>:degraded_sequential` verification reference
  and may be explained in notes, then recovered into `WaveRun.dispatch_mode`.
  There is no dedicated
  `slipway evidence --dispatch-mode` flag yet; such a flag would improve capture
  ergonomics only. The structured contract itself is implemented through
  wave-orchestration verification references. Malformed, conflicting,
  unknown-wave, or no-longer-parallel advisory dispatch references are ignored
  rather than blocking execution sync.
- The engine does not independently observe host scheduler behavior. A
  parallel-eligible wave defaults to `dispatch_mode: parallel` unless the host
  records the degraded-sequential reference above. This is the current
  host-driven evidence boundary and does not create a completion blocker.
- Same-Go-package tasks in one wave are file-disjoint but not safe to compile
  concurrently; this is a known limitation of the single-tree, file-disjoint
  choice (surfaced honestly in the wave-orchestration evidence for Wave 1).

## Rollback Readiness
Additive change: new fields are `omitempty`, `parallelization` defaults to
forced, and the skill wording is template-only. Rollback = revert the branch; no
data migration. Verification command: `go build ./... && go vet ./... && go test ./... -count=1`.

## Archive Decision
Archived as `done`. Final closeout passed after S4 goal verification with
`closeout:evidence_freshness=pass`, `closeout:test_suite=pass:23/23`, and
`closeout:assurance_complete=pass`; the archived `change.yaml` records
`status: done` / `current_state: DONE`.
