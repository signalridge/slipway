# Tasks

## Task List

- [x] `t-01` Fix `--hydrate-ref` help placeholders and focused help coverage.
  - depends_on: []
  - target_files: ["cmd/status.go", "cmd/review.go", "cmd/health.go", "cmd/template_flag_contract_test.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-004]
  - evidence: help output and focused cmd test result
  - acceptance: `status --help`, `review --help`, and `health --help` render `--hydrate-ref <skill-id>/<name>` instead of `--hydrate-ref --hydrate`; focused command help coverage fails if the placeholder regresses.

- [x] `t-02` Align generated skill and command-surface descriptions with current routing.
  - depends_on: []
  - target_files: ["cmd/fix.go", "cmd/next_skill_view.go", "cmd/progression_next_test.go", "cmd/review.go", "cmd/status_view_build.go", "cmd/validate.go", "internal/engine/capability/registry_b2.go", "internal/engine/progression/authority.go", "internal/engine/progression/authority_test.go", "internal/engine/progression/skill_resolution.go", "internal/engine/progression/skill_resolution_test.go", "internal/engine/skill/skill.go", "internal/engine/skill/skill_test.go", "internal/model/recovery.go", "internal/model/recovery_test.go", "internal/tmpl/templates/_partials/command-run-body.tmpl", "internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl", "internal/tmpl/templates/skills/workflow/SKILL.md.tmpl", "internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl", "internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/worktree-preflight/SKILL.md", "internal/tmpl/templates/skills/security-review/SKILL.md", "internal/tmpl/templates/skills/security-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/git-recovery/SKILL.md", "internal/tmpl/templates/skills/gha-security-review/SKILL.md", "internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl", "internal/tmpl/templates/skills/workflow/command-reference.md.tmpl", "internal/tmpl/templates_test.go", "internal/toolgen/toolgen.go", "internal/toolgen/toolgen_test.go"]
  - task_kind: doc
  - covers: [REQ-002, REQ-003, REQ-004]
  - evidence: focused internal/toolgen test result and reviewed template diffs
  - acceptance: Generated skill templates and recovery text describe current resolver and host behavior for goal verification, worktree preflight, security review, git recovery, wave orchestration, command references, command-surface grouping, and CLI-only helpers; docs-profile S3 surfaces skip `code-quality-review` while retaining selected `security-review`; unsupported path-glob or blocker-route claims are removed; `pin-actions` helper guidance is covered.

- [x] `t-03` Align user-facing docs and diagram descriptions with runtime evidence paths and command surfaces.
  - depends_on: []
  - target_files: ["docs/ai-tools.md", "docs/commands.md", "docs/design.md", "docs/workflow.md", "docs/index.md", "docs/operator-guide.md", "docs/installation.md", "docs/assets/diagrams/architecture.svg"]
  - task_kind: doc
  - covers: [REQ-003, REQ-004]
  - evidence: targeted stale-phrase checks and surface manifest check
  - acceptance: Docs describe runtime task evidence under `.git/slipway/runtime/changes/<slug>/evidence/...` and bundle-local verification artifacts under `artifacts/changes/<slug>/verification/`; touched docs no longer claim every CLI command ships a host/tool surface; diagram labels show command examples without implying an exhaustive split; workflow docs attribute review dispatch to host adapters after engine selection and document profile-filtered S3 reviewer selection.

- [x] `t-04` Refresh generated surface inventory and run focused verification.
  - depends_on: ["t-01", "t-02", "t-03"]
  - target_files: ["docs/SURFACE-MANIFEST.json"]
  - task_kind: verification
  - covers: [REQ-004]
  - evidence: manifest check output, focused test outputs, and full test-suite output
  - acceptance: `go run ./internal/toolgen/cmd/gen-surface-manifest --check`, focused cmd tests, and focused toolgen tests pass; `go test ./...` is attempted and any failure is fixed or recorded with exact blocker details; S3 `assurance.md` remains deferred to the review phase.
