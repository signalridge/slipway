# Tasks

## Project Context
- Tech Stack: Go CLI with Cobra commands and filesystem-backed governance artifacts.
- Conventions: Use focused tests first, avoid public CLI surface changes, and preserve final `go test ./...` plus `go build ./...` verification.
- Test Command: `go test ./...`
- Build Command: `go build ./...`
- Languages: Go

## Task List

- [x] `t-01` Record baseline and identify the dominant runtime bottleneck.
  - wave: 1
  - depends_on: []
  - target_files: [`artifacts/changes/optimize-test-runtime-and-governance-verification-workflow/research.md`]
  - task_kind: verification
  - covers: [REQ-001]
  - evidence: Baseline JSON log plus research notes naming the slowest packages and measured `cmd` package runtime.
  - acceptance: Package timing summary identifies `cmd` as the bottleneck and records the baseline full-suite real time.

- [x] `t-02` Remove strict-subset tests while preserving stronger command contract coverage.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [`cmd/stats_test.go`, `cmd/progression_next_test.go`, `cmd/worktree_preflight_test.go`]
  - task_kind: code
  - covers: [REQ-002]
  - evidence: Redundant tests removed, retained tests still covering the same stats, next-auto-pass, and worktree-preflight behavior.
  - acceptance: Focused `go test ./cmd -run ... -count=1` commands covering retained tests pass.

- [x] `t-03` Reduce fixture setup cost for repository identity.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [`cmd/new_test.go`, `cmd/common_test.go`]
  - task_kind: code
  - covers: [REQ-003]
  - evidence: `ensureTestGitRepo` prepares a minimal valid `.git` layout without repeated git subprocesses for tests that only need repository identity.
  - acceptance: New/root/worktree-preflight targeted tests that rely on repository fixtures pass.

- [x] `t-04` Add private command-root injection and migrate safe command tests.
  - wave: 3
  - depends_on: [t-01, t-02, t-03]
  - target_files: [`cmd/root.go`, `cmd/common.go`, `cmd/init.go`, `cmd/repair.go`, `cmd/health.go`, `cmd/common_test.go`, `cmd/health_test.go`, `cmd/error_contract_test.go`, `cmd/cli_e2e_test.go`, `cmd/progression_next_test.go`, `cmd/lifecycle_commands_test.go`]
  - task_kind: code
  - covers: [REQ-003, REQ-004]
  - evidence: Commands can be exercised against temp roots without global cwd mutation; isolated health and CLI workflow tests use `t.Parallel()`.
  - acceptance: `go test ./cmd -run '^TestHealth|^TestCLIEndToEnd|^TestInit' -count=1` and `go test ./cmd -count=1` pass.

- [x] `t-05` Narrow governance verification guidance without weakening final proof.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [`internal/tmpl/templates/skills/worktree-preflight/SKILL.md`, `internal/tmpl/templates_test.go`, `docs/workflow-test-menu.md`]
  - task_kind: code
  - covers: [REQ-005]
  - evidence: Worktree-preflight and workflow-test guidance prefer bounded intermediate verification and one fresh final full-suite/build proof.
  - acceptance: Template tests pass and docs clearly distinguish intermediate from final verification.

- [x] `t-06` Run targeted verification for changed behavior.
  - wave: 4
  - depends_on: [t-02, t-03, t-04, t-05]
  - target_files: [`artifacts/changes/optimize-test-runtime-and-governance-verification-workflow/verification/`]
  - task_kind: verification
  - covers: [REQ-002, REQ-003, REQ-004, REQ-005]
  - evidence: Evidence file lists focused command/template checks and their pass results.
  - acceptance: Targeted checks for retained tests, root injection, health, CLI e2e, init, and template wording pass.

- [x] `t-07` Run final full-suite and build verification with timing comparison.
  - wave: 5
  - depends_on: [t-06]
  - target_files: [`artifacts/changes/optimize-test-runtime-and-governance-verification-workflow/assurance.md`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - evidence: Final `go test ./...` JSON timing log, `go build ./...` result, and assurance summary comparing baseline and final runtime.
  - acceptance: Final full-suite and build pass; assurance records runtime delta and residual risks.

- [x] `t-08` Record governed execution and readiness evidence.
  - wave: 6
  - depends_on: [t-07]
  - target_files: [`artifacts/changes/optimize-test-runtime-and-governance-verification-workflow/verification/`, `artifacts/changes/optimize-test-runtime-and-governance-verification-workflow/assurance.md`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005]
  - evidence: Verification sidecars and assurance document explain what was changed, why it is covered, and how rollback works.
  - acceptance: Slipway validation shows no blockers caused by missing or stale plan/execution evidence.
