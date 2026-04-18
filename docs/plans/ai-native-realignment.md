# AI-Native Realignment

> Status: Draft
> Author: @LuYixian
> Date: 2026-04-17

## Context

Independent audit of Slipway against superpowers (obra), GSD (gsd-build), and
spec-kitty (Priivacy-ai) identified targeted areas where implicit governance
intervention adds ceremony without proportional safety benefit. Slipway's
baseline is stronger than the earlier audit implied: behavior shaping already
spans dozens of IRON LAW / red-flag templates, wave planning and static
conflict checks exist in the Go runtime, context telemetry is already exposed
via `next --json` (`contextBudget` struct), and `--quick` mode already provides
a light-ceremony path.

The problem is narrower than "governance dominates." Specifically:

- **Implicit intervention**: GuardrailDomain silently overrides user-chosen
  presets and control modes, forces skill substitution, and injects extra
  review layers — acting as a "maximum ceremony" switch rather than a risk
  signal
- **Hook noise**: Per-tool-call context-monitor injection adds latency
  without agent request
- **Split state authority**: Two mutable YAML files per change create
  merge/hydration complexity
- **Template framing gap**: HARD RULE markers imply runtime enforcement
  that does not exist

This plan covers the items **not already addressed** by the two existing plans:

- `thin-kernel-refactor.md` — capability demotion, toolgen simplification,
  gate threshold tuning, command surface reduction
- `runtime-contract-hardening.md` — hidden mutation removal, structured
  AdvanceSummary, reconcile projection/apply split

Where this plan overlaps with those, the overlap is noted and one plan is
designated as owner. No item should be executed twice.

## Architecture Stance

Slipway keeps its runtime-owned workflow progression (this is NOT a move to
"raw facts only, AI decides everything"). The change targets ceremony
reduction in five specific areas:

- Controls default to advisory, not blocking (fail-open for non-sensitive)
- Skill dispatch is uniform (no domain-triggered skill switching)
- Hooks inform on session start only; no per-tool-call injection
- State authority converges to one mutable file per change
- Template content is honest about what the runtime enforces vs what is
  behavioral guidance for agents

All 55 skills are retained. Model tiering is not in scope.

### What This Plan Does Not Touch (Already Strong)

| Capability | Evidence | Status |
|------------|----------|--------|
| Behavior shaping | Dozens of IRON LAW / red-flag / rationalization sections across governance and agent templates | Strong enough baseline; this plan does not need to add more doctrine |
| Wave planning primitives | `internal/engine/wave/PlanWaves()` + SKILL.md.tmpl tool-conditional dispatch (Claude Code Task(), Codex --task, Cursor sequential) | Internal capability exists; user-facing parallel runtime remains a deferred boundary |
| Context telemetry | `cmd/next.go` returns `contextBudget` with tokens, utilization, remaining, health, guard action, thresholds, breakdown | Already sufficient for a pull-based model |
| Quick mode | `cmd/run.go:69`, `cmd/next.go:222` register `--quick`, disabling advisory controls via `AdvanceOptions{QuickMode: quickMode}` | Already shipped |

## Relationship to Existing Plans

| Item | Owner Plan | Notes |
|------|-----------|-------|
| Gate threshold tuning | thin-kernel Wave 4.1 | This plan's Wave 1.1 goes further: changes default MODE, not just threshold |
| `--quick` mode | thin-kernel Wave 4.2 | Owned there; not duplicated here |
| Capability surface demotion | thin-kernel Wave 2 | This plan's Wave 3.1 is a subset focused on class count |
| `--focus`/`--view` flag removal | thin-kernel Wave 2 Phase B | Owned there; Wave 3.1 here prepares the data model |
| Toolgen simplification | thin-kernel Wave 3 | Not duplicated here |
| Command surface reduction | thin-kernel Wave 5 | Not duplicated here |
| Hidden mutation removal | runtime-contract-hardening | Not duplicated here |
| AdvanceSummary contract | runtime-contract-hardening | Not duplicated here |
| Reconcile projection split | runtime-contract-hardening | Not duplicated here |

---

## Wave 1: Fail-Open Defaults and Dispatch Unbinding

**Outcome**: Controls stop blocking non-sensitive changes by default. Guardrail
domain stops forcing skill substitution. The governance system becomes advisory-
first for the common case while remaining fail-closed for genuinely dangerous
domains.

### 1.1 Controls: blocking → advisory for review gates

**Problem**: `internal/engine/control/config.go:22-29` sets `domain-review` and
`independent-review` to blocking by default. Combined with
`internal/engine/control/derive.go:103-132` (any non-empty GuardrailDomain
activates both), this means every auth/security/schema change is blocked
waiting for human review before the AI can proceed.

**Action**: Change `defaultControlModes` for two controls:

