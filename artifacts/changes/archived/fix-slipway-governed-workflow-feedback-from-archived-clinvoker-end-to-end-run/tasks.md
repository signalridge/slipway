# Tasks

## Project Context
- Tech Stack: Go CLI governance engine
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, Markdown, YAML

## Task List

- [x] `t-01` Fix handoff/action contract clarity for S0 research and S1 bundle plan-audit routing.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/governance/actions.go", "internal/engine/governance/runtime_actions.go", "cmd/next_skill_view.go", "cmd/*next*_test.go"]
  - task_kind: code
  - covers: [REQ-002]

- [x] `t-02` Add scaffold-only codebase-map detection and expose it through command/stats/health/next behavior.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/artifact/codebase_map.go", "cmd/codebase_map.go", "internal/state/stats.go", "cmd/codebase_map*_test.go", "internal/state/stats_test.go"]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-03` Align generated research, verification, and plan-audit guidance with runtime schemas and lifecycle boundaries.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/tmpl/templates/skills/research-orchestration/SKILL.md", "internal/tmpl/templates/_partials/verification-doctrine.tmpl", "internal/tmpl/templates/skills/plan-audit/SKILL.md", "internal/tmpl/templates_test.go", ".codex/skills"]
  - task_kind: doc
  - covers: [REQ-004, REQ-005]

- [x] `t-04` Extend task metadata parsing for evidence and acceptance fields with semantic-hash coverage.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/wave/parse.go", "internal/engine/wave/parse_test.go"]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-05` Resolve worktree, locking, timeout, and archive-path feedback with focused fixes or explicit policy documentation.
  - wave: 2
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: ["internal/state/lifecycle.go", "internal/state/lifecycle_test.go", "docs/agent-contracts.md", "docs/workflow-test-menu.md", "cmd/worktree_preflight_test.go"]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-06` Record active workflow-feedback dispositions and new friction found during this run.
  - wave: 2
  - depends_on: [t-01, t-02, t-03, t-04]
  - target_files: ["artifacts/changes/fix-slipway-governed-workflow-feedback-from-archived-clinvoker-end-to-end-run/workflow-feedback.md"]
  - task_kind: doc
  - covers: [REQ-001]

- [x] `t-07` Run targeted, full, build, and governed workflow verification; update assurance and evidence.
  - wave: 3
  - depends_on: [t-05, t-06]
  - target_files: ["artifacts/changes/fix-slipway-governed-workflow-feedback-from-archived-clinvoker-end-to-end-run/assurance.md", "artifacts/changes/fix-slipway-governed-workflow-feedback-from-archived-clinvoker-end-to-end-run/verification"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
