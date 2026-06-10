# Tasks

## Task List

- [x] `t-01` Add failing template regressions for remaining heavy-stage disk handoff.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/evidence_task_test.go, internal/tmpl/thin_host_content_test.go, internal/tmpl/templates_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-02` Update remaining heavy stage authored skill surfaces with path-based disk-handoff and short-confirmation guidance.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [cmd/evidence.go, internal/state/verification.go, internal/tmpl/templates/skills/research-orchestration/SKILL.md, internal/tmpl/templates/skills/plan-audit/SKILL.md, internal/tmpl/templates/skills/intake-clarification/SKILL.md, internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl, internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl, internal/toolgen/toolgen.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-03` Prove the template contract and generated-surface behavior with targeted and package-level tests.
  - wave: 3
  - depends_on: [t-02]
  - target_files: [cmd/evidence_task_test.go, internal/state/verification_test.go, internal/tmpl/thin_host_content_test.go, internal/tmpl/templates_test.go, internal/toolgen/toolgen_test.go]
  - task_kind: test
  - covers: [REQ-004]

- [x] `t-04` Run implementation verification commands and record execution evidence for governed review handoff.
  - wave: 4
  - depends_on: [t-03]
  - target_files: [cmd/evidence_task_test.go, internal/state/verification_test.go, internal/tmpl/thin_host_content_test.go, internal/tmpl/templates_test.go, artifacts/codebase/ARCHITECTURE.md, artifacts/codebase/CONCERNS.md, artifacts/codebase/STRUCTURE.md, artifacts/codebase/TESTING.md, artifacts/changes/resolve-github-issue-151-thin-host-disk-handoff-return-contr/verification/execution-summary.yaml]
  - task_kind: test
  - covers: [REQ-005]