```go
// config.go — defaultControlModes
ControlClarification:      ControlModeBlocking,  // keep: prevents garbage-in
ControlResearch:           ControlModeBlocking,  // keep: discovery must complete
ControlDomainReview:       ControlModeAdvisory,  // was: blocking
ControlIndependentReview:  ControlModeAdvisory,  // was: blocking
ControlWorktreeIsolation:  ControlModeAdvisory,  // already advisory
ControlRollbackRequired:   ControlModeAdvisory,  // already advisory
```

**Escape hatch preserved**: Users who want blocking behavior can override in
`.slipway.yaml`:
```yaml
governance:
  controls:
    domain-review: blocking
    independent-review: blocking
```

The derive logic (`derive.go:103-152`) is unchanged — controls still activate
and appear in `status --json` output. They just don't block progression.

**Files**:
- `internal/engine/control/config.go` — change 2 map entries
- `internal/engine/control/derive_test.go` — update mode expectations
- `cmd/` tests that assert blocking behavior for these controls

**Relationship to thin-kernel Wave 4.1**: That wave changes blast-radius
thresholds. This wave changes default modes. They are complementary — execute
this wave first, then thin-kernel 4.1 adjusts when controls activate at all.

### 1.2 Remove GuardrailDomain → tdd-governance hard binding

**Problem**: The current kernel binds guardrail-domain execution to
`tdd-governance` in **two places**, not one:

1. `internal/engine/progression/skill_resolution.go:72-79` dispatches
   S2_EXECUTE to `tdd-governance` instead of `wave-orchestration`
2. `internal/engine/skill/skill.go` marks `tdd-governance` as
   `GuardrailRequired`, so required-skill evaluation and `next` evidence still
   demand it for guardrail-domain changes

The first binding chooses the next skill. The second binding keeps
`tdd-governance` in the required-skill/evidence contract even if dispatch is
changed. Fixing only the dispatcher would create drift between `next_skill` and
required-skill evidence.

The primary dispatch problem is here:
```go
if change.GuardrailDomain != "" {
    return SkillTDDGovernance, string(model.StateS2Execute)
}
return SkillWaveOrchestration, string(model.StateS2Execute)
```

Any change tagged with a guardrail domain gets forced into `tdd-governance`
instead of `wave-orchestration`. This is a product-level skill substitution
that should be a template-level behavioral recommendation instead.

The wave-orchestration SKILL.md template already contains a TDD gate
(SKILL.md.tmpl:206-214) that downgrades verdicts to `incomplete/tdd_violation`
when implementation precedes a failing test. This handles TDD discipline
without kernel-level skill switching.

**Action**: Remove the kernel-level hard binding completely. All S2_EXECUTE
changes dispatch to `wave-orchestration`, and `tdd-governance` stops being an
auto-required guardrail skill:

```go
func resolveS2Execute(change ResolveChange) (string, string) {
    if change.NeedsDiscovery && change.WorktreePath == "" {
        return SkillWorktreePreflight, string(model.StateS2Execute)
    }
    return SkillWaveOrchestration, string(model.StateS2Execute)
}
```

Then update the governance skill registry so `tdd-governance` remains
available but is no longer injected into the required-skill set solely because
`GuardrailDomain != ""`.

**Files**:
- `internal/engine/progression/skill_resolution.go` — remove domain check
- `internal/engine/skill/skill.go` — stop auto-requiring `tdd-governance`
  for guardrail domains
- `internal/engine/progression/skill_resolution_test.go` — update cases
  that expect `SkillTDDGovernance` to expect `SkillWaveOrchestration`
- `internal/engine/skill/skill_test.go` — update required-skill expectations
  for guardrail-domain execution
- `cmd/*next*_test.go` — update any tests that assert required skill evidence
  or next-skill/evidence alignment

**Risk**: Medium-low. The `tdd-governance` skill template still exists and can
be invoked explicitly. The change must land atomically, though — partially
removing only the dispatcher would leave readiness/evidence contracts behind.

### 1.3 Remove GuardrailDomain override cascade in preset_policy

**Problem**: Wave 1.1 changes default control modes in `config.go`, but
`internal/engine/governance/preset_policy.go:148-160` contains a hard-coded
override that **forces domain-review back to blocking** whenever
GuardrailDomain is non-empty:

```go
// preset_policy.go:148-160
if strings.TrimSpace(change.GuardrailDomain) != "" {
    overrides.ModeOverrides[model.ControlDomainReview] = model.ControlModeBlocking
    overrides.DisabledControls = removeDisabledControl(
        overrides.DisabledControls, model.ControlDomainReview,
    )
    if isRollbackSensitiveDomain(change.GuardrailDomain) {
        overrides.ModeOverrides[model.ControlRollbackRequired] = model.ControlModeBlocking
        // ...
    }
}
```

Without this fix, Wave 1.1 is **silently negated** for any change with a
guardrail domain — the config default changes to advisory, but the preset
policy overrides it back to blocking at runtime.

Additionally, `preset_policy.go:68-71` forces preset upgrade to `standard`
when any guardrail domain is set:

