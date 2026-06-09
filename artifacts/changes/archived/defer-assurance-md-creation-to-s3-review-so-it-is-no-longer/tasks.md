# Tasks

## Task List

- [x] `t-01` Add `assurance.md` to `deferredToSkillAuthoring` so the engine stops scaffolding it during S1_PLAN bundle creation
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/artifact/manager.go]
  - task_kind: code
  - covers: [REQ-001]

- [x] `t-02` Make `AssuranceContractBlockers` the sole owner of assurance.md existence by skipping it in the generic pre-S3 existence gates (`GovernedBundleBlockers` and the artifact-readiness evaluator) via one shared predicate
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/progression/validation.go, internal/engine/progression/readiness.go, internal/state/repair.go]
  - task_kind: code
  - covers: [REQ-002, REQ-003]

- [x] `t-03` Remove the obsolete plan-audit-digest exclusion rationale for assurance.md in `addPlanningArtifactInputs`
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/progression/evidence_digests.go]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-04` Align the cmd surfaces to deferred authoring: rewrite the `assurance` guidance in `instructionsGuidance` (drop "engine may create the scaffold; replace it") and correct the `slipway preset` re-scaffold comment that still claimed it materializes assurance.md
  - wave: 2
  - depends_on: [t-01]
  - target_files: [cmd/instructions.go, cmd/preset.go]
  - task_kind: code
  - covers: [REQ-004]

- [x] `t-05` Align the generated host-skill templates: plan-audit must not list assurance.md as required-present at S1; final-closeout must not assume an engine-created early scaffold
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: [internal/tmpl/templates/skills/plan-audit/SKILL.md, internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl]
  - task_kind: doc
  - covers: [REQ-004]

- [x] `t-06` Align end-user docs that describe assurance.md scaffolding/timing, and the pivot/rescope command docs (valid rescope states and the non-destructive scope-drift recovery path)
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: [docs/workflow.md, docs/design.md, docs/commands.md, docs/operator-guide.md, docs/index.md]
  - task_kind: doc
  - covers: [REQ-004, REQ-006]

- [x] `t-08` Make `slipway pivot --rescope` reachable from S2_EXECUTE/S3_REVIEW/S4_VERIFY (not S2 only) so out-of-scope drift recovery is reachable after a stale-evidence reopen; keep it rejected before execution (S0_INTAKE/S1_PLAN)
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/gate/gate.go, cmd/pivot_validation.go]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-09` Make the scope_contract_drift recovery guidance accurate: lead with the non-destructive amend-tasks.md-target_files path and stop describing `pivot --rescope` as a non-destructive target_files edit (state that it resets the change to S0_INTAKE)
  - wave: 2
  - depends_on: [t-02]
  - target_files: [internal/model/recovery.go, internal/engine/progression/readiness.go]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-07` Add/update tests for the deferred behavior and realign pinned tests broken by the surface changes: assurance.md not scaffolded on standard/strict and absent on light; no missing-block before S3; fail-closed (missing/scaffold) at S3+; bundle-readiness no longer requires assurance.md; instructions guidance describes deferred authoring; plan-audit digest excludes assurance without special-case prose; repair diagnostic silent pre-S3 and error at S3+; rescope reachable from S2/S3/S4 and rejected before execution; scope-drift guidance leads with the non-destructive path and describes rescope honestly
  - wave: 3
  - depends_on: [t-01, t-02, t-03, t-08, t-09]
  - target_files: [internal/engine/artifact/manager_test.go, internal/engine/progression/validation_test.go, internal/engine/progression/evidence_digests_test.go, cmd/instructions_test.go, cmd/progression_next_test.go, internal/state/repair_test.go, internal/engine/gate/gate_test.go, cmd/pivot_validation_test.go, cmd/lifecycle_commands_test.go, cmd/scope_contract_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
