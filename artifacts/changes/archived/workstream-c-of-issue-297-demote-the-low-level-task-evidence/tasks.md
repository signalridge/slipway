# Tasks

## Task List

- [x] `t-01` Update contract/template tests to expect the demoted (result-file-only) agent surface
  - depends_on: []
  - target_files: ["internal/tmpl/templates_test.go", "internal/toolgen/toolgen_test.go", "cmd/template_flag_contract_test.go", "cmd/command_description_contract_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-007]
  - acceptance:
    - Tests assert the tdd-governance, wave-orchestration, and evidence agent surfaces teach `slipway evidence task --result-file` and DO NOT teach the manual `--task-id`/`--run-summary-version`/`--task-kind`/`--verdict`/`--evidence-ref`/`--target-file` protocol.
    - A test asserts exactly one `--help` breadcrumb remains for the manual fallback in the evidence command body.
    - The toolgen evidence Arguments assertion expects no manual `evidence task` flag variant.
    - The reverse Cobra flag coverage test is updated with a narrow, documented exemption for evidence-task manual-mode flags so they remain Cobra-visible and black-box `--help` discoverable while being intentionally omitted from the agent-facing Arguments contract.
    - These tests are RED against current templates/toolgen (evidence is `go test ./internal/tmpl ./internal/toolgen ./cmd -run 'Evidence|Template|Arguments|Description|FlagContract'` showing the expected failures).

- [x] `t-02` Demote manual-flag teaching in the four agent-facing source surfaces
  - depends_on: ["t-01"]
  - target_files: ["internal/tmpl/templates/skills/tdd-governance/SKILL.md.tmpl", "internal/tmpl/templates/_partials/command-evidence-body.tmpl", "internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl", "internal/toolgen/toolgen.go"]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-006]
  - acceptance:
    - tdd-governance template frames investigation/doc recording as result-import; no `--task-kind`/`--verdict`/`--evidence-ref` flag instruction remains.
    - command-evidence-body presents `evidence task` as `--result-file`/`--json`/`--change` plus ONE `--help` breadcrumb; the 10-field manual Flags enumeration and manual-only Contract bullets are removed; `evidence skill` section preserved.
    - toolgen evidence Arguments expose only `task --result-file ...`, `skill ...`, `suite-result ...`.
    - wave-orchestration template no longer frames manual flag mode as an agent path; result-JSON guardrail intent preserved.
    - The t-01 tests now pass (evidence is the same focused `go test` transcript, GREEN).

- [x] `t-03` Regenerate derived adapter surfaces, docs, and surface manifest from the worktree binary
  - depends_on: ["t-02"]
  - target_files: ["docs/SURFACE-MANIFEST.json", "docs/commands.md", "docs/reference/commands.md", "docs/reference/ai-tools.md", ".codex/skills/slipway-evidence/SKILL.md", ".codex/skills/slipway-tdd-governance/SKILL.md", ".codex/skills/slipway-wave-orchestration/SKILL.md", ".claude/commands/slipway/evidence.md", ".claude/skills/slipway-tdd-governance/SKILL.md", ".claude/skills/slipway-wave-orchestration/SKILL.md"]
  - task_kind: doc
  - covers: [REQ-005]
  - acceptance:
    - Generated local adapter surfaces, docs, and manifest are regenerated with the worktree binary built from the t-02 edits (rebuild first; never a stale binary).
    - `go run ./internal/toolgen/cmd/gen-surface-manifest --write` updates `docs/SURFACE-MANIFEST.json`, then `go run ./internal/toolgen/cmd/gen-surface-manifest --check` passes.
    - `rg "evidence task --task-id .*--run-summary-version|--task-kind <kind>|--target-file <path>"` over `internal/tmpl`, `.codex/skills`, `.claude/skills`, `.claude/commands/slipway/evidence.md`, `docs/` returns no agent-facing manual-protocol teaching.
    - Evidence includes the committed docs/manifest diff plus explicit content checks for ignored local adapter outputs under `.codex/` and `.claude/`. (If the refresh touches additional generated files, add them to this task's target_files before recording evidence.)

- [x] `t-04` Run focused verification, surface-manifest check, full suite, and black-box help checks
  - depends_on: ["t-01", "t-02", "t-03"]
  - target_files: ["artifacts/changes/workstream-c-of-issue-297-demote-the-low-level-task-evidence/verification/**"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
  - acceptance:
    - `go test ./internal/tmpl ./internal/toolgen ./cmd` passes.
    - `go run ./internal/toolgen/cmd/gen-surface-manifest --check` passes.
    - `go test ./...` passes.
    - The black-box check confirms `slipway evidence task --help` still shows `--result-file` and the manual flags labeled "Manual flag mode only" (REQ-006 invariant), and that generated evidence/tdd/wave skills teach only `--result-file` plus the breadcrumb.
    - Evidence is a verification checklist and command transcript references under the change's verification directory.
