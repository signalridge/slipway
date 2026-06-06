# Tasks
## Project Context
- Tech Stack: Go
- Conventions: cmd/* CLI over internal/engine/* kernel; generated skills/docs via toolgen; table-driven tests
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` (#91) Make the engine's default requirements/tasks seed an
  obviously-not-real honest placeholder. In `internal/engine/artifact/manager.go`
  rewrite `appendRequirementBlock` (and `seedRequirements`/`seededRequirementsContent`
  default path) and `seedTasks` so the default scaffold emits headings plus honest
  guidance prose ("replace with…", quality bar) instead of a fabricated
  `REQ-001: The system MUST <request>` and the tautology GIVEN/WHEN/THEN; keep the
  `--from-doc` title derivation but drop fabricated normative bodies/tautology
  scenarios. Keep templates `internal/tmpl/templates/artifacts/requirements.md` and
  `tasks.md` rendering the seeded content (adjust if needed so the rendered default
  is detectably placeholder). Test-first: `manager_test.go` (or the existing seed
  tests) assert the default-seeded requirements/tasks are `LooksLikeTemplatePlaceholder`
  and no longer contain the old tautology strings.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/artifact/manager.go", "internal/tmpl/templates/artifacts/requirements.md", "internal/tmpl/templates/artifacts/tasks.md", "internal/engine/artifact/manager_test.go"]
  - task_kind: code
  - covers: [REQ-001]

- [x] `t-02` (#91) Add requirements substance detection + gate. In
  `internal/engine/artifact/manager.go` extend `LooksLikeTemplatePlaceholder` with
  the requirements scaffold sentinels (the legacy GIVEN/WHEN/THEN tautology lines,
  the honest-seed requirement/scenario placeholder markers, the placeholder
  verification-objective marker, and the requirements fallback marker; exact
  strings pinned in artifact-package tests). In
  `internal/engine/artifact/requirements_contract.go` extend
  `EvaluateRequirementsContract` so each REQ-* body contains MUST/SHALL, each
  requirement has >=1 concrete non-tautology/non-placeholder scenario, and
  placeholder-only content is rejected (invalid with an actionable message);
  authored content stays valid. Test-first: `requirements_contract_test.go` (reject
  mechanical/no-MUST/tautology-only; accept authored) and a placeholder-sentinel
  test.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/engine/artifact/manager.go", "internal/engine/artifact/requirements.go", "internal/engine/artifact/requirements_contract.go", "internal/engine/artifact/requirements_contract_test.go", "internal/engine/artifact/manager_test.go"]
  - task_kind: code
  - covers: [REQ-002, REQ-003]

- [x] `t-03` (#91) Add a tasks substance validator and wire it into the governed
  validation path. Add `EvaluateTasksContract` (new
  `internal/engine/artifact/tasks_contract.go`) rejecting placeholder task
  objectives (the engine's seeded task/verification objective markers) and
  non-substantive task lists; invoke it from the same gate path that consumes
  `EvaluateRequirementsContract` (locate via `validate`/plan-audit artifact-gate
  wiring) so `slipway validate` fails a placeholder tasks.md. Test-first:
  `tasks_contract_test.go` (reject placeholder, accept authored) and the
  validate-gate test that exercises the wiring.
  - wave: 3
  - depends_on: [t-02]
  - target_files: ["internal/engine/artifact/tasks_contract.go", "internal/engine/artifact/tasks_contract_test.go", "internal/engine/progression/validation.go", "internal/engine/progression/validation_test.go", "internal/model/reason_code.go", "internal/model/recovery.go", "cmd/validate.go", "cmd/validate_requirements_contract_test.go", "cmd/cli_e2e_test.go", "cmd/governance_gate_consistency_test.go", "cmd/progression_next_test.go", "cmd/review_test.go", "cmd/lifecycle_commands_test.go"]
  - task_kind: code
  - covers: [REQ-004]

- [x] `t-04` (#91) Keep the runtime placeholder helper scoped to decision.md; do
  NOT generalize it to `requirements.md`/`tasks.md`. `artifactSectionHasSubstantiveContent`
  in `internal/engine/governance/runtime_actions.go` is only invoked for
  `decision.md`/`assurance.md` (via `hasRollbackDocumentation`), so a
  requirements/tasks branch would be dead, unreachable code; their substance is
  owned by the progression substance gate + the validate contracts (REQ-003/REQ-004).
  Document the deliberate scoping in `runtime_actions.go`.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/governance/runtime_actions.go"]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-05` (#91) Add the `slipway instructions <artifact>` command. New
  `cmd/instructions.go` returning the named artifact's template + authoring guidance
  (text and `--json`), with an actionable error for unknown names; register it in
  the command tree and the command registry/capability surface used by toolgen.
  Test-first: `cmd/instructions_test.go` (requirements/tasks return template +
  guidance, text and json; unknown name errors).
  - wave: 2
  - depends_on: []
  - target_files: ["cmd/instructions.go", "cmd/instructions_test.go", "cmd/root.go", "cmd/root_help_test.go"]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-06` (#91) Regenerate all generated surfaces and lock the suite. Run the
  toolgen self-loop to regenerate generated skills, command references, and docs
  (covering the new `instructions` command surface and any seed/guidance text that
  flows into generated skills) and confirm zero drift; then run
  `go build ./... && go vet ./... && go test ./...` green.
  - wave: 4
  - depends_on: [t-01, t-02, t-03, t-04, t-05]
  - target_files: ["internal/toolgen/toolgen.go", "internal/toolgen/toolgen_test.go", "docs/commands.md", "internal/tmpl/templates/skills/plan-audit/SKILL.md"]
  - task_kind: doc
  - covers: [REQ-007]
