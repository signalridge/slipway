# Tasks

## Task List

- [x] `t-01` Implement the per-change handoff artifact core: fenced engine machine-header model
  (slug, monotonic generation, session_owner, git branch/worktree, updated_at, staleness derived
  from updated_at vs the change's latest lifecycle event — NO current_state/substep and NO
  next_skill/next_command snapshot), deterministic render + parse, narrative skeleton, and the
  write/show core that regenerates the header while preserving existing narrative sections.
  - depends_on: []
  - target_files: [internal/state/handoff.go, internal/state/handoff_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]

- [x] `t-02` Add the `slipway handoff` command (`write`/`show`, bare = write) with flags
  `--change`/`--section`/`--json`/`--brief`; wire slug-free active-change resolution
  from the invoking worktree binding (reuse the existing resolver), `--change` override, and graceful
  no-op when no active change is bound. The command does NOT read, derive, or embed
  next_skill/next_command; `show`/`--brief` and the narrative direct the resumer to
  `slipway status`/`next` for authority (the `slipway next` command is unchanged).
  - depends_on: [t-01]
  - target_files: [cmd/handoff.go, cmd/handoff_test.go, cmd/root.go, cmd/common.go, cmd/common_test.go]
  - task_kind: code
  - covers: [REQ-001, REQ-003, REQ-004]

- [x] `t-03` Register `handoff` in the toolgen `commandRegistry` (`HasPromptSurface: true`) and
  generate a hook-agnostic `slipway-handoff` command/skill surface that documents WHEN to write and
  to read on resume, explicitly not assuming any hook fired.
  - depends_on: [t-02]
  - target_files: [internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/toolgen/adapter_contract_test.go, internal/toolgen/testdata/skill_tree_inventory.codex.golden, internal/tmpl/templates/_partials/command-handoff-body.tmpl]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-04` Replace the inline "Runtime Session Handoff" prose in the workflow skill template with
  a pointer to the owned `slipway-handoff` surface (Agent Instruction Boundary).
  - depends_on: [t-03]
  - target_files: [internal/tmpl/templates/skills/workflow/SKILL.md.tmpl, internal/tmpl/templates/_partials/command-run-body.tmpl, internal/tmpl/templates_test.go]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-05` Re-point the EXISTING PostToolUse context-pressure nudge so its remediation tells the
  agent to run `slipway handoff write` (today it points at a generic "workflow handoff contract"),
  keeping the message host-agnostic and the hook fail-silent. No new hook event, launcher template,
  or toolgen hook-registry entry is added; narrative stays agent-authored, captured while a turn remains.
  - depends_on: [t-02]
  - target_files: [cmd/context_pressure_hook.go, cmd/context_pressure_hook_test.go]
  - task_kind: code
  - covers: [REQ-006]

- [x] `t-06` Extend the SessionStart hook to run `slipway handoff show --brief` and emit a BOUNDED
  pointer (single worktree-bound change → one-line descriptor + path; multiple/ambiguous → at most
  slugs+paths or a count; never a handoff body) after and subordinate to the authoritative
  `slipway next` block.
  - depends_on: [t-02]
  - target_files: [cmd/session_start_hook.go, cmd/session_start_hook_test.go]
  - task_kind: code
  - covers: [REQ-007]

- [x] `t-07` Lock the advisory-only invariant with a test asserting no gate/evidence/freshness path
  depends on the handoff, and update the README command contract for `slipway handoff` (keeping the
  toolgen README contract test green).
  - depends_on: [t-02]
  - target_files: [README.md, internal/state/handoff_invariant_test.go]
  - task_kind: code
  - covers: [REQ-008]

- [x] `t-08` Make the `slipway hook` handlers host-portable for Codex: `slipway hook session-start`
  must accept Codex's SessionStart stdin payload (resolve the change from `cwd`; honor `source` incl.
  `compact`) and emit Codex-compatible output (`hookSpecificOutput.additionalContext`); add a
  staleness-conditioned write nudge for Codex `UserPromptSubmit` (no Codex usage metrics exist, so
  condition on engine-derivable handoff staleness/absence, terse, silent when fresh) routing to
  `slipway handoff write`. Keep Claude behavior unchanged.
  - depends_on: [t-05, t-06]
  - target_files: [cmd/session_start_hook.go, cmd/context_pressure_hook.go, cmd/session_start_hook_test.go, cmd/context_pressure_hook_test.go]
  - task_kind: code
  - covers: [REQ-009, REQ-006, REQ-007]

- [x] `t-09` Generate repo-local Codex hooks in toolgen: extend the Codex `ToolConfig` and emit a
  `.codex/config.toml` `[hooks]` block (`[[hooks.SessionStart]]` → `slipway hook session-start`;
  `[[hooks.UserPromptSubmit]]` → the write nudge) using Codex's documented schema, invoking the shared
  handlers. Source the hooks where Codex reads them for Slipway-provisioned worktrees (root checkout's
  `.codex/`). Do not auto-grant trust. Keep the README/toolgen contract tests green.
  - depends_on: [t-03, t-08]
  - target_files: [internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/toolgen/adapter_contract_test.go]
  - task_kind: code
  - covers: [REQ-009]

- [x] `t-10` Surface the Codex hook activation caveat: init/provision output and the README must state
  that the generated Codex hooks are inert until the repo is trusted and each hook is trusted, and that
  Slipway never edits the user's global trust config.
  - depends_on: [t-07, t-09]
  - target_files: [README.md, docs/SURFACE-MANIFEST.json, docs/reference/ai-tools.md, docs/reference/commands.md, cmd/init.go, cmd/init_test.go, internal/toolgen/toolgen.go, internal/toolgen/toolgen_test.go, internal/toolgen/surface_manifest.go]
  - task_kind: code
  - covers: [REQ-009]
