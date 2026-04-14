# Skills Integration Plan

## 1. Goal

Refactor the current skills-integration design from a pack-centered plan into a
catalog-centered plan.

Slipway will distill the `skills_ref/` working set into a catalog of 25
independent Slipway skills organized by `domain x function`, then bind those
skills back into the existing Slipway framework through a Go-owned binding
registry and an automatic capability resolver.

Source corpus note: `skills_ref/` remains the authoritative source corpus. At
the time of this plan it contains 80 authoritative `SKILL.md` entries plus one
embedded sample fixture at `alirezarezvani/skill-tester/assets/sample-skill/
SKILL.md` that ships inside the `skill-tester` skill as test data. The fixture
is **not** a source-corpus entry and is excluded from disposition and
provenance-coverage accounting. If delivery batching uses a narrower working
set, that batching view must not replace disposition or provenance coverage
accounting.

This plan keeps four non-negotiable rules:

1. Keep a single kernel. `ResolveNextSkill` remains the only progression
   authority.
2. Do not require manual skill invocation. Inside Slipway, absorbed skills are
   selected and attached automatically.
3. Distill methods, not packages. Source skills are inputs to a new Slipway
   catalog, not vendored runtime units.
4. Keep routing authority in code, not prose. Generated `SKILL.md` files are
   descriptive and export-facing; runtime binding stays in Go registries.

## 2. Hard Constraints

### 2.1 Runtime authority

Slipway keeps one authoritative control loop:

1. `ResolveNextSkill` is the only progression authority.
2. The current state-selected governed hosts remain:
   `intake-clarification`, `research-orchestration`, `plan-audit`,
   `worktree-preflight`, `wave-orchestration`, `tdd-governance`,
   `spec-compliance-review`, `code-quality-review`, `goal-verification`, and
   `final-closeout`.
3. `review`, `validate`, `repair`, `status`, and `health` remain command
   surfaces. They may gain routers or views, but they do not become a second
   workflow engine.

### 2.2 Product boundary

This plan does not change Slipway's product identity:

1. Slipway remains multi-tool capable.
2. Slipway does not import a mission/work-package/dashboard runtime.
3. Slipway does not become a home-directory skill installer.
4. Heavy scanners and provider-specific tools stay behind explicit routed
   command paths.

### 2.3 Binding authority

The current implementation matters:

1. Toolgen currently renders adapter skills from `SKILL.md` and
   `SKILL.md.tmpl` sources.
2. Governance-skill loading currently parses `name` and `description`, then
   resolves runtime behavior from Go-owned defaults.
3. In current code truth there are 9 registry-backed governance definitions;
   `worktree-preflight` remains a kernel-owned standalone surface rather than a
   default registry entry.
4. Therefore, new catalog metadata in `SKILL.md` frontmatter is descriptive,
   auditable, and export-facing, but not the authority for runtime binding.
5. Any catalog-to-host or catalog-to-command binding introduced by this plan
   must live in a Go-owned registry and testable resolver path.

## 3. Target Architecture

### 3.1 Three-layer model

| Layer | Purpose | Authority |
|------|---------|-----------|
| Kernel layer | Governed progression through existing Slipway hosts | `ResolveNextSkill` and current governance logic |
| Catalog layer | 25 independent Slipway skills organized by `domain x function` | No progression authority |
| Binding layer | Maps catalog skills into hosts, routed commands, hints, views, and exports | Go-owned binding registry plus auto resolver |

Rules:

1. The catalog does not replace the kernel.
2. The kernel does not need one runtime state per independent skill.
3. Capability packs are demoted to documentation tags; they are no longer the
   primary architectural unit.

### 3.2 Independent skill contract

Each new Slipway skill is an independent unit defined by function, not by the
name of any upstream source skill.

Each target skill carries this conceptual contract:

| Field | Meaning |
|------|---------|
| `skill_id` | Stable Slipway skill identifier |
| `domain` | Concern area such as review, verification, or repair |
| `function` | The one job this skill performs |
| `tier` | `T1` core capability, `T2` specialist route, or `T3` diagnostic view |
| `primary_attachment` | One of `posture`, `procedure`, `checklist`, `tool-recipe`, or `report-schema` |
| `summary` | Trigger-oriented description mirrored into frontmatter and export surfaces |
| `trigger_signals[]` | Bounded trigger DSL clauses used by the capability resolver |
| `evidence_contract` | `verdict`, `artifact`, or `checklist` contract |
| `bindings[]` | Mirror of Go-owned host, command, hint, view, or export bindings |
| `provenance_ref` | Path to structured `provenance.yaml` for source absorption details |

Rules:

