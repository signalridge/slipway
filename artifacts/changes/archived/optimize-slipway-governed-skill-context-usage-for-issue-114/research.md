## Research Findings

### Architecture
- Affected modules: generated governance skill templates under
  `internal/tmpl/templates/skills/`, especially
  `goal-verification/SKILL.md.tmpl`, `worktree-preflight/SKILL.md`, and
  `wave-orchestration/SKILL.md.tmpl`. The execution repair discovered during
  this change is narrowly limited to S2 Scope Contract workspace-diff sampling
  in `internal/engine/progression/readiness.go`.
- Dependency chains: templates -> `internal/toolgen` generated surfaces ->
  agent runtime host behavior. Lifecycle routing remains engine-owned through
  `next_skill.name`; this change should not alter state transitions or artifact
  schemas.
- Blast radius: host instructions, generated-surface contract tests, and one
  Scope Contract sampler exemption for durable `artifacts/codebase/` discovery
  artifacts. No lifecycle state-machine rewrite, no artifact-schema change, no
  hand edits to generated `.codex/`, `.claude/`, `.cursor/`, or `.gemini/`
  surfaces.
- Constraints: `goal-verification` remains safety-sensitive because it produces
  `high_risk_check:<domain>.safety_baseline=pass` when a guardrail domain is
  active; `worktree-preflight` still needs worktree path/branch/baseline
  references; `wave-orchestration` still needs task evidence through
  `slipway evidence task`.
- Current Slipway baseline: `goal-verification` tells the host to run
  `validate`, `status`, acceptance checks, stub scans, and SAST inline; this is
  functionally correct but keeps bulky output in the main host context
  (`internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl:25`,
  `:61`, `:91`). `worktree-preflight` similarly asks the host to run and
  capture a baseline command directly (`worktree-preflight/SKILL.md:50`).
  `wave-orchestration` already dispatches executors but tells the coordinator
  to read broad codebase-map files before execution
  (`wave-orchestration/SKILL.md.tmpl:25`) and only later says the orchestrator
  must stay under 15% context (`:109`).

### Patterns
- GSD thin-host pattern: `execute-phase` states that the orchestrator stays lean
  and delegates plan execution to subagents
  (`get-shit-done/workflows/execute-phase.md:1-7`). It loads initialization
  context in one query call (`:65-74`), resolves runtime support and fallback
  behavior before dispatch (`:9-24`, `:80-95`), and adapts prompt richness to
  available context window (`:113-129`).
- GSD verifier pattern: `gsd-verifier` is a dedicated verifier agent with its
  own tool list and goal-backward verification stance
  (`agents/gsd-verifier.md:1-18`). It explicitly distrusts summary claims
  (`:21-38`), performs re-verification by fully rechecking failed items and
  only regression-checking previous passes (`:79-98`), then runs artifact,
  wiring, anti-pattern, and probe checks inside the verifier context
  (`:217-263`, `:404-444`, `:492-520`).
- Superpowers controller/subagent split: `subagent-driven-development` says
  subagents must not inherit the controller session's history and the controller
  constructs exactly the context they need (`SKILL.md:8-12`). The controller
  reads/extracts tasks once, dispatches implementer and reviewer subagents, and
  preserves its own context for coordination (`:63-85`, `:204-221`). It also
  calls out a red flag directly relevant to Slipway: do not make the subagent
  read the whole plan file if the controller can pass the relevant task text
  (`:236-245`).
- Spec Kitty delta/context pattern: specs are concise change requests, while
  code is the source of truth for current implementation
  (`spec-driven.md:23-33`, `:49-64`, `:86`). Planning kicks off explicit
  research and agent context refresh before tasks are generated
  (`spec-driven.md:173-183`), and task generation creates bounded work-package
  prompt bundles rather than one unbounded implementation blob (`:185-193`).
