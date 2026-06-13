# Assurance

## Scope Summary
Wave assignment moved from hand-declared `wave:` task metadata to
engine-computed scheduling: `PlanWaves` now derives each task's wave from
`depends_on` (max of dependency waves + 1; roots share wave 1) with
deterministic, task-ID-ordered, same-wave-only conflict bumping over
`target_files` (exact, parent/child, case-insensitive, glob — reusing the
pre-existing conflict semantics). The review-found deferred-root/dependent
tie-break edge case is fixed and pinned by regression coverage. The `wave:` key
is retired fail-closed with a dedicated parser error carrying remediation. The
declared-wave validation blocker
(`plan_dimension_execution_missing_wave`) left the validation logic, the
reason-code registry, and the remediation vocabulary. Planning surfaces
(`slipway instructions tasks`, plan-audit skill, tasks.md template exemplar,
docs/workflow.md) now teach the computed-wave contract and the three width
rules (honest dependencies, precise target files, same-file absorption).
Delivered across 8 tasks in 3 waves; 31 repository test files migrated off
legacy wave fixtures.

## Verification Verdict
PASS. All 8 planned tasks carry passing run-2 task evidence recorded through
`slipway evidence task` after the self-host waveless migration; the
wave-orchestration verification records full parallel subagent fan-out with
no degraded_sequential waves.

## Evidence Index
- verification/wave-orchestration.yaml — dispatch evidence: 10 executor
  handles, dispatch_mode parallel_subagents for waves 1-2, run_version 2.
- verification/integration-gate-notes.md (t-08) — gofmt clean,
  `go build ./...`, `go vet ./...`, `go test ./... -count=1` all green
  for the listed packages; dev-binary dogfood: live retirement parse_error
  quoted from this bundle's then-legacy tasks.md; positive shape via pinned CLI
  acceptance tests; `gen-surface-manifest --check` up to date; guidance text
  verified via `instructions tasks` on the dev binary.
- Runtime task evidence ledger (run 2, all 8 tasks) at
  .git/slipway/runtime/changes/<slug>/evidence/tasks.
- Self-host migration proof: re-materialized computed plan from waveless
  tasks.md reproduced the declared shape exactly — wave 1 =
  {t-01,t-02,t-03,t-06,t-07} parallel:true, wave 2 = {t-04,t-05}
  parallel:true, wave 3 = {t-08} parallel:false.
- verification/plan-audit-notes.md — original 8D audit plus two re-audit
  addenda (fixture-sweep target expansion; waveless migration).

## Requirement Coverage
- REQ-001 (computed minimal assignment): wave package tests
  (roots/fan-in/chain/depth), CLI tests
  (TestDerivedWavePlanPreviewComputesWavesFromDependencies,
  TestMaterializeWavePlanComputesWavesFromDependencies), live re-materialized
  plan shape parity. PASS.
- REQ-002 (deterministic conflict bumping): TestPlanWavesBumpsExactTargetConflictDeterministically
  (10× repeat + input-permutation equality),
  TestPlanWavesBumpsConflictingTargetsForEachConflictKind (8 conflict kinds),
  cascade and same-wave-only pins, plus
  TestPlanWavesBumpsLaterTaskIDWhenDeferredRootConflictsWithDependent for the
  independent-review ordering bug. PASS.
- REQ-003 (fail-closed retirement): TestParseTaskPlan_RejectsRetiredWaveKey,
  TestWavePlanRejectsDeclaredWaveMetadata (preview + materialization), live
  negative dogfood on this bundle. PASS.
- REQ-004 (validation vocabulary retired):
  TestValidateTasksChecklistDetailed_WavelessChecklistPassesStructuralValidation,
  TestDeclaredWaveBlockerVocabularyRetired, snapshot test updated; this
  bundle's waveless checklist validates as valid. PASS.
- REQ-005 (hash inputs drop wave): TestTaskPlanHashesAreStableForWavelessPlan;
  hash entries carry authored contract fields only (all three modes); one-time
  staleness handled live through the engine-offered reopen flow. PASS.
- REQ-006 (planning-surface guidance): pinned instructions test
  (TestInstructionsTasksGuidanceTeachesComputedWaves), plan-audit skill 8D
  rewrite, dev-binary guidance verification at t-08. PASS.
- REQ-007 (docs/manifest alignment): docs/workflow.md rewritten;
  SURFACE-MANIFEST verified registry-derived and fresh via --check; template
  exemplar aligned (TestArtifactTemplatesParseUnderEngineParsers green). PASS.

## Residual Risks and Exceptions
- One-time hash-semantics change strands other in-flight bundles' wave plans
  on first touch by the new binary (intentional, REQ-005, no shim): recovery is
  the existing public staleness flow, proven live on this very bundle. The
  main checkout currently has another active change
  (resolve-issue-163-decisions-gate) that will hit the retirement error and
  the same recovery on its next touch — expected, documented, fail-closed.
- The S2 migration UX gap found during review is fixed: the
  `tasks_checklist_invalid_format` blocker now carries the parser's
  retirement detail, so operators see the named task, retired `wave:` key,
  delete-line remediation, and real-`depends_on` guidance directly from
  validate/status/next blocker paths.
- The post-review scheduling gap is fixed: a root task deferred by a wave-1
  conflict can no longer claim a later wave ahead of a lower-ID dependent task
  that becomes eligible in that wave. Scheduling now scans each wave in sorted
  task-ID order across currently eligible tasks.
- Wider computed waves raise concurrent-executor counts; dispatch-side safety
  (overlap preflight, post-result conflict check, integration gate) is
  unchanged and owns that risk.

## Rollback Readiness
Pure code+surface change; no persisted-data migration. A revert restores the
declared-wave contract; wave-plan.yaml re-materializes from tasks.md on the
next S1→S2 pass in either direction. Bundles authored waveless would need
wave declarations restored if rolled back — same one-time staleness flow in
reverse. No rollback prerequisites beyond a normal revert commit.

## Archive Decision
Ready to stop at `done_ready` once S4 goal-verification and final-closeout
records are fresh and `validate --json` shows all governance gates approved.
Do not run `slipway done` until an explicit finalization request; the active
bundle should remain inspectable in this worktree at the done-ready boundary.