1. One source skill may feed multiple target skills.
2. One target skill may absorb multiple source skills.
3. `tier` encodes semantic role, not required binding count or density. A T1
   capability may still be narrowly bound (for example `threat-modeling` or
   `differential-review`); it is still T1 because it expresses a reusable
   method, not a tool route or a view.
4. `primary_attachment` is authored-side metadata. Runtime injection position
   is decided by the resolver based on attachment mode.
5. `bindings[]` are mirrored in authoring metadata, but runtime authority stays in
   Go registries.
6. `trigger_signals[]` use a bounded operator set; they are not arbitrary prose.
7. `provenance_ref` is structured so by-source indexing stays auditable and
   coverage checks can be automated.

### 3.3 Binding types

Each catalog skill may bind to one or more of these targets:

| Binding type | Meaning |
|------|---------|
| `host-embedded` | Injected into a governed host as directive, checklist, or partial |
| `command-auto` | Selected automatically by a routed command |
| `command-manual` | Addressable by explicit command flag override |
| `technique-hint` | Surfaced through the existing `TechniqueHints` surface in `cmd/next_skill_view.go` without affecting progression |
| `command-view` | Exposed as a read-only command surface or diagnostics view |
| `export-only` | Materialized for adapter export, not for core runtime |

Rules:

1. `technique-hint` reuses the existing `TechniqueHints` rendering path; the
   Go side returns skill id plus hint kind, and the host LLM organizes the
   actual hint text.
2. Binding type determines runtime attachment surface; attachment mode (see
   §3.3.1) determines how content is shaped when attached.

### 3.3.1 Attachment modes

Every catalog skill declares one `primary_attachment` and may extend per
binding. The five modes are frozen:

| Mode | Meaning | Typical carrier |
|------|---------|-----------------|
| `posture` | Persistent stance injected at the top of a host prompt | "enforce TDD"; "fresh verification required" |
| `procedure` | Ordered steps | `RED -> GREEN -> REFACTOR`; `Extract -> Dedup -> Reframe -> Anchor` |
| `checklist` | Discrete check items | security review items; spec-trace pairs |
| `tool-recipe` | Tool or command invocation pattern | semgrep config; codeql query scaffold |
| `report-schema` | Structured output constraint | verdict shape; incident timeline schema |

Rules:

1. The five modes are frozen in `docs/distillation/schema.md`.
2. Typical template mapping: `PROSE.tmpl` -> `posture` / `procedure`;
   `CHECKLIST.tmpl` -> `checklist`; `VERDICT.tmpl` -> `report-schema`;
   `tool-recipe` lives in `scripts/` or inline.
3. The resolver uses attachment mode to decide injection position (prompt
   top, checklist section, tool-invocation hint, output-constraint section).
4. `primary_attachment` is mandatory; additional attachment modes may be
   declared per binding when a single skill carries multiple shapes.

### 3.4 Auto capability resolver

Slipway remains AI-driven by resolving catalog skills automatically instead of
asking the operator to invoke them manually.

The shipped auto capability resolver currently consumes the
`internal/engine/capability.Signals` set:

1. explicit command context such as `review`, `validate`, `repair`, `status`,
   or `health`
2. current governed host when attaching support skills
3. blocker reasons
4. changed-file signals
5. referenced path signals
6. user-text matches when the caller provides them

The caller may derive those signals from workflow state/sub-step, guardrail
classification, evidence freshness, or artifact context, but those are not
first-class fields in the current shipped resolver struct.

It returns:

1. one bound route when a command needs an automatic mode/view choice
2. zero to three ranked support skills when a governed host needs additional
   guidance
3. `hydrate_references[]` indicating which `references/*.md` files should be
   injected on demand (spec-kitty-style conditional hydration)
4. an optional `llm_tiebreak` record listing candidate skill ids plus a
   decision criterion when DSL scoring produces ties; the host LLM resolves
   the tie against user text in-prompt
5. a short `reason` for every automatic attachment, copied from the matched
   trigger clause

Rules:

1. The resolver must never change the next governed host selected by
   `ResolveNextSkill`.
2. Explicit operator flags override automatic route selection.
3. Manual skill invocation is not required for absorbed catalog skills.
4. Exported skills may still be invoked directly by other tools; that does not
   change Slipway's internal runtime model.
5. `hydrate_references[]` and `llm_tiebreak` are optional extension outputs,
   not mandatory B1 fields. B1 only proves route selection, support
   attachment, and `technique-hint`.
6. `technique-hint` bindings emit skill id and hint kind through the existing
   `TechniqueHints` surface in `cmd/next_skill_view.go`; Go does not own the
   hint wording.
7. `hydrate_references[]` first enters no earlier than B2, when
   reference-heavy capabilities such as `context-assembly` become real owners
   of conditional hydration.
8. `llm_tiebreak` is the only documented AI hand-off, but it is introduced
   only when a later batch creates a genuine unresolved DSL tie. B1 does not
   implement or test it.

