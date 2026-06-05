# Research

## Research Findings

### Architecture
- Affected modules: `internal/engine/progression/wave_sync.go` owns wave/task evidence loading, plan-drift diagnosis, execution-summary materialization, wave-run writes, and task checklist sync. `internal/engine/progression/readiness.go` owns read-only governance readiness blockers consumed by `validate`, `status`, and `next --diagnostics`. `cmd/progression_next_test.go` provides command-surface regressions for `next`, `validate`, and `status`.
- Dependency chains: command surfaces call `progression.EvaluateGovernanceReadiness`; readiness uses `EvaluateRequiredSkillsForChange` with `execCtx.LatestRunVersion`; run-summary-bound `wave-orchestration` currently reports `run_summary_missing` when no execution summary exists. The mutating execution path already calls `SyncGovernedWaveExecution`, which can diagnose task-evidence staleness, parse issues, and `tasks_plan_changed_since_task_evidence`.
- Blast radius: limited to S2 read-only blocker rendering and wave-sync diagnostic reuse. The mutating sync path must keep existing writes: `wave-runs`, `execution-summary.yaml`, and task checklist completion.
- Constraints: `next`, `validate`, and `status` are query/readiness surfaces and must not materialize execution summaries or mutate task checkboxes. Absent task evidence must remain actionable as missing task evidence; only present but invalid/stale/drifted evidence should replace `wave-orchestration:run_summary_missing`.

### Patterns
- Existing conventions: reason-code blockers are normalized with `model.NormalizeReasonCodes`; command tests assert stable `model.ReasonSpecs`; read-only surfaces share `EvaluateGovernanceReadiness`; wave sync reports specific blockers before writing derived summaries.
- Reusable abstractions: `SyncGovernedWaveExecution` already computes the correct execution-path diagnosis, so a read-only preview can share the same core calculation without duplicating command-specific logic.
- Convention deviations: none required. The change should add a small preview wrapper and readiness refinement, not a new command surface or readiness subsystem.
- Codebase-map advisory: `artifacts/codebase` is `partial`; populated docs describe general CLI/state/test layout, but some entries still reference an older create-guard change. They are treated as weak layout context only, not as #72 authority.

### Risks
- Technical risks: medium risk of accidentally turning read-only diagnostics into a write path; medium risk of hiding the legitimate missing-evidence path; low risk of duplicate blockers on mixed parse/drift scenarios.
- Guardrail domains: `external_api_contracts`, because JSON blocker content changes on `validate --json`, `status --json`, and `next --json --diagnostics`.
- Reversibility: source-only rollback. No schema migration or persistent runtime migration is introduced.

### Test Strategy
- Existing coverage: `internal/engine/progression/wave_sync_test.go` already covers mutating sync blockers for stale task evidence, parse issues, and task-plan drift. Existing command tests cover missing task evidence.
- Infrastructure needs: a command-layer fixture with S2 state, passing wave-orchestration evidence, task evidence captured against an old wave plan, then a semantically changed `tasks.md` with no execution summary.
- Verification approach: assert `next --json --diagnostics`, `validate --json`, and `status --json` surface `tasks_plan_changed_since_task_evidence:<task>` and do not include `wave-orchestration:run_summary_missing`; assert read-only commands do not create `execution-summary.yaml`; rerun focused command tests, focused progression tests, then full Go verification.

## Alternatives Considered
- Option A: Leave read-only readiness as `wave-orchestration:run_summary_missing` until `run` materializes execution diagnostics. This preserves current behavior but keeps #72 unresolved because users cannot see that task evidence exists and is specifically stale or plan-mismatched.
- Option B: Call the mutating `SyncGovernedWaveExecution` from read-only command surfaces. This would reuse exact execution logic but violates the read-only contract by writing `execution-summary.yaml`, `wave-runs`, or task checkboxes from `next`, `validate`, or `status`.
- Option C: Extract a non-mutating wave execution preview and let S2 readiness replace only the misleading `wave-orchestration:run_summary_missing` blocker when preview blockers prove task evidence exists but is invalid, stale, or plan-drifted.
- Selected: Option C. It reuses the execution-path diagnosis, keeps command surfaces read-only, and preserves the absent-evidence path.

## Unknowns
- Resolved: Does current source reproduce #72? Yes, issue evidence and local code inspection show read-only readiness uses `execCtx.LatestRunVersion` and reports run-summary-bound missing before applying wave-sync task-evidence diagnosis.
- Resolved: Is #71 in scope? No. The operator explicitly closed #71 as not a problem.
- Remaining: None.

## Assumptions
- The issue fixture's plan-drift case is representative of the failing user experience because it has passing wave evidence, present task evidence, no derived execution summary, and a changed task plan. Evidence: issue #72 comment and `internal/engine/progression/wave_sync.go`.
- The read-only contract matters for all three surfaces. Evidence: `next` command comment says state advancement is owned by `run`, and `validate` says it inspects readiness without advancing state.

## Canonical References
- `artifacts/changes/fix-issue-72-align-read-only-s2-task-evidence-diagnostics-wi/intent.md` for the original request and intake context.
- `internal/engine/progression/wave_sync.go` for execution-path task-evidence diagnosis and sync writes.
- `internal/engine/progression/readiness.go` for shared read-only readiness blockers.
- `internal/engine/progression/evidence.go` for run-summary-bound required-skill blocker behavior.
- `cmd/next.go`, `cmd/validate.go`, and `cmd/status_view_build.go` for the affected JSON command surfaces.
- `cmd/progression_next_test.go` and `internal/engine/progression/wave_sync_test.go` for regression patterns.
