# Concerns

Re-authored for change
`resolve-github-issue-184-add-gsd-style-automatic-subagent-di`
(GitHub issue #184).

- Contract ambiguity risk: the current reference says a `parallel: true` wave is
  concurrent by default, but the Codex section still points at `codex -q --task`.
  That can let hosts degrade to shell-oriented or same-context execution while
  the product surface implies real fan-out.
- Governance weakening risk: if degraded sequential remains nonblocking for a
  capable runtime, a coordinator can pollute its own context and still record a
  passing wave. The fix should distinguish "runtime lacks a primitive" from
  "runtime has a primitive but dispatch failed or was skipped".
- Evidence ownership risk: executors may report refs, but `slipway evidence
  task` remains the only task evidence ledger writer. Generated prose must not
  imply subagents can self-stamp `captured_at`, freshness inputs, or governed
  verification YAML.
- Isolation overclaim risk: GSD's Claude worktree isolation model cannot be
  copied into Codex guidance because GSD itself documents that Codex
  `spawn_agent` has no direct `isolation="worktree"` mapping.
- Test drift risk: this repository does not track generated `.claude`,
  `.codex`, `.cursor`, `.gemini`, or `.opencode` adapter copies. Tests must
  exercise template rendering and toolgen-generated temporary trees instead of
  relying on committed generated files.
- Manifest scope risk: `docs/SURFACE-MANIFEST.json` tracks public surface rows.
  This change edits an existing surface contract and should not require a
  manifest row update unless a new command, skill, JSON contract, adapter, or
  documentation surface is added.
