# Intent

## Summary
Add an engine-owned, per-change `slipway handoff` command (write/show) that becomes the writer for the runtime session handoff at .git/slipway/runtime/changes/<slug>/handoff.md. The engine stamps a deterministic machine header (slug, generation counter, session_owner, git/worktree, updated_at, staleness) and ensures the narrative skeleton; the agent fills narrative substance. The header deliberately does NOT snapshot lifecycle position or the next step — current_state/substep and next_skill/next_command stay owned by `slipway status`/`next`, and the handoff points the resumer to those authoritative commands rather than embedding a copy that goes stale the moment the lifecycle advances and reads as a second authority. Generate a hook-agnostic `slipway-handoff` command/skill surface via toolgen. Reuse the two EXISTING Claude-Code hooks rather than adding any new hook event: re-point the existing PostToolUse context-pressure nudge's remediation to `slipway handoff write` (so the agent authors the narrative while it still has a turn), and have the existing SessionStart hook surface a BOUNDED handoff pointer (`show --brief`) after (and subordinate to) the authoritative next-action. The command plus the generated skill are the portable, load-bearing core that closes the cross-session loop on any host or fresh agent, because narrative authorship is agent-side on every host - no host hook can author it, not Claude's and not Codex's (Codex 0.141.0+ even has its own hooks - SessionStart/PreCompact/PostCompact/PostToolUse/Stop - and native session resume/fork). The existing Claude-Code hooks are a graceful-degradation acceleration layer where they are wired. Generate repo-local Codex hooks too (`.codex/config.toml`) so Codex participates through its OWN hook system - a SessionStart bounded pointer (also covering post-compaction re-injection via source==compact) plus a staleness-conditioned write nudge - both invoking the same `slipway hook` handlers. Handoff remains advisory only - never lifecycle authority, governed evidence, freshness input, or a gate.
## Complexity Assessment
complex
<!-- Rationale: spans a new public CLI command surface (write/show), an engine-owned
     deterministic artifact header, toolgen command/skill generation, reuse of two EXISTING
     hooks (SessionStart bounded pointer + the PostToolUse context-pressure nudge re-pointed at
     the command — no new hook event), a hook-agnostic generated skill that must also close the
     loop on any host without depending on a host hook, repo-local Codex hook generation
     (`.codex/config.toml`) reusing the shared handlers, and contract tests (README + toolgen). Multiple coupled surfaces
     must stay aligned, which is the complex tier, not critical (no sensitive guardrail domain). -->

## Guardrail Domains
<!-- none detected -->

## In Scope
- A new engine-owned public command `slipway handoff` with subcommands:
  - `write [--change <slug>] [--section <name>]` — the missing writer for the
    per-change runtime handoff at `.git/slipway/runtime/changes/<slug>/handoff.md`. Bare
    `slipway handoff` = write. Every write regenerates the engine machine header and preserves
    existing narrative; `--section <name>` lets the agent author one narrative section.
  - `show [--change <slug>] [--json] [--brief]` — resume-oriented read of the current handoff;
    `--brief` renders only the engine descriptor (generation, updated_at, session_owner,
    staleness) plus the path and a one-line focus, for the SessionStart hook. No lifecycle/next field.
- An engine-stamped, fenced machine header inside the handoff artifact, regenerated on every
  `write` and never hand-edited: slug, monotonic generation counter, session_owner,
  git branch/worktree, updated_at, and a staleness indicator (derived from updated_at versus the
  change's latest lifecycle event — i.e. stale when the lifecycle advanced after the handoff was
  written). The header deliberately does NOT snapshot current_state/substep or next_skill/
  next_command: the authoritative lifecycle position and next step stay owned by
  `slipway status`/`next`, and the handoff points the resumer to those commands rather than
  embedding a copy that goes stale and competes as a second authority. The engine also guarantees
  the narrative skeleton headings; the agent fills narrative substance (same engine-structure /
  agent-substance split as intent.md).
