# Tasks

## Task List

- [x] `t-01` enforce selected-review evidence context-origin before persistence
  - depends_on: []
  - target_files: ["cmd/evidence.go", "cmd/evidence_skill_test.go", "internal/model/context_attestation.go", "internal/model/context_attestation_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-004]
  - acceptance: focused `go test ./cmd` evidence-skill tests and `go test ./internal/model` context-attestation tests pass; invalid selected-review pass evidence leaves no reviewer YAML.

- [x] `t-02` update agent-facing selected-review evidence surfaces
  - depends_on: []
  - target_files: ["cmd/next.go", "cmd/progression_next_test.go", "internal/engine/capability/resolver.go", "internal/engine/capability/resolver_test.go", "internal/engine/capability/registry_default.go", "internal/engine/capability/registry_scale_foundation.go", "internal/tmpl/templates/_partials/command-evidence-body.tmpl", "internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/independent-review/SKILL.md", "internal/tmpl/templates/skills/independent-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/security-review/SKILL.md", "internal/tmpl/templates/skills/security-review/SKILL.md.tmpl", "internal/tmpl/templates_test.go", "internal/toolgen/surface_manifest.go", "internal/toolgen/surface_manifest_test.go", "docs/SURFACE-MANIFEST.json", "docs/reference/commands.md", "docs/commands.md", "docs/zh/reference/commands.md", "docs/zh/commands.md", "docs/ja/reference/commands.md", "docs/ja/commands.md"]
  - task_kind: code
  - covers: [REQ-003, REQ-004]
  - acceptance: template, toolgen, capability, and next-surface tests pass and generated examples include context-origin, fallback reference, and `*-notes.md`.

- [x] `t-03` verify issue 394 acceptance boundary
  - depends_on: [t-01, t-02]
  - target_files: ["cmd/evidence_skill_test.go", "cmd/evidence_task_test.go", "cmd/progression_next_test.go", "internal/model/context_attestation_test.go", "internal/engine/capability/resolver_test.go", "internal/tmpl/templates_test.go", "internal/toolgen/surface_manifest_test.go", "artifacts/changes/fix-s3-review-evidence-guard/verification/final-focused-verification.md"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
  - acceptance: final focused verification, package tests, `git diff --check`, `slipway validate`, and readiness checks pass with fresh evidence.