### 3.5 Source authoring layout

The catalog skill source layout becomes a fixed core contract with constrained
support directories:

```text
internal/tmpl/templates/skills/<skill-id>/
  SKILL.md
  provenance.yaml
  PROSE.tmpl           # optional
  CHECKLIST.tmpl       # optional
  VERDICT.tmpl         # optional
  references/          # optional
  scripts/             # optional
```

Notes:

1. `SKILL.md` and `provenance.yaml` are the required core files for every
   catalog skill.
2. `PROSE.tmpl`, `CHECKLIST.tmpl`, and `VERDICT.tmpl` are typed optional
   templates consumed only when a binding or evidence contract needs them.
3. `references/` is a one-hop support shelf for long-form examples,
   anti-patterns, framework variants, and source notes; it is never routing
   authority.
4. `scripts/` is reserved for deterministic helpers, validators, aggregators,
   and report generators with explicit input/output contracts; it is never
   progression authority.
5. Assembler order is fixed: frontmatter -> `SKILL.md` body -> conditional
   injection of `PROSE` / `CHECKLIST` / `VERDICT` by binding type.
6. Support directories are opt-in. A reference or script must be named from the
   skill body or assembler config before it is consumed.
7. Catalog skills now use the assembler. Existing non-catalog governed hosts
   may still use their single-file or `.tmpl` sources directly.

### 3.6 Distillation workflow

Every source-skill absorption follows the same four-step workflow:

1. `Extract`: keep only trigger conditions, decision rules, counterexamples,
   and evidence-bearing checks.
2. `Deduplicate`: merge repeated rules into the most specific formulation; if
   sources conflict, keep the more conservative rule and record the conflict in
   `provenance.yaml`.
3. `Reframe`: rewrite around the target Slipway skill's single function instead
   of preserving source-skill naming, tone, or narrative structure.
4. `Anchor`: keep a rule only if it maps to `trigger_signals[]`,
   `evidence_contract`, or one typed template consumer.

Rules:

1. Narrative background, long examples, and source-specific storytelling move to
   `references/` or are dropped.
2. Rules that cannot be anchored to runtime selection, evidence, or typed
   prompt assembly are removed instead of being carried as prose noise.
3. The catalog layer remains lean by default: CI targets compact `SKILL.md`
   bodies and pushes overflow into typed templates or `references/`.
4. The distillation work itself is executed by a Claude Code session acting as
   the distiller; there is no separate `slipway distill` subcommand. Session
   hand-off between batches is carried by the merged `provenance.yaml` plus
   `docs/distillation/by-source.md`, not by bespoke hand-off notes.

### 3.7 Trigger DSL

`trigger_signals[]` is a bounded DSL owned by Go rather than free-text matching
rules.

Example:

```yaml
trigger_signals:
  - all_of:
      - command: review
      - path_includes: ".github/workflows"
      - changed_files_include: "**/*.{yml,yaml}"
    reason: "GitHub Actions workflow modified under review"
```

Supported operators in the first cut:

- `all_of`
- `any_of`
- `not`
- `command`
- `host`
- `blocker_reason`
- `changed_files_include`
- `path_includes`
- `user_text_matches`

Rules:

1. The operator list is frozen in Go under `internal/engine/capability/trigger.go`.
2. Scoring remains Go-owned; the resolver ranks matches and returns one routed
   command mode/view or up to three support attachments.
3. Trigger clauses are declarative evidence for routing, not a second workflow
   engine.

### 3.8 Structured provenance

Each catalog skill carries a structured `provenance.yaml` so that source
tracking, conflict notes, and by-source indexing stay auditable instead of
being left as loose narration.

Minimum shape:

```yaml
sources:
  - source: superpowers/systematic-debugging
    absorbed_as: standalone
    extracted:
      - trace the root cause before fixing
    dropped:
      - long narrative debugging stories
    conflicts_with: []
```

Rules:

1. Every source whose `by-source.md` disposition is `standalone` or
   `partial-only` must appear in either `extracted`, `dropped`, or
   `conflicts_with`. `posture-only`, `absorbed`, `view-only`, `route-only`,
   and `deferred` remain tracked in `by-source.md` rather than provenance-gated.
2. `absorbed_as` records whether the source became a standalone target,
   posture-only input, or partial-only input.
3. `docs/distillation/by-source.md` is a manually maintained reverse index that
   cites `provenance.yaml` and rollout status; it is not a generated source of
   truth.

### 3.9 Distillation quality gates

Distillation is not considered complete until these gates pass in CI:

1. `schema-lint`: read-only parsing of frontmatter, typed-template references,
   and trigger operators, asserting valid structure and operator whitelist use.
