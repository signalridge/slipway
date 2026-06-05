# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: 

## Summary
Add a generalized language-agnostic test-design technique skill (layer 2: test-double strategy, behavior-vs-implementation, formal case enumeration, property reasoning, test-data discipline) plus a language-aware capability hint that routes the host to its own installed language testing skill (issue 73, Parts A+B+C). Slipway owns the universal judgment layer and routes to the language layer without vendoring language idiom.
## Complexity Assessment
complex
<!-- Rationale: multi-surface change touching the capability registry, the
binding-compare gate, the technique-hint emission path, the wave-orchestration
host skill, and generated/golden toolgen surfaces. Each surface has a hard gate
(frontmatter↔registry equality, host-skill count, deterministic generation), so
coordination cost is high even though no single edit is risky. -->

## Guardrail Domains
<!-- none detected -->
No sensitive guardrail domain. The change adds skill templates and read-only
handoff hints; it does not touch auth/authz, credentials, PII, financial flows,
schema/data migration, irreversible ops, or external API contracts.

## In Scope
**Part A — generalized `test-design` catalog skill (layer 2, judgment):**
- Registry entry `internal/engine/capability/registry_b6.go` (`testDesign()`):
  `DomainVerification`, `TierT1`, `AttachmentProcedure`, `EvidenceArtifact`,
  `BindingTechniqueHint` → `wave-orchestration` **only** (the single live
  next-skill authoring host: `ResolveNextSkill` returns `wave-orchestration` for
  S2_EXECUTE; `tdd-governance` is `ExportOnlyExtra` and never resolves as a
  next-skill, so a technique-hint binding to it would never emit in production —
  see research.md), plus the hydrate-reference set. Registered in
  `registry_default.go`.
- Templates `internal/tmpl/templates/skills/test-design/SKILL.md` (thin router)
  + `references/*.md` carrying the judgment depth (test-double strategy,
  behavior-vs-implementation, formal case enumeration with explicit oracles,
  property reasoning, test-data discipline). Language-agnostic only.
- Add `test-design` to `hostSkillExportAllowlist` (`internal/toolgen/toolgen.go`).
- Update gate fixtures: `registry_test.go` ID list (+`test-design`),
  `toolgen_test.go` exported host-skill count (22 → 23), and regenerate the
  `internal/toolgen/testdata/skill_tree_inventory.codex.golden` manifest (adds
  `slipway-test-design/SKILL.md` + one row per `references/*.md`) via
  `UPDATE_GOLDEN=1 go test ./internal/toolgen/...`, then re-assert without it.

**Part B — language-aware capability hint bridge (layer 2 → 3):**
- Extend `techniqueHint` (`cmd/next.go`) with `kind` / `capability` /
  `language` / `optional` fields.
