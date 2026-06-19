# Tasks

## Task List

- [x] `t-01` Extend the toolgen adapter model and registry for P1 tools.
  - depends_on: []
  - target_files: [`internal/toolgen/toolgen.go`, `internal/toolgen/surface_manifest.go`]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-02` Update generated adapter contract and refresh tests for P1 paths, triggers, settings, and ownership behavior.
  - depends_on: [`t-01`]
  - target_files: [`internal/toolgen/toolgen_test.go`, `internal/toolgen/adapter_contract_test.go`]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-005]

- [x] `t-03` Update adapter reference documentation and regenerate the public surface manifest for P1 adapters.
  - depends_on: [`t-01`]
  - target_files: [`docs/ai-tools.md`, `docs/reference/ai-tools.md`, `docs/installation.md`, `docs/SURFACE-MANIFEST.json`]
  - task_kind: doc
  - covers: [REQ-004]

- [x] `t-04` Run targeted and full verification for the P1 adapter implementation and capture implementation evidence.
  - depends_on: [`t-02`, `t-03`]
  - target_files: [`artifacts/changes/expand-ai-tool-adapters/verification/implementation.md`]
  - task_kind: verification
  - covers: [REQ-005]
