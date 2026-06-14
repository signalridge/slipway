# Tasks

## Task List

- [x] `t-01` Add `ExemptContextFiles` to `scopecontract.Report` (and `Clone`), and populate it in readiness from the dirty `artifacts/codebase/**` files the scope-contract filter currently drops silently; unit-test that exempted files land in the field while staying out of `changed_files`.
  - depends_on: []
  - target_files: [internal/engine/scopecontract/evaluate.go, internal/engine/progression/readiness.go, internal/engine/progression/readiness_optimization_test.go]
  - task_kind: code
  - covers: [REQ-001]
- [x] `t-02` Surface `exempt_context_files` on the shared `scopeContractView` and populate it in `buildScopeContractView` so `validate`/`status`/`review` JSON all disclose it; cmd test asserts the field appears with changed_files still omitting the file and status pass.
  - depends_on: [t-01]
  - target_files: [cmd/validate.go, cmd/validate_test.go]
  - task_kind: code
  - covers: [REQ-001]
- [x] `t-03` Document the `artifacts/codebase/**` scope-contract exemption and the new `exempt_context_files` field in the user-facing docs.
  - depends_on: []
  - target_files: [docs/commands.md, docs/operator-guide.md]
  - task_kind: code
  - covers: [REQ-001]
- [x] `t-04` Stop emitting `run_summary_version=0` when no execution summary exists: make the `run_summary_version` JSON field omit-on-zero on `status` and `next`, map it through the view builders, and keep human output from printing a 0; cmd test asserts omission with no summary and the real value once a run exists.
  - depends_on: []
  - target_files: [cmd/status.go, cmd/status_view_build.go, cmd/next.go, cmd/next_context_build.go, cmd/status_test.go]
  - task_kind: code
  - covers: [REQ-002]
- [x] `t-05` Make the first task-evidence run version (`1`) discoverable via the `evidence task` help/guidance surface while keeping the `>=1` rejection intact; cmd test asserts the guidance text and that `--run-summary-version 0` still fails with `evidence_task_run_summary_version_invalid`.
  - depends_on: []
  - target_files: [cmd/evidence.go, cmd/evidence_test.go]
  - task_kind: code
  - covers: [REQ-003]
