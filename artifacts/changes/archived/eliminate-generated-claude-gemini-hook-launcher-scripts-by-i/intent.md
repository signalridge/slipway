# Intent

## Summary
Eliminate generated host hook launcher scripts by inlining the `slipway hook`
command directly into host settings.json, and make the `slipway hook`
subcommands fail-silent (always exit 0). This removes the missing-file,
relative-path/CWD, and cross-platform-launcher failure modes that caused
PostToolUse hooks to fail with `/bin/sh: .claude/hooks/...: No such file or
directory`.

Additionally remove the worktree host-surface provisioning
(`ProvisionWorktreeHostSurfaces` copy + its `GenerateWorktreeLocal` regenerate
step) that materializes `.claude`/`.gemini`/etc. into governed worktrees.
Governance and per-change worktrees are UNCHANGED: `slipway new` still creates the
worktree/branch and edits still land in the worktree's files. The supported model
is root-startup — the governed tool session (claude/codex) is launched from the
PROJECT ROOT, so it discovers all skills and hooks from the root host surfaces and
never relies on host surfaces copied inside the worktree. Under root-startup the
provisioning is dead weight, so it is removed. (Provisioning was added to support
running a session with cwd inside the worktree; that cwd-in-worktree configuration
is not the supported model here, so the earlier "in-worktree session reads
worktree-local surfaces" finding describes an unsupported configuration, not the
owner's flow.) No governance principle or worktree-creation behavior changes; the
only doc clarification is to state the root-startup expectation and drop
descriptions of worktree host-surface provisioning.

## Complexity Assessment
complex

Rationale: touches the generated host-adapter contract (settings.json for
claude/gemini), the CLI hook subcommands, shared launcher infrastructure used by
multiple hosts (claude/gemini/cursor/opencode), templates, generated docs, the
surface manifest, and tests. The Cursor/OpenCode mechanism question is now
resolved (see Open Questions): they are file-by-path hosts retained as-is, so the
change is keyed cleanly on `SettingsPath`.

## Guardrail Domains
External API contract (generated host-adapter settings.json + generated docs are
consumed by external AI hosts). No Auth/Credentials/PII/Financial/Schema/
Irreversible domains.

## In Scope
- Registration strategy keyed on `SettingsPath` (the clean split confirmed by the
  open-question research):
  - **Settings-capable hosts** (`SettingsPath != ""` → claude `.claude/settings.json`,
    gemini `.gemini/settings.json`): in `internal/toolgen/toolgen.go` register the
    hook event command as a **bare inline command** (`slipway hook context-pressure`,
    `slipway hook session-start`) instead of a quoted launcher path; stop emitting
    their launcher script files (the no-suffix POSIX entry plus `.ps1`/`.cmd`
    variants); prune the now-orphaned launcher files on `--refresh`.
  - **File-by-path hosts** (`SettingsPath == ""` but `SessionHook != ""` → cursor,
    opencode): UNCHANGED — keep emitting their advisory session-start launcher
    files; that file is their only registration contract and is not subject to the
    settings.json bug class.
- `cmd/context_pressure_hook.go`, `cmd/session_start_hook.go`: guarantee the
  `slipway hook context-pressure` and `slipway hook session-start` subcommands
  always exit 0 (swallow their own errors) so the bare inline command never
  surfaces a non-blocking failure.
- Shared launcher infrastructure (`hookLauncherOutputs`, `nativeHookPath`,
  `hookLauncherCommand`, `hookLauncherOutput`, `renderHookLauncher`, legacy
  cleanup): RETAINED — file-by-path hosts (cursor/opencode) still need it. The
  Generate hook loop becomes conditional on `SettingsPath` (emit launcher files
  only for file-by-path hosts; register inline for settings-capable hosts). The
  `session-start.{sh,ps1,cmd}.tmpl` templates STAY; only the claude-only
  `context-pressure-post-tool-use.{sh,ps1,cmd}.tmpl` templates become dead and are
  removed (adjust the `//go:embed templates/hooks/*.tmpl` set accordingly). Keep/
  repurpose `pruneLegacySlipwayHookCommands` so an existing settings.json that still
  references the old launcher *path* command is migrated to the inline command on
  `--refresh`, and extend the orphan-cleanup to remove the no-suffix/`.ps1`/`.cmd`
  launcher files now retired for settings-capable hosts (not just the legacy `.sh`).
- **Init logic** (`internal/bootstrap/init.go` -> `toolgen.Generate`, the launcher
  write loops in `Generate`): with the conditional above, `init` and
  `init --refresh` produce inline-command settings.json and no launcher files for
  settings-capable hosts, while continuing to emit launcher files for file-by-path
  hosts.
- **Worktree host-surface provisioning REMOVAL** (decision final — stay-in-root):
  delete `ProvisionWorktreeHostSurfaces` and its helper tree (`copyHostAdapterTree`,
  `excludeFromHostAdapterCopy`, `adapterSkillsRel`, `isSlipwayOwnedSkillDir`,
  `copyHostAdapterFileNoClobber`) in `internal/toolgen/worktree_provision.go`
  (delete the file); remove the `WorktreeProvisioner` type/`provision` method in
  `internal/state/worktree_provision.go` and drop the provisioner parameter from
  `EnsureDefaultWorktreeForChange` (`internal/state/worktree.go`) plus its
  provision call sites; update the `cmd/new.go` wiring
  (`state.EnsureDefaultWorktreeForChange(root, &change, toolgen.ProvisionWorktreeHostSurfaces)`)
  to the new signature and drop the now-unused `toolgen` import if it falls away;
  delete `GenerateWorktreeLocal` if it has no remaining caller. Delete the
  provisioning test files (`internal/toolgen/worktree_provision_test.go`,
  `internal/state/worktree_provision_test.go`) and update
  `internal/state/worktree_test.go` / `cmd/new_test.go` call sites and provisioned-
  surface assertions. Net effect: creating a governed worktree no longer copies or
  regenerates `.claude`/`.gemini`/etc. into it.
- **Root-startup doc clarification** (minimal): in the appropriate doc(s), state
  that the governed tool session starts at the PROJECT ROOT and reads skills/hooks
  from the root host surfaces while edits land in the per-change worktree, and
  remove any documentation that describes provisioning host surfaces INTO
  worktrees. Do NOT change governance principles or `CLAUDE.md` Lifecycle
  Authority — governance and worktree creation are unchanged.
- Cursor/OpenCode hook coverage: retained as-is (file-by-path launcher files);
  confirm docs state their advisory session hook is delivered as a launcher file
  and is not subject to the settings.json missing-file bug class.
- Generated docs that enumerate launcher files (`docs/ai-tools.md`,
  `docs/installation.md`) and the surface manifest (`gen-surface-manifest`).
- Regenerate host surfaces via the public flow so `.claude`/`.gemini` settings
  and the manifest reflect the new contract.
- Tests in `internal/toolgen/*_test.go` and `cmd/*_test.go` that pin the old
  launcher-path command strings or launcher-file outputs.

## Out of Scope
- Codex: has no hooks at all (`SessionHook`/`SettingsPath` empty); untouched.
- Hook *behavior/semantics*: context-pressure thresholds, the session-start
  output payload, and `next`/handoff logic are unchanged — only the launch
  mechanism and exit-code discipline change.
- Non-hook generated surfaces (commands, skills, triggers).
- Changing `.gitignore` treatment of host directories.

## Constraints
- The inlined settings.json command must be a **bare command with no shell
  operators** so it is portable across `/bin/sh -c`, `cmd /c`, and PowerShell;
  cross-platform robustness lives in the binary, not in shell glue.
- If `slipway` is not on PATH at runtime, a benign non-blocking "command not
  found" is the accepted trade-off (per user decision); no POSIX `command -v`
  guard in the inlined string.
- `.claude`/`.gemini` are gitignored and regenerated; never hand-edit them as
  evidence — regenerate through the public flow.
- Build the Slipway CLI to a path OUTSIDE this worktree when verifying, to avoid
  tripping the S2 scope-contract gate (known worktree CLI scope-drift hazard).
- `gen-surface-manifest --check` and the full Go test suite must pass.

## Acceptance Signals
- `.claude/settings.json` registers PostToolUse = `slipway hook context-pressure`
  and SessionStart = `slipway hook session-start` as bare commands; `.gemini/settings.json`
  registers SessionStart = `slipway hook session-start`; neither references a
  `.claude/hooks/`/`.gemini/hooks/` launcher path.
- No launcher script files are generated for claude/gemini (and stale ones are
  pruned on `--refresh`).
- `slipway hook context-pressure` and `slipway hook session-start` exit 0 even on
  malformed/empty input or internal error.
- Cursor/OpenCode launcher files still generated unchanged; docs note they are
  not subject to the settings.json missing-file bug class.
- `go build` of the CLI succeeds; `go test ./...` passes; `gen-surface-manifest --check` passes.
- Generated docs no longer describe launcher files for hosts that drop them.

## Open Questions
- VERIFIED (was the worktree-consumer safety gate): `git rev-parse
  --show-toplevel` run from inside a linked worktree returns the WORKTREE path,
  not the shared main repo root. Both `gitWorkspaceRoot`
  (`internal/state/worktree.go:571`) and `projectRootFromCommand`
  (`cmd/common.go:616`) resolve project root via `--show-toplevel`. Therefore an
  in-worktree slipway/host session resolves `root = worktree` and reads
  worktree-local `.claude/skills`, `.claude/commands`, `.claude/settings.json`,
  which exist ONLY because provisioning materializes them. CONCLUSION: provisioning
  IS load-bearing for the in-worktree-session workflow and is safe to remove ONLY
  under a root-startup model where the governed session is launched from the
  project root (never with cwd inside the worktree) — which is the owner's
  supported flow (see next item). The removal therefore proceeds, paired only with
  a minimal root-startup doc clarification.
- [x] PRODUCT DECISION (owner, clarified): REMOVE worktree provisioning; KEEP
  governance and per-change worktree creation. The supported model is root-startup
  — the governed tool session (claude/codex) is launched from the PROJECT ROOT,
  reads all skills/hooks from the root host surfaces, and edits the worktree's
  files. The session never runs with cwd inside the worktree, so worktree-local
  host surfaces are never read and provisioning is dead weight. (The earlier
  "in-worktree session reads worktree-local surfaces" finding remains technically
  true but describes an unsupported cwd-in-worktree configuration, not the owner's
  flow.) The only scope obligation is a minimal root-startup doc clarification —
  NOT a governance/CLAUDE.md rewrite.
- [x] RESOLVED (Cursor/OpenCode hook discovery): they have no settings.json
  (`SettingsPath == ""`, `SessionEvent == ""`) and deliver only an *advisory*
  session-start hook as a launcher file at a path convention
  (`.cursor/hooks/slipway-session-start`, `.opencode/hooks/slipway-session-start`
  + `.ps1`/`.cmd`). Confirmed by the registry (`internal/toolgen/toolgen.go:64-109`),
  by the Generate flow (settings merge runs only when `SettingsPath != ""`,
  `toolgen.go:1010`), and by docs ("Settings-capable adapters register the native
  launcher", `docs/ai-tools.md:140-151`). The launcher FILE is their registration
  contract, so they are NOT subject to the settings.json missing-file bug class
  and there is no inline-command equivalent to apply. CONCLUSION: retain their
  launcher files unchanged; the shared launcher infrastructure and the
  `session-start.{sh,ps1,cmd}.tmpl` templates STAY (file-by-path hosts still need
  them). Only the claude-only `context-pressure-post-tool-use.{sh,ps1,cmd}.tmpl`
  templates become dead and are removed.

## Deferred Ideas
- Consider whether codex should gain an advisory session hook later (currently
  none); explicitly not part of this change.

## Approved Summary
User-confirmed 2026-06-15 (worktree model corrected to root-startup per owner
clarification: tools start at project root and read root skills/hooks; the
worktree holds edited files; governance and worktree creation are unchanged).

This change eliminates the generated host hook *launcher script* failure class for
the settings-capable hosts and removes the now-dead worktree host-surface
provisioning, under a stay-in-project-root product model.

What it does:
1. Launcher -> inline for settings-capable hosts (claude, gemini): register the
   hook event in settings.json as a bare command (`slipway hook context-pressure`,
   `slipway hook session-start`) with no shell operators (portable across
   sh/cmd/PowerShell); stop generating their launcher files; prune orphaned
   launcher files and migrate stale launcher-path commands on `--refresh`. The
   `slipway hook *` subcommands are made fail-silent (always exit 0).
2. Cursor/OpenCode unchanged: they have no settings.json and deliver an advisory
   file-by-path session hook, which is not subject to the settings.json
   missing-file bug class; their launcher files and the `session-start.*` templates
   are retained. Only the claude-only `context-pressure-post-tool-use.*` templates
   become dead and are removed. The generation strategy is keyed on `SettingsPath`.
3. Remove worktree host-surface provisioning (copy + regenerate) only. Governance
   and per-change worktree creation are UNCHANGED (`slipway new` still creates the
   worktree/branch; edits land in the worktree). The supported model is
   root-startup: the tool session starts at the project root and reads skills/hooks
   from the root host surfaces, so provisioning is dead weight. Docs get a minimal
   root-startup clarification and drop provisioning descriptions; `CLAUDE.md`
   governance principles are NOT rewritten.

Key scope boundaries: codex (no hooks) untouched; hook behavior/semantics
(thresholds, payloads, `next`) unchanged; cursor/opencode launcher surface
retained; `.gitignore` treatment unchanged.

Primary acceptance signal: `.claude/settings.json` registers the bare
`slipway hook ...` commands (no `.claude/hooks/` path), claude/gemini emit no
launcher files, `slipway hook *` exits 0 on malformed/empty input, cursor/opencode
launchers still generate, and `go test ./...` + `gen-surface-manifest --check`
pass with the CLI built outside the worktree.
