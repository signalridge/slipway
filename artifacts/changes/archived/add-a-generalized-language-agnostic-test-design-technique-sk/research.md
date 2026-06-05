# Research

## Research Findings

### Architecture
- **Capability catalog registry** is the home for layer-② skills. Skills are Go
  records (`internal/engine/capability/registry.go:91` `Skill` struct) registered
  in `defaultSkills()` (`registry_default.go:5`), batched B1–B5. A new
  `test-design` skill is a new `registry_b6.go` entry registered there.
- **Binding-compare gate** enforces 1:1 equality between each catalog skill's
  Go record and its `SKILL.md` frontmatter: bindings
  (`gates_test.go:44` `TestFrontmatterMirrorsRegistryBindings`), hydrate refs
  (`gates_test.go:62`), required fields (`gates_test.go:115`), tier size budget
  (`gates_test.go:79`, T1 target 2560 / hard-max 6 KB), and a required source
  directory (`gates_test.go:268`). The `test-design/SKILL.md` frontmatter must
  mirror the registry exactly or these fail.
- **Technique-hint emission path** (consumed by both `next` and `run`, since
  `cmd/run.go:173` calls `buildNextView`): `appendCatalogHints`
  (`cmd/next_skill_view.go:547`) runs the capability resolver against the current
  host and emits `skill:<id>` hints for supports that pass
  `toolgen.ShouldExportAsHostSkill`. `appendWorkflowProfileTechniqueHints`
  (`cmd/next_skill_view.go:598`) adds profile hints. Both gate on the allowlist
  (`cmd/next_skill_view.go:566,604`).
- **Resolver supports** come from `BindingHostEmbedded` / `BindingTechniqueHint`
  matching `Signals.Host` (`resolver.go:114` `collectSupports`,
  `resolver.go:246` `pickSupportAttachment`), capped at 3, sorted by skill ID.
  Binding `test-design` via `BindingTechniqueHint` → `wave-orchestration` makes
  that host emit `skill:test-design`. **`tdd-governance` is intentionally NOT a
  hint target** (see Unknowns): it is `ExportOnlyExtra` and never resolves as a
  next-skill, so a hint bound to it would never emit in production.
- **Allowlist export** (`internal/toolgen/toolgen.go:352` `hostSkillExportAllowlist`)
  is the single switch that both gates the hint and drives `renderCatalogSkill`
  (`toolgen.go:808`) to export the host SKILL.md. `toolgen_test.go:209` pins the
  exported host-skill count (currently 22).
- **Change language source**: `model.Change.ProjectContext.Languages`
  (`internal/model/change.go:55` → `config.go:83`) first; codebase-map
  `STACK.md` (`internal/engine/artifact/codebase_map.go:449`, format
  `- Languages: Go, Python`; empty `joinFacts` ⇒ blank) is consulted **only as a
  fallback when `ProjectContext.Languages` is empty** — never merged. In this
  repo `STACK.md` lists `Go, Python` while the change is Go-only, so the
  precedence rule is load-bearing and pinned by a test.
- **`disabled_controls` scope**: the control IDs (`clarification`, `research`,
  `domain_review`, `independent_review`, `worktree_isolation`,
  `rollback_required`) are governance controls; none governs technique-hint
  emission. `capability:language-testing` therefore does not read
  `disabled_controls` and is pinned independent of it.
- **Part C surface — wave-orchestration**: the host SKILL.md.tmpl Dispatch
  Contract (`internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl:58-65`)
  already encodes host-enforced execution rules: "one isolated executor context
  per task" (line 59), "pass file paths only; executors read source in their own
  context" (line 60), and "for `task_kind=code`, enforce RED→GREEN→REFACTOR"
  (line 64). The task model already distinguishes `task_kind=test` vs
  `task_kind=code` (`internal/engine/wave/parse.go:252-254`) and orders work via
  waves + `depends_on`. `tdd-governance` already verifies test-first via git
  history (`tdd-governance/SKILL.md.tmpl:48-58`), so a frozen test-authoring
  commit is the natural RED proof.

### Patterns
- **Catalog skill = thin `SKILL.md` router + `references/*.md` depth**, declared
  via `HydrateReferences` (mirrors the `security-review/` pattern,
  `registry_b3.go:38`). `test-design` follows this exactly.
- **Technique-hint binding** is the established way a skill overlays a host
  without being on the progression path (`registry_b2.go:17` context-assembly →
  research-orchestration; `registry_b2.go:37` root-cause-tracing →
  wave-orchestration).
- **Per-language capability hint is genuinely new**: every existing emitted hint
  names a Slipway-owned skill and is allowlist-gated. The bridge adds a
  `capability:`-kind hint that is NOT allowlist-gated and names a capability +
  language the host resolves to its own installed skill.
- **Part C reuses existing structure**: test-before-impl is already expressible
  as a `task_kind=test` authoring task in an earlier wave + a `task_kind=code`
  task that `depends_on` it. The Dispatch Contract is the idiomatic place to add
  the context-isolation rule (consistent with lines 59/60/64).

