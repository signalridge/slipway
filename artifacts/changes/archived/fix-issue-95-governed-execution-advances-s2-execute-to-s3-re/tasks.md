# Tasks
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; generated skills/docs via toolgen; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` (#95) Enforce execution completeness in the wave sync. In
  `internal/engine/progression/wave_sync.go` add `IncompleteExecutionTaskBlockers(plan, runs)`
  (planned `WavePlan.TaskIDs()` minus tasks with a recorded run → one
  `incomplete_execution_task:<taskID>` each) and call it in BOTH the preview and
  mutate branches of `evaluateGovernedWaveExecution`, guarded by
  `len(planDriftBlockers) == 0`. Register the canonical `incomplete_execution_task`
  reason (`internal/model/reason_code.go`) and its recovery remediation
  (refresh-execution class) in `internal/model/recovery.go`. Test-first: extend
  `wave_sync_test.go` (missing-task blocks, all-recorded advances, drift
  suppresses) and the `recovery_test.go` contract (add to the recovery-relevant
  exact set, `sampleRecoveryDetail`, and `inScopeProducedRecoverySpecs`).
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/progression/wave_sync.go", "internal/engine/progression/wave_sync_test.go", "internal/model/reason_code.go", "internal/model/recovery.go", "internal/model/recovery_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-03` (#88) Remove the false-promise `--focus sast` from the `repair`
  command surface. Delete the `repair`+`sast` `SurfaceRecord` in
  `internal/engine/capability/surfaces.go` (keep `review`/`validate`); update the
  `sast-orchestration` summary/comment in
  `internal/engine/capability/registry_b3.go` to drop `repair` from its trigger
  list; enhance `validateFocus` in `cmd/route_flags.go` to redirect a
  `repair`+`sast` selection to `slipway review --focus sast` /
  `slipway validate --focus sast`. Test-first: update `surfaces_test.go`
  (repair → no explicit focuses), `route_flags_test.go` (drop the repair+sast
  accept case, add a rejection+redirect test), and `route_surface_command_test.go`
  (drop the repair sast-orchestration mode case).
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/capability/surfaces.go", "internal/engine/capability/surfaces_test.go", "cmd/route_flags.go", "cmd/route_flags_test.go", "internal/engine/capability/registry_b3.go", "cmd/route_surface_command_test.go"]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-02` (#88) Make high-risk blockers carry the next action and surface the
  required token in the handoff. In `internal/model/recovery.go` upgrade the
  `high_risk_check_missing` / `high_risk_check_failed` remediations to name the
  exact `high_risk_check:<domain>.safety_baseline` token, the producing skill
  (goal-verification), and a real SAST run. Add a `required_high_risk_tokens`
  field to the `slipway next` skill-constraints surface
  (`cmd/next.go` / `cmd/next_skill.go`), populated from
  `gate.RequiredHighRiskChecks(change.GuardrailDomain)` when the next skill is
  goal-verification and a guardrail domain is set. Test-first: extend
  `recovery_test.go` (remediation names token + goal-verification; expand
  `sampleRecoveryDetail` to `<domain>.safety_baseline`; add to
  `inScopeProducedRecoverySpecs`) and `next_skill_constraints_test.go`
  (token populated when guardrail domain set).
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/model/recovery.go", "internal/model/recovery_test.go", "cmd/next.go", "cmd/next_skill.go", "cmd/next_skill_constraints_test.go"]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-05` (worktree) Provision a dedicated worktree for every governed change
  (engine/state core). Add a `governance.auto_provision_worktree` config (default
  true) to `internal/model/config.go`; in
  `internal/state/worktree.go` `EnsureDefaultWorktreeForChange`, replace the
  `!change.NeedsDiscovery` skip with the config gate so ALL governed changes
  provision `.worktrees/<slug>` on `feat/<slug>` (keep the git-availability
  skips; when disabled, skip with `worktree_provisioning_disabled`). Test-first:
  extend the worktree/state tests (non-discovery change binds a worktree when
  enabled; skips with the disabled reason when off; git-availability skips
  intact).
  - wave: 1
  - depends_on: []
  - target_files: ["internal/model/config.go", "internal/state/worktree.go", "internal/state/worktree_test.go"]
  - task_kind: code
  - covers: [REQ-008]

- [x] `t-06` (worktree) Wire worktree provisioning into `slipway new` and keep
  the single-active-change guard correct. In `cmd/new.go`, ensure
  `EnsureDefaultWorktreeForChange` and the conflict guard order so the guard sees
  the change's target worktree, and update `newChangeTargetWorkspaceRoot` (in
  `cmd/common.go`) to use the bound/would-be worktree path so a worktree-isolated
  change does not wrongly block creation. Test-first: update `cmd/new_test.go`
  and `cmd/common_test.go` (set `auto_provision_worktree=false` where worktrees
  are not under test; add coverage that a non-discovery change binds a worktree
  and that isolated changes do not conflict); update any affected
  `cmd/cli_e2e_test.go` bundle-path assertions.
  - wave: 2
  - depends_on: [t-05]
  - target_files: ["cmd/new.go", "cmd/common.go", "cmd/new_test.go", "cmd/common_test.go", "cmd/cli_e2e_test.go"]
  - task_kind: code
  - covers: [REQ-008]

- [x] `t-04` (#88, #95) Update generated-skill sources and regenerate all
  surfaces with zero drift. Edit
  `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` (state the
  completeness-before-review contract; rescope to drop a task — REQ-003),
  `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl` (document
  recording `high_risk_check:<domain>.safety_baseline=pass|fail` from a real SAST
  run when a guardrail domain is set; fail-closed — REQ-004), and
  `internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl` (clarify the
  guardrail-baseline recheck/reuse). Regenerate all generated skills, command
  references, and docs via the toolgen self-loop and confirm zero drift; run
  `go build ./... && go vet ./... && go test ./...`. Regeneration also absorbs
  any generated-surface drift from the repair-focus (t-03) and worktree (t-06)
  changes, including the `sast-orchestration/SKILL.md` frontmatter (its
  `bindings`/`summary`/`trigger_signals` must mirror the registry's dropped
  `repair` binding so the binding-compare gate stays green).
  - wave: 3
  - depends_on: [t-01, t-02, t-03, t-05, t-06]
  - target_files: ["internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl", "internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl", "internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl", "internal/tmpl/templates/skills/sast-orchestration/SKILL.md"]
  - task_kind: doc
  - covers: [REQ-003, REQ-004, REQ-007]