- New emitter in `cmd/next_skill_view.go` (called for both `next` and `run`)
  that emits a `capability:language-testing` hint **per detected language**
  (user decision — supersedes the issue's "at most one"). Language source is
  `Change.ProjectContext.Languages` when non-empty, and falls back to
  codebase-map `STACK.md` **only** when it is empty — the two are never merged,
  so a Go-only change must not inherit `Python` from a `STACK.md` that lists
  `Go, Python`. Gated on the presence of the `skill:test-design` hint; **not**
  allowlist-gated; deduplicated; emits nothing when no language is detected
  (tolerating a scaffold/empty `- Languages:` line).
- `disabled_controls` does not gate this hint: `capability:language-testing` is a
  vendor-neutral advisory technique hint, not a governance control, and maps to
  no control ID. Pinned by a test so the semantics are explicit, not guessed.
- Document the `capability:language-testing` host-resolution contract in CLAUDE.md.

**Part C — structural test/implementation isolation in `wave-orchestration` (layer 1):**
- Make "test the behavior, not the implementation" an architectural property of
  a wave: a test-bearing task splits into an isolated **test-authoring step**
  that sees only the spec / acceptance criteria + public API signatures (never
  the implementation) and an **implementation step** (a later `depends_on` task)
  that must satisfy the now-frozen tests. Pairs with the existing
  `tdd-governance` RED-before-GREEN gate (the frozen test-authoring commit is the
  RED proof). Mechanism resolved in research → **Option A: Dispatch-Contract
  isolation**, SKILL.md prose only, anchored on the existing
  `task_kind=test`→`task_kind=code` + `depends_on` model; no parser / wave-model /
  hash / evidence change. **Host-enforced** (same trust model as the existing
  Dispatch-Contract rules), not engine-rejected.

**Cross-cutting:**
- Tests for the A/B/C acceptance signals, plus a guard test asserting the
  Slipway-owned `test-design` templates contain no language-specific test syntax.

## Out of Scope
- Vendoring or maintaining language-specific testing skills (`golang-testing`,
  `python-testing`, …). Layer 3 idiom stays owned by the host ecosystem; Slipway
  only routes to it.
- Changing `tdd-governance` gate semantics or `coverage-analysis` denominator
  logic.
- A dedicated API-only test-generation skill (the enumeration matrices are
  absorbed into `test-design` as language-neutral checklists instead).
- Binding `test-design` / the language hint at `goal-verification` (test
  adequacy at S4 is owned by `coverage-analysis`); the hints surface at the
  authoring/TDD-gate hosts only.

## Constraints
- Binding-compare gate: `test-design/SKILL.md` frontmatter must mirror the Go
  registry record exactly (bindings + hydrate_references), or the gate fails.
- Tier size budget: `test-design` SKILL.md body within the T1 budget (target
  ~2.5 KB, hard-max 6 KB); depth lives in `references/`.
- Deterministic toolgen output; the generated host-skill set and its count must
  stay in sync with fixtures.
- Generalized-only: no language-specific test syntax in Slipway-owned
  `test-design` templates.
- Part C must not break existing wave plans or `tdd-governance` evidence.
- `go build ./...` and `go test ./...` green.

## Acceptance Signals
- GIVEN a change at the `wave-orchestration` execution host WHEN `next`/`run`
  THEN `technique_hints` include `skill:test-design`.
- GIVEN a change with N detected languages THEN `technique_hints` include one
  `capability:language-testing` hint per language (Go-only → exactly the Go
  hint); GIVEN no language THEN only `skill:test-design`, no language hint.
- GIVEN the `test-design` skill is loaded THEN it teaches mock/test-double
  judgment, formal case enumeration (boundary / equivalence / decision-table /
  oracle), and property reasoning independent of language; the capability hint
  carries idiom routing only.
- GIVEN Slipway's own `test-design` templates WHEN scanned THEN they contain no
  language-specific test syntax (generalized-only guard test passes).
- GIVEN Part C WHEN a test-bearing wave task is executed THEN the test-authoring
  step is isolated from the implementation (spec + public API only) and the
  frozen tests gate the implementation step.
- `go build ./...` and `go test ./...` pass.

## Open Questions
<!-- All intake-time questions were resolved during S1 research; full resolution
     lives in research.md (## Unknowns / ## Alternatives Considered) and
     decision.md. Listed below as checked items so the section is non-blocking. -->
- [x] Part C mechanism — RESOLVED: Option A (Dispatch-Contract isolation; SKILL.md prose only; anchored on the existing task_kind=test->code + depends_on model; no parser/wave-model/hash/evidence change).
- [x] Part C <-> tdd-governance — RESOLVED: the frozen test-authoring commit is the RED proof consumed by the existing git-history verification; no new evidence competes with the gate.
- [x] STACK.md language sourcing — RESOLVED: ProjectContext.Languages first; fall back to STACK.md only when it is empty (never merged); a scaffold/empty Languages line yields no language.
- [x] Domain classification for test-design — RESOLVED: verification (with coverage-analysis / property-testing / mutation-testing).
- [x] test-design technique-hint hosts — RESOLVED: wave-orchestration only; tdd-governance is ExportOnlyExtra and never resolves as a next-skill, so a hint bound to it would never emit in production; Part C still adds the RED-proof note to its SKILL.md (delivered via SKILL.md export, not technique hints).

## Deferred Ideas
<!-- Identified but postponed ideas -->
- None. Part C, previously framed in the issue as an optional follow-up, is
  pulled into this change per the user's intake decision.

## Approved Summary
Confirmed by user 2026-06-05T05:01:40Z.

Deliver issue #73 Parts A + B + C:
- **A (judgment layer):** a generalized, language-agnostic `test-design` catalog
  skill — test-double strategy, behavior-not-implementation assertions, formal
  case enumeration (boundary / equivalence / decision-table / state-transition /
  pairwise / MC-DC with explicit oracles), property reasoning, and test-data
  discipline — routed via technique-hint to `wave-orchestration` (research scoped
  this to `wave-orchestration` only; `tdd-governance` is `ExportOnlyExtra` and
  never emits hints in production — see Open Questions).
- **B (layer 2 → 3 bridge):** a `capability:language-testing` hint emitted **one
  per detected language** (sourced from `ProjectContext.Languages`, then
  codebase-map `STACK.md`), gated on the `skill:test-design` hint being present,
  **not** allowlist-gated, deduplicated, and absent when no language is detected.
  The host resolves the capability to its own installed language testing skill;
  the resolution contract is documented in CLAUDE.md.
- **C (governance-layer structuring):** `wave-orchestration` can isolate a
  test-authoring step (spec + public API signatures only, never the
  implementation) from the implementation step (must satisfy the frozen tests),
  pairing with the `tdd-governance` RED-before-GREEN gate.

Scope boundaries: no vendoring of language-specific testing skills (layer 3
stays host-owned); no change to `tdd-governance` gate semantics or
`coverage-analysis` denominator logic; the hints are not bound at
`goal-verification`. `test-design` is classified under the `verification` domain.

Primary acceptance: at the `wave-orchestration` execution host, `next`/`run`
`technique_hints` include `skill:test-design` plus one
`capability:language-testing` hint per detected language; the Slipway-owned
`test-design` templates contain no language-specific test syntax; `go build
./...` and `go test ./...` are green.
