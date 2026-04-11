# Skills Integration Plan — Ambitious Build (2026-04-11, v5)

> Supersedes v3 (`2026-04-11-skills-integration-plan.md`), the v3-critique (`...v3-critique.md`), and v4 (`...v4-merged.md`). v3 was right to reject v2's hallucinations; v4 was right to ground every claim in repo facts. v5 keeps both wins and stops being timid: the engine, the registry contract, the CLI, the chezmoi pool, the slipway template surface — all expand together in one coherent rev.
>
> **Status:** Draft, ready for owner sign-off on §13.
> **Audited on:** 2026-04-11, against the local machine.
> **Target shape:** **19 gates + 12 references + 4 techniques** registered through `internal/engine/skill/skill.go`, all surfaced via an extended `next` CLI, with the existing `defaultGovernanceRegistry` shape extended (not replaced).

---

## 1. Executive Summary

v4 over-corrected. Half the things v4 deferred ("no new gate, no frontmatter expansion, no new CLI surface") are not fundamental constraints — they are self-imposed rules that exist because the test suite was written that way. The test suite is yours; the contract is yours. The right time to break the minimal-frontmatter contract and add gate ordering is **now**, in one coordinated rev, not across four future plans.

v5 ships in four waves. Each wave is a self-contained PR but the four together compose into one coherent contract change:

- **Wave 1 — Engine + schema rev.** `Definition` grows 6 new fields (`Phase`, `Subject`, `Tier`, `References`, `OrderAfter`, conditional flags). `governanceFrontMatter` parser extended. Topo-sort added for same-state gate ordering. New `defaultReferenceRegistry` map. New `slipway next --references` CLI flag. `chezmoi` pulls 5 missing upstream sources. Test contract migrated to the new shape. Zero new skills. Zero behavior change for the existing 9 gates.
- **Wave 2 — Existing-skill enhancements + `checklist-quality` decomposition.** The 6 enhancement merges from v4 §7.2 land (intake / code-review-protocol / codebase-mapping / spec-compliance-review / wave-orchestration / tdd-governance). The 7-line `checklist-quality.md` sidecar is decomposed into 4 domain checklists (`intake`, `plan`, `review`, `test`). All enhanced skills gain the new frontmatter fields.
- **Wave 3 — New gates (10).** 10 new governance gates land in the registry, taking total gate count from 9 to 19. Each is sourced from a verified upstream skill (full citations in §6). Each has slipway-flavored frontmatter, a reason code, a hard-gate marker, and conditional firing rules where applicable. End-to-end smoke test on a scratch project.
- **Wave 4 — Reference shelf (12).** 12 slipway-flavored reference skills land in `defaultReferenceRegistry`. Each is ~150–200 LoC, sourced from the upstream content already verified in v4 §4.1. `next --references` surfaces them filtered by phase/subject. The reference shelf is the "what else should I read" layer that v4 wanted to do via doc cross-links — but as a structured registry instead.

End state: slipway's skill surface goes from **9 gates + 4 techniques** to **19 gates + 4 techniques + 12 references** = 35 registered skills. The registry contract supports phase × subject filtering, gate ordering, conditional firing, and reference surfacing — all in one place.

---

## 2. Facts That Grounded This Plan

These are the verified ground truths from the v4 audit. v5 inherits all of them; nothing here changes between v4 and v5.

### 2.1 Current `slipway` runtime routing contract

`slipway` routes governance skills through these `Definition` fields (`internal/engine/skill/skill.go:10-20`):

```go
type Definition struct {
    Name                string
    State               model.WorkflowState
    PlanSubStep         model.PlanSubStep
    Mitigation          string
    RunSummaryBound     bool
    DiscoveryOnly       bool
    GuardrailRequired   bool
    CloseoutConditional bool
    AgentHint           string
}
```

All 9 existing entries set `AgentHint`. v5 keeps this convention for every new gate.

### 2.2 Current registry size: 9 gates

| State | Skill | Notes |
|---|---|---|
| `S0_INTAKE` | `intake-clarification` | `AgentHint: slipway-planner` |
| `S1_PLAN` | `research-orchestration` | `PlanSubStep: research`, `DiscoveryOnly: true`, `AgentHint: slipway-researcher` |
| `S1_PLAN` | `plan-audit` | `PlanSubStep: audit`, `AgentHint: slipway-auditor` |
| `S2_EXECUTE` | `wave-orchestration` | `RunSummaryBound: true`, `AgentHint: slipway-orchestrator` |
| `S2_EXECUTE` | `tdd-governance` | `GuardrailRequired: true`, `RunSummaryBound: true`, `AgentHint: slipway-orchestrator` |
| `S3_REVIEW` | `spec-compliance-review` | `RunSummaryBound: true`, `AgentHint: slipway-reviewer` |
| `S3_REVIEW` | `code-quality-review` | `RunSummaryBound: true`, `AgentHint: slipway-reviewer` |
| `S4_VERIFY` | `goal-verification` | `RunSummaryBound: true`, `AgentHint: slipway-verifier` |
| `S4_VERIFY` | `final-closeout` | `CloseoutConditional: true`, `RunSummaryBound: true`, `AgentHint: slipway-closer` |

### 2.3 Current technique-only skills (not registered as gates)

`tdd`, `systematic-debugging`, `code-review-protocol`, `codebase-mapping`. These stay technique skills in v5 — they are invoked *within* gates, not as state checkpoints. v5 does not promote them.

### 2.4 Template file shapes

- Static `SKILL.md`: most skills.
- Rendered from `SKILL.md.tmpl`: `spec-compliance-review`, `wave-orchestration`, `tdd-governance`. Edits go in the `.tmpl` source; Wave-2 structural assertions render first.

### 2.5 `~/.agents/skills` is already populated with 167 SKILL.md files

`~/.local/share/chezmoi/.chezmoiexternal.toml.tmpl` pulls from `wshobson/agents`, `anthropics/skills`, `getsentry/skills`, `openai/skills`, `trailofbits/skills`, `huggingface/skills`, `cloudflare`, `vercel-labs`, `supabase`, `expo`, `microsoft`, `signalridge`, multilingual humanizers, `nextlevelbuilder/ui-ux-pro-max-skill`, `sickn33/antigravity-awesome-skills`. v5 adds 5 more sources in Wave 1 (§9).