```go
if strings.TrimSpace(change.GuardrailDomain) != "" &&
    effective.Rank() < model.WorkflowPresetStandard.Rank() {
    effective = model.WorkflowPresetStandard
}
```

This means a user who explicitly chose `light` preset gets silently upgraded
to `standard` simply because a guardrail domain was tagged. Combined with
the control override, GuardrailDomain acts as a "maximum ceremony" switch
that overrides all user preferences.

**Action**: Remove both overrides. Let the user's config and defaults govern:

1. **Delete lines 148-160** (`preset_policy.go`): Remove the entire
   `GuardrailDomain != ""` block that forces domain-review and
   rollback-required back to blocking. Users who want blocking for
   guardrail domains can set it in `.slipway.yaml` governance config.

2. **Delete lines 68-71** (`preset_policy.go`): Remove the forced preset
   upgrade from GuardrailDomain. If a user chose `light`, respect it.

**What GuardrailDomain still does after this change**:
- `control/derive.go:103-112`: domain-review control still **activates**
  (appears in status output) — it just respects the configured mode
- `gate.go:134`: G_ship still evaluates high-risk checks — safety_baseline
  evidence is still required for shipping guardrail-domain changes
- `review.go:25-36`: Extra review layers (R3, IR3) still required — these
  are review-depth checks, not blocking gates
- `inference.go:85-90`: GuardrailDomain still requires `needs_discovery=true`
  and `complexity >= complex` at intake — this is intake validation, not
  runtime ceremony

The net effect: GuardrailDomain remains a **risk signal** that activates
controls and requires evidence, but stops being a **ceremony override** that
hijacks user preferences.

**Files**:
- `internal/engine/governance/preset_policy.go` — delete lines 68-71 and
  148-160
- `internal/engine/governance/preset_policy_test.go` — update tests that
  assert forced upgrade and forced blocking mode
- `internal/engine/governance/health_test.go` — update any tests that
  depend on domain-driven preset upgrade

**Risk**: Medium. This is the most consequential change in Wave 1. The
escape hatch is that `.slipway.yaml` governance config can restore blocking
behavior per-project. The G_ship gate still requires safety_baseline
evidence, so genuinely dangerous changes cannot ship without verification.

**Must land with 1.1**: If 1.1 lands without 1.3, the default mode change
is silently overridden and the plan's stated outcome is not achieved.

### Wave 1 Execution Card

| Dimension | Detail |
|-----------|--------|
| **PR scope** | Single PR. 1.1 + 1.2 + 1.3 must land atomically — 1.1 without 1.3 is silently negated, 1.2 without the skill registry change creates next/evidence drift |
| **Rollback** | Revert the PR. All three changes touch config/dispatch/policy; reverting any subset leaves inconsistent behavior. Full revert restores blocking defaults + tdd-governance dispatch + preset override |
| **Must-not-break** | (1) `slipway status --json` still shows domain-review/independent-review controls when GuardrailDomain is set — they activate, they just don't block. (2) G_ship safety_baseline evidence gate unchanged — guardrail-domain changes still cannot ship without verification. (3) `tdd-governance` skill template still loadable via explicit invocation. (4) `.slipway.yaml` governance config override still respected |
| **Primary evidence** | `go test ./internal/engine/control/... ./internal/engine/progression/... ./internal/engine/governance/... ./internal/engine/skill/... -v` all pass. Manual: `slipway new --guardrail-domain auth_authz`, then `slipway status --json` shows domain-review as advisory |

---

## Wave 2: Hook Noise Reduction

**Outcome**: Session start provides essential context. Per-tool-call injection
is eliminated. Agents pull state when they need it via explicit CLI calls.

**Tradeoff**: This wave trades proactive context telemetry push (the runtime
tells the agent when context is getting tight) for pull-based (the agent must
call `slipway status --json` to check). For comparison:

- **Superpowers**: No telemetry at all, but has compact reinjection via
  `hooks.json` compact matcher — session-start re-injects compressed context
  when conversation is compacted
- **GSD**: Keeps a per-tool context-monitor hook (35%/25% thresholds) as one
  of 2 default hooks
- **Spec-Kitty**: Bootstrap/compact two-stage context modes with glossary
  pipeline

The pull-based model is simpler but requires agents to be disciplined about
checking context budget. Slipway's `next --json` already exposes the full
`contextBudget` struct, so agents have the information available — they just
need to ask for it. If pull-based proves insufficient, a future iteration
could add an opt-in compact reinjection hook (like superpowers) without
reverting to per-tool injection.

### 2.1 Remove post-tool-context-monitor hook

**Problem**: `internal/tmpl/templates/hooks/post-tool-context-monitor.js.tmpl`
fires on every PostToolUse event, debounces to every 5th tool call, and runs
`slipway next --preview --context-guard`. This is implicit injection that:

- adds ~200ms latency every 5 tool calls
- injects governance state the agent didn't ask for
- duplicates what session-start already provides
- cannot be disabled without removing the generated hook file