2. `size-lint`: read-only measurement of `SKILL.md` body size and prompt-density
   discipline. Budgets vary by tier:
   - T1 core capability: target <= 2 KB; warn 2-6 KB; rationale required above 6 KB.
   - T2 specialist route: target <= 3 KB (tool-recipe overhead); warn 3-8 KB;
     rationale required above 8 KB.
   - T3 diagnostic view: target <= 1.5 KB; report-schema-first, with concise
     posture or anti-pattern context allowed inside the same budget.
   Warning-band entries are informational logs only.
   Failures are reserved for unbounded prose, long examples that should move to
   typed templates or `references/`, or oversized bodies without an approved
   exception.
3. `binding-compare`: read-only comparison between authoring `bindings[]` and
   the Go-owned registry, requiring a 1:1 match.
4. `provenance-coverage-scan`: read-only scan that checks whether every
   `standalone` / `partial-only` source listed in `by-source.md` appears in the
   union of `provenance.yaml` records, plus the reverse check that every
   provenance source is indexed in `by-source.md`.

Gate automation is now implemented in Go tests and expected in CI. During the
earlier B0-B7 rollout these gates were enforced by PR-review checklist until
the registry schema, frontmatter contract, and by-source index stabilized.

### 3.10 Pattern absorptions from superpowers and spec-kitty

Two source frameworks shape Slipway's distillation posture without being
imported as runtime units:

| Absorbed pattern | Source | Slipway landing |
|---|---|---|
| `description`-as-dispatcher | `superpowers` | Every catalog `SKILL.md` frontmatter `summary` uses the `Use when ... / Triggers on ...` phrasing so export-facing adapters can rely on description-level triage. |
| Catalog entrypoint manifest | `superpowers/using-superpowers` | Toolgen export generates a single `using-slipway-catalog.md` aimed at external agents. It does not enter the Slipway kernel. |
| Conditional `references/` hydration | `spec-kitty` / `runtime-next` | Schema reserves `hydrate_references[]`; once emitted by a later resolver batch, the host decides whether to inline the cited `references/*.md` file. |
| Auto-discovery manifest posture | `sickn33/agent-orchestrator` | Informs toolgen multi-file assembler behavior at B8 rather than becoming a standalone catalog skill. |

Rules:

1. These absorbed patterns are declarative contracts; they do not add runtime
   progression authority.
2. No source repository is mirrored one-for-one; only the pattern is absorbed.

## 4. Domain x Function Catalog

`skills_ref/` remains the authoritative source corpus for this plan. Any
rollout batching working set must be enumerated explicitly in `by-source.md` or
the disposition matrix rather than silently narrowing provenance coverage.

### Tier distribution

The 25 catalog skills split across three tiers (see §3.2 for tier semantics):

| Tier | Count | Members |
|---|---|---|
| **T1** core capability | 18 | `scope-clarification`, `context-assembly`, `plan-authoring`, `tdd-proof`, `parallel-executor-contract`, `fresh-verification-evidence`, `root-cause-tracing`, `independent-review`, `multi-reviewer-calibration`, `security-review`, `threat-modeling`, `spec-trace`, `differential-review`, `variant-analysis`, `coverage-analysis`, `property-testing`, `mutation-testing`, `performance-profiling` |
| **T2** specialist route | 6 | `sast-orchestration`, `gha-security-review`, `supply-chain-audit`, `ci-triage`, `review-comment-triage`, `git-recovery` |
| **T3** diagnostic view | 1 | `incident-response` |

Tier is a semantic-role tag. Some T1 skills (for example `threat-modeling` or
`differential-review`) are narrowly bound but remain T1 because they express
reusable methods, not tool routes or views.

### A. Intake and Framing

| # | Skill | Function | Primary bindings | Source inspirations |
|---|---|---|---|---|
| 1 | `scope-clarification` | converge intent and scope before planning | `intake-clarification`, `technique-hint` | `brainstorming`, `ask-questions-if-underspecified` |
| 2 | `context-assembly` | assemble product, codebase, and risk context | `research-orchestration`, `plan-audit`, `technique-hint` | `context-driven-development`, `audit-context-building`, `spec-kitty` action-scoped context posture |
| 3 | `plan-authoring` | turn requirements into bounded, auditable implementation tasks | `plan-audit`, `host-embedded`, `export-only` | `writing-plans`, `workflow-patterns`, `agent-workflow-designer` |

### B. Execution Discipline

| # | Skill | Function | Primary bindings | Source inspirations |
|---|---|---|---|---|
| 4 | `tdd-proof` | enforce RED-GREEN-REFACTOR and test-first proof | `tdd-governance`, `wave-orchestration`, `technique-hint` | `test-driven-development`, `workflow-patterns` |
| 5 | `parallel-executor-contract` | bounded parallel subagent dispatch with reviewable handoff | `wave-orchestration` | `dispatching-parallel-agents`, `subagent-driven-development`, `spec-kitty-implement-review` |
| 6 | `fresh-verification-evidence` | block completion claims without fresh commands and fresh proof | `goal-verification`, `final-closeout`, `tdd-governance` | `verification-before-completion` |