### 2.6 v2's misclassifications, all confirmed

The full evidence table from v4 §2.5 stands without changes. Highlights v5 still depends on:

- `agentic-actions-auditor` is **GitHub-Actions-AI-CI only** — not a generic diff scanner. v5 does not turn it into a gate; it appears only as a reference for users reviewing `.github/workflows/*.yml` (`tier: reference`, `subject: safety`, `phase: review`).
- `trailofbits/spec-to-code-compliance` is blockchain-specific. v5's `spec-compliance-review` gate borrows only the evidence-citation rule, never the full process.
- `trailofbits/testing-handbook-skills` is 15 fuzzing-tool skills, not a methodology bundle. v5 does not register a `testing-strategies` gate.
- Slipway's `spec-compliance-review/SKILL.md.tmpl` is **stricter** than upstream. v5 §6.2.2 does not weaken it.
- `worktree-preflight, checkpoint, done, pivot, repair, cancel, abort` are CLI commands, **not registered skills**. v5 keeps them as commands.
- `checklist-quality.md` is a 7-line sidecar, not a skill. v5 §7 decomposes it into 4 domain checklists, still as sidecars (still not registered as skills).

Full file:line citations in §14.

---

## 3. Target Skill Surface

End state of v5 = **35 registered skills** (19 gates + 4 techniques + 12 references) under `internal/tmpl/templates/skills/` and `internal/engine/skill/skill.go`.

### 3.1 Gates by state (post-v5)

| State | Gate | Status | Source |
|---|---|---|---|
| `S0_INTAKE` | `intake-clarification` | existing (enhanced in Wave 2) | + `trailofbits/ask-questions-if-underspecified` |
| `S1_PLAN` | `research-orchestration` | existing | — |
| `S1_PLAN` | `plan-audit` | existing | — |
| `S1_PLAN` | `threat-model-screen` | **NEW Wave 3** | `openai/.curated/security-threat-model` (already installed) |
| `S1_PLAN` | `adr-discipline` | **NEW Wave 3** | `wshobson/.../architecture-decision-records` (already installed) |
| `S2_EXECUTE` | `wave-orchestration` | existing (enhanced in Wave 2) | + `wshobson/.../workflow-patterns` + `wshobson/.../task-coordination-strategies` |
| `S2_EXECUTE` | `tdd-governance` | existing (enhanced in Wave 2) | + `wshobson/.../workflow-patterns` (wave-aware TDD) |
| `S2_EXECUTE` | `code-simplification-review` | **NEW Wave 3** | `getsentry/.../code-simplifier` (already installed) |
| `S2_EXECUTE` | `error-handling-discipline` | **NEW Wave 3** | `wshobson/.../error-handling-patterns` (already installed) |
| `S3_REVIEW` | `spec-compliance-review` | existing (enhanced in Wave 2) | narrow borrow from `trailofbits/spec-to-code-compliance` |
| `S3_REVIEW` | `code-quality-review` | existing (enhanced in Wave 2) | + `developer-essentials/code-review-excellence` (already installed) |
| `S3_REVIEW` | `audit-context-readiness` | **NEW Wave 3** | `trailofbits/audit-context-building` (already installed) |
| `S3_REVIEW` | `differential-risk-review` | **NEW Wave 3** | `trailofbits/differential-review` (already installed) |
| `S3_REVIEW` | `security-review` | **NEW Wave 3** | `getsentry/security-review` + `openai/security-best-practices` (both already installed) |
| `S4_VERIFY` | `goal-verification` | existing | — |
| `S4_VERIFY` | `final-closeout` | existing | — |
| `S4_VERIFY` | `changelog-emission` | **NEW Wave 3** | `wshobson/.../changelog-automation` (already installed) |
| `S4_VERIFY` | `postmortem-readiness` | **NEW Wave 3** | `wshobson/.../postmortem-writing` (already installed) |

**19 gates total.** 9 existing + 10 new.

### 3.2 References (post-v5)

12 slipway-flavored reference skills, each ~150–200 LoC, registered in the new `defaultReferenceRegistry`. Each is `tier: reference`, `hard_gate: false`, surfaced via `next --references`.

| # | Slipway target | Phase | Subject | Source |
|---|---|---|---|---|
| 1 | `audit-context-building` | review | safety | `~/.agents/skills/ecosystem/trailofbits/audit-context-building/SKILL.md` |
| 2 | `differential-review-methodology` | review | safety | `~/.agents/skills/ecosystem/trailofbits/differential-review/SKILL.md` |
| 3 | `property-based-testing` | execute | correctness | `~/.agents/skills/ecosystem/trailofbits/property-based-testing/SKILL.md` |
| 4 | `e2e-testing-patterns` | execute | correctness | `~/.agents/skills/developer-essentials/e2e-testing-patterns/SKILL.md` |
| 5 | `error-handling-patterns` | execute | refactor | `~/.agents/skills/developer-essentials/error-handling-patterns/SKILL.md` |
| 6 | `code-simplification` | execute | refactor | `~/.agents/skills/ecosystem/getsentry/code-simplifier/SKILL.md` |
| 7 | `architecture-decision-records` | plan | architecture | `~/.agents/skills/documentation-generation/architecture-decision-records/SKILL.md` |
| 8 | `threat-modeling` | plan | safety | `~/.agents/skills/ecosystem/openai/security-threat-model/SKILL.md` |
| 9 | `incident-runbooks` | verify | process | `wshobson/.../incident-runbook-templates` (Wave-1 chezmoi pull) |
| 10 | `postmortem-writing` | verify | process | `~/.agents/skills/.../postmortem-writing/SKILL.md` |
| 11 | `changelog-authoring` | verify | process | `~/.agents/skills/documentation-generation/changelog-automation/SKILL.md` |
| 12 | `debugging-strategies` | execute | debug | `~/.agents/skills/developer-essentials/debugging-strategies/SKILL.md` |

### 3.3 Techniques (unchanged)

`tdd`, `systematic-debugging`, `code-review-protocol`, `codebase-mapping`. Stay as technique-only `.md` files in `internal/tmpl/templates/skills/`. Not in either registry.

---

## 4. Engine + Schema Changes (Wave 1)

### 4.1 New `Definition` fields

Extend `internal/engine/skill/skill.go:10-20`:

```go
type Definition struct {
    // existing
    Name                string              `json:"name"`
    State               model.WorkflowState `json:"state"`
    PlanSubStep         model.PlanSubStep   `json:"plan_substep,omitempty"`
    Mitigation          string              `json:"mitigation"`
    RunSummaryBound     bool                `json:"run_summary_bound"`
    DiscoveryOnly       bool                `json:"discovery_only,omitempty"`
    GuardrailRequired   bool                `json:"guardrail_required,omitempty"`
    CloseoutConditional bool                `json:"closeout_conditional,omitempty"`
    AgentHint           string              `json:"agent_hint,omitempty"`

    // new in v5
    Phase           Phase    `json:"phase,omitempty"`            // intake|plan|execute|review|verify|meta
    Subject         Subject  `json:"subject,omitempty"`          // correctness|safety|architecture|refactor|process|debug|authoring
    Tier            Tier     `json:"tier"`                       // gate|reference|technique
    References      []string `json:"references,omitempty"`       // pool paths (e.g. "shared:ecosystem/trailofbits/differential-review")
    OrderAfter      string   `json:"order_after,omitempty"`      // gate ordering within same state
    ReasonCode      string   `json:"reason_code,omitempty"`      // key in internal/model/reason_code.go
    HardGate        bool     `json:"hard_gate,omitempty"`        // requires explicit user approval to advance
    SubjectGated    bool     `json:"subject_gated,omitempty"`    // fires only when change.Subject matches this.Subject
    PivotConditional      bool `json:"pivot_conditional,omitempty"`       // fires only when execution log has pivot/repair/abort
    UserFacingConditional bool `json:"user_facing_conditional,omitempty"` // fires only when changed files include user-facing surface
    ErrorPathConditional  bool `json:"error_path_conditional,omitempty"`  // fires only when changed files touch error/exception paths
}
```

Three companion enums (`Phase`, `Subject`, `Tier`) with constants and a validator. Validator runs at registry load and rejects invalid combinations (e.g. `Tier: reference` + `HardGate: true`).

### 4.2 New `governanceFrontMatter` fields

Extend `internal/engine/skill/registry_loader.go:40-44` to parse `phase`, `subject`, `tier`, `references`, `order_after`, `reason_code`, `hard_gate`, `subject_gated`, `pivot_conditional`, `user_facing_conditional`, `error_path_conditional`. Update `parseGovernanceSkillFromFile()` (lines 168–199) to populate the new `Definition` fields after the name lookup.

For backward compatibility: a frontmatter without any new fields is still valid; the loader sets `Tier: gate` by default if the skill is in `defaultGovernanceRegistry`, `Tier: reference` if in `defaultReferenceRegistry`, `Tier: technique` otherwise.

### 4.3 New `defaultReferenceRegistry`

Add to `internal/engine/skill/skill.go`:

```go
var defaultReferenceRegistry = map[string]Definition{
    "audit-context-building": {
        Name:       "audit-context-building",
        Tier:       TierReference,
        Phase:      PhaseReview,
        Subject:    SubjectSafety,
        References: []string{"shared:ecosystem/trailofbits/audit-context-building"},
    },
    // ... 11 more from §3.2 ...
}
```

Reference skills do not have `State`. They are not iterated by the progression engine; they are surfaced only via `next --references` (§4.6) and via `References` cross-links inside other skills.

### 4.4 Topo-sort for `OrderAfter`

`RequiredSkillsForStateWithRegistry` (`skill.go:115-158`) currently iterates the registry by state with no ordering guarantees. Replace the iteration with:

1. Filter by state into a slice of `Definition`.
2. Build a DAG using `OrderAfter` edges.
3. Topological sort. Cycles or unknown dependencies fail loud at registry-load time.
4. Return the sorted slice.

This unblocks gates like `differential-risk-review` that must run *after* `code-quality-review` in S3.

### 4.5 Conditional firing logic

Add to `internal/engine/progression/skill_resolution.go`:

```go
type ChangeContext struct {
    Subject        Subject       // tagged at intake
    GuardrailDomain bool         // existing
    ExecutionLog   []ExecutionEvent  // pivot, repair, abort
    TouchedFiles   []string      // for user-facing / error-path detection
}

func (d Definition) ShouldFire(ctx ChangeContext) bool {
    if d.SubjectGated && d.Subject != ctx.Subject {
        return false
    }
    if d.PivotConditional && !hasPivot(ctx.ExecutionLog) {
        return false
    }
    if d.UserFacingConditional && !touchesUserFacing(ctx.TouchedFiles) {
        return false
    }
    if d.ErrorPathConditional && !touchesErrorPath(ctx.TouchedFiles) {
        return false
    }
    return true
}
```

Detection helpers (`hasPivot`, `touchesUserFacing`, `touchesErrorPath`) are simple file-glob matchers — `touchesUserFacing` checks for changes under `cmd/`, `internal/cli/`, `web/`, `frontend/`; `touchesErrorPath` checks for files containing `errors.go`, `*_error.*`, `recovery.*`, etc. The patterns are configurable in `internal/model/config.go`.

`ChangeContext` is built by the existing change resolver, not by each gate. Gates only declare what they care about via the conditional flags.

### 4.6 New CLI flag: `slipway next --references`

Extend `cmd/next.go` to support `--references` (also `--refs` short). When set:

- `slipway next` prints the normal gate output.
- After gate output, prints a `## References` section listing all `defaultReferenceRegistry` entries whose `Phase` matches the current state and whose `Subject` matches the change subject (if known) or any subject (if unknown).
- Each line: `slipway-<name>: <one-line description> → <pool path>`.
- JSON output (`--json`) gains a `references` array.

The flag is opt-in; `next` default behavior is unchanged.

### 4.7 Test contract migration

`internal/tmpl/templates_test.go` currently asserts on minimal frontmatter. Wave 1 migrates the assertions to:

- Required fields: `name`, `description`, `tier`.
- For `tier: gate`: also require `phase`, `subject`, `reason_code`.
- For `tier: reference`: also require `phase`, `subject`, `references` (non-empty).
- For `tier: technique`: only require `name` and `description`.
- Topo-sort assertion: `TestGateOrderingNoCycles` walks `defaultGovernanceRegistry` and confirms `OrderAfter` forms a DAG.
- Reference assertion: `TestReferenceTargetsExist` walks `defaultReferenceRegistry` and confirms every `References` entry resolves to a real file in `~/.agents/skills/` (or skips with a soft warning if running in CI without the chezmoi pool).

