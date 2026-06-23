# Requirements

## Requirements

### Requirement: Engine-owned per-change handoff writer
REQ-001: The system MUST provide a `slipway handoff write` command that creates or refreshes
the per-change runtime session handoff at `.git/slipway/runtime/changes/<slug>/handoff.md`,
becoming the engine-owned writer for an artifact that today has no non-test writer. Bare
`slipway handoff` MUST behave as `slipway handoff write`.

#### Scenario: write creates the canonical per-change handoff
GIVEN an active governed change with no handoff file yet
WHEN the agent runs `slipway handoff write`
THEN a handoff file is created at `.git/slipway/runtime/changes/<slug>/handoff.md`
AND it contains the engine machine header and the narrative skeleton headings.

#### Scenario: write refreshes an existing handoff in place
GIVEN a handoff file already exists for the active change
WHEN the agent runs `slipway handoff write` again
THEN the engine machine header is regenerated and the existing narrative sections are preserved.

### Requirement: Deterministic engine-stamped machine header
REQ-002: Every `slipway handoff write` MUST regenerate a fenced, engine-owned machine-header block
that is never hand-edited and that records: slug, a monotonic generation counter, session_owner,
git branch/worktree, updated_at, and a staleness indicator (derived from updated_at versus the
change's latest lifecycle event — stale when the lifecycle advanced after the handoff was written).
The header MUST NOT snapshot the lifecycle position or the next step — no current_state/substep and
no next_skill/next_command — because those remain owned by `slipway status`/`next`; the handoff
points the resumer to those authoritative commands instead of embedding a copy that would go stale
and act as a second authority. This change does NOT modify the `slipway next` command itself, and
the handoff neither reads nor embeds its output.

#### Scenario: header carries no lifecycle/next snapshot
GIVEN the lifecycle reports a specific state and next_skill via `slipway status`/`next`
WHEN `slipway handoff write` stamps the machine header
THEN the header contains no current_state/substep field and no next_skill/next_command field
AND the handoff directs the resumer to run `slipway status`/`next` for the authoritative next step
AND no second authority is introduced (the header is identity + freshness, not a source of truth).

#### Scenario: staleness reflects lifecycle advancement
GIVEN a handoff written while the change was at one lifecycle position
WHEN the lifecycle later advances and the agent runs `slipway handoff show`
THEN the staleness indicator reports the handoff as stale (the lifecycle moved since it was written).

#### Scenario: generation counter is monotonic across writes
GIVEN a handoff at generation N
WHEN `slipway handoff write` runs again
THEN the regenerated header reports generation N+1.

### Requirement: Resume-oriented read command
REQ-003: The system MUST provide `slipway handoff show` that reads back the current per-change
handoff for a fresh or resuming session, supporting `--json` (structured) and `--brief` (engine
descriptor — generation, updated_at, session_owner, staleness — plus the path and a one-line focus;
no lifecycle/next field).

#### Scenario: show returns the current handoff
GIVEN a handoff exists for the active change
WHEN the agent runs `slipway handoff show`
THEN the command prints the handoff (human-readable by default, structured under `--json`).

#### Scenario: brief renders only the descriptor for hook surfacing
GIVEN a handoff exists for the active change
WHEN `slipway handoff show --brief` runs
THEN only the engine descriptor (no state/next field), the path, and a one-line focus are emitted,
suitable for a SessionStart hook.

### Requirement: Slug-free active-change resolution with graceful no-op
REQ-004: `slipway handoff write` and `show` MUST resolve the target change from the invoking
worktree's bound active change WITHOUT requiring a slug, reusing the existing active-change
resolver. `--change <slug>` MUST be an optional explicit override. When no active change is bound
to the invoking worktree, the commands MUST no-op gracefully (no error, no write), so a statically
configured hook that cannot pass a slug works correctly.

#### Scenario: hook resolves the active change without a slug
GIVEN a worktree bound to an active change
WHEN the SessionStart hook invokes `slipway handoff show --brief` with no `--change`
THEN the active change bound to that worktree is resolved and its descriptor is surfaced.

#### Scenario: graceful no-op when no active change is bound
GIVEN a worktree with no active bound change
WHEN `slipway handoff write` runs (e.g. from a hook)
THEN the command exits without error and without writing any handoff file.

#### Scenario: explicit override targets a specific change
GIVEN multiple changes exist
WHEN the agent runs `slipway handoff show --change <slug>`
THEN the named change's handoff is shown regardless of the worktree's active binding.

### Requirement: Generated hook-agnostic skill surface
REQ-005: The system MUST generate a `slipway-handoff` command/skill surface via the toolgen
`commandRegistry` that instructs agents WHEN to write (meaningful moments, before stopping, before
a review split) and to read (`show` on resume). The skill MUST be hook-agnostic: it drives
write+read through the command and MUST NOT assume a PreCompact/SessionStart hook fired, so the
cross-session loop closes on any host or fresh agent that does not wire (or ignores) a Slipway hook
(e.g. Codex, which has its own hooks plus native session resume but no hook that authors narrative).
The inline "Runtime Session Handoff" prose
in the workflow `SKILL.md` template MUST be replaced with a pointer to this owned surface.

#### Scenario: generated surface exists for host adapters
GIVEN toolgen generation runs
WHEN host surfaces are produced
THEN a `slipway-handoff` command/skill surface exists and documents the write+read flow.

#### Scenario: skill does not assume a hook fired
GIVEN a host or fresh agent with no Slipway hook wired (the skill must not rely on one)
WHEN an agent follows the `slipway-handoff` skill
THEN it still writes before stopping and reads on resume via the command, closing the loop.

### Requirement: Write is agent-authored, nudge-triggered (no dedicated PreCompact hook)
REQ-006: Handoff narrative MUST be authored by the agent via `slipway handoff write` at meaningful
moments (task completion, before a review split, before stopping) and when context pressure is
signalled. The system MUST NOT rely on a dedicated PreCompact hook to capture narrative: a hook runs
a deterministic command with no access to agent reasoning, so it can only stamp re-derivable metadata
(obtainable any time from `slipway next`/`status`) and CANNOT author the narrative that compaction
actually puts at risk. Instead, the EXISTING context-pressure surface (PostToolUse nudge) MUST route
its remediation to `slipway handoff write` so the agent writes while it still has a turn. This is
host-agnostic and does not depend on any PreCompact hook — no host's PreCompact can author narrative
(Codex has a PreCompact hook too; it still cannot).

#### Scenario: agent writes narrative on a context-pressure nudge
GIVEN rising context pressure during an active change
WHEN the context-pressure surface prompts the agent
THEN the agent runs `slipway handoff write` and authors the narrative before any compaction occurs.

#### Scenario: no dedicated PreCompact hook is required
GIVEN that no host's PreCompact hook can author the narrative (Codex has one and it still cannot)
WHEN the agent follows the `slipway-handoff` skill at checkpoints
THEN narrative capture still occurs via the command, with no PreCompact dependency.

#### Scenario: metadata-only snapshots are not treated as continuity
GIVEN any automated trigger that cannot author narrative
WHEN only the engine machine header would be stamped
THEN that alone MUST NOT be relied on as captured continuity, since it is re-derivable from the lifecycle.

### Requirement: SessionStart bounded pointer (no content dump)
REQ-007: The SessionStart hook MUST surface handoffs as a BOUNDED POINTER only — never a content
dump — so that multiple concurrently active changes cannot pollute the session context. For the
single change bound to the invoking worktree it MAY emit the `slipway handoff show --brief`
descriptor (generation, updated_at, session_owner, staleness; no lifecycle state/substep and no
next_skill/next_command), plus the path, AFTER and SUBORDINATE TO the authoritative `slipway next`
block. When resolution is ambiguous (multiple active changes, none uniquely worktree-bound, e.g.
invoked from the repo root) it MUST emit at most a list of slugs+paths or a count, and MUST NOT emit
any handoff body, focus text, lifecycle state/substep, or next_skill/next_command. The continuation
content read is PULL-based via `slipway handoff show`. The hook MUST invoke the owned command, not
parse the artifact.

#### Scenario: single bound change emits a one-line pointer, not a body
GIVEN a worktree bound to exactly one active change with a handoff
WHEN a session starts and the SessionStart hook fires
THEN at most a one-line descriptor plus the path is emitted after the `slipway next` block
AND no narrative or focus body is injected
AND no lifecycle state/substep or next_skill/next_command field is injected.

#### Scenario: multiple active changes do not dump multiple bodies
GIVEN multiple active changes with no single worktree-bound one (e.g. invoked from the repo root)
WHEN the SessionStart hook fires
THEN only a bounded list of slugs+paths (or a count) is emitted
AND no per-change handoff body, focus text, lifecycle state/substep, or next_skill/next_command is
injected, so the session context stays bounded.

#### Scenario: continuation content is pulled on demand for one change
GIVEN the agent has identified the change it is continuing
WHEN it needs the full handoff
THEN it runs `slipway handoff show [--change <slug>]` and reads exactly that one handoff.

### Requirement: Advisory-only invariant preserved
REQ-008: The handoff MUST remain advisory only. The change MUST NOT make the handoff lifecycle
authority, governed evidence, freshness input, or a gate, and MUST NOT add any bypass, force-close,
or attestation path. A fresh session MUST still rely on `slipway status`/`next` and CLI-owned
freshness and evidence checks.

#### Scenario: handoff never gates the lifecycle
GIVEN a stale, missing, or hand-edited handoff
WHEN the lifecycle evaluates readiness and gates
THEN no gate, evidence verdict, or freshness result depends on the handoff content.

### Requirement: Generated Codex hook integration (symmetric, grounded in Codex's hook system)
REQ-009: toolgen MUST generate repo-local Codex hooks so Codex participates in the same handoff
automation through its OWN hook system, not only via the hook-agnostic skill. Codex (0.141.0+) loads a
`[hooks]` table from a project-layer `.codex/config.toml` (`[[hooks.<Event>]]` groups, each with a
`hooks` array of `{ type = "command", command = ... }`). The generated hooks MUST invoke the SAME
`slipway hook` handlers used for Claude (no Codex-specific handoff logic; the command owns format and
content). Two hooks are generated:
- `SessionStart` → `slipway hook session-start`, surfacing the BOUNDED pointer (REQ-007) via
  `hookSpecificOutput.additionalContext`, subordinate to the authoritative next-action. Because Codex
  re-fires `SessionStart` with `source == "compact"` after compaction, this single hook ALSO covers
  post-compaction re-injection (Codex `PostCompact` cannot inject context, so it is not used).
- `UserPromptSubmit` → a write nudge routing remediation to `slipway handoff write` (consistent with
  REQ-006). Because Codex delivers NO context-window/token metrics to command hooks (unlike Claude's
  `context_utilization`), the Codex nudge MUST condition on the engine-derivable signal of handoff
  staleness/absence — NOT on a fabricated pressure number — and MUST stay terse and emit nothing when
  the handoff is fresh, so it is not noisy.
The change MUST NOT auto-grant Codex project or hook trust (user consent only); init/provision output
MUST surface that the generated hooks are INERT until the repo is trusted and each hook is trusted. For
Slipway-provisioned worktrees, the hooks MUST be written where Codex actually reads them (the root
checkout's `.codex/`), not only a divergent linked-worktree copy. REQ-008's advisory-only invariant
applies unchanged to the Codex hooks.

#### Scenario: Codex SessionStart hook surfaces the bounded pointer
GIVEN a trusted repo with an active worktree-bound change
WHEN a Codex session starts and the generated SessionStart hook runs `slipway hook session-start`
THEN the bounded pointer is injected via additionalContext, subordinate to the next-action, with no body.

#### Scenario: post-compaction re-injection rides SessionStart
GIVEN Codex compacts an ongoing session and restarts it
WHEN Codex re-fires SessionStart with source == "compact"
THEN the bounded pointer is re-injected, since Codex PostCompact cannot inject context.

#### Scenario: Codex write nudge conditions on staleness, not fabricated metrics
GIVEN Codex provides no context-window metrics to the hook
WHEN the UserPromptSubmit hook runs and the active change's handoff is missing or stale
THEN a terse reminder to run `slipway handoff write` is injected
AND when the handoff is fresh, nothing is emitted.

#### Scenario: generation does not grant trust
GIVEN toolgen generates the Codex hooks
WHEN init/provision completes
THEN the `.codex/config.toml` hooks exist but are documented as inert until the user trusts the repo and hooks
AND Slipway does not modify the user's global trust configuration.
