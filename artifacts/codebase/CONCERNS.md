# Concerns

- Second-authority risk: handoff content must never become lifecycle authority.
  Machine header and `show --brief` output must omit lifecycle state/substep and
  next_skill/next_command, and must point resumers to `slipway status` and
  `slipway next` instead.
- Context pollution risk: SessionStart can fire from the repo root or when
  multiple changes exist. The hook must emit bounded slugs/paths or a count in
  ambiguous contexts and never inject full handoff bodies or focus prose.
- Hook portability risk: Claude and Codex payloads differ. Shared hook handlers
  must fail silent, preserve existing Claude behavior, accept Codex SessionStart
  and UserPromptSubmit payloads, and use Codex-compatible output only where
  Codex expects it.
- Trust risk: generating `.codex/config.toml` is not the same as enabling hooks.
  The README and setup output must state that repo trust and hook trust are
  user-granted and that Slipway never edits global Codex trust config.
- Worktree placement risk: Slipway-provisioned worktrees may not be where Codex
  reads project hooks. Toolgen must write Codex hook config where Codex actually
  reads it for the root checkout while keeping governed code edits in the bound
  worktree.
- Generated-surface drift risk: adding a command must keep command registry,
  command skills, surface manifest expectations, README command contracts, and
  generated workflow guidance aligned. Stale negative tests that currently assert
  no `.codex/config.toml` must be updated intentionally.
- Advisory invariant risk: a stale, missing, or hand-edited handoff must not
  affect lifecycle gates, evidence freshness, readiness, or bypass/force-close
  paths. A dedicated invariant test should guard this.
