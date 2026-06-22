# Research

## Alternatives Considered

### Architecture
- Affected modules: `cmd/evidence.go` owns public task evidence recording,
  task result validation, ledger payload construction, and skill evidence run
  context. `internal/state/wave_execution.go` owns wave-plan materialization.
  `internal/model/change.go` and/or `internal/model/wave_execution.go` need the
  engine-owned execution-run boundary. `internal/engine/progression/wave_sync.go`
  consumes task evidence and builds execution summaries. Generated guidance flows
  through `internal/tmpl/templates/**`, `internal/toolgen/**`, and `docs/**`.
- Dependency chains:
  `tasks.md -> state.MaterializeWavePlanTransactionOpAt -> wave-plan.yaml ->
  evidence task -> runtime task evidence -> evidence skill wave-orchestration ->
  SyncGovernedWaveExecution -> execution-summary.yaml`.
- Blast radius: command surface, runtime task evidence, execution summary
  freshness, generated skill guidance, docs/manifest examples, and S2/S3
  lifecycle repair behavior.
- Constraints: `changed_files` must remain executor-provided; `task_kind`,
  `target_files`, `captured_at`, `freshness_inputs`, and `run_summary_version`
  must be Slipway-derived; mixed or malformed run-version state must fail
  closed; `next` and `validate` remain read-only.

### Patterns
- Existing command pattern: `makeEvidenceTaskCmd` resolves the active change,
  locks change state, validates the lifecycle state, loads the current wave
  plan, validates task ID/kind/verdict/paths, writes the runtime payload, then
  immediately reparses it through `progression.ParseTaskEvidence`.
- Existing state pattern: wave-plan materialization is already transaction-ready
  through `state.MaterializeWavePlanTransactionOpAt`, and S1->S2 transition
  applies that op with the `change.yaml` transition.
- Existing fail-closed pattern: `waveOrchestrationTaskEvidenceRunVersion`
  rejects ambiguous task evidence run versions before wave-orchestration
  evidence can pass.
- Existing guidance pattern: workflow host templates describe the evidence
  contract and toolgen emits command docs, generated skills, and manifest rows.
  B should change the source templates/registry, then regenerate or validate
  derived surfaces.

### Risks
- High: adding `--result-file` without an engine-owned run boundary would only
  hide the old protocol while still requiring a hidden source for run versions.
- High: relying on existing S3 repair flow for fresh re-execution is incorrect;
  `fix` and `review` do not transition back to S2 or rematerialize a wave plan.
- Medium: changing task evidence shape could weaken scope/overlap safety if
  `changed_files` is omitted for code/test/doc/ops/other tasks that require it.
- Medium: generated docs or diagnostics can keep teaching the old protocol after
  CLI support lands.
- Low: keeping old flags as a temporary compatibility path may be acceptable if
  default generated guidance and result-file help teach the compact import path.
- Guardrail domains: no user-data sensitive domain, but this is governance
  kernel behavior and must fail closed.
- Reversibility: additive fields and command path are reversible, but run
  version authority affects freshness semantics and therefore needs focused
  regression tests.

### Test Strategy
- Existing coverage: `cmd/evidence_task_test.go` covers old flag-based task
  evidence recording and parser integration; `internal/engine/progression`
  covers wave sync, ambiguous task evidence versions, scope-contract safety, and
  resume derivation; `internal/tmpl` and `internal/toolgen` cover generated
  guidance contracts.
- Coverage gaps: no compact result-file import tests; no engine-owned active
  execution run boundary; no proof that result files cannot override
  Slipway-owned fields; no S3/fix re-entry run-version increment path.
- Infrastructure needs: command-level fixtures can reuse
  `createEvidenceTaskFixture`; state/progression tests should cover run-version
  boundary helpers; template/toolgen tests should assert result import guidance
  and absence of old long ledger command guidance.
