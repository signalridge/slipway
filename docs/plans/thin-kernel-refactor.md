# Slipway Thin-Kernel Refactor Plan

## Context

Slipway has grown from a thin governance CLI into a platform that owns runtime
authority over skill routing, capability resolution, toolgen mirroring, and
execution orchestration. The core diagnosis from both independent analyses:

> **The problem is not that governance exists, but that governance has been
> productized into the runtime kernel's main narrative.** Slipway is deciding
> HOW the AI works instead of telling it WHAT comes next.

This plan adopts high-confidence items backed by code evidence and defers
architectural items that need further design.

---

## Wave 1: Kernel Mechanics (high confidence, low risk)

### 1.1 Consolidate dual auto-pass invocation

**Problem**: `advance_governed.go:105-110` and `:174-180` call
`attemptAutoPassSequence` twice with different `startState` args.

**Code-verified finding**: The first call (fromState, fromState) acts only when
already at S3_REVIEW/S4_VERIFY. The second call (fromState, toState) acts when
transitioning into those states. They are not identical — but the first call
bypasses skill evidence evaluation and gate checks (G_scope/G_plan/G_ship),
creating an inconsistent code path.

**Action**: Remove the first invocation (line 105-110). Keep only the post-
`ComputeNextGovernedState` call. For re-entry at S3/S4, the existing flow
already checks prerequisites then computes toState = same state, and the
second auto-pass fires. Net effect: same behavior, one code path, skill
evidence and gates always evaluated.

**Files**:
- `internal/engine/progression/advance_governed.go` — delete lines 105-110
- `internal/engine/progression/advance_test.go` — verify existing tests pass;
  add test for re-entry auto-pass at S3_REVIEW

### 1.2 Flatten S1_PLAN substep state machine

**Problem**: 4-step linear progression (research -> bundle -> audit -> validate)
encoded as a mini state machine with `computeNextPlanSubStep()` switch +
50 lines of orchestration in `advance_governed.go:237-286`.

**Action**: Replace `computeNextPlanSubStep()` with a simple ordered slice
lookup. Inline the side-effect logic (bundle scaffolding, audit-validate
recovery) as sequential checks rather than state-dispatch branches.

**Files**:
- `internal/engine/progression/advance_governed.go` lines 237-352
- Keep `PlanSubStep` type in model (needed for persistence), but simplify
  the transition logic

---

## Wave 2: Capability System Demotion (high confidence, medium effort)

### 2.1 Demote capability catalog from runtime authority to static metadata

**Problem**: `internal/engine/capability/` (2,272 lines across 10 files) owns
a parallel routing system (registry + resolver + trigger DSL + surfaces) whose
output is advisory — `registry.go:14-17` explicitly says *"the kernel's host
is free to consume or ignore"*. But the infrastructure cost is that of a
binding authority.

**Current flow**:
```
capability.DefaultRegistry() -> capability.Resolve(reg, signals)
  -> Supports[]  -> appended as TechniqueHints in next_skill_view.go:149
  -> Routes[]    -> consumed by route_flags.go for --focus/--view/--list-*
  -> Suggested[] -> surfaced in ai_skill_hint.go:71
```

**Action — Phase A (this wave)**: Freeze the capability catalog output as
static data. Replace runtime resolution with a precomputed lookup table keyed
by (host_skill, command). Delete the trigger DSL and scoring resolver.

Concretely:
1. Replace `resolver.go` (494 lines) + `trigger.go` (319 lines) with a
   flat `map[string][]Attachment` built at init time from the registry
