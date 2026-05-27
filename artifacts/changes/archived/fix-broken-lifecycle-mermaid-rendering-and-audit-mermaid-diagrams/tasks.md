# Tasks

## Project Context
- Tech Stack: Go CLI, Markdown documentation
- Conventions: Keep implementation surgical and verify behavior with focused tests.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Markdown, Go

## Task List

- [x] `t-01` Replace the README Lifecycle Mermaid block and append the repo status image.
  - wave: 1
  - depends_on: []
  - target_files: ["README.md"]
  - task_kind: doc
  - covers: [REQ-001, REQ-003]
  - acceptance: README Lifecycle Mermaid parses with Mermaid CLI, and the final README section embeds the provided camo image URL.

- [x] `t-02` Normalize explicit none markers in Open Questions detection.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/stringutil/html.go"]
  - task_kind: code
  - covers: [REQ-004]
  - acceptance: `- None.` and similar exact none markers are resolved; unchecked checklist entries remain blocking.

- [x] `t-03` Add focused Open Questions regression coverage and workflow docs.
  - wave: 2
  - depends_on: [t-02]
  - target_files: ["internal/stringutil/html_test.go", "internal/engine/progression/advance_test.go", "docs/workflow.md"]
  - task_kind: test
  - covers: [REQ-004, REQ-005]
  - acceptance: Focused helper and intake progression tests cover `- None.` as resolved and unchecked checklist entries as blocking.

- [x] `t-04` Audit Mermaid blocks in README/docs.
  - wave: 3
  - depends_on: [t-01]
  - target_files: ["README.md", "docs/ai-tools.md", "docs/design.md", "docs/workflow.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002]
  - acceptance: Every Mermaid code block in `README.md` and `docs/*.md` parses with Mermaid CLI.

- [x] `t-05` Run repo-native validation for the bounded code/docs change.
  - wave: 4
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: ["internal/stringutil/html.go", "internal/stringutil/html_test.go", "internal/engine/progression/advance_test.go", "README.md", "docs/workflow.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - acceptance: Focused Go tests pass, Mermaid parsing passes, and full `go test ./...` passes or any failure is documented with cause.
