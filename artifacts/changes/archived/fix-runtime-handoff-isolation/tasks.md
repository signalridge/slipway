# Tasks

## Task List

- [x] `t-01` Add per-change session handoff path helpers and path tests
  - depends_on: []
  - target_files: [`internal/state/local_runtime_paths.go`, `internal/state/local_runtime_paths_test.go`]
  - task_kind: code
  - covers: [REQ-001]
  - evidence: `go test ./internal/state -run 'TestGitScopedPaths|TestChangeHandoff'`
  - acceptance: `state` exposes a per-change handoff path under the existing `runtime/changes/<slug>` root and tests pin root plus nested-scope behavior without duplicating runtime root derivation.

- [x] `t-02` Route session-start handoff reporting through the current change
  - depends_on: [`t-01`]
  - target_files: [`cmd/session_start_hook.go`, `cmd/session_start_hook_test.go`]
  - task_kind: code
  - covers: [REQ-001]
  - evidence: `go test ./cmd -run TestSessionStartHook`
  - acceptance: session-start tests construct two bound changes/worktrees, report only the current resolved change's per-change handoff path, and do not embed handoff body content.

- [x] `t-03` Update runtime handoff guidance in hooks, templates, and generated-surface tests
  - depends_on: [`t-01`]
  - target_files: [`cmd/context_pressure_hook.go`, `cmd/context_pressure_hook_test.go`, `internal/tmpl/templates/_partials/command-run-body.tmpl`, `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`, `internal/tmpl/templates_test.go`, `internal/toolgen/toolgen_test.go`]
  - task_kind: code
  - covers: [REQ-004]
  - evidence: `go test ./cmd -run TestContextPressureHook && go test ./internal/tmpl ./internal/toolgen`
  - acceptance: generated and hook-facing guidance names the per-change handoff contract and keeps the advisory non-authority warning.

- [x] `t-04` Add runtime hygiene health/repair command handling
  - depends_on: [`t-01`, `t-05`]
  - target_files: [`internal/state/health.go`, `internal/state/health_test.go`, `cmd/health.go`, `cmd/health_test.go`, `cmd/repair.go`, `cmd/repair_test.go`, `internal/model/reason_code.go`, `internal/model/reason_code_contract_test.go`]
  - task_kind: code
  - covers: [REQ-002, REQ-003]
  - evidence: `go test ./internal/state -run Health && go test ./cmd -run 'TestHealth|TestRepair' && go test ./internal/model -run TestCanonicalReasonCodeTaxonomySnapshot`
  - acceptance: health/repair report or safely clean repo-level handoff files, retired `.git/slipway/changes` directories, and repair-command lock-anchor cleanup using canonical runtime hygiene reason codes without hiding ambiguous content.

- [x] `t-05` Add safe empty lock-anchor cleanup primitive while preserving global create and repair locks
  - depends_on: []
  - target_files: [`internal/fsutil/lock.go`, `internal/fsutil/lock_test.go`]
  - task_kind: code
  - covers: [REQ-003]
  - evidence: `go test ./internal/fsutil -run Lock`
  - acceptance: `fsutil` exposes a safe cleanup primitive that removes only unheld lock anchors without `.meta`, preserves active locks, and leaves global `change-create.lock` plus `repair.lock` semantics to command-level wiring.

- [x] `t-06` Verify runtime hygiene behavior and update operator documentation
  - depends_on: [`t-02`, `t-03`, `t-04`, `t-05`]
  - target_files: [`docs/operator-guide.md`, `docs/index.md`, `docs/commands.md`, `README.md`]
  - task_kind: doc
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004]
  - evidence: `go test ./cmd ./internal/state ./internal/fsutil ./internal/tmpl ./internal/toolgen && go run . validate --json`
  - acceptance: operator docs describe per-change handoff, legacy cleanup, and global lock boundaries without claiming repo-level handoff is current authority.
