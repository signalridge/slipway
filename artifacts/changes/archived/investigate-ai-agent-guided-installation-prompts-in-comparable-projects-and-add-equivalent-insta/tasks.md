# Tasks
## Project Context
- Tech Stack: Go, MkDocs
- Conventions: Documentation uses concise sections, copyable fenced commands, stable relative links, and `mkdocs build --strict` as the rendering proof.
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go, Markdown

## Task List

- [x] `t-01` Add a README AI-agent install entry.
  - wave: 1
  - depends_on: []
  - target_files: ["README.md"]
  - task_kind: doc
  - covers: [REQ-001, REQ-003]
  - evidence: README contains a copyable AI-agent setup prompt without duplicating the full platform matrix.
  - acceptance: README prompt tells the agent to install or initialize Slipway, use documented sources, preserve user files, and report verification.

- [x] `t-02` Add a prominent short AI-agent prompt to installation docs.
  - wave: 1
  - depends_on: []
  - target_files: ["docs/installation.md"]
  - task_kind: doc
  - covers: [REQ-002, REQ-003]
  - evidence: `docs/installation.md` presents manual install paths and AI-assisted setup without contradiction.
  - acceptance: The short prompt links or points to the detailed checklist and keeps the current release/package/source installation contract.

- [x] `t-03` Tighten agent safety and adapter-contract wording.
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: ["docs/installation.md", "README.md"]
  - task_kind: doc
  - covers: [REQ-003]
  - evidence: The docs include explicit safety boundaries for AI agents.
  - acceptance: Generated adapter paths and verification commands remain aligned with `docs/ai-tools.md`.

- [x] `t-04` Verify docs and governed readiness.
  - wave: 3
  - depends_on: [t-01, t-02, t-03]
  - target_files: ["artifacts/changes/archived/investigate-ai-agent-guided-installation-prompts-in-comparable-projects-and-add-equivalent-insta/verification"]
  - task_kind: verification
  - covers: [REQ-004]
  - evidence: Verification evidence records `mkdocs build --strict` and governed validation results.
  - acceptance: `mkdocs build --strict` passes; `go run . validate --json` has no blocker for the completed stage.