The agent can call `slipway status --json` or `slipway next --preview --json`
at any time. Pull > push.

**Action**:
1. Delete `internal/tmpl/templates/hooks/post-tool-context-monitor.js.tmpl`
2. Remove post-tool registration from the tool registry entries in
   `internal/toolgen/toolgen.go` (Claude/Gemini currently carry
   `PostToolEvent`/`PostToolHook` metadata)
3. Remove post-tool hook generation and settings merge branches from
   `internal/toolgen/toolgen.go` so generated settings no longer reference a
   nonexistent hook path
4. Update tests that assert hook count or generated settings content

**Files**:
- `internal/tmpl/templates/hooks/post-tool-context-monitor.js.tmpl` — delete
- `internal/toolgen/toolgen.go` — remove registry fields, generation call,
  and settings merge branch for post-tool hooks
- `internal/toolgen/adapter_contract_test.go` — update expectations
- `internal/tmpl/hooks_behavior_test.go` — remove post-tool hook test cases
- Any toolgen fixture/settings tests that assert generated `hooks` JSON

### 2.2 Slim session-start hook

**Problem**: `internal/tmpl/templates/hooks/session-start.sh.tmpl:63-68` runs
`slipway next --preview --context-guard` as a third read-only query. With the
post-tool monitor removed, context-guard has no debounce companion and becomes
a one-shot check that adds latency without ongoing value.

**Action**: Remove the context-guard call block (lines 63-68 and the
corresponding variable interpolation in the output template). Keep:
- `slipway status --json` (active change + health)
- `slipway next --json --preview` (next skill context)
- `handoff.md` read (session metadata)

**Files**:
- `internal/tmpl/templates/hooks/session-start.sh.tmpl` — remove context-guard
  block
- `internal/tmpl/hooks_behavior_test.go` — update session-start expectations

---

## Wave 3: Surface Taxonomy Simplification

**Outcome**: Surface classes collapse from 4 to 2. Suggested skills stop being
auto-recommended. Explicit-focus remains as the opt-in mechanism.

### 3.1 Collapse surface classes: 4 → 2

**Problem**: `internal/engine/capability/surfaces.go:15-19` defines four
surface classes: `primary`, `suggested`, `explicit_focus`, `view`. The
`suggested` class (lines 65-135) auto-recommends 13 skills without agent
request. The `view` class (lines 169-179) adds a `--view` flag that duplicates
`--focus` semantics. This also leaks into routed command outputs via the public
`suggested_capabilities[]` field on `review`, `validate`, and `repair`.

**Action**:
1. Remove `SurfaceSuggested` and `SurfaceView` from enum
2. Remove suggested entries (lines 65-135) from `surfacePolicy`
3. Remove view entries (lines 169-179) from `surfacePolicy`
4. Collapse `LookupView` into `LookupFocus` (accept view aliases as focus
   aliases)

**Resolver changes** (`internal/engine/capability/resolver.go`):
1. Delete `collectSuggestedCapabilities()` (lines 193-236)
2. Remove `SuggestedCapabilities` from `Resolution` struct
3. Simplify `resolveRoute` to handle only primary and explicit_focus

**Cmd changes**: Commands that use `--view` (`status.go`, `health.go`) switch
to `--focus` for consistency, **but only if thin-kernel Wave 2 Phase B has not
landed first**.

**Relationship to thin-kernel Wave 2 Phase B**: That phase removes
`--focus`/`--view`/`--list-*` flags entirely. This wave is a prerequisite —
it simplifies the data model first. If thin-kernel 2B lands, the remaining
`--focus` flag goes away too. If Phase B lands first, skip this wave rather
than doing an intermediate `--view` -> `--focus` migration that is immediately
deleted.

**Files**:
- `internal/engine/capability/surfaces.go` — remove 2 classes + entries
- `internal/engine/capability/resolver.go` — remove suggested collection
- `cmd/review.go`, `cmd/validate.go`, `cmd/repair.go` — remove
  `suggested_capabilities` emission from command JSON/text surfaces
- `cmd/status.go`, `cmd/health.go` — `--view` → `--focus`
- `cmd/route_flags.go` — simplify view handling
- Tests in `internal/engine/capability/*_test.go`
- `cmd/route_flags_test.go`, `cmd/route_surface_command_test.go`,
  `cmd/repair_test.go` — update routed-command expectations

---

## Wave 4: State Authority Convergence

**Outcome**: One mutable YAML file per change instead of two. Runtime sidecar
fields merge into `change.yaml`. `execution-summary.yaml` stays separate as
append-only evidence.

### 4.1 Merge runtime-state.yaml into change.yaml

**Problem**: The Change struct (`internal/model/change.go:57-61`) tags 5 fields
with `yaml:"-"` to exclude them from `change.yaml`. These fields are persisted
separately in `runtime-state.yaml` and merged back at load time via
`loadAndApplyChangeRuntimeState()` (`internal/state/change_runtime.go:164-181`).
This split creates:

- A merge/hydration path that can fail or diverge
- A rollback dance in `SaveChange` (store.go:453-475) managing two files
- A strict-decoding story that is currently coupled to sidecar exclusion
- Conceptual overhead: "which file owns what" is a question that shouldn't exist
- Separate health/repair drift logic that still reads `runtime-state.yaml`
  directly instead of trusting unified lifecycle state

**Action**:

1. **`internal/model/change.go`** — Replace `yaml:"-"` with proper YAML tags:
   ```go
   Artifacts                map[string]ArtifactState `yaml:"artifacts,omitempty"`
   LastAutoPassedStates     []AutoPassedState        `yaml:"last_auto_passed_states,omitempty"`
   EvidenceRefs             map[string]string        `yaml:"evidence_refs,omitempty"`
   ReviewIntentDriftFailures int                     `yaml:"review_intent_drift_failures,omitempty"`
   InterruptedExecutionAt   time.Time                `yaml:"interrupted_execution_at,omitempty"`
   ```

2. **`internal/state/change_runtime.go`** — Replace sidecar authority helpers
   with compatibility-only helpers:
   - Delete `ChangeRuntimeState` struct (lines 43-51)
   - Delete `buildChangeRuntimeState()` (lines 99-114)
   - Delete `loadAndApplyChangeRuntimeState()` (lines 164-181)
   - Delete `loadChangeRuntimeStateFromPath()` (lines 131-153)
   - Keep the file only for backward-compat migration helpers (see step 5)

3. **`internal/state/store.go`** — Simplify SaveChange:
   - Remove runtime-state.yaml write (lines 467-475)
   - Remove rollback logic for dual-file writes (lines 453-460)
   - SaveChange writes one `change.yaml` containing all fields

4. **`internal/state/store.go`** — Simplify LoadChange:
   - Remove the `loadAndApplyChangeRuntimeState` merge step after decoding
     `change.yaml`
   - Keep strict decoding (`KnownFields(true)`), but update the persisted schema
     so runtime fields are now legitimate members of `change.yaml`

5. **Migration**: Add a one-time compat path in LoadChange:
   ```go
   // If legacy runtime-state.yaml exists alongside change.yaml,
   // load it, merge fields into change, and delete the file on next save.
   if runtimePath exists {
       merge legacy runtime fields into change
       mark change as dirty (will be saved as unified file)
       delete runtime-state.yaml on successful save
   }
   ```
   This avoids needing a separate `slipway migrate` command.

   **Compat policy for broken legacy sidecars**:
   - If legacy `runtime-state.yaml` is readable, merge it and delete it on the
     next successful save.
   - If legacy `runtime-state.yaml` exists but is unreadable, do **not**
     silently discard it. Surface a compat-only `legacy_runtime_state_unreadable`
     health finding during the migration window and block compat migration for
     that change until the stale sidecar is repaired or removed.
   - This preserves observability during migration without keeping
     `runtime-state.yaml` as a long-term second authority.

6. **`internal/state/lifecycle.go`** — Simplify ArchiveChange:
   - Remove separate runtime-state.yaml archival steps
   - Archive writes one `change.yaml` to archive location

7. **`internal/state/health.go` + `internal/state/execution_repair.go`** —
   replace runtime-state authority checks with compat-only migration checks:
   - Health should stop treating `runtime-state.yaml` as an active authority
     file that can drift from `change.yaml`
   - Repair should stop consulting `runtime-state.yaml` to decide whether
     lifecycle state drift exists
   - During the compat window, health may still surface a broken legacy sidecar
     as migration debt; after successful save, it disappears

**execution-summary.yaml is unchanged**: It is append-only evidence produced
at the end of execution. It does not participate in load/save cycles or state
hydration. Keeping it separate is correct.

**Files**:
- `internal/model/change.go` — change 5 struct tags
- `internal/state/change_runtime.go` — remove sidecar authority code and keep
  compat migration helper
- `internal/state/store.go` — simplify SaveChange and LoadChange
- `internal/state/lifecycle.go` — simplify ArchiveChange
- `internal/state/health.go` — drop runtime-state authority/drift findings
- `internal/state/execution_repair.go` — drop runtime-state drift checks
- `internal/state/paths.go` — remove `RuntimeStatePath` if it exists as
  a dedicated helper
- All tests in `internal/state/*_test.go` and `cmd/*_test.go` that create
  or assert on `runtime-state.yaml`

**Risk**: Medium-high. This is the largest mechanical and semantic change in
this plan. It is not just a store-layer rewrite; health/repair semantics change
too. The migration path (step 5) ensures existing changes don't break. Test
coverage for state persistence is strong (store_test.go, lifecycle_test.go),
but doctor/repair expectations must be updated in the same PR.

### Wave 4 Execution Card