### C. Debugging

| # | Skill | Function | Primary bindings | Source inspirations |
|---|---|---|---|---|
| 7 | `root-cause-tracing` | trace root cause before attempting fixes, including competing-hypothesis branches when needed | `wave-orchestration`, `repair`, `technique-hint` | `systematic-debugging`, `debugging-strategies`, `debug-buttercup` triage posture, `parallel-debugging` |

### D. Code Review - Quality

| # | Skill | Function | Primary bindings | Source inspirations |
|---|---|---|---|---|
| 8 | `independent-review` | fresh-context review with explicit verdict contract plus review handoff discipline | `spec-compliance-review`, `code-quality-review`, `review` | `code-review`, `code-reviewer`, `code-review-excellence`, `spec-kitty-runtime-review`, `requesting-code-review`, `receiving-code-review` |
| 9 | `multi-reviewer-calibration` | dedupe findings and calibrate severity across reviewers | `code-quality-review`, `review` | `multi-reviewer-patterns`, `adversarial-reviewer`, `code-review-ai-ai-review` |

### E. Code Review - Security

| # | Skill | Function | Primary bindings | Source inspirations |
|---|---|---|---|---|
| 10 | `security-review` | secure-default and framework-specific security review | `review`, `spec-compliance-review`, `code-quality-review` | `insecure-defaults`, `sharp-edges`, `security-review`, `security-best-practices` |
| 11 | `threat-modeling` | trust-boundary, abuse-path, and owner-aware threat modeling | `review`, `validate`, `export-only` | `security-threat-model`, `security-ownership-map` |
| 12 | `gha-security-review` | review GitHub Actions and AI-agent CI attack paths | `review`, `repair` | `gha-security-review`, `agentic-actions-auditor` |
| 13 | `supply-chain-audit` | dependency, takeover, CVE, and license risk review | `review`, `repair`, `status` | `supply-chain-risk-auditor`, `dependency-auditor` |
| 14 | `sast-orchestration` | run and merge Semgrep, CodeQL, and SARIF-based findings | `review`, `validate`, `repair` | `semgrep`, `codeql`, `sarif-parsing`, `audit-augmentation` |

### F. Code Review - Change Shape

| # | Skill | Function | Primary bindings | Source inspirations |
|---|---|---|---|---|
| 15 | `differential-review` | risk-prioritized diff review with blast-radius awareness | `review` | `differential-review`, `find-bugs`, `pr-review-expert` |
| 16 | `variant-analysis` | search for variants of already-known bug or vulnerability patterns | `review`, `repair` | `variant-analysis` |
| 17 | `spec-trace` | bidirectional spec-to-code and code-to-spec trace review | `spec-compliance-review`, `validate`, `review` | `spec-to-code-compliance`, `spec-kitty-mission-review` |

### G. Verification

| # | Skill | Function | Primary bindings | Source inspirations |
|---|---|---|---|---|
| 18 | `coverage-analysis` | coverage plus critical-path and end-to-end proof review | `validate`, `goal-verification` | `coverage-analysis`, `e2e-testing-patterns` |
| 19 | `property-testing` | invariant, round-trip, and decoder property testing | `validate`, `goal-verification` | `property-based-testing` |
| 20 | `mutation-testing` | run mutation campaigns and interpret signal strength | `validate`, `goal-verification` | `mutation-testing` |
| 21 | `performance-profiling` | profiling, before/after comparison, and load-oriented verification | `validate`, `goal-verification`, `status` | `performance-profiler`, distributed-tracing checklist material |

### H. Repair and CI Loop

| # | Skill | Function | Primary bindings | Source inspirations |
|---|---|---|---|---|
| 22 | `ci-triage` | summarize CI failures and produce bounded remediation plan | `repair`, `status` | `gh-fix-ci`, `iterate-pr` |
| 23 | `review-comment-triage` | fetch, classify, and process PR or issue comments | `repair` | `gh-address-comments`, `iterate-pr` |
| 24 | `git-recovery` | recover from rebase, bisect, reflog, worktree, or hook-bypass problems | `repair`, `status`, `worktree-preflight` failure support | `git-advanced-workflows`, `spec-kitty-git-workflow`, `block-no-verify-hook` |

### I. Ops and Diagnostics

| # | Skill | Function | Primary bindings | Source inspirations |
|---|---|---|---|---|
| 25 | `incident-response` | severity classification, timeline reconstruction, and PIR flow | `status`, `health`, `export-only` (T3 diagnostic view; not routed through `repair`) | `incident-commander`, `incident-response`, `acceptance-orchestrator` gate posture |