- Existing Slipway reusable pattern: `wave-orchestration` already references a
  lazy `references/executor-dispatch-reference.md` file and has a dispatch
  contract; the goal/preflight stages lack equivalent delegation language and
  summary-only output constraints. It also already uses portable conditional
  phrasing — "dispatch tasks in parallel when the tool supports it" and "one
  isolated executor context per task when supported; otherwise execute
  sequentially" (`wave-orchestration/SKILL.md.tmpl:69`, `:76`) — which is the
  exact idiom the goal/preflight stages should reuse rather than inventing a
  runtime-specific delegation API.
- Existing Scope Contract boundary: S2 changed-file drift checks merge runtime
  execution-summary `changed_files` with `git diff --name-only HEAD` so real
  implementation/test dirt cannot sneak past task evidence. Durable
  `artifacts/codebase/` documents, however, are discovery context consumed by
  intake, research, and plan-audit; treating them as S2 implementation outputs
  creates a false-positive recovery loop after legitimate codebase-map
  refreshes. The narrow pattern is to exempt `artifacts/codebase/` only from S2
  Scope Contract diff sampling, while leaving implementation diffs visible and
  keeping `WorkspaceChangedFilesForDoneArchive` dirty advisories unchanged.
- Spec-Kit (GitHub) summary-first pattern: prerequisite scripts emit a single
  JSON payload (`FEATURE_DIR` + `AVAILABLE_DOCS`) so the host parses once and
  loads only present artifacts instead of re-scanning or holding full output
  (`scripts/bash/check-prerequisites.sh:156-177`); a `lean` command preset proves
  the same work runs with ~10-line instead of ~170-line host instructions
  (`presets/lean/commands/speckit.plan.md`). Lesson: the portable, cross-runtime
  lever is "host keeps a bounded summary + a reference, not the full output";
  subagent isolation is an enhancement layered on top, not the precondition.
- OpenSpec constraint/content separation: injected `context`/`rules` are wrapped
  as explicit "constraints for you — do NOT include in your output" so background
  never leaks into the produced artifact (`src/commands/workflow/instructions.ts:150-168`);
  dependencies are returned as path + done-status, read lazily, never inlined
  (`src/core/artifact-graph/instruction-loader.ts:346-360`). Lesson for Slipway:
  delegated context fed to a verifier/executor must be tagged as host-only
  constraint so it does not inflate `verification/*.yaml`.

### Risks
- High: weakening safety gates while moving checks into delegated contexts.
  Mitigation: main host keeps final verdict ownership and must fail closed when
  delegated verifier/preflight evidence is missing, stale, inconclusive, or
  reports blockers. For `goal-verification` specifically, the SAST verdict must
  stay anchored to a real, host-referenceable artifact: a delegated verifier
  records `high_risk_check:<domain>.safety_baseline=pass` with
  `fresh:command_ref=<path to the real SAST output>` only — the host must never
  record the safety token from a subagent's prose verdict alone, or the
  delegation becomes the private-attestation path the lifecycle forbids. The
  existing evidence contract (`fresh:command_ref=<path or transcript ref>`)
  already supports this; delegation changes *where output is read*, not *that the
  reference points to a real artifact*.
- High: wave-orchestration codebase-map slimming can regress the codebase-map
  staleness self-check shipped in PR #112 (issue #80). `wave-orchestration`
  (`SKILL.md.tmpl:34-43`) is one of three skills that consume
  `codebase_map_status`/`codebase_map_doc_states` and are instructed to judge
  scope-relevance and re-author stale map sections inline. Moving the four-doc
  codebase-map read out of the coordinator (to shed the held context that drives
  its ~15% all-tok span) must NOT delete that self-check — it must relocate it:
  either each executor performs the relevance/refresh for the docs it reads, or
  the coordinator decides route-vs-flag from `codebase_map_doc_states` metadata
  (not the doc bodies). Mitigation: plan-audit must treat "PR #112 self-check
  preserved" as an explicit acceptance item, and a focused test must assert the
  staleness-advisory handling survives.
- Medium: runtime portability. Codex currently does not generate exported agent
  directories, so template language must say "use a fresh subagent/verifier
  context when supported; otherwise use a bounded structured-summary fallback"
  instead of requiring one runtime-specific `Task` API.
