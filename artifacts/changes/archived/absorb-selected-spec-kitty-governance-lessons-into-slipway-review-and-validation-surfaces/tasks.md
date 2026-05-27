# Tasks

- [x] `t-01` implement Scope Contract evaluator
  - wave: 1
  - depends_on: []
  - target_files: ["internal/engine/scopecontract/evaluate.go", "internal/engine/scopecontract/evaluate_test.go"]
  - task_kind: code
  - acceptance: REQ-001, REQ-002, and REQ-003 are covered by focused evaluator tests.
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-02` surface Scope Contract drift in governance validation and review readiness
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["cmd/validate.go", "cmd/review.go", "cmd/status.go", "cmd/status_view_build.go", "cmd/scope_contract_test.go", "cmd/execution_summary_test_helper_test.go", "internal/engine/progression/readiness.go", "internal/model/reason_code.go"]
  - task_kind: code
  - acceptance: REQ-004 is covered by CLI/readiness tests that fail on out-of-scope changed files after execution evidence exists.
  - covers: [REQ-004]

- [x] `t-03` make codebase-map output worktree-local for updateable artifacts
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["cmd/codebase_map.go", "cmd/codebase_map_command_test.go", "cmd/codebase_map_context_test.go", "internal/state/paths.go", "internal/state/paths_test.go", "internal/state/store.go"]
  - task_kind: code
  - acceptance: REQ-007 is covered by command/context/path tests that prove a worktree invocation updates worktree-local `artifacts/codebase` and leaves the main checkout untouched.
  - covers: [REQ-007]

- [x] `t-04` tighten host skill guidance for contract evidence checks
  - wave: 3
  - depends_on: [t-01]
  - target_files: ["internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl", "internal/tmpl/templates/skills/spec-trace/SKILL.md", "internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl", "internal/engine/capability/contract_absorption_test.go"]
  - task_kind: doc
  - acceptance: REQ-005 is covered by tests that lock contract evidence guidance in generated host skill templates.
  - covers: [REQ-005]

- [x] `t-05` verify governance boundaries and final behavior
  - wave: 4
  - depends_on: [t-02, t-03, t-04]
  - target_files: ["artifacts/changes/absorb-selected-spec-kitty-governance-lessons-into-slipway-review-and-validation-surfaces/**", "artifacts/codebase/**"]
  - task_kind: verification
  - acceptance: REQ-006 and REQ-007 are documented and tests pass without introducing lane/platform/adaptor expansion or root-checkout codebase-map mutation from a worktree.
  - covers: [REQ-006, REQ-007]