2. Keep `surfaces.go` as-is (it's already a flat registry)
3. Keep `registry*.go` definitions but remove `Trigger` fields

**Action — Phase B (Wave 3)**: Remove `--focus`/`--view`/`--list-*` flags
from review/validate/repair/status commands. The AI tool should choose
capabilities via its own skill routing, not Slipway CLI flags.

**Files**:
- `internal/engine/capability/resolver.go` — replace with flat lookup
- `internal/engine/capability/trigger.go` — delete
- `internal/engine/capability/registry*.go` — remove Trigger fields
- `cmd/route_flags.go` — Phase B: simplify to remove focus/view/list
- `cmd/ai_skill_hint.go` — simplify to list available skills, drop
  resolution scoring
- Tests in `capability/*_test.go` — update

### 2.2 Remove ai_skill_hint_prompt from command output

**Problem**: `buildAISkillHintPrompt()` generates a freeform natural-language
prompt string embedded in JSON output. This is the runtime telling the AI how
to think — the opposite of what we want.

**Action**: Remove `ai_skill_hint_prompt` field from review/validate/repair
JSON output. The structured fields (`next_skill`, `blockers`,
`suggested_capabilities` as a simple list) already provide everything needed.
AI tools consume structure, not embedded prompts.

**Files**:
- `cmd/ai_skill_hint.go` — delete `buildAISkillHintPrompt()`
- `cmd/review.go`, `cmd/validate.go`, `cmd/repair.go` — remove
  `AISkillHintPrompt` field from JSON views

---

## Wave 3: Toolgen Simplification (high confidence, high effort)

### 3.1 Replace mirrored skill tree with thin bootstrap + JSON protocol

**Problem**: `toolgen.go:475` (`generateForTool`) generates for each of 5
tools: adapter skills, governance skills, catalog skills, standalone skills,
technique skills, commands, agents, global prompts, hooks, settings, manifest.
This produces ~53 SKILL.md files + 11 agents + commands + hooks PER TOOL.
Slipway must track each tool's product-form changes indefinitely.

**Target model** (inspired by spec-kitty):
```
slipway init --tools claude
  -> .claude/commands/slipway/next.md     (thin bootstrap: calls slipway next --json, reads prompt_path)
  -> .claude/commands/slipway/new.md      (thin bootstrap)
  -> .claude/commands/slipway/status.md   (thin bootstrap)
  -> .claude/commands/slipway/run.md      (thin bootstrap)
  -> .claude/commands/slipway/done.md     (thin bootstrap)
  -> .claude/agents/slipway-orchestrator.md  (agent definitions stay)
  -> .claude/agents/slipway-*.md             (keep governance-mapped agents)
  -> .slipway/skills/*/SKILL.md              (ONE copy, tool-agnostic, in project root)
```

**Key change**: Skills live in `.slipway/skills/` (ONE copy), not mirrored
per-tool. The bootstrap command reads `prompt_path` from `slipway next --json`
and loads the skill from the shared location. Agent definitions stay per-tool
(since formats differ: .md vs .toml).

**Action**:
1. Change `generateForTool` to emit only: bootstrap commands + agent defs
2. Skills go to `.slipway/skills/` once, shared across tools
3. Remove adapter/catalog/standalone/technique skill generation per tool
4. Keep `catalogManifestFileName` as a single shared file

**Files**:
- `internal/toolgen/toolgen.go` — major simplification of `generateForTool`
- `internal/toolgen/toolgen_test.go` — update
- `internal/tmpl/` — skill templates remain, output location changes
- Add new `SkillsDir` resolution: `.slipway/skills/` as canonical, tool
  configs point `prompt_path` there

---

## Wave 4: Default Gate Severity (medium-high confidence)

### 4.1 Lower default control activation for non-guardrail changes

**Problem**: `DeriveControls` (`control/derive.go:36`) activates
`independent_review` and `worktree_isolation` at medium+ blast radius OR
any domain. For AI-driven local development, worktree isolation and
independent review are friction for non-sensitive changes.

**Action**: Change default thresholds:
- `independent_review`: trigger only at `high` blast radius (was `medium`)
  when no guardrail domain
- `worktree_isolation`: trigger only at `high` blast radius (was `medium`)
  when no guardrail domain
- Guardrail domain changes: keep current behavior (fail-closed)

**Files**:
- `internal/engine/control/derive.go` — adjust threshold logic
- `internal/engine/control/derive_test.go` — update expectations

### 4.2 Add `--quick` mode to `slipway next`

**Problem**: No way to run a fast path that skips advisory controls. GSD
has `mode: yolo`; Slipway has no equivalent.

**Action**: Add `--quick` flag to `slipway next` and `slipway run`. When
set, automatically passes `disabled_controls: [clarification, research,
independent_review, worktree_isolation]` — keeps only fail-closed guardrail
protections (domain_review, rollback_required for sensitive domains).

**Files**:
- `cmd/next.go` — add `--quick` flag, translate to `AdvanceOptions`
- `cmd/run.go` — same
- `internal/engine/progression/advance_governed.go` — respect quick-mode
  disabled controls

---

## Wave 5: Command Surface Reduction (medium confidence)

### 5.1 Merge or hide secondary commands

**Current**: 19 top-level commands (root.go:138-156).

**Core commands** (keep as-is):
- `init`, `new`, `next`, `run`, `status`, `done`

**Merge candidates**:
- `preset` -> fold into `new` (preset is only meaningful at creation time)
- `validate-requirements` -> fold into `validate`
- `health` -> fold into `status --health`
- `stats` -> fold into `status --stats`
- `root-path` -> fold into `status --root`
- `codebase-map` -> keep but move to `slipway map` (shorter)

**Keep as separate** (distinct lifecycle actions):
- `review`, `validate`, `repair`, `pivot`, `abort`, `cancel`, `checkpoint`

**Action**: Phase this across multiple PRs. Start with `health`/`stats`/
`root-path` -> `status` subcommands.

---

## Deferred (needs design discussion)

These items are real concerns but require architectural decisions beyond
mechanical refactoring:

1. **Event-sourced state** — replacing `change.yaml` mutation with JSONL
   event log. High value for crash recovery and audit, but touches every
   state read/write site. Needs separate design spike.

2. **Orchestration handoff** — making wave-orchestration a pure agent
   workflow pattern instead of kernel-owned execution. Requires rethinking
   how `slipway run` drives the loop vs the AI tool driving it.

3. **ResolveNextSkill as advisory** — currently hard-routes state->skill.
   Making it advisory ("here are the skills available at this state, you
   choose") is philosophically right but breaks the evidence-gate contract.
   Needs careful design.

---

## Verification

After each wave:
```bash
go build ./...
go test ./...
```

Wave-specific checks:
- Wave 1: `go test ./internal/engine/progression/... -run AutoPass -v`
- Wave 2: `go test ./internal/engine/capability/... -v`
- Wave 3: `slipway init --tools claude` in a test repo, verify output
- Wave 4: `go test ./internal/engine/control/... -v`
- Wave 5: `slipway --help` shows reduced command list

---

## Execution Order

```
Wave 1 (kernel mechanics)     ~1-2 PRs, low risk
  |
Wave 2 (capability demotion)  ~2 PRs, medium risk
  |
Wave 3 (toolgen simplify)     ~1 large PR, high effort
  |
Wave 4 (gate severity)        ~1-2 PRs, low risk
  |
Wave 5 (command surface)      ~2-3 PRs, low risk, can parallelize
```

Waves 1 and 4 are independent and can run in parallel.
Wave 2 should precede Wave 3 (removing surface resolver before
simplifying toolgen avoids generating catalog skills).