### J. Non-catalog disposition matrix

| Source / surface | Disposition | Landing zone | Reason |
|---|---|---|---|
| `review-queue` | `view-only` | `status` view | thin queue aggregation wrapper, not a reusable method skill |
| `observability-query` | `view-only` | `status` / `health` view | read-only inspection surface, better modeled as diagnostics view |
| `claude-settings-audit` | `view-only` | `health` / `validate` diagnostics | repo permission and config audit belongs in diagnostics rather than the governed runtime method layer |
| `skill-scanner` | `view-only` | `health` / `validate` diagnostics | skill security checking is better surfaced as an audit/report view |
| `skill-security-auditor` | `view-only` | `health` / `validate` diagnostics | high overlap with `skill-scanner`; retained as security-audit input rather than a catalog node |
| `skill-tester` | `view-only` | `validate` diagnostics | quality gate and reporting surface, not a governed workflow method |
| `gh-review-requests` | `view-only` | `status` review queue view | queue/query helper rather than a reusable method node |
| `sentry` | `view-only` | `status` / `health` observability view | provider-specific read-only query wrapper |
| `second-opinion` | `route-only` | explicit `review` route or override | valuable review surface, but not a core reusable method |
| `skill-factory` | `deferred` | future repo-local command family | current CLI does not expose a `skill` command family |
| `prompt-governance` | `deferred` | future prompt-system governance surface | real capability, but outside the current code-change governance rollout |
| `agent-workflow-designer` | `absorbed` | `plan-authoring` authoring guidance | authoring meta-skill is better distilled into SOP/checklists |
| `designing-workflow-skills` | `absorbed` | distiller SOP / `plan-authoring` guidance | workflow-skill design rules belong to the authoring process rather than the runtime catalog |
| `writing-skills` | `absorbed` | distiller SOP / adapter export guidance | TDD-for-skills process is for authors, not runtime |
| `antigravity-workflows` | `absorbed` | distiller SOP / workflow routing heuristics | orchestration meta-skill should not become a Slipway runtime unit |
| `acceptance-orchestrator` | `absorbed` | `incident-response` / gate posture | gate posture is preserved without promoting a standalone surface |
| `block-no-verify-hook` | `absorbed` | `git-recovery` / policy guidance | hook-specific policy, not a reusable catalog method |
| `spec-kitty-charter-doctrine` | `absorbed` | `plan-authoring` / runtime constraints commentary | doctrine framing is already absorbed into planning and runtime constraints |
| `simplification-pass` | `absorbed` | `independent-review` and `code-quality-review` partials | better as an internal review technique than a standalone node |
| `review-request-response` | `absorbed` | `independent-review` and `review-comment-triage` | spans two lifecycle points and creates noisy boundaries |
| `hypothesis-arbitration` | `absorbed` | `root-cause-tracing` | overlaps heavily with the debugging core and is cleaner as an advanced branch |

### K. Sources absorbed as posture, not promoted as standalone skills

| Source | Absorbed into |
|---|---|
| `superpowers/using-superpowers` | project- and agent-level skill-first posture text |
| `superpowers/executing-plans` | `plan-authoring` execution-contract sections |
| `spec-kitty/mission-system` | `plan-authoring` taxonomy and procedure commentary |
| `spec-kitty/runtime-next` | Slipway runtime documentation and resolver constraints |
| `sickn33/agent-orchestrator` | auto capability resolver matching heuristics |
| `wshobson/error-handling-patterns` | `independent-review` and `code-quality-review` partials |

## 5. Binding and Distillation Model

### 5.1 Binding registry

This refactor introduces a dedicated binding registry, conceptually under:

```text
internal/engine/capability/
  registry.go
  trigger.go
  resolver.go
  provenance.go
```

Responsibilities:

1. own the 25-skill catalog metadata
2. own the bounded trigger DSL operator set and evaluator rules
3. own binding targets for hosts, command routes, hints, and views
4. keep runtime routing testable and independent from rendered prose files
5. expose the minimum export metadata needed by tool adapters

### 5.2 Host bindings

Governed hosts remain few, but they absorb catalog skills intentionally:

| Governed host | Bound catalog skills |
|---|---|
| `intake-clarification` | `scope-clarification` |
| `research-orchestration` | `context-assembly` |
| `plan-audit` | `plan-authoring`, `context-assembly` |
| `worktree-preflight` | kernel-owned; current runtime also host-embeds `git-recovery` here as worktree failure support |
| `wave-orchestration` | `tdd-proof`, `parallel-executor-contract`, `root-cause-tracing` |
| `tdd-governance` | `tdd-proof`, `fresh-verification-evidence` |
| `spec-compliance-review` | `independent-review`, `spec-trace`, `security-review` |
| `code-quality-review` | `independent-review`, `multi-reviewer-calibration`, `security-review`, plus embedded simplification guidance |
| `goal-verification` | `fresh-verification-evidence`, `coverage-analysis`, `property-testing`, `mutation-testing`, `performance-profiling` |
| `final-closeout` | `fresh-verification-evidence` plus residual-risk closeout language |