- Medium: false token optimization by trimming prose only. Issue #114 rejects
  this as low-leverage; template wording should target context ownership, not
  cosmetic description shortening.
- Low: generated-surface drift. Mitigation: edit source templates and protect
  them with embedded-template tests.
- Low: hiding real drift under the codebase-map exemption. Mitigation: the
  exemption matches only `artifacts/codebase/`; ordinary implementation/test
  files remain in Scope Contract changed-file sampling, and `done` still reports
  codebase-map files as dirty.
- Reversibility: high. These are template and test changes; reverting restores
  previous host instructions without data migration.

### Test Strategy
- Existing coverage: `internal/tmpl/templates_test.go` already asserts generated
  surface contract strings; `internal/tmpl/wave_isolation_content_test.go`
  asserts wave dispatch content; `internal/toolgen/toolgen_test.go` protects
  exported host inventory and generated layouts.
- Infrastructure needs: focused tests that read the embedded templates and
  assert thin-host constraints for `goal-verification`, `worktree-preflight`,
  and `wave-orchestration`.
- Verification approach:
  - Assert `goal-verification` requires a fresh verifier context, structured
    delegated verdict, main-host final verdict ownership, and fail-closed safety
    baseline behavior.
  - Assert `worktree-preflight` requires baseline command output to be reduced
    to command, exit code, and failure summary in the main host while preserving
    required parseable references.
  - Assert `wave-orchestration` no longer asks the coordinator to read broad
    codebase-map docs before dispatch; instead, it passes relevant codebase-map
    paths to executors and keeps coordinator context under budget.
  - Assert Scope Contract workspace-diff sampling excludes
    `artifacts/codebase/STRUCTURE.md` during S2 while still including real
    implementation diffs, and assert `done` dirty-worktree reporting still keeps
    codebase-map artifacts visible.
  - Run focused `go test ./internal/tmpl`, then full `go test ./...`.

## Alternatives Considered
- Approach A: Minimal template thin-host refactor.
  - Design: change the three heavy host templates so the main host enumerates
    scope, dispatches or isolates bulky verification/preflight/execution
    reading in fresh contexts, receives structured summaries, writes the final
    verification artifacts, and fails closed on missing delegated evidence.
  - Tradeoffs: highest issue #114 leverage with low engine risk; does not
    create first-class token telemetry or model-routing features.
  - Status: **Selected** (user-confirmed 2026-06-07), with the later narrow
    Scope Contract sampler repair added after execution exposed a false-positive
    recovery loop on discovery-owned `artifacts/codebase/` dirt. The portable
    main contract is summary-first (host keeps `command + exit + bounded summary
    + command_ref`, not full output); a fresh subagent/verifier context is an
    optional enhancement "when supported," reusing the existing
    `wave-orchestration` "when supported; otherwise <fallback>" idiom so Codex
    and other no-subagent surfaces stay truthful.
- Approach B: Engine-level `input_context` packing and runtime evidence
  summarization.
  - Design: add richer `slipway next --json` payloads so hosts can avoid
    turn-by-turn artifact discovery, and possibly provide command-output
    transcript refs directly from the runtime.
  - Tradeoffs: stronger long-term architecture, but broader blast radius and
    likely touches progression/serialization/test fixtures beyond the issue's
    fastest fix.
  - Status: defer unless template-only changes prove insufficient.
- Approach C: Surface-budget/profile system.
  - Design: add configurable light/standard/strict skill-surface or model
    profiles inspired by GSD minimal profiles and context-window adaptation.
  - Tradeoffs: useful for future right-sizing, but it changes product behavior
    and configuration semantics rather than fixing the current heavy-stage
    context span.
  - Status: defer.

## Unknowns
- Resolved: Which local pattern best maps to issue #114? -> GSD's
  `execute-phase` + `gsd-verifier` and Superpowers' subagent-driven controller
  pattern both support a thin host with delegated heavy verification/reading.
- Resolved: Does Spec Kitty suggest making specs larger? -> No. Its useful
  lesson is delta focus plus explicit research/context refresh, not embedding
  comprehensive system state in the prompt.
