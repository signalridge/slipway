# Tasks
## Project Context
- Tech Stack: Go CLI, Cobra, Slipway governance runtime
- Conventions: command metadata in `internal/toolgen`, command wiring in `cmd/root.go`, compact YAML verification records, flat runtime task evidence JSON
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Add the `slipway evidence task` CLI surface and controlled task evidence writer.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/evidence.go, cmd/root.go, internal/toolgen/toolgen.go, docs/commands.md]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - acceptance: command writes parseable flat task evidence with computed freshness inputs and rejects invalid verdict/run/task input.

- [x] `t-02` Add command-level regressions for task evidence recording.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [cmd/evidence_task_test.go, cmd/template_flag_contract_test.go, internal/toolgen/toolgen_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-006]
  - acceptance: tests prove JSON output path, parse compatibility, controlled freshness inputs, and validation failures.

- [x] `t-03` Update wave-orchestration host/user guidance to require the supported task evidence surface.
  - wave: 3
  - depends_on: [t-01, t-02]
  - target_files: [internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl, internal/tmpl/templates_test.go, docs/commands.md]
  - task_kind: code
  - covers: [REQ-003, REQ-006]
  - acceptance: rendered wave-orchestration skill names `slipway evidence task`, required fields, and execution-summary dependency.

- [x] `t-04` Tighten repair coverage for rebuildable vs missing task evidence.
  - wave: 3
  - depends_on: [t-01, t-02]
  - target_files: [cmd/repair.go, cmd/repair_test.go, internal/engine/progression/wave_sync.go]
  - task_kind: code
  - covers: [REQ-004, REQ-006]
  - acceptance: repair rebuilds from supported task evidence and reports missing source task evidence with an actionable command hint.

- [x] `t-05` Implement final-closeout goal-verification reuse validation.
  - wave: 3
  - depends_on: [t-01, t-02]
  - target_files: [internal/engine/progression/evidence.go, internal/engine/progression/authority.go, internal/engine/progression/evidence_test.go, internal/engine/progression/authority_test.go, cmd/progression_next_test.go]
  - task_kind: code
  - covers: [REQ-005, REQ-006]
  - acceptance: matching reuse passes, missing/mismatched/stale reuse blocks with an actionable reason code.

- [x] `t-06` Update final-closeout generated guidance and template tests for the reuse branch.
  - wave: 4
  - depends_on: [t-05]
  - target_files: [internal/tmpl/templates/skills/final-closeout/SKILL.md.tmpl, internal/tmpl/templates/_partials/fresh-evidence-contract.tmpl, internal/tmpl/templates_test.go]
  - task_kind: code
  - covers: [REQ-005, REQ-006]
  - acceptance: final-closeout guidance explains when reuse is allowed, what references to record, and when rerun remains mandatory.

- [x] `t-07` Run focused and full verification, then stop before `slipway done`.
  - wave: 5
  - depends_on: [t-03, t-04, t-05, t-06]
  - target_files: [artifacts/changes/complete-issue-53-tier-2-evidence-contract-and-closeout-reus/verification/**]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
  - acceptance: targeted tests, `go test ./...`, `go build ./...`, `go vet ./...`, `git diff --check`, and `slipway validate --json` pass; workflow is ready for closeout but `slipway done` is not run.
