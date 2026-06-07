# Decision
## Project Context
- Tech Stack: Go
- Conventions: # Slipway Agent Principles

Slipway is the lifecycle authority for governed work. This file is not a
command manual, classification guide, JSON reference, or recovery cookbook. It
sets the principles an AI agent must follow when working in this repository.

## Lifecycle Authority

- Treat the current worktree's Slipway CLI as the source of truth.
- Use the Slipway behavior produced by the current worktree, not stale installed
  binaries, remembered flows, or copied recipes.
- Let Slipway decide ...
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

- **Approach A — Minimal template thin-host / summary-first refactor.** Change
  only the three heavy host templates so the main host enumerates scope and
  keeps bounded summaries + artifact references, delegating bulky reading to a
  fresh context when supported, and fails closed on missing delegated evidence.
  Tradeoffs: highest issue #114 leverage with the lowest intended blast radius
  (no schema, `input_context`, or lifecycle state-machine change); does not
  deliver token telemetry or model-routing. A later narrow internal Scope
  Contract sampler repair was added after S2 execution exposed that
  discovery-owned `artifacts/codebase/` dirt could falsely block otherwise
  complete runtime task evidence.
- **Approach B — Engine-level `input_context` packing + runtime evidence
  summarization.** Thicken `slipway next --json` so hosts avoid turn-by-turn
  artifact discovery, and emit command-output transcript refs from the runtime.
  Tradeoffs: stronger long-term architecture, but broad blast radius into
  progression/serialization/test fixtures — disproportionate to the issue's
  fastest fix.
- **Approach C — Surface-budget / profile right-sizing system.** Add
  configurable light/standard/strict skill-surface or model profiles (GSD-style).
  Tradeoffs: useful future right-sizing, but it changes product behavior and
  configuration semantics rather than fixing the current heavy-stage context
  span; explicitly deferred by intent.

### Constraints (from source document)
- Source of truth for generated skill text is under
  `internal/tmpl/templates/skills/`; generated tool copies must not be hand-edited.
- The change must keep the main-thread host responsible for final verdict and
  hard-gate decisions, while delegating bulky inspection/output reading.
- Sensitive verification surfaces must fail closed if delegated evidence is
  missing, inconclusive, stale, or unsafe.


## Selected Approach
**Approach A — Minimal template thin-host / summary-first refactor.**

Rationale: issue #114's root cause is bulky evidence held in the main-thread
context and re-charged as cache-read every turn, not skill prose width. Approach
A removes that held context at the source (the three named templates) with the
smallest clean product change. The portable contract is summary-first — the host
keeps `command + exit + bounded summary + command_ref` — and
fresh-context/subagent isolation is layered on only "when supported," reusing
the idiom `wave-orchestration` already ships, so no-subagent runtimes (e.g.
Codex) stay truthful. B and C are deferred (B is a larger architecture move; C
changes product/config semantics) and remain available if template-only changes
prove insufficient.

During S2 execution, Scope Contract reopened the change because durable
`artifacts/codebase/` discovery-context files were treated as implementation
workspace dirt requiring task `changed_files` coverage. The selected approach now
includes the narrow internal repair required to keep that discovery context out
of S2 execution-drift sampling while preserving real implementation drift checks
and `done` dirty-worktree visibility.

This direction must continue honoring the documented constraints:
- Source of truth for generated skill text is under `internal/tmpl/templates/skills/`; generated tool copies must not be hand-edited.
- The change must keep the main-thread host responsible for final verdict and hard-gate decisions, while delegating bulky inspection/output reading.
- Sensitive verification surfaces must fail closed if delegated evidence is missing, inconclusive, stale, or unsafe.

## Interfaces and Data Flow
No CLI, JSON, artifact-schema, or lifecycle state-machine interface changes.
Primary data flow is templates
(`internal/tmpl/templates/skills/<stage>/SKILL.md[.tmpl]`) → `internal/toolgen`
generated per-tool surfaces → agent host behavior. The `slipway next --json`
`input_context` payload is unchanged: the wave slimming relies only on
metadata/paths the engine already exposes (`codebase_map_dir`,
`codebase_map_status`, `codebase_map_doc_states`, `wave_plan`). The internal
Scope Contract sampler also excludes `artifacts/codebase/` only from S2
workspace-diff drift checks; `done` dirty-worktree advisory behavior stays
visible. Generated `.codex/`, `.claude/`, `.cursor/`, `.gemini/` copies are
regenerated from templates, never hand-edited.

## Rollout and Rollback
Rollout: edit the three source templates and add/extend embedded-template
contract tests; add the narrow Scope Contract sampler regression; regenerate
tool surfaces through the repo-native init/refresh command at closeout if
repository policy requires the generated copies updated. Verification: focused
`go test ./internal/tmpl`, focused `go test ./internal/engine/progression`, then
full `go test ./...`, then `slipway validate --json` for governed readiness.
Rollback: revert the template/test edits and the sampler exemption — there is no
data migration or persisted-state change, so reverting restores prior behavior
(reversibility: high).

## Risk
- **High — safety-gate weakening.** Mitigation: the main host keeps final-verdict
  ownership and the HARD-GATE; the `goal-verification` `safety_baseline` token
  stays anchored to a real, host-referenceable SAST artifact (`fresh:command_ref`),
  never a delegated prose verdict; missing/stale/inconclusive delegated evidence
  fails closed (`high_risk_check_missing`).
- **High — wave slimming regressing the PR #112 codebase-map staleness
  self-check.** Mitigation: relocate the self-check (per-executor refresh, or a
  `codebase_map_doc_states`-driven coordinator decision) instead of deleting it;
  plan-audit treats "PR #112 self-check preserved" as an explicit acceptance item
  and a focused test asserts it survives.
- **Medium — runtime portability.** Mitigation: summary-first is the portable
  baseline; subagent isolation is phrased as "when supported; otherwise <bounded
  inline fallback>".
- **Medium — false token optimization from prose trimming only.** Mitigation:
  target context ownership (who holds bulky output), not cosmetic description
  shortening, which issue #114 rejects as low-leverage.
- **Low — generated-surface drift.** Mitigation: edit source templates only and
  protect them with embedded-template tests.
- **Low — Scope Contract exemption hiding real drift.** Mitigation: exempt only
  `artifacts/codebase/` from S2 execution changed-file sampling; tests assert
  real implementation diffs remain included and `done` still reports
  codebase-map dirt.
