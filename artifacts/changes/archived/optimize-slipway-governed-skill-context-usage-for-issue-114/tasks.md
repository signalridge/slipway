# Tasks
## Project Context
- Tech Stack: Go
- Conventions: # Slipway Agent Principles

Slipway is the lifecycle authority for governed work. This file is not a
command manual, classification guide, JSON reference, or recovery cookbook. It
sets the principles an AI agent must follow when working in this repository.

## Lifecycle Authority

- Treat the current worktree's Slipway CLI as the source of truth.
- Use the Slipway behavior produced by the current worktree, not stale installed
  binaries, remembered flows, or copied recipes.
- Let Slipway decide ...
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Author embedded-template contract tests (RED) that assert thin-host / summary-first behavior for all three stages and lock in the preserved governed contracts: goal-verification keeps bulky evidence out of the main context with a fresh-verifier-"when supported" delegation and a fail-closed `safety_baseline` anchored to a real `fresh:command_ref` (never a prose-only verdict); worktree-preflight reduces the baseline to command + exit + bounded summary + reference; wave-orchestration coordinator no longer reads the four codebase-map docs and the PR #112 staleness self-check is relocated, not removed; and every template still carries its IRON LAW, HARD-GATE, freshness/`run_version`, evidence-artifact, and `slipway evidence task` language with no bypass token. Tests must fail before implementation.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/tmpl/thin_host_content_test.go", "internal/tmpl/templates_test.go"]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
  - evidence: verdict

- [x] `t-02` Refactor the goal-verification template to a thin host: enumerate acceptance criteria in the main host, delegate the stub/placeholder scan, SAST run, and fresh-test reading to an isolated verifier context "when supported; otherwise" a bounded structured-summary fallback, and keep the host owning the final verdict and HARD-GATE. Preserve the guardrail `safety_baseline` requirement by recording `high_risk_check:<domain>.safety_baseline=pass` only with a `fresh:command_ref` to a real SAST output artifact, failing closed (`high_risk_check_missing`) on missing/stale/inconclusive delegated evidence. Make t-01's goal-verification assertions pass (GREEN).
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl"]
  - task_kind: code
  - covers: [REQ-001, REQ-004, REQ-005]
  - evidence: verdict

- [x] `t-03` Refactor the worktree-preflight template so the host retains only the baseline command, exit code, and a bounded failure summary in the main context, writes the full baseline output to a referenceable artifact, and still records the required worktree path, branch, and exact baseline command references. Use the portable "isolated context when supported; otherwise bounded summary" idiom. Make t-01's worktree-preflight assertions pass (GREEN).
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/tmpl/templates/skills/worktree-preflight/SKILL.md"]
  - task_kind: code
  - covers: [REQ-002, REQ-004, REQ-005]
  - evidence: verdict

- [x] `t-04` Slim the wave-orchestration coordinator so it no longer reads STRUCTURE/CONVENTIONS/TESTING/CONCERNS into its own context: pass `input_context.codebase_map_dir` plus the relevant document paths to executors and limit the coordinator to the engine-authoritative `wave_plan` metadata. Relocate (do not delete) the PR #112 (issue #80) codebase-map relevance/staleness self-check — to per-executor refresh or a `codebase_map_doc_states`-driven coordinator decision — and update `references/executor-dispatch-reference.md` accordingly. Make t-01's wave-orchestration assertions pass (GREEN).
  - wave: 2
  - depends_on: [t-01]
  - target_files: ["internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl", "internal/tmpl/templates/skills/wave-orchestration/references/executor-dispatch-reference.md"]
  - task_kind: code
  - covers: [REQ-003, REQ-004, REQ-005]
  - evidence: verdict

- [x] `t-05` Run the full `go test ./...` suite (and focused `go test ./internal/tmpl`) and confirm the thin-host contract tests from t-01 are now green across all three stages with no regression elsewhere. This is the holistic green gate after the three parallel template refactors.
  - wave: 3
  - depends_on: [t-02, t-03, t-04]
  - target_files: ["internal/tmpl"]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
  - evidence: verdict

- [x] `t-06` Repair the S2 Scope Contract false positive uncovered during execution: exclude durable `artifacts/codebase/` discovery-context artifacts from S2 workspace-diff drift checks while preserving implementation/test drift detection and `done` dirty-worktree visibility. Add regression coverage for the Scope Contract sampler and an S2 complete-runtime-evidence advancement path.
  - wave: 4
  - depends_on: [t-05]
  - target_files: ["internal/engine/progression/readiness.go", "internal/engine/progression/readiness_optimization_test.go", "internal/engine/progression/advance_test.go"]
  - task_kind: code
  - covers: [REQ-007]
  - evidence: verdict
