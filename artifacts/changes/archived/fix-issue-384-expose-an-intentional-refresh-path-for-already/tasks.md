# Tasks

## Task List

- [x] `t-01` Add explicit current-review refresh behavior to the evidence skill command
  - depends_on: []
  - target_files: [cmd/evidence.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-02` Add focused regression coverage for duplicate rejection and explicit refresh success
  - depends_on: [t-01]
  - target_files: [cmd/evidence_skill_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002]

- [x] `t-03` Document the refresh option in public command surfaces
  - depends_on: [t-01]
  - target_files: [internal/tmpl/templates/_partials/command-evidence-body.tmpl, internal/toolgen/toolgen.go, internal/toolgen/surface_manifest.go, internal/toolgen/surface_manifest_test.go, docs/SURFACE-MANIFEST.json, docs/reference/commands.md, docs/commands.md, docs/ja/reference/commands.md, docs/ja/commands.md, docs/zh/reference/commands.md, docs/zh/commands.md]
  - task_kind: doc
  - covers: [REQ-003]
