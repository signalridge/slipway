# Research

## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; generated skills/docs via toolgen; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Research Findings

### Architecture
- Affected modules:
  - #95: `internal/engine/progression/wave_sync.go` (`evaluateGovernedWaveExecution`),
    `internal/model/reason_code.go`, `internal/model/recovery.go`,
    `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`.
  - #88: `internal/engine/gate/gate.go` (highRiskCatalog/EvaluateGShip),
    `internal/engine/progression/evidence.go` (ExtractHighRiskChecks/
    ParseHighRiskCheckReference), `internal/engine/progression/authority.go`
    (buildShipAuthorityFromReadiness), `internal/model/recovery.go`,
    `internal/engine/capability/surfaces.go`, `internal/engine/capability/registry_b3.go`,
    `cmd/route_flags.go`, `cmd/next.go`/`cmd/next_skill.go`, and the
    goal-verification / final-closeout SKILL templates.
  - worktree: `internal/state/worktree.go` (`EnsureDefaultWorktreeForChange`),
    `internal/model/config.go` (ConfigGovernance), `cmd/new.go`, `cmd/common.go`.
- Dependency chains:
  - `slipway run` → `AdvanceGoverned` → (S2) `SyncGovernedWaveExecution`
    → `LoadWavePlanForChange` + `LoadExecutionTasksFromEvidence` → blockers.
  - G_ship → `EvaluateShipAuthority` → `buildShipAuthorityFromReadiness`
    → `ExtractHighRiskChecks(verifyPassingSkills)` → `gate.EvaluateGShip`.
  - `slipway new` → `createDirectGovernedChange` → `EnsureDefaultWorktreeForChange`.
- Blast radius: governance kernel + CLI surfaces + generated skills/docs + tests.
  The worktree change touches change-creation; ~9 `slipway new`-based tests are
  affected, while ~250+ engine tests construct `model.Change` directly and are
  unaffected.
- Constraints: sensitive-domain gates stay fail-closed (no bypass); the wave-plan
  is the authority for the full task set; the goal-verification verification
  record's References already feed the high-risk gate; toolgen self-loop must
  report zero drift.

### Patterns
- Existing conventions:
  - Reason codes registered in `canonicalReasonDefinitions` (reason_code.go) with
    recovery entries in `blockerRemediations` (recovery.go); contract tests in
    `recovery_test.go` enforce remediation coverage
    (`recoveryRelevantCanonicalCodes`, `sampleRecoveryDetail`,
    `inScopeProducedRecoverySpecs`).
  - High-risk checks satisfied by reference tokens
    `high_risk_check:<id>=<verdict>` parsed by `ParseHighRiskCheckReference`
    against `gate.IsRegisteredCheckID`.
  - Explicit `--focus` aliases registered per command in
    `capability/surfaces.go` (review/validate/repair × focus); `validateFocus`
    rejects unknown aliases with an allowed-list.
  - Worktree default path `.worktrees/<slug>` and branch `feat/<slug>` already
    exist in `EnsureDefaultWorktreeForChange`; only the `!NeedsDiscovery` skip
    gates it off.
- Reusable abstractions: `WavePlan.TaskIDs()`/`TotalTasks` (full planned set),
  `ExecutionSummary.TaskRunMap()`, `gate.RequiredHighRiskChecks(domain)`,
  `ConfigGovernance` (config gating), `ResolveChangePaths`/`changeWorkspaceRoot`
  (bundle routes into the worktree automatically when bound).
- Convention deviations: none required; all fixes extend existing patterns.

