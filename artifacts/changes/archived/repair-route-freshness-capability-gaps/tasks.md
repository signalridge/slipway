# Tasks

- [x] `t-01` Fact-check pasted opt.md findings against current code and live GitHub settings
  - depends_on: []
  - target_files: ["artifacts/changes/repair-route-freshness-capability-gaps/intent.md", "artifacts/changes/repair-route-freshness-capability-gaps/requirements.md", "artifacts/changes/repair-route-freshness-capability-gaps/decision.md", "artifacts/changes/repair-route-freshness-capability-gaps/research.md", "artifacts/changes/repair-route-freshness-capability-gaps/tasks.md", "artifacts/codebase/ARCHITECTURE.md", "artifacts/codebase/TESTING.md", "artifacts/codebase/CONCERNS.md", "artifacts/changes/repair-route-freshness-capability-gaps/verification/intake-clarification-notes.md", "artifacts/changes/repair-route-freshness-capability-gaps/verification/research-orchestration-notes.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-02` Add non-success invocation route surfaces and archived-local route kind
  - depends_on: ["t-01"]
  - target_files: ["cmd/common.go", "cmd/errors.go", "cmd/status.go", "cmd/status_view_build.go", "cmd/status_render.go", "cmd/validate.go", "cmd/active_change_resolution_test.go"]
  - task_kind: code
  - covers: [REQ-001]

- [x] `t-03` Add next/done split freshness fields and status text split freshness prose
  - depends_on: ["t-01"]
  - target_files: ["cmd/next.go", "cmd/next_handoff.go", "cmd/done.go", "cmd/status_view_build.go", "cmd/status_render.go", "cmd/progression_next_test.go", "cmd/lifecycle_commands_test.go", "cmd/status_render_test.go"]
  - task_kind: code
  - covers: [REQ-002, REQ-003]

- [x] `t-04` Move host capability requirements into registry/template metadata
  - depends_on: ["t-01"]
  - target_files: ["internal/engine/capability/registry.go", "internal/engine/capability/registry_default.go", "internal/engine/capability/resolver.go", "internal/engine/capability/gates_test.go", "internal/engine/capability/resolver_test.go", "internal/tmpl/templates/skills/independent-review/SKILL.md", "internal/tmpl/templates/skills/independent-review/SKILL.md.tmpl"]
  - task_kind: code
  - covers: [REQ-004]

- [x] `t-05` Run focused, full, coverage, and performance verification
  - depends_on: ["t-02", "t-03", "t-04"]
  - target_files: ["cmd/active_change_resolution_test.go", "cmd/progression_next_test.go", "cmd/lifecycle_commands_test.go", "cmd/status_render_test.go", "internal/engine/capability/gates_test.go", "internal/engine/capability/resolver_test.go", "artifacts/changes/repair-route-freshness-capability-gaps/assurance.md", "artifacts/changes/repair-route-freshness-capability-gaps/verification/plan-audit-notes.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
