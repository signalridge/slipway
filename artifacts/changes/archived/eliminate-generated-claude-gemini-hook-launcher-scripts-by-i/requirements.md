# Requirements

## Requirements

### Requirement: Inline hook command for settings-capable hosts
REQ-001: For every host with a non-empty `SettingsPath` (claude
`.claude/settings.json`, gemini `.gemini/settings.json`), the generator MUST
register each hook event in settings.json as a bare inline command
(`slipway hook context-pressure` for `PostToolUse`, `slipway hook session-start`
for `SessionStart`) that contains no shell operators, MUST NOT reference any
`.claude/hooks/` or `.gemini/hooks/` launcher path, and MUST keep the command
portable across `/bin/sh -c`, `cmd /c`, and PowerShell.

#### Scenario: Claude settings.json registers the bare command
GIVEN a repository initialized for the claude adapter
WHEN `slipway init` (or `slipway init --refresh`) regenerates host surfaces
THEN `.claude/settings.json` registers `PostToolUse` as `slipway hook context-pressure`
AND registers `SessionStart` as `slipway hook session-start`
AND neither command string contains a `.claude/hooks/` path or a shell operator.

#### Scenario: Gemini settings.json registers the bare command
GIVEN a repository initialized for the gemini adapter
WHEN host surfaces are regenerated
THEN `.gemini/settings.json` registers `SessionStart` as `slipway hook session-start`
AND the command string contains no `.gemini/hooks/` path.

### Requirement: Settings-capable hosts stop emitting launcher files and clean up stragglers
REQ-002: For hosts with a non-empty `SettingsPath`, the generator MUST NOT write
hook launcher script files (the extensionless POSIX entry and its `.ps1`/`.cmd`
variants). On `--refresh` the generator MUST prune any previously generated
launcher files for those hosts and MUST migrate an existing settings.json whose
hook command still references the old launcher path to the new bare inline command.

#### Scenario: No launcher files generated for claude
GIVEN a clean repository
WHEN `slipway init` generates the claude adapter
THEN no file exists at `.claude/hooks/slipway-session-start` (nor `.ps1`/`.cmd`)
AND no file exists at `.claude/hooks/slipway-context-pressure-post-tool-use` (nor `.ps1`/`.cmd`).

#### Scenario: Refresh migrates a stale launcher-path command and prunes the orphaned file
GIVEN a `.claude/settings.json` whose `SessionStart` command points at a
`.claude/hooks/slipway-session-start` launcher path AND that launcher file exists
WHEN `slipway init --refresh` runs
THEN the `SessionStart` command becomes the bare `slipway hook session-start`
AND the orphaned launcher file is removed
AND any user-authored hook entries in settings.json are preserved.

### Requirement: Hook subcommands are fail-silent
REQ-003: The `slipway hook context-pressure` and `slipway hook session-start`
subcommands MUST exit with status 0 on every input, including empty or malformed
stdin and any internal error, so the inlined bare command can never surface a
blocking or non-zero hook failure.

#### Scenario: Malformed input still exits zero
GIVEN arbitrary malformed or empty data on stdin
WHEN `slipway hook context-pressure` runs
THEN the process exits with status 0
AND WHEN `slipway hook session-start` runs on the same input
THEN the process exits with status 0.

### Requirement: File-by-path hosts are unchanged
REQ-004: For hosts with an empty `SettingsPath` but a non-empty `SessionHook`
(cursor, opencode), the generator MUST continue to emit the advisory session-start
launcher files (extensionless POSIX entry plus `.ps1`/`.cmd`) exactly as before;
the choice between inline registration and launcher-file emission MUST be keyed on
whether the host has a `SettingsPath`.

#### Scenario: Cursor and OpenCode launchers still generated
GIVEN a repository initialized for the cursor and opencode adapters
WHEN host surfaces are regenerated
THEN `.cursor/hooks/slipway-session-start` (plus `.ps1`/`.cmd`) still exists
AND `.opencode/hooks/slipway-session-start` (plus `.ps1`/`.cmd`) still exists.

### Requirement: Retire only the dead hook templates
REQ-005: The build MUST remove the now-unused
`context-pressure-post-tool-use.{sh,ps1,cmd}.tmpl` templates (used only by the
claude PostToolUse launcher that is being inlined) and MUST retain the
`session-start.{sh,ps1,cmd}.tmpl` templates required by the file-by-path hosts.
The package MUST still build (the `//go:embed templates/hooks/*.tmpl` glob remains
satisfied).

#### Scenario: Package builds after removing dead templates
GIVEN the three `context-pressure-post-tool-use.*` templates are deleted
WHEN the module is built
THEN `go build ./...` succeeds
AND the `session-start.*` templates remain present and embedded.

### Requirement: Remove worktree host-surface provisioning without changing governance
REQ-006: Creating a governed worktree MUST NOT copy or regenerate host-adapter
surfaces (`.claude`/`.gemini`/etc.) into the worktree. The
`ProvisionWorktreeHostSurfaces` function and its helpers MUST be removed, the
`WorktreeProvisioner` injection MUST be removed, and
`EnsureDefaultWorktreeForChange` MUST no longer take a provisioner parameter.
`slipway new` MUST still create the per-change worktree and branch; no governance
gate, lifecycle state, or worktree-creation behavior changes.

#### Scenario: New governed change still creates a worktree but no provisioned surfaces
GIVEN a repository with host adapters initialized at the project root
WHEN a governed change is created via `slipway new`
THEN a per-change worktree is created under `.worktrees/<branch>`
AND no host-adapter surface (`.claude`, `.gemini`, etc.) is copied or regenerated
into that worktree.

#### Scenario: Authority layer does not depend on the surface renderer for provisioning
GIVEN the provisioning code is removed
WHEN the module is built and the architecture dependency tests run
THEN `internal/state` does not import `internal/toolgen` for provisioning
AND `go test ./...` passes.

### Requirement: Generated docs and surface manifest reflect the new contract
REQ-007: Generated documentation MUST stop describing launcher files for the
settings-capable hosts (claude/gemini), MUST state that the governed tool session
starts at the project root and reads skills/hooks from the root host surfaces,
MUST remove descriptions of provisioning host surfaces into worktrees, and the
surface manifest MUST be regenerated through the public flow so that
`gen-surface-manifest --check` passes. Governance principle docs (`CLAUDE.md`
Lifecycle Authority) MUST NOT be rewritten.

#### Scenario: Surface manifest check passes after regeneration
GIVEN the generator changes for REQ-001..REQ-005 are implemented
WHEN `go run ./internal/toolgen/cmd/gen-surface-manifest --write` is run and then
`--check`
THEN `--check` exits successfully with no drift
AND the docs no longer enumerate `.claude/hooks/`/`.gemini/hooks/` launcher files.