### Risks
- Technical risks:
  - Fail-closed integrity (high): the #88 satisfy-path MUST NOT add a bypass or a
    CLI that stamps the safety_baseline without a real goal-verification verdict.
    Mitigation: the token comes only from goal-verification References (already
    validated by `ParseHighRiskCheckReference`); no new stamp command.
  - Test blast radius (medium): worktree-at-creation changes `slipway new`
    behavior. Mitigation: `governance.auto_provision_worktree` config (default
    true) lets the ~9 affected `new`-based tests disable it centrally; engine
    tests that build `model.Change` directly are unaffected.
  - Single-active-change guard correctness (medium): with every change in its own
    worktree, the worktree-scoped guard (#50) must treat each as isolated.
    Mitigation: order worktree binding vs the guard and have
    `newChangeTargetWorkspaceRoot` use the bound/would-be worktree path.
  - False positives in the #95 gate (low): a planned task could be intentionally
    dropped. Mitigation: rescoping `tasks.md` removes it from `WavePlan.TaskIDs()`
    (the documented out-of-scope path); the check is suppressed under plan-drift.
- Guardrail domains: none for THIS change (it edits Slipway's own engine; the
  safety_baseline gate is edited but kept fail-closed). Public CLI/JSON/generated
  skills/docs reviewed as external contracts.
- Reversibility: high — additive reason code, additive config flag (default
  preserves current high-risk behavior), focus removal is a surface change. All
  guarded by tests.

### Test Strategy
- Existing coverage: `wave_sync_test.go`, `recovery_test.go` (contract),
  `gate_test.go`, `evidence` parse tests, `surfaces_test.go`,
  `route_flags_test.go`, `new_test.go`, `next_skill_constraints_test.go`.
- Infrastructure needs: a config-driven test default for worktree provisioning
  (`auto_provision_worktree=false`) in `new`-based tests; no new mocks otherwise.
- Verification approach per acceptance criterion:
  - #95: table-driven wave_sync tests (missing-task blocks, all-recorded
    advances, drift suppresses) + recovery contract tests.
  - #88: gate/evidence tests for the domain token, recovery remediation tests,
    surfaces/route_flags tests for repair-focus removal + redirect,
    next-handoff token test.
  - worktree: worktree_test (bind/skip/disabled), new_test (non-discovery binds),
    common_test (guard isolation).
  - cross-cutting: `go build/vet/test` + toolgen zero-drift self-loop.

## Alternatives Considered
- **#88 safety_baseline producer** — (1) a new dedicated `guardrail-baseline`
  skill made conditionally required; (2) **goal-verification documents + records
  the token** (selected). goal-verification already runs at S4 and its
  References already feed `ExtractHighRiskChecks`, so the satisfy-path already
  exists structurally — the only gap is guidance + actionable remediation. No new
  required skill, smallest change, fail-closed.
- **#88 repair --focus sast** — (1) make `repair` actually run semgrep; (2) emit
  a routing hint from repair; (3) **remove the false focus from repair, redirect
  to review/validate** (selected). repair is bounded local-state integrity and
  must never run external scanners; removing the false promise + redirect is the
  honest, smallest design.
- **#95 incomplete execution** — (1) a single aggregate blocker; (2) **one
  `incomplete_execution_task:<taskID>` per missing task** (selected, mirrors
  `non_pass_task`, names the next task). Recovery: fail-closed complete-or-rescope
  (selected) vs a new defer/skip bypass (rejected — it would weaken the gate).
- **worktree provisioning** — (A) **default-true unconditional with config
  opt-out** (selected); (B) preset/complexity-gated; (C) opt-in default-false.
  A directly frees the main checkout for all governed changes (the user's goal),
  is the most consistent, and `git worktree add` is cheap; opt-out preserves
  control. Confirmed with the user as "A".
- Selected: the per-concern selections above, all confirmed with the user; the
  locked decision is recorded in `decision.md`.

## Unknowns
- Resolved: which skill produces `<domain>.safety_baseline` -> goal-verification
  (its References already feed the gate via authority.go).
- Resolved: worktree test blast radius -> bounded to `slipway new`-based tests;
  the config flag minimizes churn.
- Resolved: bundle location when worktree-bound -> `ResolveChangePaths` routes the
  bundle into the worktree automatically.
- Remaining: None.

## Assumptions
- goal-verification is the sole producer of high-risk check verdicts - Evidence:
  `internal/engine/progression/authority.go:145,171` (verifyPassingSkills feeds
  ExtractHighRiskChecks) and `internal/engine/progression/evidence.go:140-203`.
- The S2 worktree gate and `applyPendingWorktreePreflight` short-circuit when a
  change is already worktree-bound - Evidence:
  `internal/engine/progression/advance_governed.go:178,411` (guards on
  `WorktreePath == ""`).
- Most engine/state tests bypass `EnsureDefaultWorktreeForChange` - Evidence: it
  is called only from `cmd/new.go:475`.

## Canonical References
- `internal/engine/progression/wave_sync.go` (#95 sync + blockers)
- `internal/engine/gate/gate.go`, `internal/engine/progression/evidence.go`,
  `internal/engine/progression/authority.go` (#88 high-risk gate + token flow)
- `internal/engine/capability/surfaces.go`, `internal/engine/capability/registry_b3.go`,
  `cmd/route_flags.go` (#88 repair-focus surface)
- `internal/state/worktree.go`, `internal/model/config.go`, `cmd/new.go`,
  `cmd/common.go` (worktree provisioning)
- `internal/model/reason_code.go`, `internal/model/recovery.go`,
  `internal/model/recovery_test.go` (reason codes + recovery contract)
