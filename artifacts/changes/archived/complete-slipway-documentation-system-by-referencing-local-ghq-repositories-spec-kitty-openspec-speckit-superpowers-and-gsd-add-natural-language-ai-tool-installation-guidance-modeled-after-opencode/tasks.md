# Tasks

## Project Context
- Tech Stack: Go CLI, Markdown documentation, MkDocs Material
- Conventions: keep target files explicit; keep runtime changes narrow; verify docs and Go behavior from the governed worktree
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, Markdown, YAML

## Task List

- [x] `t-01` Create the tracked MkDocs documentation system and README entrypoint.
  - wave: 1
  - depends_on: []
  - target_files: ["README.md", "mkdocs.yml", "docs/index.md", "docs/design.md", "docs/installation.md", "docs/workflow.md", "docs/commands.md", "docs/ai-tools.md", "docs/operator-guide.md", "docs/contributing.md", ".github/workflows/docs.yml"]
  - task_kind: doc
  - evidence: tracked docs tree, MkDocs nav, design page, README product entrypoint
  - acceptance: REQ-001 nav targets exist; REQ-003 user/operator/contributor path is complete; REQ-007 README explains value and links to detailed docs; README docs workflow claim is backed by a tracked workflow
  - covers: [REQ-001, REQ-003, REQ-007]

- [x] `t-02` Add AI-tool and OpenCode-modeled installation guidance.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["docs/installation.md", "docs/ai-tools.md", "README.md"]
  - task_kind: doc
  - evidence: platform install matrix, copy-paste AI install prompt, supported adapter table, OpenCode paths and invocation notes
  - acceptance: REQ-004 prompt names inspect/build/init/verify steps; REQ-006 covers configured platform/package channels and fallback paths; OpenCode docs name .opencode skills and commands paths; Codex global prompt caveat is documented
  - covers: [REQ-002, REQ-004, REQ-006]

- [x] `t-03` Keep the Open Questions fix narrow and tested.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/stringutil/html.go", "internal/stringutil/html_test.go", "internal/engine/progression/advance_intake.go", "internal/engine/progression/advance_test.go", "internal/engine/governance/traceability.go"]
  - task_kind: code
  - evidence: shared Open Questions helper, intake and traceability callers, focused regression tests
  - acceptance: REQ-005 resolved entries do not block; unchecked/plain bullet entries still block
  - covers: [REQ-005]

- [x] `t-04` Verify documentation and Go behavior from the governed worktree.
  - wave: 3
  - depends_on: [t-01, t-02, t-03]
  - target_files: ["mkdocs.yml", "docs/index.md", "docs/installation.md", "docs/workflow.md", "docs/commands.md", "docs/ai-tools.md", "docs/operator-guide.md", "docs/contributing.md", "internal/stringutil/html_test.go", "internal/engine/progression/advance_test.go"]
  - task_kind: verification
  - evidence: docs build result or explicit unavailable-tool note, targeted Open Questions test result, full Go test result, Go build result
  - acceptance: mkdocs nav resolves; targeted regression passes; go test -timeout=20m ./... -count=1 passes; go build ./... passes
  - covers: [REQ-001, REQ-003, REQ-004, REQ-005]

- [x] `t-05` Record reference traceability and final governed evidence.
  - wave: 4
  - depends_on: [t-04]
  - target_files: ["artifacts/changes/complete-slipway-documentation-system-by-referencing-local-ghq-repositories-spec-kitty-openspec-speckit-superpowers-and-gsd-add-natural-language-ai-tool-installation-guidance-modeled-after-opencode/research.md", "artifacts/changes/complete-slipway-documentation-system-by-referencing-local-ghq-repositories-spec-kitty-openspec-speckit-superpowers-and-gsd-add-natural-language-ai-tool-installation-guidance-modeled-after-opencode/assurance.md"]
  - task_kind: verification
  - evidence: research references preserved, assurance maps requirements to proof, residual risks listed
  - acceptance: REQ-002 reference repositories have concrete borrowed patterns; all requirements have evidence in assurance before closeout
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]

- [x] `t-06` Apply review correction for complete, non-redundant docs design.
  - wave: 5
  - depends_on: [t-01, t-02, t-05]
  - target_files: ["README.md", "mkdocs.yml", ".gitignore", "docs/index.md", "docs/design.md", "docs/installation.md", "docs/workflow.md", "docs/ai-tools.md", "artifacts/changes/complete-slipway-documentation-system-by-referencing-local-ghq-repositories-spec-kitty-openspec-speckit-superpowers-and-gsd-add-natural-language-ai-tool-installation-guidance-modeled-after-opencode/requirements.md", "artifacts/changes/complete-slipway-documentation-system-by-referencing-local-ghq-repositories-spec-kitty-openspec-speckit-superpowers-and-gsd-add-natural-language-ai-tool-installation-guidance-modeled-after-opencode/tasks.md", "artifacts/changes/complete-slipway-documentation-system-by-referencing-local-ghq-repositories-spec-kitty-openspec-speckit-superpowers-and-gsd-add-natural-language-ai-tool-installation-guidance-modeled-after-opencode/assurance.md"]
  - task_kind: doc
  - evidence: README redesign, Design Philosophy page, Mermaid diagrams, complete platform/package install matrix, MkDocs strict build
  - acceptance: REQ-006 platform/package coverage is explicit; REQ-007 README includes philosophy, capabilities, lifecycle diagram, and docs map without duplicating the full installation page
  - covers: [REQ-001, REQ-003, REQ-006, REQ-007]
