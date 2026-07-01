# Tasks

## Task List

- [x] `t-01` Add slot-based subagent config model, validation, resolution, and config catalog entries.
  - depends_on: []
  - target_files: [`internal/model/config.go`, `internal/model/config_catalog.go`, `internal/model/config_test.go`, `internal/model/config_catalog_test.go`, `internal/model/recovery.go`]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]
  - acceptance: `go test ./internal/model -run 'TestConfig.*Subagent|TestConfigCatalog.*Subagent|TestConfig(Set|Get).*Subagent' -count=1` passes and includes negative coverage for removed `profile`, `prompt`, `subagent_provider_profiles`, review substep slots, and invalid `type`.

- [x] `t-02` Project resolved subagent directives into next/fix/wave/review/verify host JSON surfaces.
  - depends_on: [`t-01`]
  - target_files: [`cmd/common_test.go`, `cmd/next.go`, `cmd/next_skill_view.go`, `cmd/next_wave_plan.go`, `cmd/next_handoff.go`, `cmd/next_plan_audit_handoff_test.go`, `cmd/fix.go`, `cmd/progression_next_test.go`, `cmd/fix_test.go`]
  - task_kind: code
  - covers: [REQ-004]
  - acceptance: focused `cmd` tests prove `next_skill.subagent` for plan-audit and verify, `input_context.wave_plan.executor_subagent`, `review_batch.subagent`, and `fix --json contract.subagent` all use `type`, `name`, `session_instructions`, `timeout`, and generated capabilities.

- [x] `t-03` Update generated command and skill sources/templates to describe slot directives and `session_instructions`.
  - depends_on: [`t-02`]
  - target_files: [`internal/tmpl/templates/skills/workflow/command-reference.md.tmpl`, `internal/tmpl/templates/skills/plan-audit/SKILL.md`, `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`, `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl`, `internal/tmpl/templates/skills/code-quality-review/SKILL.md.tmpl`, `internal/tmpl/templates/skills/independent-review/SKILL.md`, `internal/tmpl/templates/skills/independent-review/SKILL.md.tmpl`, `internal/tmpl/templates/skills/security-review/SKILL.md`, `internal/tmpl/templates/skills/security-review/SKILL.md.tmpl`, `internal/tmpl/templates/skills/ship-verification/SKILL.md.tmpl`]
  - task_kind: code
  - covers: [REQ-005]
  - acceptance: `go test ./internal/tmpl ./internal/toolgen -run 'Test.*Subagent|Test.*Command.*Subagent|Test.*Generated.*Subagent|Test.*Skill.*Subagent' -count=1` passes, and generated review templates no longer claim user-configured review delegation is native-only.

- [x] `t-04` Add and link user-facing subagent configuration documentation in English, Chinese, and Japanese.
  - depends_on: [`t-01`]
  - target_files: [`README.md`, `docs/commands.md`, `docs/design.md`, `docs/index.md`, `docs/reference/commands.md`, `docs/reference/subagents.md`, `docs/workflow.md`, `docs/ja/commands.md`, `docs/ja/reference/commands.md`, `docs/ja/reference/subagents.md`, `docs/zh/commands.md`, `docs/zh/reference/commands.md`, `docs/zh/reference/subagents.md`, `website/astro.config.mjs`]
  - task_kind: doc
  - covers: [REQ-005]
  - acceptance: English, Chinese, and Japanese docs describe only `default`, `plan_audit`, `executor`, `review`, `fix`, and `verify`; they explain `session_instructions` as delegated-session guidance; and no updated docs introduce `subagent_provider_profiles`, `allowed_skills`, `allowed_mcp_servers`, or user-configurable `tool_policy`.

- [x] `t-05` Run focused and broad verification for model, command, template, docs, and full Go behavior.
  - depends_on: [`t-01`, `t-02`, `t-03`, `t-04`]
  - target_files: [`artifacts/changes/refactor-subagent-configuration-to-slot-based-session-instru/verification/implementation-verification-notes.md`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - acceptance: record command output for `git diff --check`, focused `go test ./internal/model ./cmd ./internal/tmpl ./internal/toolgen -count=1`, and `go test ./... -timeout=20m -count=1` in implementation verification notes.