| Dimension | Detail |
|-----------|--------|
| **PR scope** | Single PR, solo — do not combine with any other wave. Touches model tags, store load/save, lifecycle archive, health findings, repair checks, and adds a migration path. Mixing with other waves makes rollback forensics painful |
| **Rollback** | **Cannot simply revert code.** During the unified window, `SaveChange` writes runtime fields (`artifacts`, `evidence_refs`, etc.) into `change.yaml`. The old code's `decodeChangeStrict()` uses `yaml:"-"` + `KnownFields(true)` which **rejects** these fields (confirmed by `TestLoadChangeRejectsRuntimeFieldsInChangeAuthority`). Rollback procedure: (1) write a one-off script that reads each `change.yaml`, extracts the 5 runtime fields into `runtime-state.yaml`, and strips them from `change.yaml`; (2) run the script across all active changes; (3) then revert the code. Document the rollback script in the PR and test it before merge. Alternative: ship with a build-time feature flag (`unifiedChangeYAML`) so rollback is just flipping the flag without data migration |
| **Must-not-break** | (1) `slipway status --json` output unchanged — runtime fields were already hydrated into the Change struct before serialization. (2) `slipway health` still detects genuine state corruption — only the "runtime-state drift" finding category is removed, replaced by the compat-only unreadable-sidecar finding during migration window. (3) `execution-summary.yaml` untouched — append-only evidence path is not part of this change. (4) Archived changes still loadable — archive path writes unified `change.yaml` |
| **Primary evidence** | `go test ./internal/state/... ./cmd/... -v` all pass. Manual: (a) create new change, verify single `change.yaml` with runtime fields; (b) place a legacy `runtime-state.yaml` next to a `change.yaml`, run any command, verify migration merges and deletes sidecar; (c) place a corrupted `runtime-state.yaml`, verify `slipway health` surfaces `legacy_runtime_state_unreadable`; (d) verify `slipway repair` on a post-migration change does not reference runtime-state |

---

## Wave 5: Template Honesty

**Outcome**: Orchestrator and wave-orchestration templates clearly mark the
boundary between runtime-enforced behavior and aspirational agent guidance.
Tool-specific dispatch syntax becomes examples, not doctrine.

### 5.1 Mark aspirational rules in orchestrator template

**Problem**: `internal/tmpl/templates/agents/slipway-orchestrator.md` and
`internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` contain
HARD RULE markers for:

- 15% context budget (SKILL.md.tmpl:137-142)
- Fresh subagent per task (SKILL.md.tmpl:169)
- Parallel dispatch within wave (SKILL.md.tmpl:72-116)
- Atomic commit per task (SKILL.md.tmpl:199-204)

None of these are enforced by the runtime engine. They are behavioral guidance.
But the HARD RULE framing implies enforcement, creating a gap between what the
templates promise and what the system delivers.

**Action**: Add a runtime-boundary block at the top of the dispatch protocol
section (SKILL.md.tmpl, before line 167):

```markdown
## Runtime Boundary

The rules in this section are behavioral guidance for AI agents. Slipway's
core engine does not enforce subagent isolation, context budgets, parallel
dispatch, or atomic commits at runtime. Compliance depends on the host tool's
capabilities and the agent's adherence to these guidelines.

- Tools with subagent support (Claude Code Task tool, Codex --task) should
  follow the dispatch protocol below.
- Tools without subagent support should execute tasks inline sequentially.
- HARD RULE markers indicate rules that significantly impact quality when
  violated, not rules enforced by code.
```

**Files**:
- `internal/tmpl/templates/agents/slipway-orchestrator.md` — add boundary note
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` — add
  boundary block before dispatch protocol

### 5.2 Downgrade tool-specific dispatch to examples

**Problem**: SKILL.md.tmpl lines 226-290 hardcode dispatch syntax for specific
tools inside `{{- if eq .ToolID "claude" }}` / `{{- if eq .ToolID "codex" }}`
blocks. This couples the skill template to tool-specific APIs (Claude Code's
Task tool, Codex's `codex -q --task` CLI) and requires template updates when
tool APIs change.

**Action**: Restructure into a generic dispatch section + tool-specific
examples. Keep the Go template conditionals (`{{- if eq .ToolID "claude" }}`)
so each generated SKILL.md only includes the active tool's example — do NOT
ship all tools' examples in every output, as that wastes agent context tokens.

The template structure becomes:

```
### Dispatching a Task

Spawn a fresh agent/subagent for each task. Pass the task contract (ID, name,
files, action, verify, done criteria) as the agent's prompt. Do not pass full
file contents — pass paths only.

{{- if eq .ToolID "claude" }}
#### Example: Claude Code
Use the Task tool with subagent_type="slipway-executor" ...
{{- else if eq .ToolID "codex" }}
#### Example: Codex
Use codex -q --task ...
{{- else }}
#### Example: Generic
Spawn a subagent with the task contract as prompt ...
{{- end }}
```

The key change: tool-specific syntax moves from structural dispatch protocol
to a reference example section. The generic description above the conditional
is the contract; the conditional block is an illustration.

**Files**:
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl` —
  restructure dispatch section

---

## Wave 6: Lightweight Next Path