### 5.3 Command bindings

Command surfaces bind catalog skills through auto routes first, then optional
manual overrides:

| Command | Catalog skills |
|---|---|
| `review` | `independent-review`, `multi-reviewer-calibration`, `security-review`, `threat-modeling`, `gha-security-review`, `supply-chain-audit`, `sast-orchestration`, `differential-review`, `variant-analysis`, `spec-trace`, plus `second-opinion` as an explicit route or override |
| `validate` | `spec-trace`, `coverage-analysis`, `property-testing`, `mutation-testing`, `performance-profiling` |
| `repair` | `root-cause-tracing`, `ci-triage`, `review-comment-triage`, `git-recovery`, `supply-chain-audit`, `gha-security-review`, `variant-analysis` |
| `status` | `incident-response`, `supply-chain-audit`, `ci-triage`, `performance-profiling` summaries, plus `review-queue` and `observability-query` as views |
| `health` | diagnostics-first integrity and observability views; current change-scoped auto-route defaults to `incident-response`, with `observability-query` available as an explicit view override |

Rules:

1. Routed command bindings are shipped in current code:
   `review` / `validate` / `repair` expose `--mode`, and
   `status` / `health` expose `--view`.
2. Automatic selection is the default posture.
3. Explicit `--mode` / `--view` overrides take precedence over resolver
   auto-route fallback. Route-only non-catalog overrides are also accepted:
   `review --mode second-opinion`,
   `status --view review-queue|observability-query`,
   `health --view observability-query`.
4. `status` / `health` currently use a shared payload renderer. For concrete
   active/selected changes, current auto-route selects the shipped
   `incident-response` T3 view; in diagnostics fallback with no active change,
   `view` remains empty unless the operator passed `--view`.
5. Current non-catalog explicit `--view` overrides are
   `review-queue` and `observability-query`. Other `view-only` entries remain
   documented diagnostics landing zones, and `validate` has no standalone
   `--view` selector in current code.
6. `incident-response` is T3: it binds only to `status` / `health` / export,
   not to `repair`. `repair` routing leans on `root-cause-tracing`,
   `ci-triage`, and `git-recovery` instead.
7. `fresh-verification-evidence` remains host-bound (`goal-verification`,
   `final-closeout`, `tdd-governance`) and is not a direct `validate` route.
8. `command-auto` is reserved for low-latency, high-signal defaults.
   Scanner-heavy or provider-coupled routes stay `command-manual` and are
   selected explicitly through `--mode`.

### 5.4 Distillation documentation

The documentation model becomes catalog-first:

```text
docs/distillation/
  schema.md
  catalog.md
  by-source.md
  domains/
    intake-and-framing.md
    execution-discipline.md
    debugging.md
    code-review-quality.md
    code-review-security.md
    code-review-change-shape.md
    verification.md
    repair-and-ci.md
    ops-and-diagnostics.md
  routed-surfaces.md
```

`schema.md` freezes:

1. the authoring contract for frontmatter, typed templates, and `provenance.yaml`
2. the bounded trigger DSL operators that the resolver may consume
3. the CI gates that determine whether a distilled catalog skill is mergeable

`catalog.md` is target-indexed:

1. one row per Slipway catalog skill
2. domain, function, primary attachment, bindings, and provenance summary
3. implementation status and test coverage status

`by-source.md` is source-indexed, but manually maintained:

1. one row per authoritative source-corpus entry
2. its disposition, the target catalog skills that consume it, and rollout status
3. auditability maintained by citing `provenance.yaml`, not by code generation

`routed-surfaces.md` records:

1. the fixed list of `view-only`, `route-only`, and `deferred` surfaces
2. the command landing zone and boundary for each surface
3. which sources are intentionally classified as non-catalog

### 5.5 Why packs are no longer the main architecture

Capability packs still exist as tags and documentation views, but no longer
define the system shape.

Reasons:

1. packs are useful for surveying work, not for binding runtime behavior
2. `domain x function` yields cleaner skill boundaries
3. the binding layer can attach one skill to multiple surfaces without turning
   packs into pseudo-runtime objects

## 6. Rollout Record

This section records the B0-B8 rollout that is now implemented. Deferred
surfaces such as `skill-factory` and `prompt-governance` remain deferred, but
the catalog registry, routed flags, assembler, export manifest, and Go test
gates described below are shipped.

### 6.1 Batch map

