# Research

## Research Findings

### Architecture
- Affected modules: `internal/engine/progression/advance_governed.go` owns
  mutating lifecycle advancement and currently returns blocked immediately when
  `LoadRelevantExecutionSummaryContext` reports execution-summary issues
  (`advance_governed.go:95-100`). This is the point that turns S3/S4 stale
  planning drift into a dead end instead of a recovery transition.
- Affected modules: `internal/state/execution_summary.go` classifies planning
  drift separately from execution drift. `collectExecutionSummaryIssuesFromDiagnostics`
  emits `stale_planning_evidence` when stale pairs contain planning evidence
  (`execution_summary.go:288-309`), while `stalePlanningPairs` builds the
  plan-audit -> wave-plan -> execution-summary evidence chain
  (`execution_summary.go:553-686`).
- Affected modules: `cmd/next_skill_view.go` resolves the user/agent handoff.
  Its no-skill branch only adds `run_slipway_run_to_advance` when no blockers
  exist (`next_skill_view.go:193-210`), so a stale-planning blocker currently
  lacks a machine-actionable recovery hint.
- Affected modules: `cmd/pivot_validation.go` and
  `internal/engine/gate/gate.go` disagree on rescope boundaries. The CLI
  restricts `--rescope` to `S2_EXECUTE` (`pivot_validation.go:24-33`), while
  `EvaluateGPivot` allows rescope from S1/S2/S3/S4 (`gate.go:93-99`).
- Dependency chains:
  `next/run` -> `progression.AdvanceGoverned` -> `state.LoadRelevantExecutionSummaryContext`
  -> `state.ExecutionSummaryFreshnessDiagnostics`; and
  `S1_PLAN/audit` -> `state.MaterializeWavePlan` -> `S2_EXECUTE`
  -> `SyncGovernedWaveExecution` -> `state.SaveExecutionSummary`.
- Blast radius: lifecycle transition semantics, JSON reason codes, and
  recovery diagnostics for governed changes in S3/S4. This is an external API
  contract domain because `next/run/validate/pivot --json` surfaces change.
- Constraints: stale planning and scope drift must stay fail-closed; recovery
  must not delete runtime task evidence under `.git/slipway/runtime/changes`.

### Patterns
- Existing conventions: recovery-only planning already uses
  `S1_PLAN/validate` with `RecoveryOnly` summaries for post-audit machine
  validation failures (`advance_governed.go:302-320`), so reopening an S1
  substep is an established lifecycle pattern.
- Existing conventions: mutating advancement emits side effects and lifecycle
  events from `AdvanceSummary` (`advance_governed.go:518-632`). New recovery
  should use the same summary path instead of a command-specific side channel.
- Existing conventions: planning evidence is stored in the governed bundle
  verification directory (`verification/plan-audit.yaml`, `wave-plan.yaml`,
  `execution-summary.yaml`); runtime task evidence is stored separately under
  the git-common-dir runtime path. That separation lets recovery clear derived
  bundle verification while preserving task ledgers.
- Reusable abstractions: `state.RemoveExecutionSummary`, `state.WavePlanFileName`,
  `state.ExecutionSummaryFileName`, `model.NewReasonCode`, and
  `model.Change` substep helpers cover the needed changes without a new
  persistent state field.
- Convention deviations: none required. A new public command would be a larger
  API addition and is not needed for Tier 3.

### Risks
- High: deleting too much execution state would violate the Tier 3
  non-destructive recovery requirement. Recovery must not call
  `ResetPivotExecutionResidue`, `ResetWaveExecution`, or remove the runtime
  change directory.
- High: reusing stale `plan-audit.yaml` or `wave-plan.yaml` after reopening
  planning would falsely pass. Recovery must clear those derived files before
  presenting `plan-audit`.
- Medium: preserving old wave-orchestration evidence can be unsafe if
  `tasks.md` changed semantically after task evidence capture. Existing
  `SyncGovernedWaveExecution` checks task evidence timestamps against the
  current tasks plan and emits `tasks_plan_changed_since_task_evidence` blockers
  (`wave_sync.go:530-558`), so preservation remains fail-closed.
