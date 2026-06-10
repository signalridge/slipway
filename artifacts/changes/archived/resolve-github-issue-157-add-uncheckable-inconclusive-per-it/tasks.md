# Tasks

## Task List

- [x] `t-01` Add focused RED regression for Issue #157 template contract
  - wave: 1
  - depends_on: []
  - target_files: [`internal/tmpl/templates_test.go`]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-02` Extend spec-trace report schema and coverage matrix vocabulary
  - wave: 2
  - depends_on: [`t-01`]
  - target_files: [`internal/tmpl/templates/skills/spec-trace/SKILL.md`, `internal/tmpl/templates/skills/spec-trace/CHECKLIST.tmpl`]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `t-03` Add spec-compliance-review fail-closed uncertain-trace guidance
  - wave: 2
  - depends_on: [`t-01`]
  - target_files: [`internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl`]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-04` Verify template contract and governed readiness evidence
  - wave: 3
  - depends_on: [`t-02`, `t-03`]
  - target_files: [`internal/tmpl/templates_test.go`, `artifacts/changes/resolve-github-issue-157-add-uncheckable-inconclusive-per-it/verification`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