- Verification approach: run focused packages for `cmd`,
  `internal/state`, `internal/engine/progression`, `internal/tmpl`, and
  `internal/toolgen`, then `go run ./internal/toolgen/cmd/gen-surface-manifest
  --check`, then `go test ./...`.

### Options
- Option 1: Result-file wrapper over the existing long flag protocol.
  Tradeoff: smallest CLI patch, but it cannot satisfy issue #297 B because the
  run version still lacks an engine-owned source and S3 re-execution still
  cannot advance versions.
- Option 2: Add result-file import plus an engine-owned run boundary on
  wave-plan materialization, and update B-owned guidance. Tradeoff: broader
  model/state change, but it aligns with the existing S1->S2 transaction and
  keeps ledger ownership inside Slipway.
- Option 3: Add result-file import plus a full S3->S2 lifecycle reopen command.
  Tradeoff: most complete for re-execution semantics, but it widens B into a
  larger lifecycle redesign and risks colliding with Workstream A/C.
- Selected: Option 2, with an explicit plan task to make review/fix-driven
  re-execution advance the engine-owned run version through a narrow, tested
  S3 repair/re-execution hook if implementation evidence proves the current
  lifecycle cannot otherwise satisfy the acceptance criterion. This keeps the
  B slice focused while not pretending current `fix` already re-enters S2.

## Unknowns
- Resolved: Which existing wave-plan materialization path is the safest
  authority for persisting and advancing the engine-owned run version? -> The
  S1->S2 `AdvanceGoverned` materialization transaction is the safest existing
  boundary; it already writes `wave-plan.yaml` atomically with lifecycle
  authority.
- Resolved: Does current S3 -> S2 re-entry re-materialize a wave plan? -> No.
  The workflow path is forward-only, `fix` is S3-only and does not mutate
  lifecycle state, and `review` does not reopen S2.
- Resolved: Which generated templates/tests are the minimal B-owned guidance
  surface? -> `wave-orchestration/SKILL.md.tmpl`,
  `_partials/command-evidence-body.tmpl`, toolgen command/manifest examples,
  docs command references, and their template/toolgen tests.
- Remaining: None for planning. Exact field placement for the engine-owned run
  boundary (`Change` vs `WavePlan`) should be decided in implementation using
  the smallest durable model change that passes the acceptance tests.

## Assumptions
- `changed_files` remains in the compact result schema because only executors
  can attribute changed paths to a specific task in a shared worktree. Evidence:
  scope-contract tests and issue #297 B acceptance both depend on per-task
  changed-file safety.
- The long flag protocol may remain only as non-default compatibility during B.
  Evidence: issue #297 B allows a temporary host-internal path but requires the
  generated happy path to stop teaching agents the ledger fields.
- The repo does not currently check generated `.codex` or `.claude` skill
  directories in this worktree; source templates, docs, and manifest remain the
  relevant B-owned surfaces unless the generator creates them later.

## Canonical References
- `cmd/evidence.go:600-876`
- `cmd/evidence.go:918-1054`
- `cmd/evidence.go:1297-1320`
- `cmd/evidence_task_test.go:18-95`
- `internal/model/change.go:14-67`
- `internal/model/wave_execution.go:14-43`
- `internal/state/wave_execution.go:89-190`
- `internal/state/execution_summary.go:363-375`
- `internal/engine/progression/advance_governed.go:131-139`
- `internal/engine/progression/advance_governed.go:461-489`
- `internal/engine/progression/wave_sync.go:44-56`
- `internal/engine/progression/wave_sync.go:82-205`
- `internal/engine/progression/wave_boundary_evidence_test.go:60-288`
- `cmd/fix.go:82-147`
- `internal/engine/action/workflow.go:10-17`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:183-225`
- `internal/tmpl/templates/_partials/command-evidence-body.tmpl:10-44`
- `internal/toolgen/toolgen.go:333-335`
- `internal/toolgen/surface_manifest.go:154-160`
- `docs/commands.md:216-306`
- `docs/reference/commands.md:61`
- `docs/SURFACE-MANIFEST.json:304-310`