This is the contract break v3/v4 wanted to avoid. v5 does it once, then locks the new shape.

---

## 5. Existing Skill Enhancements (Wave 2)

The 6 enhancements from v4 §7.2, restated here. Each lands in Wave 2 alongside the new frontmatter fields from Wave 1.

### 5.1 `intake-clarification` ← `trailofbits/ask-questions-if-underspecified`

Borrow: 5-category question taxonomy (scope / acceptance / constraint / risk / stakeholder), explicit stop condition ("restate requirements in 1–3 sentences + key constraints"). Add as `## Question Taxonomy` and `## Stop Condition` subsections under existing "Clarification Loop" step. Do not touch existing Rationalization Red Flags or Scope Boundary Precision Rules.

New frontmatter:

```yaml
---
name: slipway-intake-clarification
description: "Verify scope, acceptance criteria, and constraints before planning"
tier: gate
phase: intake
subject: correctness
reason_code: intake_clarification_required
hard_gate: true
---
```

### 5.2 `code-quality-review` (was `code-review-protocol` in v4) ← `wshobson/multi-reviewer-patterns` + `developer-essentials/code-review-excellence`

Borrow `multi-reviewer-patterns`: review-dimension allocation table (Security / Performance / Architecture / Testing / Accessibility), finding-deduplication protocol. Borrow `code-review-excellence`: "Goals vs Not Goals" anti-list, constructive-feedback rubric. Add as `## Reviewer Role Splits` and `## Feedback Discipline` sections after the existing iron-law block.

