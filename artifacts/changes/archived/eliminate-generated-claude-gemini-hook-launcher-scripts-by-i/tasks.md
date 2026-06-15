# Tasks

## Task List

- [x] `t-01` Make `slipway hook context-pressure` and `slipway hook session-start` fail-silent (always exit 0) on empty/malformed stdin and internal errors
  - depends_on: []
  - target_files: [cmd/context_pressure_hook.go, cmd/session_start_hook.go]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-02` Key hook registration on `SettingsPath`: register the bare inline `slipway hook ...` command in settings.json for settings-capable hosts, stop emitting their launcher files, keep emitting launcher files for file-by-path hosts (cursor/opencode), prune orphaned launcher files and migrate stale launcher-path commands on `--refresh`; also remove the now-dead `GenerateWorktreeLocal` helper defined in `toolgen.go` (its sole caller `internal/toolgen/worktree_provision.go` is removed by t-04 in the same wave)
  - depends_on: []
  - target_files: [internal/toolgen/toolgen.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-004, REQ-006]

- [x] `t-03` Delete the now-dead `context-pressure-post-tool-use.{sh,ps1,cmd}.tmpl` templates (retain the `session-start.*` templates)
  - depends_on: [t-02]
  - target_files: [internal/tmpl/templates/hooks/context-pressure-post-tool-use.sh.tmpl, internal/tmpl/templates/hooks/context-pressure-post-tool-use.ps1.tmpl, internal/tmpl/templates/hooks/context-pressure-post-tool-use.cmd.tmpl]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-04` Remove worktree host-surface provisioning end-to-end: delete `ProvisionWorktreeHostSurfaces` and its copy/exclude helpers in `internal/toolgen/worktree_provision.go`, remove the `WorktreeProvisioner` type in `internal/state/worktree_provision.go`, drop the provisioner parameter from `EnsureDefaultWorktreeForChange` in `internal/state/worktree.go`, and update the `slipway new` wiring in `cmd/new.go` (keep worktree creation). The now-dead `GenerateWorktreeLocal` in `toolgen.go` is removed by t-02 in the same wave
  - depends_on: []
  - target_files: [internal/toolgen/worktree_provision.go, internal/state/worktree_provision.go, internal/state/worktree.go, cmd/new.go]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-05` Update the hook-subcommand tests to assert exit 0 on empty/malformed input for both subcommands
  - depends_on: [t-01]
  - target_files: [cmd/context_pressure_hook_test.go, cmd/session_start_hook_test.go]
  - task_kind: test
  - covers: [REQ-003]

- [x] `t-06` Update toolgen and adapter-contract tests: assert the bare inline command in settings.json, no launcher files for settings-capable hosts, retained cursor/opencode launchers, and the refresh prune+migrate behavior
  - depends_on: [t-02]
  - target_files: [internal/toolgen/toolgen_test.go, internal/toolgen/adapter_contract_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-004]

- [x] `t-07` Update the template/hook-behavior tests to drop assertions on the removed post-tool-use templates and keep session-start coverage
  - depends_on: [t-03]
  - target_files: [internal/tmpl/hooks_behavior_test.go, internal/tmpl/templates_test.go]
  - task_kind: test
  - covers: [REQ-005]

- [x] `t-08` Delete the provisioning test files and update worktree/new tests for the dropped provisioner parameter and removed provisioned-surface assertions
  - depends_on: [t-04]
  - target_files: [internal/toolgen/worktree_provision_test.go, internal/state/worktree_provision_test.go, internal/state/worktree_test.go, cmd/new_test.go]
  - task_kind: test
  - covers: [REQ-006]

- [x] `t-09` Update generated docs: drop `.claude/hooks/`/`.gemini/hooks/` launcher enumeration for settings-capable hosts, add the root-startup clarification, and remove descriptions of provisioning host surfaces into worktrees
  - depends_on: []
  - target_files: [docs/ai-tools.md, docs/installation.md, README.md]
  - task_kind: doc
  - covers: [REQ-007]

- [x] `t-10` Regenerate the surface manifest through the public generator so it matches the new hook contract
  - depends_on: [t-02, t-03, t-04]
  - target_files: [docs/SURFACE-MANIFEST.json]
  - task_kind: code
  - covers: [REQ-007]

- [x] `t-11` Final verification: build the CLI to a path outside the worktree, run `go test ./...` and `gen-surface-manifest --check`, and record the evidence
  - depends_on: [t-01, t-02, t-03, t-04, t-05, t-06, t-07, t-08, t-09, t-10]
  - target_files: [artifacts/changes/eliminate-generated-claude-gemini-hook-launcher-scripts-by-i/verification/build-test-notes.md]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
