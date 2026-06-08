# Tasks

## Task List

- [x] `t-01` Add `pending_decisions` to `skillConstraints` and gate locked-vs-pending on the `G_plan` state; thread the gate status into skill_constraints assembly and split `ParseDecisionLockedDecisions` output (locked when G_plan approved, else pending)
  - wave: 1
  - depends_on: []
  - target_files: [cmd/next.go, cmd/next_skill.go, cmd/next_skill_view.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]

- [x] `t-02` Preserve `pending_decisions` when `skill_constraints` is cloned for handoff payloads
  - wave: 2
  - depends_on: [t-01]
  - target_files: [cmd/next_handoff.go]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-03` Update the `spec-compliance-review` template so Decision Fidelity enforces only `locked_decisions` and treats `pending_decisions` as advisory (edit template source; generated copies regenerate via toolgen)
  - wave: 2
  - depends_on: [t-01]
  - target_files: [internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-04` Add/extend tests: unit tests for the locked-vs-pending split and the new field (pending when G_plan unapproved, locked after approval, both empty for placeholder), an e2e asserting the JSON field split, and a template test asserting the regenerated spec-compliance-review pending advisory
  - wave: 3
  - depends_on: [t-01, t-02, t-03]
  - target_files: [cmd/next_skill_constraints_test.go, cmd/cli_e2e_test.go, internal/tmpl/templates_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