Note: `code-review-protocol` stays as a technique skill (cited from `code-quality-review`'s body). The `code-quality-review` gate is the registered enforcer.

### 5.3 `wave-orchestration` ← `wshobson/workflow-patterns` + `wshobson/task-coordination-strategies`

Borrow workflow-patterns: 11-step TDD lifecycle (lifecycle list only), phase-completion protocol, quality-gates checkpoint structure. Borrow task-coordination-strategies: Dependency Graph Principles (parallel-safe vs sequential-required criteria). Add as `## Coordination Strategies` and `## Lifecycle Checkpoints` sections.

`SKILL.md.tmpl` source — edits go in the template, render in a follow-up.

### 5.4 `tdd-governance` ← `wshobson/workflow-patterns` (continuation)

Add `## Wave-Aware TDD` section: when to split tests across waves, when to fail-fast. Cross-link to `wave-orchestration`. `SKILL.md.tmpl` source.

### 5.5 `codebase-mapping` ← `trailofbits/entry-point-analyzer` (concept-only borrow)

Borrow the 5-category entry-point classification (CLI, HTTP, scheduled jobs, message-queue consumers, event handlers), rewritten language-agnostic. Add as `## Entry-Point Discovery` subsection. `codebase-mapping` stays a technique skill.

### 5.6 `spec-compliance-review` ← `trailofbits/spec-to-code-compliance` (narrow borrow)

Borrow only: Phase 0 evidence-citation rule (every claim must cite `file:line`) and exhaustiveness-of-changed-files phase (read every changed file, not just diff hunks). Add as `## Evidence Citation` subsection inside the existing "Independent Verification Mandate" block. Do not remove or weaken the existing Iron Law, Mandatory Checklist, Review Layers, or Rationalization Red Flags. `SKILL.md.tmpl` source.

### 5.7 `checklist-quality` decomposition

Replace the single 7-line `internal/tmpl/templates/skills/checklist-quality.md` with a directory:

```
internal/tmpl/templates/skills/checklist-quality/
├── intake.md   — clarification completeness checklist (5-question taxonomy)
├── plan.md     — plan-audit checklist
├── review.md   — review/audit checklist (current content goes here, expanded)
└── test.md     — test sufficiency checklist
```

Each is still a sidecar (no frontmatter, no registry entry). Skills reference specific checklists via the new `references:` frontmatter field. Wave 2 updates `spec-compliance-review/SKILL.md.tmpl:35` to reference `checklist-quality/review.md` specifically instead of the whole file.

---

## 6. New Gate Specs (Wave 3)

Each new gate gets its own subsection: source citation, slipway frontmatter, body skeleton, reason code, ordering, conditional logic.

### 6.1 `S1_PLAN` additions

#### 6.1.1 `threat-model-screen`

**Source:** `~/.agents/skills/ecosystem/openai/security-threat-model/SKILL.md` (already installed; 82 LoC; STRIDE-grade workflow).

**Frontmatter:**
```yaml
---
name: slipway-threat-model-screen
description: "STRIDE-lite threat model screen for safety-subject changes during planning"
tier: gate
phase: plan
subject: safety
reason_code: threat_model_screen_required
hard_gate: true
subject_gated: true       # only fires when change.Subject == "safety"
order_after: plan-audit
references:
  - "shared:ecosystem/openai/security-threat-model"
agent_hint: slipway-auditor
---
```

**Body skeleton:** 6-section STRIDE walk (boundaries / assets / entry points / abuse paths / mitigations / open questions). Verification YAML written to `artifacts/changes/{slug}/verification/threat-model-screen.yaml`.

**Reason code:** `threat_model_screen:safety_subject_unscreened` in `internal/model/reason_code.go`.

**Ordering:** runs after `plan-audit` in the same state when `subject == safety`.

#### 6.1.2 `adr-discipline`

**Source:** `~/.agents/skills/documentation-generation/architecture-decision-records/SKILL.md` (already installed; 441 LoC).

**Frontmatter:**
```yaml
---
name: slipway-adr-discipline
description: "Material design changes require an ADR"
tier: gate
phase: plan
subject: architecture
reason_code: adr_discipline_required
hard_gate: false
order_after: plan-audit
references:
  - "shared:documentation-generation/architecture-decision-records"
agent_hint: slipway-auditor
---
```

**Conditional firing:** Wave-1 detection helper `touchesArchitecture()` matches changes under `internal/engine/`, `internal/model/`, `cmd/root.go`, anything renaming exported types. Gate fires only when `touchesArchitecture(ctx.TouchedFiles)`. Implemented as a new flag `MaterialDesignConditional bool` (added to §4.1's struct in v5 final).

**Body skeleton:** read existing ADRs in `docs/decisions/`, decide if a new one is needed, write or skip with explicit justification.

### 6.2 `S2_EXECUTE` additions

#### 6.2.1 `code-simplification-review`

**Source:** `~/.agents/skills/ecosystem/getsentry/code-simplifier/SKILL.md` (already installed; 119 LoC).

**Frontmatter:**
```yaml
---
name: slipway-code-simplification-review
description: "Apply simplification heuristics before TDD lockdown"
tier: gate
phase: execute
subject: refactor
reason_code: code_simplification_review_required
hard_gate: false
order_after: wave-orchestration
references:
  - "shared:ecosystem/getsentry/code-simplifier"
  - "slipway:reference:code-simplification"
agent_hint: slipway-orchestrator
---
```

**Body skeleton:** language-agnostic checklist (reduce branches, flatten nesting, name things, delete dead code), executed against the just-completed wave's diff. Output: a "simplifications applied" list or an explicit "none needed" justification.

#### 6.2.2 `error-handling-discipline`

**Source:** `~/.agents/skills/developer-essentials/error-handling-patterns/SKILL.md` (already installed; 632 LoC — large; v5 borrows the decision matrix, not the full body).

**Frontmatter:**
```yaml
---
name: slipway-error-handling-discipline
description: "Touched error paths must declare fail-fast vs fallback vs retry vs circuit-break"
tier: gate
phase: execute
subject: refactor
reason_code: error_handling_discipline_required
hard_gate: true
guardrail_required: true
error_path_conditional: true
order_after: tdd-governance
references:
  - "shared:developer-essentials/error-handling-patterns"
agent_hint: slipway-orchestrator
---
```

**Conditional firing:** `error_path_conditional: true` — fires only when changed files match error-path patterns (configurable in `internal/model/config.go`; default patterns: `*errors*.go`, `*recover*.go`, `*panic*.go`, files containing `defer recover()`, etc.).

### 6.3 `S3_REVIEW` additions

#### 6.3.1 `audit-context-readiness`

**Source:** `~/.agents/skills/ecosystem/trailofbits/audit-context-building/SKILL.md` (already installed; ~200 LoC).

**Frontmatter:**
```yaml
---
name: slipway-audit-context-readiness
description: "Build line-by-line architectural context before risky review"
tier: gate
phase: review
subject: safety
reason_code: audit_context_readiness_required
hard_gate: true
subject_gated: true
order_after: ""           # runs first in S3 when fired
references:
  - "shared:ecosystem/trailofbits/audit-context-building"
agent_hint: slipway-reviewer
---
```

**Conditional firing:** `subject_gated: true` — fires only when `subject == safety`. Runs *before* `spec-compliance-review` when fired (no `order_after`, but the topo-sort puts unbound gates first by registration order).

#### 6.3.2 `differential-risk-review`

**Source:** `~/.agents/skills/ecosystem/trailofbits/differential-review/SKILL.md` (already installed).

**Frontmatter:**
```yaml
---
name: slipway-differential-risk-review
description: "Risk-aware diff review with blast-radius and rationalizations"
tier: gate
phase: review
subject: correctness
reason_code: differential_risk_review_required
hard_gate: true
order_after: code-quality-review
references:
  - "shared:ecosystem/trailofbits/differential-review"
agent_hint: slipway-reviewer
---
```

**Body skeleton:** 6-phase walk adapted from upstream — Triage → Blast Radius → Test Coverage → Risk Classification → Adversarial → Report. Severity stance: Important on first deployment, escalate to Critical after 1-month observation.

#### 6.3.3 `security-review`

**Source:** `~/.agents/skills/ecosystem/getsentry/security-review/SKILL.md` + `~/.agents/skills/ecosystem/openai/security-best-practices/SKILL.md` (both already installed).

**Frontmatter:**
```yaml
---
name: slipway-security-review
description: "Confidence-based security findings for safety-subject changes"
tier: gate
phase: review
subject: safety
reason_code: security_review_required
hard_gate: true
subject_gated: true
order_after: differential-risk-review
references:
  - "shared:ecosystem/getsentry/security-review"
  - "shared:ecosystem/openai/security-best-practices"
agent_hint: slipway-reviewer
---
```

**Body skeleton:** OWASP-grounded checklist + language/framework-aware passive detection. Confidence-tiered findings (High = block; Medium = warn; Low = informational).

### 6.4 `S4_VERIFY` additions

#### 6.4.1 `changelog-emission`

**Source:** `~/.agents/skills/documentation-generation/changelog-automation/SKILL.md` (already installed; 572 LoC).

**Frontmatter:**
```yaml
---
name: slipway-changelog-emission
description: "User-facing changes require a changelog entry before closeout"
tier: gate
phase: verify
subject: process
reason_code: changelog_emission_required
hard_gate: false
user_facing_conditional: true
order_after: goal-verification
references:
  - "shared:documentation-generation/changelog-automation"
  - "slipway:reference:changelog-authoring"
agent_hint: slipway-closer
---
```

**Conditional firing:** `user_facing_conditional: true` — fires only when changed files include `cmd/`, `docs/`, `README*`, or anything with semver impact.

#### 6.4.2 `postmortem-readiness`

**Source:** `~/.agents/skills/.../postmortem-writing/SKILL.md` (already installed; 390 LoC).

**Frontmatter:**
```yaml
---
name: slipway-postmortem-readiness
description: "Pivots/repairs/aborts during execution require a blameless postmortem"
tier: gate
phase: verify
subject: process
reason_code: postmortem_readiness_required
hard_gate: false
pivot_conditional: true
order_after: goal-verification
references:
  - "slipway:reference:postmortem-writing"
agent_hint: slipway-closer
---
```

**Conditional firing:** `pivot_conditional: true` — fires only when execution log contains a pivot, repair, or abort event.

### 6.5 Reason codes added in Wave 3

Add to `internal/model/reason_code.go`:

```go
const (
    ReasonThreatModelScreen      ReasonCode = "threat_model_screen:safety_subject_unscreened"
    ReasonADRDiscipline          ReasonCode = "adr_discipline:material_design_undocumented"
    ReasonCodeSimplification     ReasonCode = "code_simplification_review:not_applied"
    ReasonErrorHandlingDiscipline ReasonCode = "error_handling_discipline:undeclared_strategy"
    ReasonAuditContextReadiness  ReasonCode = "audit_context_readiness:context_unbuilt"
    ReasonDifferentialRiskReview ReasonCode = "differential_risk_review:risk_unclassified"
    ReasonSecurityReview         ReasonCode = "security_review:safety_subject_unreviewed"
    ReasonChangelogEmission      ReasonCode = "changelog_emission:user_facing_undocumented"
    ReasonPostmortemReadiness    ReasonCode = "postmortem_readiness:pivot_unanalyzed"
)
```

Plus a tenth code for any new ordering field collision detection.

---

## 7. Reference Shelf Specs (Wave 4)

12 reference skills, each ~150–200 LoC. Each is a slipway-flavored adaptation of an upstream skill, with slipway frontmatter and a `## Source` section citing the upstream path.

### 7.1 Reference frontmatter template

```yaml
---
name: slipway-<reference-name>
description: "<one-line intent>"
tier: reference
phase: <intake|plan|execute|review|verify>
subject: <correctness|safety|architecture|refactor|process|debug|authoring>
references:
  - "shared:<pool-path>"
hard_gate: false
---
```

### 7.2 Reference body shape

```markdown
# <Title>

> **Source:** `<full pool path>` (mirrored upstream)
> **Use when:** <one-paragraph trigger>
> **Surfaced via:** `slipway next --references` when phase=<phase> and subject=<subject>

## Quick Reference

<5–10 line summary of the upstream methodology>

## When to invoke

<3–5 bullets>

## When NOT to invoke

<3–5 bullets>

## Borrowed essentials

<the 100–150 LoC slipway cares about — heuristics, decision matrix, anti-patterns>

## See also

- <related slipway gates>
- <other reference shelf entries>
```

### 7.3 The 12 references

| # | Slipway target | Source | Phase | Subject | LoC target |
|---|---|---|---|---|---|
| 1 | `audit-context-building` | `ecosystem/trailofbits/audit-context-building` | review | safety | ~150 |
| 2 | `differential-review-methodology` | `ecosystem/trailofbits/differential-review` | review | safety | ~200 |
| 3 | `property-based-testing` | `ecosystem/trailofbits/property-based-testing` | execute | correctness | ~180 |
| 4 | `e2e-testing-patterns` | `developer-essentials/e2e-testing-patterns` | execute | correctness | ~150 |
| 5 | `error-handling-patterns` | `developer-essentials/error-handling-patterns` | execute | refactor | ~180 |
| 6 | `code-simplification` | `ecosystem/getsentry/code-simplifier` | execute | refactor | ~120 |
| 7 | `architecture-decision-records` | `documentation-generation/architecture-decision-records` | plan | architecture | ~150 |
| 8 | `threat-modeling` | `ecosystem/openai/security-threat-model` | plan | safety | ~150 |
| 9 | `incident-runbooks` | `incident-response/incident-runbook-templates` (Wave-1 chezmoi pull) | verify | process | ~180 |
| 10 | `postmortem-writing` | `incident-response/postmortem-writing` | verify | process | ~150 |
| 11 | `changelog-authoring` | `documentation-generation/changelog-automation` | verify | process | ~150 |
| 12 | `debugging-strategies` | `developer-essentials/debugging-strategies` | execute | debug | ~180 |

Total reference LoC: ~1,950. Each is reviewable in isolation.

### 7.4 Naming caveat

The 12 reference target names overlap with some of v2's invented aliases (`code-simplification`, `adr-authoring`-style). v5 keeps real upstream-aligned names where possible (`code-simplification`, `architecture-decision-records`, `changelog-authoring`) and avoids inventing new identities. The slipway prefix `slipway-` distinguishes the local adaptation from the upstream source.

---

## 8. CLI Surface Changes

### 8.1 `slipway next --references` (new flag)

```
slipway next --references           # adds References section to text output
slipway next --references --json    # adds "references" array to JSON output
slipway next --refs                  # short alias
```

Implementation: `cmd/next.go` reads the current state from change context, queries `defaultReferenceRegistry` for matching `Phase` and `Subject` (where `Subject` is either the change's tagged subject or "any"), prints the result.

### 8.2 No `slipway preset --references` (intentionally out of scope)

`preset` stays a workflow preset selector. The reference shelf is surfaced through `next`, not through `preset`. v5 does not turn `preset` into a catalog browser.

### 8.3 No `slipway next --skill <name>` (intentionally out of scope)

The `--skill` selector idea from v2 stays out of scope. Skill selection happens through the existing state-driven mechanism. References are *additive surfacing*, not selectable destinations.

---

## 9. chezmoi Expansion (Wave 1)

Add to `~/.local/share/chezmoi/.chezmoiexternal.toml.tmpl`:

| Source | Path |
|---|---|
| `wshobson/agents` plugin `conductor` skill `workflow-patterns` | `~/.agents/skills/conductor/workflow-patterns/` |
| `wshobson/agents` plugin `agent-teams` skill `task-coordination-strategies` | `~/.agents/skills/agent-teams/task-coordination-strategies/` |
| `wshobson/agents` plugin `agent-teams` skill `multi-reviewer-patterns` | `~/.agents/skills/agent-teams/multi-reviewer-patterns/` |
| `trailofbits/skills` plugin `ask-questions-if-underspecified` | `~/.agents/skills/ecosystem/trailofbits/ask-questions-if-underspecified/` |
| `trailofbits/skills` plugin `workflow-skill-design` skill `designing-workflow-skills` | `~/.agents/skills/ecosystem/trailofbits/designing-workflow-skills/` |
| `wshobson/agents` plugin `incident-response` skill `incident-runbook-templates` | `~/.agents/skills/incident-response/incident-runbook-templates/` |
| `wshobson/agents` plugin `incident-response` skill `postmortem-writing` | `~/.agents/skills/incident-response/postmortem-writing/` |

7 new chezmoi entries. After Wave 1's `chezmoi apply`, all 35 sources for Waves 2–4 are available locally under one stable path (`~/.agents/skills/`).

---

## 10. Implementation Waves

### Wave 1 — Engine + schema rev (foundation)

**Goal:** ship the contract change with zero behavior change for the existing 9 gates.

**Files touched:**
- `internal/engine/skill/skill.go` (Definition fields, defaultReferenceRegistry, Phase/Subject/Tier enums)
- `internal/engine/skill/registry_loader.go` (governanceFrontMatter parser)
- `internal/engine/progression/skill_resolution.go` (ChangeContext, ShouldFire, conditional helpers)
- `internal/engine/progression/advance_governed.go` (call ShouldFire in gate selection)
- `internal/model/config.go` (error-path / user-facing / architecture file-pattern config)
- `internal/tmpl/templates_test.go` (migrate to new contract)
- `cmd/next.go` (--references flag)
- `~/.local/share/chezmoi/.chezmoiexternal.toml.tmpl` (7 new sources)
- `docs/plans/2026-04-11-skills-integration-plan.zh-CN.md` (port v5 content)

**Files created:**
- `internal/engine/skill/phase_subject_tier.go` (enums + validators)

**Exit criteria:**
- `go test ./...` green
- `go vet ./...` clean
- `slipway next` on a scratch project produces identical output to before (existing 9 gates with new fields all set to defaults)
- `slipway next --references` produces an empty References section (no reference skills registered yet)
- `chezmoi apply` succeeds; `~/.agents/skills/conductor/workflow-patterns/SKILL.md` exists

### Wave 2 — Existing-skill enhancements + checklist-quality decomposition

**Goal:** apply the 6 v4 §7.2 merge specs and decompose `checklist-quality.md`.

**Files touched:**
- `internal/tmpl/templates/skills/intake-clarification/SKILL.md` (5.1)
- `internal/tmpl/templates/skills/code-quality-review/SKILL.md` (or `.tmpl` if applicable) (5.2)
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` (5.3)
- `internal/tmpl/templates/skills/tdd-governance/SKILL.md.tmpl` (5.4)
- `internal/tmpl/templates/skills/codebase-mapping/SKILL.md` (5.5)
- `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl` (5.6)
- `internal/engine/skill/skill.go` (add new fields to existing 9 entries: phase, subject, tier, reason_code, hard_gate, references, order_after where applicable)

**Files created:**
- `internal/tmpl/templates/skills/checklist-quality/intake.md`
- `internal/tmpl/templates/skills/checklist-quality/plan.md`
- `internal/tmpl/templates/skills/checklist-quality/review.md`
- `internal/tmpl/templates/skills/checklist-quality/test.md`

**Files deleted:**
- `internal/tmpl/templates/skills/checklist-quality.md` (replaced by directory)

**Exit criteria:**
- All 6 enhanced skills have new sections; structural test extended to assert new section markers
- All 9 existing registry entries have full v5 frontmatter
- `spec-compliance-review` references `checklist-quality/review.md` specifically

### Wave 3 — New gates (10)

**Goal:** ship 10 new gates, taking total to 19.

**Files created:**
- 10 new `internal/tmpl/templates/skills/<gate-name>/SKILL.md` files (or `.tmpl` if needed)
- Each with full frontmatter, body skeleton, mandatory checklist, failure handling

**Files touched:**
- `internal/engine/skill/skill.go` (10 new entries in `defaultGovernanceRegistry`)
- `internal/model/reason_code.go` (10 new reason code constants from §6.5)
- `internal/engine/progression/advance_governed.go` (any state-specific routing tweaks)
- `internal/tmpl/templates_test.go` (10 new skill structural assertions)
- `cmd/<various>_test.go` (end-to-end coverage for conditional firing)

**Exit criteria:**
- 19 gates registered, 19 gates pass topo-sort cycle check
- Each conditional gate fires only under its declared condition (covered by table-driven tests against synthetic ChangeContexts)
- Smoke test: `slipway init → new → next → done` on a scratch project with each conditional triggered (10 scenarios)

### Wave 4 — Reference shelf (12)

**Goal:** ship 12 slipway-flavored reference skills.

**Files created:**
- 12 new `internal/tmpl/templates/skills/<reference-name>/SKILL.md` files, each ~150–200 LoC
- Each with `tier: reference` frontmatter and the §7.2 body shape

**Files touched:**
- `internal/engine/skill/skill.go` (12 new entries in `defaultReferenceRegistry`)
- `internal/tmpl/templates_test.go` (12 new structural assertions for reference skills)
- `cmd/next.go` (--references output now non-empty; integration test)

**Exit criteria:**
- `slipway next --references` on a scratch project surfaces the correct subset for each state
- `TestReferenceTargetsExist` passes (every `references:` entry resolves to a real `~/.agents/skills/` path)
- `docs/skills/INDEX.md` regenerated from the registry (one-line per skill, grouped by phase)

---

## 11. Implementation Hazards

1. **Template rendering vs static markdown.** `spec-compliance-review`, `wave-orchestration`, `tdd-governance` are `.tmpl`. Wave-2 structural tests must render first or assert against `.tmpl` source.
2. **`checklist-quality.md` references break if not migrated atomically.** Wave 2's `spec-compliance-review/SKILL.md.tmpl:35` reference must update in the same commit as the directory creation.
3. **`AgentHint` is mandatory for gates.** All 10 new gates must set it. Reuse existing values (`slipway-planner`, `slipway-auditor`, `slipway-orchestrator`, `slipway-reviewer`, `slipway-verifier`, `slipway-closer`); do not introduce new hints in v5.
4. **Topo-sort cycles fail loud at registry-load time.** Add a unit test that the 19-gate registry has no cycles. If a future plan adds a gate that creates a cycle, the binary will refuse to start.
5. **Conditional firing depends on `ChangeContext` accuracy.** The detection helpers (`touchesUserFacing`, `touchesErrorPath`, `touchesArchitecture`) are file-glob heuristics; they will have false positives and false negatives. Mitigation: each helper has a config override in `internal/model/config.go` for project-specific patterns.
6. **`subject` tagging happens at intake.** v5 requires that the change context carries a `Subject` value. Wave 1 must extend the change resolver to pull subject from intake artifacts (`artifacts/changes/{slug}/intake/subject.yaml` or similar). If subject is unset, conditional gates default to "fire" (safe-by-default).
7. **Reason codes are stable identifiers.** Once shipped, the 10 new reason codes from §6.5 cannot be renamed without a deprecation pass. Pick the names carefully in Wave 3.
8. **chezmoi pull is local-only.** Wave 1's `.chezmoiexternal.toml.tmpl` change works only on machines that have `chezmoi apply` access to the source. CI should fall back to reading from `~/ghq/.../<repo>/...` paths or skip reference-existence assertions.
9. **`zh-CN.md` sibling drifts.** The current zh-CN file is still v2 content. Wave 1 must port v5 content; do not let it drift.
10. **Wave 3 gate count is the inflection point.** Going from 9 to 19 gates is where the schema's value lives — but it's also where review burden spikes. Stage Wave 3 as one PR per 2–3 gates if review velocity is the bottleneck (5 sub-PRs total).

---

## 12. Explicit Non-Goals (still)

- No `slipway next --skill <name>` selector. Skill selection stays state-driven.
- No `slipway preset` rework. Preset stays a workflow preset selector.
- No promotion of techniques (`tdd`, `systematic-debugging`, `code-review-protocol`, `codebase-mapping`) into gates. Techniques are invoked *within* gates.
- No `defaultReferenceRegistry` entry for `agentic-actions-auditor`, `dimensional-analysis`, `mutation-testing`, `entry-point-analyzer`, `spec-to-code-compliance`, `testing-handbook-skills/*`. These remain explicitly excluded as documented in §2.6.
- No new `AgentHint` values. Reuse the 6 existing.
- No backwards-compat shim for the old `governanceFrontMatter` shape. Wave 1 migrates the test contract once and locks it.

---

## 13. Open Decisions for Owner

These six require yes/no before Wave 1 starts. v5 takes a recommendation on each.

1. **Adopt the 6 new `Definition` fields and the new `Tier`/`Phase`/`Subject` enums (§4.1)?** *Recommendation: yes — this is the foundation for everything else.*
2. **Adopt topo-sort gate ordering with `OrderAfter` (§4.4)?** *Recommendation: yes — without this, the new S3 gates have undefined order.*
3. **Adopt the conditional firing mechanism (`SubjectGated`, `PivotConditional`, `UserFacingConditional`, `ErrorPathConditional`, `MaterialDesignConditional`) over the alternative DSL approach (§4.5)?** *Recommendation: yes — explicit bool flags are simpler than a condition DSL and easier to test.*
4. **Add `slipway next --references` (§4.6, §8.1) but **not** `--skill` and **not** `preset` rework (§8.2, §8.3)?** *Recommendation: yes — minimal CLI surface that unlocks the reference shelf.*
5. **Decompose `checklist-quality.md` into 4 domain checklists (§5.7, §7) in Wave 2, not as a separate plan?** *Recommendation: yes — the four-checklist split is a small Wave-2 add and avoids leaving the sidecar inconsistent with v5's vocabulary.*
6. **Ship Wave 3 as 5 sub-PRs (one per 2–3 gates) or one big PR?** *Recommendation: 5 sub-PRs — review burden is the bottleneck for Wave 3, not engine work.*

---

## 14. Verification Index (from v4 §13)

Files read end-to-end during the audit. Every claim in §2 maps to one of these. Re-run any of these reads to reverify.

**Trailofbits:**
- `~/ghq/github.com/trailofbits/skills/plugins/agentic-actions-auditor/skills/agentic-actions-auditor/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/differential-review/skills/differential-review/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/property-based-testing/skills/property-based-testing/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/spec-to-code-compliance/skills/spec-to-code-compliance/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/ask-questions-if-underspecified/skills/ask-questions-if-underspecified/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/entry-point-analyzer/skills/entry-point-analyzer/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/workflow-skill-design/skills/designing-workflow-skills/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/skill-improver/skills/skill-improver/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/testing-handbook-skills/skills/` (15-entry directory listing)
- `~/ghq/github.com/trailofbits/skills/plugins/mutation-testing/skills/mutation-testing/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/dimensional-analysis/skills/dimensional-analysis/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/audit-context-building/skills/audit-context-building/SKILL.md`

**Anthropics / OpenAI / Getsentry:**
- `~/ghq/github.com/anthropics/skills/skills/skill-creator/SKILL.md`
- `~/ghq/github.com/openai/skills/skills/.curated/security-threat-model/SKILL.md`
- `~/ghq/github.com/openai/skills/skills/.curated/security-best-practices/SKILL.md`
- `~/ghq/github.com/openai/skills/skills/.curated/security-ownership-map/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/skill-scanner/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/code-simplifier/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/claude-settings-audit/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/security-review/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/find-bugs/SKILL.md`

**wshobson:**
- `~/ghq/github.com/wshobson/agents/plugins/agent-teams/skills/multi-reviewer-patterns/SKILL.md` (~127 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/conductor/skills/workflow-patterns/SKILL.md` (~623 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/agent-teams/skills/task-coordination-strategies/SKILL.md` (~163 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/developer-essentials/skills/error-handling-patterns/SKILL.md` (~632 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/framework-migration/skills/dependency-upgrade/SKILL.md` (~368 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/developer-essentials/skills/e2e-testing-patterns/SKILL.md` (~535 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/documentation-generation/skills/architecture-decision-records/SKILL.md` (~441 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/incident-response/skills/incident-runbook-templates/SKILL.md` (~471 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/incident-response/skills/postmortem-writing/SKILL.md` (~390 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/documentation-generation/skills/changelog-automation/SKILL.md` (~572 LoC)
- `~/ghq/github.com/wshobson/agents/plugins/developer-essentials/skills/code-review-excellence/SKILL.md`
- `~/ghq/github.com/wshobson/agents/plugins/developer-essentials/skills/debugging-strategies/SKILL.md`

**Slipway internals:**
- `~/ghq/github.com/signalridge/slipway/internal/engine/skill/skill.go:1-100`
- `~/ghq/github.com/signalridge/slipway/internal/engine/skill/registry_loader.go:40-44, 168-199`
- `~/ghq/github.com/signalridge/slipway/internal/engine/progression/skill_resolution.go`
- `~/ghq/github.com/signalridge/slipway/internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl`
- `~/ghq/github.com/signalridge/slipway/internal/tmpl/templates/skills/intake-clarification/SKILL.md`
- `~/ghq/github.com/signalridge/slipway/internal/tmpl/templates/skills/checklist-quality.md`
- `~/.local/share/chezmoi/.chezmoiexternal.toml.tmpl`
- `~/.agents/skills/` (top-level + ecosystem/ walk; 167 SKILL.md confirmed)
