# Tasks

## Task List

- [x] `t-01` Implement deterministic surface manifest builder and committed public manifest.
  - wave: 1
  - depends_on: []
  - target_files: [`internal/toolgen/surface_manifest.go`, `docs/SURFACE-MANIFEST.json`]
  - task_kind: code
  - covers: [REQ-001]

- [x] `t-02` Add check/write regeneration entrypoint backed by the shared manifest builder.
  - wave: 2
  - depends_on: [`t-01`]
  - target_files: [`internal/toolgen/cmd/gen-surface-manifest/main.go`, `internal/toolgen/surface_manifest.go`]
  - task_kind: code
  - covers: [REQ-002]

- [x] `t-03` Add fail-closed manifest sync, regeneration, and docs-token tests while preserving existing README checks.
  - wave: 3
  - depends_on: [`t-01`, `t-02`]
  - target_files: [`internal/toolgen/surface_manifest_test.go`, `internal/toolgen/cmd/gen-surface-manifest/main_test.go`, `internal/toolgen/toolgen_test.go`, `cmd/template_flag_contract_test.go`]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-04` Document the committed surface manifest and regeneration path in public docs.
  - wave: 3
  - depends_on: [`t-01`, `t-02`]
  - target_files: [`README.md`, `docs/ai-tools.md`, `docs/commands.md`, `docs/SURFACE-MANIFEST.json`]
  - task_kind: doc
  - covers: [REQ-003]

- [x] `t-05` Verify focused packages, full Go test suite, manifest check mode, and governed readiness.
  - wave: 4
  - depends_on: [`t-03`, `t-04`]
  - target_files: [`artifacts/changes/resolve-github-issue-158-add-a-generated-surface-inventory-m/verification/execution-summary.yaml`, `artifacts/changes/resolve-github-issue-158-add-a-generated-surface-inventory-m/verification/evidence-digests.yaml`, `artifacts/changes/resolve-github-issue-158-add-a-generated-surface-inventory-m/verification/wave-orchestration.yaml`]
  - task_kind: verification
  - covers: [REQ-004]

- [x] `t-06` Repair S2 review re-certification dead-end discovered during stale review evidence recovery.
  - wave: 5
  - depends_on: [`t-05`]
  - target_files: [`internal/engine/progression/evidence_digests.go`, `internal/engine/progression/evidence_digests_test.go`]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-07` Repair S2 wave-orchestration evidence bootstrap dead-end discovered after refreshing task evidence.
  - wave: 6
  - depends_on: [`t-06`]
  - target_files: [`cmd/evidence.go`, `cmd/evidence_task_test.go`, `docs/commands.md`]
  - task_kind: code
  - covers: [REQ-006]
