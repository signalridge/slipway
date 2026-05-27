# Tasks

## Task List

- [x] `t-01` Remove the current quick-mode command and progression bypass.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/next.go", "cmd/next_handoff.go", "cmd/next_eval_fixture_test.go", "cmd/run.go", "cmd/root_help_test.go", "internal/engine/progression/advance.go", "internal/engine/progression/advance_governed.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-005]

- [x] `t-02` Tighten task evidence parsing to explicit flat JSON fields.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/common_test.go", "cmd/pivot_execution_test.go", "cmd/progression_next_test.go", "internal/engine/progression/advance_test.go", "internal/engine/progression/wave_sync.go", "internal/engine/progression/wave_sync_test.go"]
  - task_kind: code
  - covers: [REQ-002, REQ-005]

- [x] `t-03` Remove unused artifact version metadata.
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: ["internal/model/types.go", "internal/engine/artifact/manager.go", "internal/engine/artifact/manager_test.go", "internal/model/model_test.go", "internal/state/lifecycle_test.go", "artifacts/changes/archived/**"]
  - task_kind: code
  - covers: [REQ-003, REQ-005]

- [x] `t-04` Clean stale docs and tracked archived upstream references.
  - wave: 3
  - depends_on: [t-03]
  - target_files: ["docs/design.md", "docs/index.md", "docs/workflow.md", "internal/engine/governance/traceability_test.go", "internal/engine/progression/advance_test.go", "internal/stringutil/html_test.go", "internal/tmpl/templates/skills/root-cause-tracing/references/root-cause-tracing.md", "artifacts/changes/archived/**"]
  - task_kind: doc
  - covers: [REQ-004, REQ-005]

- [x] `t-05` Verify focused behavior and full repository health.
  - wave: 4
  - depends_on: [t-04]
  - target_files: ["artifacts/changes/deeply-research-and-optimize-backward-compatibility-layers-redundant-layers-unnecessary-upstream-references-and-unnecessary-version-metadata/tasks.md", "artifacts/changes/deeply-research-and-optimize-backward-compatibility-layers-redundant-layers-unnecessary-upstream-references-and-unnecessary-version-metadata/evidence/tasks/rv1/**", "artifacts/changes/deeply-research-and-optimize-backward-compatibility-layers-redundant-layers-unnecessary-upstream-references-and-unnecessary-version-metadata/verification/**", "artifacts/changes/deeply-research-and-optimize-backward-compatibility-layers-redundant-layers-unnecessary-upstream-references-and-unnecessary-version-metadata/assurance.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
