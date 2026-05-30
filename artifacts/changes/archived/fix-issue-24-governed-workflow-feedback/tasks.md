# Tasks

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `task-01` repair stale execution evidence from runtime task evidence
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/repair.go", "cmd/repair_test.go", "internal/engine/progression/wave_sync.go", "internal/engine/progression/evidence.go", "internal/engine/progression/evidence_test.go", "internal/state/execution_summary.go", "cmd/progression_next_test.go", "docs/operator-guide.md", "docs/commands.md"]
  - task_kind: code
  - covers: [REQ-001, REQ-002]

- [x] `task-02` warn when done archives dirty worktree source changes
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["cmd/done.go", "cmd/lifecycle_commands_test.go", "internal/engine/progression/readiness.go", "internal/engine/progression/readiness_optimization_test.go", "docs/operator-guide.md"]
  - task_kind: code
  - covers: [REQ-003]

- [x] `task-03` tighten governed review templates
  - wave: 3
  - depends_on: ["task-02"]
  - target_files: ["internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl", ".codex/skills/slipway-spec-compliance-review/SKILL.md", ".codex/skills/slipway-code-quality-review/SKILL.md", ".claude/skills/slipway-spec-compliance-review/SKILL.md", ".claude/skills/slipway-code-quality-review/SKILL.md", "internal/tmpl/templates_test.go"]
  - task_kind: code
  - covers: [REQ-004]

- [x] `task-04` run integrated verification and close governance evidence
  - wave: 4
  - depends_on: ["task-01", "task-02", "task-03"]
  - target_files: ["artifacts/changes/fix-issue-24-governed-workflow-feedback/assurance.md", "artifacts/changes/fix-issue-24-governed-workflow-feedback/verification/"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