**Outcome**: Session-start uses a lightweight internal next path. Ordinary
`slipway next --preview --json` keeps its existing public contract.

### 6.1 Add lightweight preview path to next

**Problem**: `cmd/next.go:242-350` runs 15 computation steps for every `next`
call regardless of mode. The session-start hook does not need the fully
enriched payload on every startup.

However, ordinary `next --preview --json` is already a public agent-facing
contract: docs and tests assert `agent_hint`, `agent_definition_path`, and
other fields on the standard path. We cannot silently shrink the default JSON
shape just because the hook happens to call it today.

**Action**: Add a hidden/internal lightweight mode for hook callers. Do **not**
repurpose plain `next --preview --json`.

Example shape:

```go
func buildNextView(...) {
    // Common: state advancement attempt (skip in preview)
    // Common: basic view initialization

    if preview && lightweight {
        // Lightweight path:
        // 1. Resolve next skill name from state machine
        // 2. Load hook-needed blockers / preset-pending state
        // 3. Return lightweight view
        return lightweightNextView(root, ref, &view)
    }

    // Full public path (existing logic):
    // ... governance readiness, wave plan, catalog hints, etc.
}
```

This can be driven by a hidden `--hook-lite` flag or an equivalent internal
call boundary used only by generated hooks. The important constraint is: no
change to the ordinary public `next --preview --json` contract.

**Skip in lightweight path**:
- `EvaluateGovernanceReadiness()` (lines 282-315)
- `appendCatalogHints()` (line 149 in next_skill_view.go)
- Context budget estimation (line 164 in next_skill_view.go)
- Review context construction (lines 151-157 in next_skill_view.go)
- Skill constraints derivation (lines 159-161 in next_skill_view.go)

**Keep in lightweight path**:
- `ResolveNextSkill()` — needed for skill name
- Basic blocker detection — needed for hook context
- Preset confirmation check — needed to avoid stale prompts
- Lightweight agent dispatch hint resolution — keep `next_skill.name` plus the
  minimal agent-identification fields the hook needs

**Files**:
- `cmd/next.go` — add hidden/internal lightweight branch without changing the
  ordinary public path
- `cmd/next_skill_view.go` — extract `lightweightSkillView()` variant
- `internal/tmpl/templates/hooks/session-start.sh.tmpl` — use the hidden
  lightweight mode from the generated hook
- `cmd/next_test.go` — add tests for both the ordinary public path and the
  lightweight hook path

---

## Explicitly Not Done

### Not in scope (user requirements)

| Item | Reason |
|------|--------|
| Delete any of the 55 skills | User requirement: all skills retained |
| Model tiering | User requirement: not needed |

### Not in scope (already strong enough for this plan)

| Item | Evidence | Why excluded here |
|------|----------|-------------------|
| Behavior shaping breadth | Dozens of IRON LAW / red-flag / rationalization sections already exist across governance and agent templates | This plan is about reducing ceremony, not adding more doctrine |
| Wave planning primitives | Go engine `PlanWaves()` + tool-conditional dispatch in SKILL.md.tmpl | Internal capability exists; productized parallel runtime remains a separate boundary decision |
| Context telemetry | `next --json` returns full `contextBudget` struct (tokens, utilization, health, guard action, thresholds, breakdown) | The issue is push vs pull, not missing telemetry |
| Quick mode | Already shipped: `cmd/run.go:69`, `cmd/next.go:222` with `AdvanceOptions{QuickMode}` | No need to redesign it in this document |

### Owned by other plans

| Item | Owner |
|------|-------|
| `--quick` mode enhancements | thin-kernel Wave 4.2 |
| Toolgen simplification | thin-kernel Wave 3 (this is the main AI-native improvement — template generation cleanup) |
| Command surface reduction | thin-kernel Wave 5 |
| Structured AdvanceSummary | runtime-contract-hardening Wave 2 |
| Reconcile projection split | runtime-contract-hardening Wave 3 |

### Deliberately deferred

| Item | Reason |
|------|--------|
| Remove governance presets | Preset logic permeates readiness/artifact/gate; ROI too low |
| Merge 7 workflow states | State machine is test-suite skeleton; risk > benefit |
| External orchestrator boundary | Template honesty (Wave 5) is sufficient |
| Event-sourced state | Deferred in thin-kernel plan; needs design spike |
| Shrink ordinary `next --json` contract | Existing docs/tests/templates already depend on it; Wave 6 adds a hook-only fast path instead |
| Compact reinjection hook | Pull-based model (Wave 2) is simpler; can add superpowers-style compact reinjection later if pull-based proves insufficient |

---

## Execution Order

