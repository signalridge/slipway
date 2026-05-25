# Tasks

## Project Context
- Tech Stack: Go CLI, generated Agent Skills templates
- Conventions: Go-owned capability metadata, deterministic toolgen output, generated surfaces tested through `internal/toolgen`.
- Test Command: `go test ./internal/toolgen ./cmd ./internal/engine/capability`
- Build Command: `go build ./...`
- Languages: Go, Markdown, Shell, Python

## Task List

- [x] `t-01` Render public focus aliases into generated lookup surfaces.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/capability/surfaces.go", "internal/engine/capability/export.go", "internal/toolgen/toolgen.go", "internal/tmpl/templates/skills/workflow/command-reference.md.tmpl", "internal/tmpl/templates/skills/workflow/SKILL.md.tmpl"]
  - task_kind: code
  - evidence: verdict
  - covers: [REQ-001, REQ-003, REQ-004]
  - acceptance:
      - command-reference output lists every existing explicit public focus alias from `surfacePolicy`
      - skill-index output includes focus aliases without exporting non-allowlisted support host paths
      - workflow prose keeps CLI authority boundaries explicit

- [x] `t-02` Add lightweight mechanical quality guardrails.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/capability/export_test.go", "internal/toolgen/toolgen_test.go"]
  - task_kind: code
  - evidence: verdict
  - covers: [REQ-001, REQ-002, REQ-004]
  - acceptance:
      - capability tests assert skill-index public focus rendering
      - toolgen tests assert generated command references include public focus aliases
      - toolgen tests assert very long reference files include `## Quick Navigation`

- [x] `t-03` Add top-level navigation to currently long reference files only.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/tmpl/templates/skills/supply-chain-audit/references/vulnerability-assessment-guide.md", "internal/tmpl/templates/skills/gha-security-review/references/expression-injection.md", "internal/tmpl/templates/skills/gha-security-review/references/permissions-and-secrets.md"]
  - task_kind: code
  - evidence: artifact
  - covers: [REQ-002, REQ-003]
  - acceptance:
      - each currently over-threshold reference starts with a concise quick navigation section
      - no broad rewrite is applied to shorter references

- [x] `t-04` Run focused and broad verification, then update assurance.
  - wave: 2
  - depends_on: [t-01, t-02, t-03]
  - target_files: ["artifacts/changes/optimize-slipway-skill-surfaces-and-quality-guardrails-without-changing-runtime-behavior/assurance.md", "artifacts/changes/optimize-slipway-skill-surfaces-and-quality-guardrails-without-changing-runtime-behavior/verification"]
  - task_kind: verification
  - evidence: verdict
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
  - acceptance:
      - `go test ./internal/engine/capability ./internal/toolgen ./cmd` passes
      - `go test ./...` passes
      - `go build ./...` passes
      - assurance records requirement coverage, evidence, residual risks, and archive readiness
