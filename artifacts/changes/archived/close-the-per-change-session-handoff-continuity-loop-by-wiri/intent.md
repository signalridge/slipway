# Intent

## Summary
Close the per-change session-handoff continuity loop by aligning its WRITE trigger and READ surfaces around Slipway's public CLI. The `slipway handoff write/show` command already shipped (#312); today nothing auto-triggers a handoff write, no documented resume path tells a fresh session how to re-enter a specific change, and a global SessionStart hook auto-injects change-state every session (it pushed a stale phantom binding into this very session). v1 scope = three coordinated edits: (W2) escalate the context-pressure hook's CRITICAL-threshold message from a soft suggestion into an imperative directive that tells the agent to author the handoff NOW via `slipway handoff write --section <name>` before its next action (covering long-stage mid-flight context death); (R2) add a 'Continuing A Change In A Fresh Session' resume-protocol section to the generated slipway workflow skill (status --json -> pick --change if multi-active -> handoff show --change <slug> -> next), plus a toolgen test guard; (R3) delete the redundant read-side SessionStart change-state injection so the explicit R2 path is the single sanctioned read surface, keeping only the entry-skill routing pointer. No engine auto-write of bytes: the WRITE is auto-TRIGGERED at the right moment but the narrative judgment is authored by the agent, so handoff updated_at stays tied to real authoring and staleness stays honest. Deferred and explicitly out of scope: W1 stage-completion directive in cmd/run.go and the engine metadata/fact auto-write floor. This alters public contracts (the context-pressure hook message, the generated skill surface, and the SessionStart hook output) so it runs through the governed flow for formal contract review.
## Complexity Assessment
simple
<!-- Rationale: -->
The design is fully converged (no discovery). Scope is two directive-only edits
plus one test guard plus one mechanical hook simplification (delete the redundant
SessionStart change-state injection and its now-dead helpers). It touches a hook
message, a generated-skill template, a toolgen test, and the SessionStart hook —
all in `cmd/` and `internal/`. No new commands, gates, hooks, or schema; the
work is well-understood and low-risk per surface. The sensitivity is that three
edits touch public contracts (the context-pressure hook message, the generated
skill surface, and the SessionStart hook output), which is why this runs through
the governed flow for contract review rather than a direct edit.

## In Scope
- **W2 — escalate the context-pressure hook CRITICAL message.** In
  `cmd/context_pressure_hook.go`, the `contextPressureMessage` CRITICAL-threshold
  branch (ratio >= 0.70) changes from a soft suggestion into an imperative
  directive: tell the agent to author the handoff NOW, before its next action,
  via `slipway handoff write --section <name>` for the judgment sections
  (Current Position, Next Session Focus, Risks And Blockers). The WARNING branch
  (ratio >= 0.60) stays soft.
- **R2 — documented fresh-session resume protocol.** In
  `internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`, add a
  "Continuing A Change In A Fresh Session" section after the existing
  `## Runtime Session Handoff` block: `slipway status --json` -> pick `--change`
  when multiple changes are active -> `slipway handoff show --change <slug>`
  (read the narrative first) -> `slipway next`.
- **R2 guard — toolgen test.** In `internal/toolgen/toolgen_test.go`, add a guard
  asserting the generated slipway workflow skill ships the resume-protocol
  section, so the contract can't silently regress.
- **R3 — delete the redundant read-side SessionStart hook injection.** In
  `cmd/session_start_hook.go`, stop auto-injecting change-state into every
  session: remove the active-worktree `next --json` view, the bound-elsewhere
  `session_handoff_info: ... bound to <worktree>` pointer, and the
  `session_handoff_present`/path/brief summary. KEEP the `slipway_entry_skill`
  routing pointer (it now routes the agent to the slipway skill carrying the R2
  resume protocol). Remove the helpers that become dead
  (`sessionStartNextJSON`, `sessionStartBoundWorktreeHandoff`,
  `firstBoundChange`, `sessionStartHandoffSummary`, and now-unused output
  plumbing) and update `cmd/session_start_hook_test.go` to the entry-skill-only
  contract. This makes the explicit R2 resume path the single sanctioned read
  surface instead of competing with a global auto-read that injects stale /
  phantom bindings (observed live this session).
- Regenerate any committed generated skill artifacts that the template change
  affects, so the generated surface stays in sync with its template.