```
Wave 1 (fail-open defaults)          1 atomic PR, medium risk
  ├── 1.1 advisory-first controls    4 lines in config.go + test updates
  ├── 1.2 remove tdd-governance bind dispatcher + required-skill registry +
  │                                   next evidence/tests
  └── 1.3 remove preset_policy       delete override cascade in preset_policy.go
          override cascade            (MUST land with 1.1 — without it, 1.1 is negated)

Wave 2 (hook noise)                  1 PR, low risk
  ├── 2.1 delete post-tool hook      delete file + tool registry/settings merge
  └── 2.2 slim session-start         remove 6 lines + test updates

Wave 3 (surface simplify)            1 PR, medium risk — SKIP if thin-kernel 2B lands first
  └── 3.1 surface classes 4 → 2     surfaces.go + resolver.go + cmd flags +
                                     review/validate/repair suggested output

Wave 4 (state convergence)           1 solo PR, medium-high risk — do not combine
  └── 4.1 merge runtime-state.yaml  model tags + store/health/repair +
                                     migration path + compat policy

Wave 5 (template honesty)            1 PR, no risk (docs only)
  ├── 5.1 runtime boundary block    orchestrator.md + SKILL.md.tmpl
  └── 5.2 tool dispatch examples    SKILL.md.tmpl restructure

Wave 6 (next perf)                   1 PR, low risk
  └── 6.1 hook-only lightweight path next.go + next_skill_view.go + hook template
```

**Dependencies**:
- Waves 1, 2, 5 are fully independent — can run in parallel
- Wave 3 should land before thin-kernel Wave 2 Phase B (which removes flags);
  if Phase B lands first, skip Wave 3 instead of doing transitional churn
- Wave 4 is independent of all other waves
- Wave 6 depends on Wave 2 (hook slimming changes what session-start needs)
- Wave 6 must preserve the ordinary public `next --preview --json` contract

**Cross-plan ordering recommendation**:
```
ai-native Wave 1 + 2 + 5  (this plan, parallel)
  ↓
thin-kernel Wave 1         (kernel mechanics)
  ↓
ai-native Wave 3           (surface simplify)
  ↓
thin-kernel Wave 2         (capability demotion, builds on simplified surfaces)
  ↓
ai-native Wave 4           (state convergence)
  ↓
runtime-hardening Wave 1   (pure derivation, uses converged state)
  ↓
ai-native Wave 6           (lightweight next, after all query paths stabilize)
  ↓
thin-kernel Wave 3         (toolgen simplify)
  ↓
thin-kernel Wave 4         (gate thresholds, after advisory defaults land)
  ↓
runtime-hardening Wave 2-3 (AdvanceSummary + reconcile)
  ↓
thin-kernel Wave 5         (command surface reduction, last)
```

## Verification

After each wave:
```bash
go build ./...
go test ./... -count=1
```

Wave-specific checks:
- Wave 1: `go test ./internal/engine/control/... -v` +
  `go test ./internal/engine/progression/... -run Skill -v` +
  `go test ./internal/engine/governance/... -run Preset -v` +
  `go test ./internal/engine/skill/... -v`
  Verify: create a change with `--guardrail-domain auth_authz` and confirm
  domain-review shows as advisory (not blocking) in `slipway status --json`;
  confirm `next_skill` and required-skill evidence no longer disagree about
  `tdd-governance`
- Wave 2: `slipway init --tools claude` in test repo, verify no post-tool hook;
  verify generated settings contain no post-tool registration; verify
  session-start output has no context-guard
- Wave 3: `go test ./internal/engine/capability/... -v` +
  `go test ./cmd/... -run 'Route|Suggested|Repair' -v`
- Wave 4: Create a change, verify single `change.yaml` contains runtime
  fields. Load a legacy change with `runtime-state.yaml`, verify migration.
  Confirm `slipway health` / `slipway repair` no longer treat runtime-state as
  a live authority after migration. For a deliberately corrupted legacy
  `runtime-state.yaml`, verify the migration window emits only the compat-only
  unreadable-sidecar signal.
- Wave 5: `slipway init --tools claude` in test repo, read generated
  SKILL.md, verify boundary block present
- Wave 6: verify ordinary `slipway next --preview --json` still returns the
  existing enriched fields (`agent_hint`, `agent_definition_path`,
  `context_budget` when applicable). Verify the hidden hook path returns the
  reduced payload and is measurably faster in the same repo (benchmark
  before/after; no fixed absolute threshold)

## Exit Criteria

This plan is complete when:

- Only clarification/research remain blocking by default; review/execution
  controls default advisory unless user config overrides them
- GuardrailDomain does not override user-chosen preset or control modes
- No skill dispatch is domain-conditional at the kernel level
- Required-skill evidence no longer auto-injects `tdd-governance` for
  guardrail-domain execution
- No hook fires per-tool-call
- Session-start injects exactly 2 read-only queries (status + next preview)
- One YAML file per change (change.yaml) contains all mutable state
- Health/repair no longer treat `runtime-state.yaml` as active authority
- Template HARD RULEs are marked as behavioral guidance, not enforcement
- Tool-specific dispatch syntax is example content, not structural template
- Ordinary `next --preview --json` keeps its public contract; only the hidden
  hook path skips governance readiness and capability resolution