- Medium: allowing `pivot --rescope` from S3/S4 would offer a destructive path
  for benign drift. Aligning `EvaluateGPivot` down to the CLI's S2 rescope
  boundary avoids advertising that as the recovery mechanism.
- Reversibility: code changes are normal CLI/state-machine changes and can be
  rolled back. Runtime recovery effects are explicit file removals from the
  active governed bundle verification directory only.

### Test Strategy
- Existing coverage: `cmd/validate_artifact_gate_test.go` and
  `cmd/review_test.go` already assert stale planning evidence remains a
  blocker; `internal/state/execution_summary_test.go` asserts planning drift is
  classified separately from stale execution evidence.
- Add coverage: `next --json --diagnostics` from S3/S4 with stale planning
  evidence should show an actionable `run_slipway_run_to_advance` recovery
  route and should not point to a stale host skill as the only action.
- Add coverage: `run --json --diagnostics` from S3/S4 with stale planning
  evidence should transition to `S1_PLAN/audit`, clear only
  `plan-audit.yaml`, `wave-plan.yaml`, and `execution-summary.yaml`, preserve
  runtime task evidence and wave-orchestration evidence, and expose
  `plan-audit` next.
- Add coverage: after refreshed plan-audit, `run` should regenerate
  `wave-plan.yaml`, then S2 wave sync should rebuild `execution-summary.yaml`
  or fail closed on `tasks_plan_changed_since_task_evidence`.
- Add coverage: `EvaluateGPivot` and CLI pivot preconditions agree on reroute
  and rescope states.

## Alternatives Considered

- Approach 1: use `pivot --rescope` for S3/S4 stale planning recovery.
  Tradeoff: reuses an existing command but calls the destructive pivot path,
  which clears runtime evidence and returns to S0 intake. Rejected because Tier
  3 requires non-destructive recovery for benign drift.
- Approach 2: add a new callable `plan-audit refresh` command surface.
  Tradeoff: explicit and targeted, but adds a new public API surface and still
  needs lifecycle mutations, evidence invalidation, and `next` guidance.
  Deferred because the existing `run` lifecycle surface can own this recovery.
- Approach 3: make `run` reopen `S1_PLAN/audit` when S3/S4 readiness reports
  `stale_planning_evidence`, clear only stale derived planning/execution
  verification files, preserve runtime task evidence, and let the normal
  plan-audit -> wave-plan -> execution-summary path refresh the chain.
  Selected: it is the smallest state-machine change that provides an
  actionable route, preserves useful runtime evidence, and keeps stale inputs
  fail-closed through existing wave-sync guards.


## Unknowns
- Resolved: whether pivot should be the recovery surface -> no; it is
  intentionally destructive and should stay separate from benign stale-planning
  recovery.
- Resolved: whether preserving wave-orchestration evidence is safe -> yes when
  paired with existing task-plan drift blockers in `SyncGovernedWaveExecution`.
- Remaining: None.


## Assumptions
- The recovery route should cover S3 and S4 only, matching the issue comment's
  B1/B2 scope. Evidence: issue #53 comment `4604547893`.
- Existing stale-planning classification is trusted. Evidence:
  `internal/state/execution_summary.go:288-309` and
  `internal/state/execution_summary_test.go:952-985`.
- Runtime task evidence must be preserved. Evidence: issue #53 Tier 3 says to
  preserve useful execution evidence while invalidating only downstream stale
  planning artifacts.


## Canonical References
- `artifacts/changes/complete-issue-53-tier-3-stale-planning-recovery-state-machi/intent.md` for the original request and intake context.
- `https://github.com/signalridge/slipway/issues/53#issuecomment-4604547893`
  for Tier 3 scope and expected tests.
- `internal/engine/progression/advance_governed.go`
- `internal/state/execution_summary.go`
- `cmd/next_skill_view.go`
- `cmd/pivot_validation.go`
- `internal/engine/gate/gate.go`
- `internal/engine/progression/wave_sync.go`