- A generated, hook-agnostic `slipway-handoff` command/skill surface via toolgen `commandRegistry`
  (`HasPromptSurface: true`), telling agents WHEN to write (meaningful moments, before stopping,
  before a review split) and to read (`show` on resume). It MUST drive write+read itself and never
  assume a hook already fired, so the cross-session loop also closes on any host or fresh agent that
  does not wire (or ignores) a Slipway hook — including Codex, which has its own hooks plus native
  `resume`/`fork` but no hook that authors narrative.
- Re-point the EXISTING PostToolUse context-pressure nudge (`cmd/context_pressure_hook.go`) so its
  remediation tells the agent to run `slipway handoff write` (today it points at a generic "workflow
  handoff contract"). No new hook event is added: the narrative stays AGENT-AUTHORED and is captured
  while a turn still remains — a deterministic hook cannot author narrative, only stamp re-derivable
  metadata. This is host-agnostic: it does not depend on any pre-compaction hook — Codex has a
  PreCompact hook too, but no host's PreCompact can author narrative.
- The existing SessionStart hook runs `slipway handoff show --brief` and surfaces a BOUNDED pointer
  AFTER and SUBORDINATE TO the authoritative `slipway next` block: a single worktree-bound change →
  one-line descriptor + path; multiple/ambiguous → at most slugs+paths or a count; never a handoff
  body, so multiple active changes cannot pollute context. The hook runs the owned command; it does
  not parse or format the artifact itself.
- Generate repo-local Codex hooks via toolgen (`.codex/config.toml` `[hooks]`) so Codex participates
  through its own hook system, reusing the same `slipway hook` handlers (no Codex-specific logic):
  `SessionStart` → bounded pointer (also covers post-compaction re-injection via `source==compact`),
  and `UserPromptSubmit` → a write nudge conditioned on engine-derivable handoff staleness (Codex gives
  hooks no context-window metrics), terse and silent when fresh. Surface the trust/activation caveat;
  never auto-grant Codex trust; for worktrees write where Codex reads (root checkout's `.codex/`).
- Replace the inline "Runtime Session Handoff" prose in the workflow `SKILL.md` template with a
  pointer to the owned `slipway-handoff` surface (Agent Instruction Boundary).
- Keep README command-contract and toolgen tests green for the new surface.

## Out of Scope
- Not a gate, attestation oracle, or Stop-hook hard block. Handoff stays advisory only.
- No second lifecycle authority — `slipway next` and governed evidence remain authoritative;
  the handoff only snapshots them. Handoff is never freshness input or governed evidence.
- No external infrastructure (Postgres/pgvector, swarm daemons, AST/TLDR indexing).
- No per-task file-claim blackboard; Slipway is change-level worktree isolation. A `session_owner`
  stamp is sufficient for multi-session non-collision.
- The per-change handoff is not committed to git by default (runtime/gitignored, freshness-bound).
- No change to lifecycle gates, readiness, or evidence semantics.
- No new Claude PreCompact hook: the Claude write trigger stays the existing PostToolUse nudge, and no
  hook on any host is relied on to author narrative. (Codex hooks ARE generated — SessionStart pointer
  + staleness write nudge — but Codex PreCompact/PostCompact are not used for narrative.)
- Slipway never auto-grants Codex project or hook trust, and never edits the user's global Codex config.
- Legacy repo-level handoff migration is out of scope: no `slipway handoff migrate` subcommand,
  and the existing `legacy_runtime_handoff` health/repair finding keeps its current manual
  guidance unchanged (status quo, no regression). Legacy retirement can be a separate change.

## Constraints
- Use the current worktree's Slipway CLI as the source of truth; build the dev binary before use.
- The machine header MUST NOT snapshot lifecycle position or the next step (no current_state/
  substep, no next_skill/next_command): those stay owned by `slipway status`/`next`, and the
  handoff points the resumer to them, so the handoff can never drift into a competing authority.
- Reuse the two existing hooks and keep them thin: the change edits the PostToolUse context-pressure
  nudge remediation (`cmd/context_pressure_hook.go`) and the SessionStart surfacing
  (`cmd/session_start_hook.go`) to invoke the owned command. No new hook event, launcher template, or
  toolgen hook-registry entry is introduced.
- Keep the README command-contract test and toolgen hook-registration tests green; do not weaken them.
- Respect the gofmt `-s` simplify CI gate.
- The generated `slipway-handoff` skill MUST be hook-agnostic: it drives write+read via the command
  and never assumes a PreCompact/SessionStart hook fired, so the loop closes on any host without
  depending on a hook (Codex has its own hooks and native session resume, but no hook authors
  narrative). Hooks only front-run the same commands where they are wired.
- Hooks must stay thin launchers that invoke the owned `slipway handoff` command; format and content
  live in the command, not in hook/template parsing.
- Codex hooks use Codex's documented schema (`.codex/config.toml` `[[hooks.<Event>]]` groups with a
  `hooks` array of `{ type = "command", command = ... }`). Generation does not activate them — repo
  trust + per-hook trust are user-granted. Codex delivers no token/context metrics to command hooks, so
  the Codex write nudge conditions on handoff staleness, not a pressure number.

## Acceptance Signals
- `slipway handoff write` creates/refreshes the per-change handoff at the canonical path with a
  fresh engine machine header (slug, generation, session_owner, git branch/worktree, updated_at,
  staleness) and the narrative skeleton; the header carries no lifecycle/next snapshot, and the
  handoff directs the resumer to `slipway status`/`next` for the authoritative next step.
- `slipway handoff show` returns the current handoff (human + `--json`); `--brief` returns just
  the engine descriptor (no state/next field) plus the path and a one-line focus.
- A generated, hook-agnostic `slipway-handoff` skill/command surface exists for the host adapters
  and instructs write+read without assuming any hook fired (loop closes on any host/agent, hook-wired or not).
- The existing PostToolUse context-pressure nudge tells the agent to run `slipway handoff write`
  (asserted on its emitted message), and the existing SessionStart hook surfaces only a bounded
  pointer (descriptor + path, or just slugs/paths when ambiguous) subordinate to the `next` block —
  never a handoff body.
- toolgen generates a `.codex/config.toml` `[hooks]` block (SessionStart + UserPromptSubmit) invoking
  the shared `slipway hook` handlers; the SessionStart hook injects the bounded pointer via
  `additionalContext`; the Codex write nudge is silent when the handoff is fresh; init/provision output
  states the hooks are inert until trusted.
- `go build ./...`, `go vet ./...`, `gofmt -s -l`, and `go test ./...` are clean; README and
  toolgen contract tests pass.

## Open Questions
None.
<!-- The earlier PreCompact-hook-wiring unknown is eliminated: the design reuses the two existing
     hooks (no new hook event), so there is no launcher/registry plumbing to resolve. -->

## Deferred Ideas
- Optional append-only investigation ledger (exact command + result) inside the handoff, borrowed
  from agent-ledger / Sonovore; can ship later as an additive narrative section.
- Cross-session `session_owner` lease/conflict warnings beyond a simple stamp.
- A `slipway handoff migrate` subcommand and/or repointing the `legacy_runtime_handoff` repair
  hint at the new writer, to auto-retire legacy repo-level handoff files — deferred to a separate change.
- A dedicated PreCompact hook is intentionally NOT built: a deterministic hook cannot author the
  narrative that compaction puts at risk (it can only stamp re-derivable metadata), and manual
  context clears do not even fire it. This holds on every host — Claude AND Codex both expose a
  PreCompact hook, and neither can author narrative — so the limit is fundamental, not host-specific.
- Richer Codex triggers beyond the SessionStart pointer + staleness nudge now in scope: e.g. a
  PreCompact metadata-stamp, a transcript-size heuristic nudge, or treating `codex resume`/`fork`
  (native rollout replay) as complementary raw-replay alongside the curated governed handoff.

## Approved Summary
Add an engine-owned, per-change `slipway handoff` command — `write` (the missing writer; every write
regenerates the machine header and preserves narrative; `--section` = one agent narrative section) and
`show` (`--json`, `--brief`) — that becomes the official writer/reader for the runtime session handoff at
`.git/slipway/runtime/changes/<slug>/handoff.md`. Every `write` regenerates a fenced, engine-stamped
machine header (slug, monotonic generation, session_owner, git branch/worktree, updated_at, staleness)
and guarantees the narrative skeleton; the agent fills only narrative substance. The header deliberately
does NOT snapshot lifecycle position or the next step (no current_state/substep, no next_skill/
next_command): those stay owned by `slipway status`/`next`, and the handoff points the resumer there —
a snapshot would go stale the moment the lifecycle advances and would read as a competing authority
(exactly the REQ-008 failure mode). Staleness is derived from updated_at versus the change's latest
lifecycle event, not from an embedded state copy.

The automation is surfaced through a generated, HOOK-AGNOSTIC `slipway-handoff` skill — the portable,
load-bearing core that drives write+read via the command and never assumes a hook fired, so the
cross-session loop also closes on any host or fresh agent regardless of hook/compaction model —
including Codex, which has its own hooks and native session resume/fork yet no hook that authors
narrative — plus a Claude-Code
acceleration layer built from the TWO EXISTING hooks (no new hook event): the PostToolUse
context-pressure nudge is re-pointed to tell the agent to run `slipway handoff write` (narrative stays
agent-authored, captured while a turn remains — a hook cannot author narrative, only stamp re-derivable
metadata), and the SessionStart hook surfaces a BOUNDED pointer via `handoff show --brief` (one-line
descriptor + path for a single bound change; only slugs/paths or a count when ambiguous; never a body),
after and subordinate to the authoritative `slipway next` block. Codex is wired through its OWN hook
system too: toolgen generates `.codex/config.toml` hooks — `SessionStart` → the bounded pointer (also
covering post-compaction re-injection via `source==compact`) and `UserPromptSubmit` → a write nudge
conditioned on handoff staleness (Codex hooks get no context metrics) — both invoking the same
`slipway hook` handlers; generation never grants Codex trust.

Scope boundaries: handoff is advisory only — never lifecycle authority, governed evidence, freshness
input, or a gate; no second authority. Out of scope: legacy repo-level handoff migration (NO
`slipway handoff migrate`; the `legacy_runtime_handoff` repair hint stays unchanged, deferred); no new
Claude PreCompact hook and no hook (any host) relied on to author narrative; no external infrastructure;
no per-task file-claim blackboard; handoff not committed to git. Slipway never auto-grants Codex trust.

Primary acceptance signal: `slipway handoff write` produces a fresh machine header (identity +
freshness fields only, no lifecycle/next snapshot) plus the narrative skeleton, and the handoff
directs the resumer to `slipway status`/`next` for the authoritative next step; `show`/`show --brief`
read it back; the existing
PostToolUse nudge points the agent at `handoff write` and the existing SessionStart hook surfaces only a
bounded pointer; the generated skill closes the loop hook-agnostically; `go build/vet/test ./...`,
`gofmt -s -l`, and the README/toolgen contract tests are all green.

Confirmed by user 2026-06-23: scope = core loop (write/show + generated hook-agnostic skill) + reuse of
the two EXISTING Claude hooks (NO new Claude PreCompact event) + NEW generated Codex hooks (SessionStart
bounded pointer + staleness-conditioned UserPromptSubmit write nudge, via `.codex/config.toml`, reusing
the shared handlers, no auto-trust); delivery = governed Slipway flow; advisory-only; runtime/gitignored;
narrative agent-authored & nudge-triggered; SessionStart = bounded pointer (no body); legacy migration
excluded; hooks are a graceful-degradation layer over a portable command+skill core.
NOTE: Codex-hook inclusion is a scope EXPANSION added 2026-06-23 at explicit user request ("我需要codex也接入hooks").
DECISION 2026-06-23 (re-confirmed at intake re-walk): the machine header is DECOUPLED from `slipway next` —
it no longer snapshots current_state/substep or next_skill/next_command. Lifecycle position and the next
step stay owned by `slipway status`/`next`; the handoff points the resumer there. This removes redundancy,
eliminates the stale-snapshot failure mode (a header copy reads misleading the moment the lifecycle moves),
and avoids any second-authority smell; staleness is derived from updated_at vs the change's latest
lifecycle event. The Claude/Codex hook→`slipway handoff` wiring is unchanged and confirmed (user: "第二个没问题").