- Resolved (intent Open Question 1): Which subagent/delegation phrasing is
  supported across Codex/Claude/Cursor/Gemini without misleading any surface? ->
  Do not invent a runtime API. The portable main contract is summary-first
  (`command + exit + bounded summary + fresh:command_ref`), which every surface
  can satisfy. Layer subagent isolation as an enhancement using the idiom already
  shipped in `wave-orchestration/SKILL.md.tmpl:69,:76` — "isolated context per
  task/check when supported; otherwise <bounded inline fallback>". Codex (no
  exported agent dirs) reads the summary-first contract; Claude additionally gets
  fresh-context isolation. Superpowers corroborates the ordering: cheap checks
  (placeholder/consistency scans) are faster and equal-quality as inline
  self-review than as dispatched subagents, so the goal is "stop holding bulky
  output," not "dispatch everything."
- Resolved (intent Open Question 2): How far can `wave-orchestration` be slimmed
  without changing engine `input_context`? -> Fully template-level. The engine
  already exposes `codebase_map_dir`, `codebase_map_status`, and
  `codebase_map_doc_states` as metadata/paths (confirmed in `slipway next --json`
  `input_context`). The slimming moves the four-doc map read
  (`SKILL.md.tmpl:25-29`) into executors (coordinator passes the map dir + doc
  paths; executors read only what their task needs) and limits the coordinator to
  the engine-authoritative `wave_plan` metadata it already reads (`:53-66`). No
  `next --json` payload change is required. Caveat: see the PR #112 risk — the
  coordinator's codebase-map relevance/staleness self-check (`:34-43`) must be
  relocated, not removed.
- Resolved: Should `artifacts/codebase/` dirt be covered by S2 task
  `changed_files`? -> No. Those files are durable discovery/planning context,
  not implementation task outputs. Scope Contract should exclude them only from
  S2 execution-drift sampling; `done` dirty-worktree reporting still keeps them
  visible so operators can commit or review them intentionally.
- Remaining (for slipway-plan-audit): Whether generated target refresh is
  required in this worktree after template edits, or whether embedded-template
  tests are sufficient until a later `slipway init --refresh --tools all`
  closeout step. Plan-audit must also confirm the PR #112 codebase-map
  self-check is preserved by the wave slimming task and covered by a focused test.

## Assumptions
- The immediate optimization should be template-level and
  functionality-preserving because issue #114 explicitly frames trimming skill
  prose as low-leverage and names heavy main-thread evidence reading as the
  root cause.
- Runtime-specific subagent APIs vary; therefore the template must describe the
  contract in portable terms and permit a bounded inline fallback.
- The main host must remain the final governance writer because verification
  artifacts and hard-gate confirmation are lifecycle authority surfaces.

## Canonical References
- `https://github.com/signalridge/slipway/issues/114`
- `/Users/yixianlu/ghq/github.com/gsd-build/get-shit-done/get-shit-done/workflows/execute-phase.md:1`
- `/Users/yixianlu/ghq/github.com/gsd-build/get-shit-done/agents/gsd-verifier.md:1`
- `/Users/yixianlu/ghq/github.com/obra/superpowers/skills/subagent-driven-development/SKILL.md:8`
- `/Users/yixianlu/ghq/github.com/Priivacy-ai/spec-kitty/spec-driven.md:23`
- `/Users/yixianlu/ghq/github.com/github/spec-kit/scripts/bash/check-prerequisites.sh:156`
- `/Users/yixianlu/ghq/github.com/github/spec-kit/presets/lean/commands/speckit.plan.md`
- `/Users/yixianlu/ghq/github.com/Fission-AI/OpenSpec/src/commands/workflow/instructions.ts:150`
- `internal/tmpl/templates/skills/goal-verification/SKILL.md.tmpl`
- `internal/tmpl/templates/skills/worktree-preflight/SKILL.md`
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`
- `internal/engine/progression/readiness.go`
- `internal/engine/progression/readiness_optimization_test.go`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/CONCERNS.md`
- `artifacts/codebase/TESTING.md`
