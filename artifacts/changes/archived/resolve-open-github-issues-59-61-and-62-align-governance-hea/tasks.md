# Tasks

## Project Context
- Tech Stack: Go CLI
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` RED tests for issue #59 health traceability gap details.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/health_test.go", "internal/engine/governance/health_test.go"]
  - task_kind: code
  - evidence: failing_test
  - covers: [REQ-001, REQ-005]

- [x] `t-02` RED tests for issue #61 confirmation action metadata.
  - wave: 1
  - depends_on: []
  - target_files: ["cmd/progression_next_test.go"]
  - task_kind: code
  - evidence: failing_test
  - covers: [REQ-002, REQ-005]

- [x] `t-03` RED test for issue #62 portable goal-verification placeholder scan.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/tmpl/templates_test.go"]
  - task_kind: code
  - evidence: failing_test
  - covers: [REQ-003, REQ-005]

- [x] `t-04` Implement issue #59 structured traceability gap details on governance health checks.
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/engine/governance/health.go", "cmd/health.go"]
  - task_kind: code
  - evidence: test_pass
  - covers: [REQ-001, REQ-004, REQ-005]

- [x] `t-05` Implement issue #61 non-checkpoint/active-checkpoint action metadata in `confirmation_requirement`.
  - wave: 2
  - depends_on: [t-02]
  - target_files: ["cmd/next.go", "cmd/next_handoff.go"]
  - task_kind: code
  - evidence: test_pass
  - covers: [REQ-002, REQ-004, REQ-005]

- [x] `t-06` Implement issue #62 portable goal-verification placeholder scan.
  - wave: 2
  - depends_on: [t-03]
  - target_files: ["internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl"]
  - task_kind: code
  - evidence: test_pass
  - covers: [REQ-003, REQ-004, REQ-005]

- [x] `t-07` Run focused and full verification before review/close-ready progression.
  - wave: 3
  - depends_on: [t-04, t-05, t-06]
  - target_files: ["cmd/health_test.go", "cmd/progression_next_test.go", "internal/engine/governance/health_test.go", "internal/tmpl/templates_test.go"]
  - task_kind: verification
  - evidence: command_output
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]

- [x] `t-08` (DESCOPED -> #66) RED regression for plan-audit source-freshness — removed before ship; an mtime guard false-positives on Slipway's own tasks.md checkbox writeback.
  - wave: 4
  - depends_on: [t-07]
  - target_files: ["internal/engine/progression/evidence_test.go"]
  - task_kind: code
  - evidence: failing_test
  - covers: [REQ-006]

- [x] `t-09` (DESCOPED -> #66) Plan-audit source-freshness guard — implemented then reverted before ship; content-digest replacement tracked in #66. Net code change is the revert only.
  - wave: 5
  - depends_on: [t-08]
  - target_files: ["internal/engine/progression/evidence.go", "internal/engine/progression/evidence_test.go", "internal/engine/progression/advance_governed.go", "internal/engine/progression/advance_test.go", "internal/engine/progression/readiness.go"]
  - task_kind: code
  - evidence: test_pass
  - covers: [REQ-006, REQ-004]

- [x] `t-10` Document command authority boundaries and confirmation metadata.
  - wave: 4
  - depends_on: [t-05]
  - target_files: ["cmd/governance_gate_consistency_test.go", "docs/commands.md", "internal/tmpl/templates/_partials/command-next-body.tmpl", "internal/tmpl/templates/_partials/command-run-body.tmpl", "internal/tmpl/templates_test.go"]
  - task_kind: code
  - evidence: test_pass
  - covers: [REQ-002, REQ-005, REQ-007]

- [x] `t-11` Refresh full verification after #59 item 2/3/4 repairs.
  - wave: 7
  - depends_on: [t-09, t-10, t-12]
  - target_files: ["cmd/governance_gate_consistency_test.go", "cmd/health_test.go", "cmd/lifecycle_commands_test.go", "cmd/progression_next_test.go", "cmd/repair_test.go", "docs/commands.md", "internal/engine/governance/health_test.go", "internal/engine/progression/advance_test.go", "internal/engine/progression/evidence_test.go", "internal/engine/progression/wave_sync_test.go", "internal/tmpl/templates_test.go"]
  - task_kind: verification
  - evidence: command_output
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]

- [x] `t-12` Enforce wave-orchestration evidence freshness against current runtime task evidence.
  - wave: 6
  - depends_on: [t-09]
  - target_files: ["cmd/lifecycle_commands_test.go", "cmd/progression_next_test.go", "cmd/repair_test.go", "internal/engine/progression/wave_sync.go", "internal/engine/progression/wave_sync_test.go", "internal/model/reason_code.go", "internal/model/model_test.go"]
  - task_kind: code
  - evidence: test_pass
  - covers: [REQ-006, REQ-004]
