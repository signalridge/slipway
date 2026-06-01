# Tasks

## Project Context
- Tech Stack: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Remove `agents` from config schema and parsing.
  - wave: 1
  - depends_on: []
  - target_files: [`internal/model/config.go`, `internal/model/model_test.go`]
  - task_kind: code
  - covers: [REQ-001]
  - evidence: config parse/serialization diff summary and focused model tests proving top-level `agents:` is rejected.

- [x] `t-02` Remove configured agent mapping support from governance registry loading.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [`internal/engine/skill/registry_loader.go`, `internal/engine/skill/skill.go`, `internal/engine/skill/skill_test.go`]
  - task_kind: code
  - covers: [REQ-002, REQ-005]
  - evidence: registry diff summary and focused skill tests proving Go-owned defaults remain authoritative without config-driven agent overrides.

- [x] `t-03` Remove embedded agent markdown templates and template tests.
  - wave: 3
  - depends_on: [t-02]
  - target_files: [`internal/tmpl/templates.go`, `internal/tmpl/templates_test.go`, `internal/tmpl/templates/agents/`]
  - task_kind: code
  - covers: [REQ-003]
  - evidence: template diff summary and focused template tests proving no `templates/agents` embed, `tmpl.AgentNames`, or `Content("agents/...")` path remains.

- [x] `t-04` Remove health validation for agent mappings/templates while preserving host skill checks.
  - wave: 4
  - depends_on: [t-02, t-03]
  - target_files: [`cmd/health.go`, `cmd/health_test.go`]
  - task_kind: code
  - covers: [REQ-004]
  - evidence: health diff summary and focused health tests proving host skill surface findings remain while `.*/agents` findings disappear.

- [x] `t-05` Update CLI/bootstrap/generated-surface tests for removed agent config.
  - wave: 5
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: [`cmd/next_agent_override_test.go`, `internal/bootstrap/init_test.go`, `cmd/init_test.go`, `internal/toolgen/toolgen_test.go`]
  - task_kind: code
  - covers: [REQ-005, REQ-006, REQ-007]
  - evidence: CLI/bootstrap/toolgen test diff summary proving `next_skill.name` remains the handoff and generated adapters still avoid `.*/agents` directories.

- [x] `t-06` Run stale-reference search.
  - wave: 6
  - depends_on: [t-01, t-02, t-03, t-04, t-05]
  - target_files: [`internal/`, `cmd/`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
  - evidence: captured output for `rg "ConfigAgents|Agents\\.Mappings|AgentNames|templates/agents|Content\\(\"agents/|agent_status|manual_only|governance_mapped" internal cmd`.

- [x] `t-07` Run focused regression tests.
  - wave: 6
  - depends_on: [t-01, t-02, t-03, t-04, t-05]
  - target_files: [`internal/model`, `internal/engine/skill`, `internal/tmpl`, `internal/bootstrap`, `cmd`, `internal/toolgen`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
  - evidence: captured output for focused `go test` package list covering config, registry, health, template, bootstrap, and toolgen behavior.

- [x] `t-08` Run full test and build verification.
  - wave: 7
  - depends_on: [t-06, t-07]
  - target_files: [`go.mod`, `main.go`, `cmd/`, `internal/`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
  - evidence: captured output for `go test ./...` and `go build ./...`.