## Out of Scope
- **W1** — stage-completion handoff directive in `cmd/run.go` after a real
  advance plus `next` Warnings rendering. Deferred.
- **Engine metadata/fact auto-write floor** — engine writing handoff bytes or
  auto-bumping `updated_at`. Deferred on purpose: auto-bump would let
  `HandoffStaleness` report fresh on an unwritten judgment section.
- No changes to the `slipway handoff` command itself (shipped in #312), no new
  hook, no PreCompact wiring, no lifecycle-gate changes. Handoff stays
  advisory-only — not authority, evidence, freshness input, or a gate.

## Constraints
- The W2 message MUST keep the substrings the existing hook tests assert:
  `slipway handoff write`, `The handoff is advisory`, `slipway status --json`,
  `slipway next --json`; and MUST avoid `lifecycle authority`,
  `governed evidence`, `freshness input` (wording that would falsely elevate the
  advisory handoff into authority).
- No engine auto-write of handoff bytes, so `updated_at` stays tied to real
  agent authoring and `HandoffStaleness` cannot lie.
- The generated skill surface stays hook-agnostic / portable across hosts
  (Claude, Codex): the resume protocol uses only CLI commands.
- R3 keeps the SessionStart hook's `slipway_entry_skill` routing pointer and its
  fail-silent (exit 0) contract on both the Claude XML and Codex JSON output
  paths; only change-state injection is removed.
- Preserve unrelated active work (the `merge-the-goal-verification-...`
  S1_PLAN change in its own worktree).

## Acceptance Signals
- `go test ./...` is green across all packages, including the new toolgen guard.
- The context-pressure hook tests pass with the escalated imperative wording and
  all four required substrings retained; none of the forbidden substrings appear.
- The generated slipway workflow skill (`.claude/skills/slipway/SKILL.md` and the
  equivalent generated host surfaces) contains the
  "Continuing A Change In A Fresh Session" resume-protocol section with the
  status -> change -> handoff show -> next sequence.
- The SessionStart hook output contains only the `slipway_entry_skill` routing
  pointer (plus hard-error diagnostics); it no longer emits any
  `session_handoff_info`, active-change `next --json`, or `session_handoff_*`
  change-state. `cmd/session_start_hook_test.go` is updated to assert this
  entry-skill-only contract and stays green.
- `gofmt -s` / golangci-lint clean; the architecture test passes.

## Open Questions
None — design converged; `needs_discovery` is false.

## Approved Summary
Close the per-change session-handoff continuity loop with three coordinated edits
that make Slipway's public CLI the single sanctioned handoff surface:

- **W2 (write trigger):** `cmd/context_pressure_hook.go` CRITICAL-threshold
  message (ratio >= 0.70) escalates from a soft suggestion into an imperative
  directive — author the handoff NOW, before the next action, via
  `slipway handoff write --section <name>`. WARNING stays soft.
- **R2 (read/resume):** the generated slipway workflow skill gains a
  "Continuing A Change In A Fresh Session" resume-protocol section
  (`status --json` -> pick `--change` if multi-active ->
  `handoff show --change <slug>` -> `next`), guarded by a toolgen test.
- **R3 (delete redundant hook):** `cmd/session_start_hook.go` stops
  auto-injecting change-state every session (removes the active-worktree
  `next --json` view, the bound-elsewhere `session_handoff_info` pointer, and the
  handoff summary), keeping ONLY the `slipway_entry_skill` routing pointer that
  routes the agent to the skill carrying R2. Dead helpers and the hook test are
  updated to the entry-skill-only contract.

The engine never auto-writes handoff bytes, so `updated_at` stays tied to real
agent authoring and `HandoffStaleness` stays honest. Handoff remains
advisory-only — not authority, evidence, freshness input, or a gate.

**Out of scope (deferred):** W1 (stage-completion handoff directive in
`cmd/run.go` + `next` Warnings rendering) and the engine metadata/fact
auto-write floor.

**Primary acceptance signal:** `go test ./...` green across all packages,
including the new toolgen guard and the updated SessionStart hook test, with the
context-pressure hook tests passing under the escalated wording (four required
substrings kept, authority-elevating ones avoided).

Confirmed by user on 2026-06-24 (scope expanded from W2+R2 to add R3 at user
request; R3 deletion boundary confirmed: keep `slipway_entry_skill`, remove only
the change-state auto-injection).
