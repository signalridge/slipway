# Tasks

## Task List

- [x] `t-01` Make `ComputeFileContentHash` binary-safe CRLF-normalizing before SHA-256, with unit tests (CRLF-invariant text, byte-exact binary, LF digest unchanged)
  - depends_on: []
  - target_files: [internal/model/evidence.go, internal/model/evidence_test.go]
  - task_kind: code
  - covers: [REQ-001]

- [x] `t-02` Add Windows-regression tests: CRLF-re-materialized governed artifact is not reported stale by reconciliation; goal-verification evidence digest is stable across an LF↔CRLF round-trip
  - depends_on: [t-01]
  - target_files: [internal/engine/artifact/manager_test.go, internal/engine/progression/evidence_digests_test.go]
  - task_kind: test
  - covers: [REQ-002, REQ-009]

- [x] `t-03` Add root `.gitattributes` enforcing `eol=lf` for hashed text/artifact globs (`artifacts/**`, `*.md`, `*.yaml`/`*.yml`, `*.tmpl`, `*.go`) and marking binary asset types `binary`
  - depends_on: []
  - target_files: [.gitattributes]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-04` Implement real Windows `isPIDAlive` in a new `//go:build windows` file using `golang.org/x/sys/windows`; narrow `process_other.go` to `!unix && !windows`; add a Windows-only liveness test
  - depends_on: []
  - target_files: [cmd/process_windows.go, cmd/process_other.go, cmd/process_windows_test.go]
  - task_kind: code
  - covers: [REQ-004, REQ-009]

- [x] `t-05` Add Windows sharing-violation bounded retry around the `os.Rename` in `WriteFileAtomic`, GOOS-guarded; add a test exercising the retry path
  - depends_on: []
  - target_files: [internal/fsutil/atomic.go, internal/fsutil/atomic_test.go]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-06` Add `os.Symlink` privilege-failure dereference fallback to worktree provisioning and the archive copy so a symlinked source materializes its target content
  - depends_on: []
  - target_files: [internal/toolgen/worktree_provision.go, internal/toolgen/worktree_provision_test.go, internal/state/lifecycle.go, internal/state/lifecycle_test.go, internal/fsutil/symlink.go, internal/fsutil/symlink_test.go]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-07` Make the generated `settings.json` hook `command` platform-portable: register direct `slipway hook <event>` commands (PATH-resolved, OS-uniform, no shell chaining operators) instead of an init-host-OS-pinned launcher path; keep launcher files generated for host-native/manual dispatch; update the toolgen settings-command contract tests
  - depends_on: []
  - target_files: [internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/toolgen/adapter_contract_test.go]
  - task_kind: code
  - covers: [REQ-007]

- [x] `t-08` Replace the Unix-only `grep`/`perl` stub scan in the goal-verification skill template with a portable, tool-agnostic instruction, and update the contract test that pins the old `perl`/`grep` constructs
  - depends_on: []
  - target_files: [internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl, internal/tmpl/templates_test.go]
  - task_kind: code
  - covers: [REQ-008]

- [x] `t-09` Regenerate the local generated adapter surfaces affected by t-07/t-08 (portable hook command in settings.json; goal-verification SKILL across all adapter trees) so ignored/local surfaces match the updated generator and template
  - depends_on: [t-07, t-08]
  - target_files: [.claude/settings.json, .gemini/settings.json, .claude/skills/slipway-goal-verification/SKILL.md, .codex/skills/slipway-goal-verification/SKILL.md, .cursor/skills/slipway-goal-verification/SKILL.md, .gemini/skills/slipway-goal-verification/SKILL.md, .opencode/skills/slipway-goal-verification/SKILL.md]
  - task_kind: code
  - covers: [REQ-007, REQ-008]

- [x] `t-10` Align public adapter documentation with the direct shell-neutral `settings.json` hook command and the separate native launcher file role
  - depends_on: [t-07]
  - target_files: [README.md, docs/ai-tools.md, docs/installation.md]
  - task_kind: code
  - covers: [REQ-007, REQ-010]
