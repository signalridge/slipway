# Tasks

## Task List

- [x] `t-01` Add failing command tests for compact task result import and ledger-field rejection
  - depends_on: []
  - target_files: ["cmd/evidence_task_test.go", "cmd/evidence_test.go", "cmd/progression_next_test.go", "cmd/repair_test.go", "internal/engine/progression/evidence_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-004, REQ-005]
  - acceptance:
    - RED tests prove `--result-file` imports a valid compact result, supports repeated files as an atomic batch, and derives ledger fields.
    - RED tests reject executor-supplied `run_summary_version`, `task_kind`, `target_files`, `captured_at`, `freshness_inputs`, and `input_hash`.
    - RED tests update missing-task-evidence diagnostics so they request result import instead of required flat ledger fields.
    - Evidence is a verdict plus `go test ./cmd ./internal/engine/progression -run 'EvidenceTask|ResultFile|missing_task_evidence|TaskEvidence'`.

- [x] `t-02` Add engine-owned execution run-version model/state tests
  - depends_on: []
  - target_files: ["internal/model/wave_execution_test.go", "internal/state/wave_execution_test.go", "internal/state/wave_execution_transaction_test.go", "internal/engine/progression/wave_boundary_evidence_test.go", "cmd/fix_test.go", "cmd/progression_next_test.go"]
  - task_kind: test
  - covers: [REQ-003, REQ-004, REQ-007]
  - acceptance:
    - RED tests prove materialized execution authority carries an engine-owned run version.
    - RED tests prove ambiguous existing task evidence blocks result import or run-version derivation.
    - RED tests cover the user-visible S3 repair/re-execution entry point and prove it advances or rematerializes active execution authority before later task result import uses the newer version.
    - Evidence is a verdict plus focused state/progression/cmd test transcript.

- [x] `t-03` Implement result-file task evidence import and derived ledger fields
  - depends_on: ["t-01", "t-02"]
  - target_files: ["cmd/evidence.go", "internal/engine/progression/wave_sync.go", "cmd/path_helpers.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-004, REQ-005]
  - acceptance:
    - `go run . evidence task --help` shows `--result-file` as the supported compact import path and documents repeated files as atomic batch import.
    - Valid result JSON writes parseable task evidence with engine-derived `task_kind`, `run_summary_version`, `target_files`, `captured_at`, and `freshness_inputs`.
    - Invalid result JSON fails closed without writing runtime task evidence, including no partial writes for a multi-file batch.
    - Evidence is task evidence command output plus focused command test transcript.

- [x] `t-04` Implement engine-owned active execution run boundary and re-execution version advancement
  - depends_on: ["t-01", "t-02"]
  - target_files: ["internal/model/change.go", "internal/model/wave_execution.go", "internal/state/wave_execution.go", "internal/state/execution_repair.go", "internal/engine/progression/advance_governed.go", "cmd/fix.go", "cmd/repair.go", "cmd/review.go", "internal/engine/action/workflow.go"]
  - task_kind: code
  - covers: [REQ-003, REQ-007]
  - acceptance:
    - S1->S2 materialization creates active execution run version 1 transactionally with wave-plan authority.
    - A bounded S3 repair/re-execution hook either preserves the forward workflow and explicitly advances execution authority, or if workflow transitions change, the workflow path and tests are updated in the same task.
    - Subsequent task result import after S3 repair uses the newer active run version and stale review evidence no longer satisfies current-run gates.
    - Evidence is a verdict plus focused lifecycle/run-version test transcript.

- [x] `t-05` Update generated guidance source templates and toolgen contract tests
  - depends_on: ["t-03", "t-04"]
  - target_files: ["internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl", "internal/tmpl/templates/_partials/command-evidence-body.tmpl", "internal/tmpl/templates/_partials/command-fix-body.tmpl", "internal/tmpl/templates_test.go", "internal/toolgen/toolgen.go", "internal/toolgen/surface_manifest.go", "internal/toolgen/toolgen_test.go"]
  - task_kind: code
  - covers: [REQ-006]
  - acceptance:
    - Template/toolgen tests assert wave-orchestration and evidence guidance use result JSON import.
    - Generated/default agent guidance does not teach agents to choose `run_summary_version`, `task_kind`, or `target_files`.
    - Generated/default fix guidance distinguishes ordinary review finding discovery from explicit `--start-reexecution` lifecycle transition.
    - Toolgen manifest example token uses `slipway evidence task --result-file <path> [--result-file <path> ...] --json`.
    - Evidence is a verdict plus `go test ./internal/tmpl ./internal/toolgen`.

- [x] `t-06` Regenerate/update B-owned docs and surface manifest examples
  - depends_on: ["t-05"]
  - target_files: ["docs/commands.md", "docs/operator-guide.md", "docs/reference/commands.md", "docs/SURFACE-MANIFEST.json"]
  - task_kind: doc
  - covers: [REQ-006]
  - acceptance:
    - Docs, operator guidance, and manifest examples present result-file task import as the public/default path and document repeated `--result-file` values as atomic batch import.
    - Search for the old long `evidence task --task-id ... --run-summary-version ... --task-kind ...` command has no live agent-facing docs/manifest hits outside legacy tests or internal compatibility references.
    - Evidence is an artifact diff plus `go run ./internal/toolgen/cmd/gen-surface-manifest --check`.

- [x] `t-07` Run focused verification, surface manifest check, and full suite
  - depends_on: ["t-03", "t-04", "t-05", "t-06"]
  - target_files: ["artifacts/changes/add-task-result-import/verification/**"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
  - acceptance:
    - `go test ./cmd ./internal/engine/progression ./internal/engine/wave ./internal/state ./internal/toolgen ./internal/tmpl` passes.
    - `go run ./internal/toolgen/cmd/gen-surface-manifest --check` passes.
    - `go test ./...` passes.
    - Black-box help checks show `evidence task --help` documents `--result-file`, `run --help` remains compatible with existing resume behavior, and `evidence suite-result --help` remains present.
    - Evidence is a verification checklist and command transcript references under `artifacts/changes/add-task-result-import/verification`.