### Risks
- **Binding-compare drift (medium):** any mismatch between registry and
  `test-design/SKILL.md` frontmatter fails a gate. Mitigation: author both from
  one source of truth, run `go test ./internal/engine/capability/...` early.
- **Fixture counts (low):** `registry_test.go:14` (ID list), `toolgen_test.go:209`
  (count 22→23), and the `skill_tree_inventory.codex.golden` manifest (new
  `slipway-test-design/` rows; regenerate via `UPDATE_GOLDEN=1 go test
  ./internal/toolgen/...`) must be updated. Because `test-design` binds to
  `wave-orchestration` only, `next_skill_capability_hints_test.go:387`
  (`TestAppendCatalogHintsVerifyHostsDoNotEmitRetiredFreshEvidence`, covering
  `tdd-governance`) stays correct **unchanged**. Only the `wave-orchestration`
  host hint assertions gain `skill:test-design`.
- **Multi-language behavior supersedes the issue (decided):** the issue says
  "emit at most one language hint"; the user chose **one hint per detected
  language**. Go-only changes still emit a single Go hint, so the issue's Go
  acceptance scenario is unaffected.
- **Part C / freshness machinery (HIGH if Option B):** `TaskPlanSemanticHash`
  (`parse.go:81`) gates all task evidence freshness via
  `ExpectedExecutionTaskFreshnessInputs` (`internal/state/execution_summary.go:311`).
  The repo's recent history (issues 53/72/74/77) is dominated by evidence-freshness
  fixes, so adding a hashed/persisted per-task field is the highest-risk path.
  Reversibility: Parts A/B are additive and trivially reversible; Part C risk
  scales with how deep it reaches into the wave-plan model.
- **Generalized-only guard:** Slipway-owned `test-design` templates must contain
  no language-specific test syntax (`t.Run`, `pytest`, `describe(`); enforced by
  a new guard test.

### Test Strategy
- **Existing coverage to extend:** `internal/engine/capability/gates_test.go`
  (frontmatter↔registry gates), `registry_test.go` (ID list),
  `internal/toolgen/toolgen_test.go` (host-skill count + render),
  `cmd/next_skill_capability_hints_test.go` (hint emission per host).
- **Infrastructure:** capability/toolgen tests are pure (no workspace);
  `cmd` hint tests call `appendCatalogHints(...)` directly and build governed
  changes via `withWorkspace`/`initTestWorkspace`/`createGovernedRequest`
  (per codebase-map `TESTING.md`).
