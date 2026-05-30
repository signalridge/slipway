# Tasks

## Project Context
- Tech Stack:
- Conventions:
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Markdown

## Task List

- [x] `task-01` add generated-surface regression assertions
  - wave: 1
  - depends_on: []
  - target_files: ["internal/toolgen/toolgen_test.go"]
  - task_kind: test
  - evidence: targeted toolgen test result
  - acceptance: generated workflow, command reference, and command prompt assertions cover JSON stdin classification and unsupported-flag wording
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]

- [x] `task-02` implement JSON stdin contract wording in generated surfaces
  - wave: 2
  - depends_on: ["task-01"]
  - target_files: ["internal/toolgen/toolgen.go", "internal/tmpl/templates/skills/workflow/SKILL.md.tmpl", "internal/tmpl/templates/skills/workflow/command-reference.md.tmpl", "internal/tmpl/templates/_partials/command-new-body.tmpl"]
  - task_kind: code
  - evidence: targeted and full Go verification results
  - acceptance: templates and metadata satisfy generated-surface assertions and full `go test ./...` plus `go build ./...` pass
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