| Batch | Purpose | Primary deliverables | Gate to advance |
|---|---|---|---|
| **B0** | Contract freeze | `docs/distillation/schema.md` frozen (tier, attachment modes, trigger DSL operators); `catalog.md`, `by-source.md`, `routed-surfaces.md` skeletons; `provenance.yaml` schema frozen | Schema review signed off |
| **B1** | End-to-end proof | `internal/engine/capability/{registry,trigger,resolver,provenance}.go`; wiring into `TechniqueHints` (`cmd/next_skill_view.go`); full distillation of five foundation T1 skills: `scope-clarification`, `plan-authoring`, `tdd-proof`, `fresh-verification-evidence`, `independent-review`; tests for registry load + resolver selection + hint emission | End-to-end loop demonstrably functional in tests |
| **B2** | Scale foundation | Remaining foundation T1 skills: `context-assembly`, `parallel-executor-contract`, `root-cause-tracing`, `security-review`, `spec-trace` | Multi-skill resolver stability demonstrated |
| **B3** | Security cluster | T1 `threat-modeling` + T2 `sast-orchestration`, `gha-security-review`, `supply-chain-audit` | T2 command-route binding proven |
| **B4** | Change shape + verification | T1 `multi-reviewer-calibration`, `differential-review`, `variant-analysis`, `coverage-analysis`, `property-testing`, `mutation-testing`, `performance-profiling` | |
| **B5** | Repair/CI + ops | T2 `ci-triage`, `review-comment-triage`, `git-recovery` + T3 `incident-response` | T3 view-only binding proven |
| **B6** | Non-catalog cleanup | `routed-surfaces.md` finalized; six posture-only absorptions annotated; disposition matrix closed | Provenance coverage scan clean for all `standalone` / `partial-only` by-source rows |
| **B7** | Routed command rollout | `review` / `validate` / `repair` auto routing and `--mode` flag; `status` / `health` `--view` flag; resolver tests for route selection and fallback | Routed flags shipped and verified |
| **B8** | Export + gate automation | Toolgen multi-file assembler; `using-slipway-catalog.md` export; automate `schema-lint`, `size-lint` (tier-aware), `binding-compare`, `provenance-coverage-scan` | Gates enforce via CI, no manual review required |

### 6.2 B1 foundation set (locked)

B1 distils these five T1 catalog skills to prove host absorption, hint
emission, and command binding all end-to-end:

1. `scope-clarification` - intake host + technique-hint; attachment: `posture` + `checklist`
2. `plan-authoring` - plan-audit host + host-embedded; attachment: `procedure` + `checklist`
3. `tdd-proof` - tdd-governance and wave-orchestration hosts; attachment: `procedure`
4. `fresh-verification-evidence` - goal-verification and final-closeout hosts; attachment: `checklist` + `report-schema`
5. `independent-review` - spec-compliance-review and code-quality-review hosts, plus `review` command; attachment: `procedure` + `checklist` + `report-schema`

This set covers four distinct governed hosts, one routed command, and four
of the five attachment modes. B2 adds the other five foundation skills;
together they complete the `docs/plans/.../plan.md` foundation ten.

### 6.3 Batch execution rules

1. Each batch lands as one PR. Inter-batch context transfer relies solely on
   merged `provenance.yaml` files and the maintained
   `docs/distillation/by-source.md`; no bespoke hand-off documents.
2. Conflict adjudication default: when sources disagree on a rule, merge
   conservatively, record the conflict under `conflicts_with` in
   `provenance.yaml`, and list the conflict in the PR body. Do not stall the
   batch on single-rule disagreements.
3. Rules unresolved by conservative merge and flagged for escalation block
   only their own skill, not the whole batch.
4. EN and zh-CN documents are updated in the same PR.

### 6.4 Historical rollout guardrails

1. B1 proved the registry and resolver before the catalog expanded to all 25
   skills.
2. CI gates were intentionally deferred until B8; they are now implemented in
   Go tests and expected in CI.
3. `--mode` / `--view` shipped in B7 and are now live.
4. Routed command rollout landed after the foundation batches rather than being
   mixed into the initial distillation proof.

## 7. Non-Goals

1. Do not add a second progression kernel beside `ResolveNextSkill`.
2. Do not require operators to manually invoke absorbed skills during normal
   Slipway flows.
3. Do not mirror source repositories one-for-one inside Slipway.
4. Do not treat generated `SKILL.md` frontmatter as runtime binding authority.
5. Do not create one top-level command per catalog skill.
6. Do not import mission/work-package/dashboard/doctrine runtime behavior.
7. Do not make capability packs the primary architecture again.
8. Do not reduce Slipway to a tool-specific skill installer.
9. Do not keep thin queue, observability, or review wrappers as standalone
   catalog skills when they fit better as routed surfaces.