- **New tests:** (1) `test-design` content test — references teach mock judgment,
  formal case enumeration, property reasoning; (2) generalized-only guard test
  scanning `test-design` templates for banned language syntax; (3) Part B
  language-hint tests — Go→one hint, Go+TS→two hints, no-language→none, dedupe,
  not allowlist-gated, present on both `next`/`run`, **ProjectContext precedence
  over STACK.md (Go-only context must not inherit STACK's Python)**, and
  **`disabled_controls` does not change emission**; (4) updated
  `wave-orchestration` host hint assertion to include `skill:test-design` (the
  `tdd-governance` "no hints" assertion stays as-is, since `test-design` does not
  bind there); (5) Part C template/content assertion (Option A — SKILL.md prose;
  no parser/model/hash test needed).
- **Verification:** `go build ./...` then `go test -count=1 ./...` before closeout
  (per codebase-map `CONCERNS.md` recheck routing).

## Alternatives Considered
Parts A (test-design skill) and B (language-aware capability hint) have a single
idiomatic implementation each, fixed by the issue + the binding-compare contract;
they carry no genuine design fork. The real fork is **Part C's mechanism** for
making "test the behavior, not the implementation" structural:

- **Option A — Dispatch-contract isolation (SKILL.md only, leverages existing
  model).** Extend `wave-orchestration` Dispatch Contract so a test-bearing unit
  is structured as a `task_kind=test` authoring step dispatched to an isolated
  context scoped to spec + public API signatures (never implementation),
  producing frozen tests, before a `task_kind=code` step that `depends_on` it and
  must satisfy those frozen tests; `tdd-governance` notes the frozen-test commit
  is the RED proof. *Tradeoffs:* no Go/model/hash change → lowest risk and fully
  backward compatible; the isolation is anchored in the existing
  wave/`task_kind`/`depends_on` structure + dispatch rules (not free-floating
  prose), enforced by the host exactly as lines 59/60/64 are today. No engine-level
  rejection if a host ignores it (same trust model as the rest of the dispatch
  contract).
- **Option C — Declared isolation marker (parser + view + SKILL).** Add a
  validated optional `tasks.md` metadata key (e.g. `test_isolation:`) to the
  parser whitelist + a wave-plan view field, with the Dispatch Contract acting on
  it. *Tradeoffs:* makes isolation an explicit, validated, declarable per-task
  property; ~2–3 Go files; partly redundant with the existing `task_kind=test`/
  `code` + `depends_on` expression; if added to `TaskPlanSemanticHash` it is
  backward compatible only because an absent key contributes nothing.
- **Option B — Full model enforcement (parser + hash + `WavePlanTask` +
  plan-audit gate + step evidence).** Thread the marker through the authoritative
  wave-plan model, hash, generation, a blocking plan-audit gate, and optional
  step-level evidence. *Tradeoffs:* strongest, engine-enforced and auditable, but
  ~10–12 Go files and it reaches into the freshness/hash machinery that recent
  issues (53/72/74/77) repeatedly destabilized — highest risk for the least
  marginal benefit over A.

- **Selected (user-confirmed 2026-06-05):** **Option A — Dispatch-contract
  isolation.** It achieves the structural-isolation goal using the wave model
  that already exists (test task isolated and frozen in an earlier wave; code
  task depends on it), keeps the isolation enforced by the same host dispatch
  contract that already governs executor context and RED→GREEN→REFACTOR, and
  stays clear of the high-risk freshness machinery. Concretely, Part C touches
  only `wave-orchestration/SKILL.md.tmpl` (a new isolation rule in the Dispatch
  Contract) and `tdd-governance/SKILL.md.tmpl` (the frozen-test commit is the RED
  proof); no Go/model/hash change. Option C (declared marker) and Option B (full
  engine enforcement) were considered and deferred — C is the fallback if a
  machine-validated per-task marker is later wanted; B is reserved for a future
  change that genuinely needs engine-level rejection.

## Unknowns
- Resolved: *Does the model already express test-before-implementation ordering?*
  → Yes — `task_kind=test`/`code` (`parse.go:252-254`) + waves + `depends_on`.
- Resolved: *Where does language come from for the bridge?* →
  `ProjectContext.Languages` first, then `STACK.md` (`codebase_map.go:449`).
- Resolved: *Is `tdd-governance` a live next-skill host?* → **No.**
  `ResolveNextSkill` (`internal/engine/progression/skill_resolution.go:13-30`)
  returns `wave-orchestration` for S2_EXECUTE and never `tdd-governance`, which is
  `ExportOnlyExtra` (`toolgen.go:233`). `technique_hints` are emitted only on the
  resolved next-skill's handoff, so a hint bound to `tdd-governance` would be
  visible only when a unit test calls `appendCatalogHints(nil, "tdd-governance",
  …)` directly — never in a real `next`/`run`. **Decision: scope the
  `test-design` technique-hint binding to `wave-orchestration` only** — this
  avoids a production-dead binding and the over-claim that `tdd-governance`
  surfaces the hint. Part C still delivers its `tdd-governance` change as SKILL.md
  prose (the RED-proof note), which reaches the host via SKILL.md export, not via
  `technique_hints`.
- Resolved: *Could the Dispatch-Contract isolation step / `tdd-governance`
  RED-proof note over-claim engine enforcement?* → No. Option A is host-enforced
  prose; the "no engine-rejection over-claim" constraint is now pinned by REQ-008's
  scenario and by task-001's Part C rendered-content assertions (matched on stable
  concept tokens, not exact prose). The final wording is authored in task-003 and
  confirmed at review — a verification step within the plan, not an open unknown.

## Assumptions
- The user's selection of Part C Option A (or C/B) will be recorded before
  execution. Evidence: research-orchestration HARD-GATE requires alternative
  selection.
- Go-only changes must keep emitting exactly one `capability:language-testing`
  hint. Evidence: issue #73 acceptance scenario 1; per-language emission is a
  superset that preserves it.
- `test-design` belongs in the `verification` domain alongside
  `coverage-analysis`/`property-testing`/`mutation-testing`. Evidence:
  `registry_b4.go:42-95`, user-confirmed in intake.
- **Codebase map is stale advisory context, not source-backed.** The
  `artifacts/codebase/*.md` map still describes the prior `slipway new`
  create-guard change, not issue #73's capability-registry / hint-path / toolgen
  surfaces (`codebase_map_status: partial`). This plan's source-backed context is
  the file:line citations in this research.md (verified against the current
  tree), not the map. The stale map is recorded as an explicit advisory gap in
  plan-audit; it is non-blocking and not relied upon for planning.

## Canonical References
- `internal/engine/capability/registry.go`, `registry_default.go`,
  `registry_b2.go`, `registry_b3.go`, `registry_b4.go` — catalog skill model.
- `internal/engine/capability/gates_test.go` — binding-compare contract.
- `internal/engine/capability/resolver.go` — support/hint resolution.
- `cmd/next_skill_view.go`, `cmd/next.go`, `cmd/next_handoff.go`, `cmd/run.go`
  — technique-hint emission shared by `next`/`run`.
- `internal/toolgen/toolgen.go` (allowlist + catalog export), `toolgen_test.go`.
- `internal/tmpl/templates/skills/security-review/` — template pattern to mirror.
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`,
  `tdd-governance/SKILL.md.tmpl`, `internal/engine/wave/parse.go`,
  `internal/model/wave_execution.go` — Part C surface.
- `artifacts/changes/add-a-generalized-language-agnostic-test-design-technique-sk/intent.md`
  — approved intake scope.
