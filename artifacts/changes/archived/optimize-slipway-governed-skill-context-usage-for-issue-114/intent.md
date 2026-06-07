# Intent

## Summary
Optimize Slipway governed skill context usage for issue #114 by studying local gsd, superpowers, and spec-kitty patterns, then applying functionality-preserving thin-host/subagent or summary-first changes to heavy verification/preflight/orchestration surfaces.
## Complexity Assessment
complex
Rationale: this touches governed skill surfaces used near verification and
closeout, requires local reference-project research, and must preserve fail-closed
evidence contracts while reducing main-thread context load.

## Guardrail Domains
<!-- none detected -->

## In Scope
- Investigate local `ghq` repositories named in issue #114 and the user request:
  `gsd-build/get-shit-done`, `obra/superpowers`, and `Priivacy-ai/spec-kitty`,
  plus directly relevant neighboring patterns if discovered locally.
- Optimize Slipway's generated governance skill source templates so heavy
  evidence reading, baseline command output, and broad codebase-map context are
  delegated to verifier/executor subagents or kept as structured summaries in
  the main host.
- Prioritize the heavy stages named in #114: `goal-verification`,
  `worktree-preflight`, and `wave-orchestration`.
- Preserve existing governed gates, IRON LAW language, freshness checks,
  high-risk safety baseline requirements, and evidence artifact contracts.
- Add or update focused tests that protect the generated skill/template contract
  and prevent regressions toward inline full-output-heavy host behavior.
- Repair the narrow S2 Scope Contract false-positive discovered during this
  change: durable `artifacts/codebase/` discovery context updates must not force
  wave task `changed_files` coverage, while implementation/test diffs and `done`
  dirty-worktree advisories remain visible.

## Out of Scope
- No bypass, force-pass, private attestation, or weakening of SAST/high-risk
  safety baseline gates.
- No runtime analytics implementation for re-running `/usage` attribution
  aggregation in this change.
- No new model-routing feature for cheaper verifier/executor models unless it
  already exists as a template-only option.
- No broad rewrite of the Slipway lifecycle engine or artifact schema; the only
  engine change allowed in this bundle is the narrow Scope Contract discovery
  artifact exemption needed to complete this governed execution without
  weakening task changed-file coverage.

## Constraints
- Source of truth for generated skill text is under
  `internal/tmpl/templates/skills/`; generated tool copies must not be hand-edited.
- The change must keep the main-thread host responsible for final verdict and
  hard-gate decisions, while delegating bulky inspection/output reading.
- Sensitive verification surfaces must fail closed if delegated evidence is
  missing, inconclusive, stale, or unsafe.

## Acceptance Signals
- Local reference-project findings are documented in the governed research
  artifacts and trace directly to implemented template choices.
- Generated skill/template tests assert thin-host behavior for at least
  `goal-verification`, `worktree-preflight`, and `wave-orchestration`.
- Scope Contract sampler tests assert `artifacts/codebase/` discovery dirt is
  not treated as S2 execution drift while real implementation dirt and `done`
  dirty-worktree advisories stay visible.
- `go test ./...` passes in the governed worktree.
- `go run . validate --json` reports governed readiness appropriate for the
  current lifecycle stage after implementation and review evidence is refreshed.

## Open Questions
- [x] Delegation phrasing across Codex/Claude/Cursor/Gemini — RESOLVED:
  use the summary-first portable contract (`command + exit + bounded summary +
  fresh:command_ref`) as the baseline, and phrase isolated verifier/executor
  contexts as "when supported; otherwise bounded inline fallback" so runtimes
  without subagent support are never told to call an unavailable API.
- [x] Wave-orchestration slimming without engine `input_context` changes —
  RESOLVED: keep this template-level by passing `codebase_map_dir` and relevant
  document paths to executors, limiting the coordinator to `wave_plan` metadata,
  and relocating the PR #112 codebase-map staleness self-check rather than
  deleting it.

## Deferred Ideas
- Runtime token attribution regression tooling that compares `attributionSkill`
  and `attributionAgent` before/after the refactor.
- First-class verifier/executor model selection knobs.
- Workflow preset right-sizing for personal/trivial work beyond the current
  standard-preset governed flow.

## Approved Summary
Confirmed by user on 2026-06-07T06:43:48Z.

Optimize Slipway's governed skill context usage for issue #114 by researching
local `gsd-build/get-shit-done`, `obra/superpowers`, and
`Priivacy-ai/spec-kitty`, then applying functionality-preserving thin-host,
subagent, or summary-first patterns to the generated skill templates for
`goal-verification`, `worktree-preflight`, and `wave-orchestration`.

The change keeps the main host responsible for final verdicts and hard gates,
preserves freshness and high-risk safety baseline requirements, and does not add
any bypass, force-pass, private attestation, runtime token analytics, broad
lifecycle rewrite, or new model-routing feature. It also includes the narrow S2
Scope Contract sampler repair discovered during execution: discovery-owned
`artifacts/codebase/` dirt is excluded from execution changed-file drift checks,
while real implementation/test dirt and `done` dirty-worktree visibility remain
covered. Completion is signaled by traceable research artifacts, focused
template/contract tests for the heavy stages, Scope Contract sampler regression
tests, passing `go test ./...`, and lifecycle validation evidence.
