# Decision
## Project Context
- Tech Stack: Go
- Conventions: catalog-skill binding-compare gate; deterministic toolgen; checkbox-native tasks.md
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

Parts A and B have a single idiomatic implementation each, fixed by issue #73 and
the binding-compare contract. The genuine design fork is **Part C's mechanism**:

- **Option A — Dispatch-contract isolation (SKILL.md only).** Extend the
  `wave-orchestration` Dispatch Contract to structure a test-bearing unit as a
  `task_kind=test` authoring step isolated to spec + public API signatures, then
  a `task_kind=code` step that `depends_on` it and satisfies the frozen tests;
  `tdd-governance` treats the frozen test commit as the RED proof. Tradeoffs:
  zero Go/model/hash change → lowest risk, fully backward compatible; isolation
  anchored on the existing wave/`task_kind`/`depends_on` structure and enforced
  by the host dispatch contract (same trust model as lines 59/64 today); no
  engine-level rejection if a host ignores it.
- **Option C — Declared isolation marker (parser + view + SKILL).** Add a
  validated optional `tasks.md` key (`test_isolation:`) to the parser whitelist
  and the wave-plan view. Tradeoffs: an explicit, machine-validated per-task
  property; ~2–3 Go files; partly redundant with `task_kind=test`/`code` +
  `depends_on`; touches `TaskPlanSemanticHash`.
- **Option B — Full model enforcement (parser + hash + WavePlanTask +
  plan-audit gate + step evidence).** Tradeoffs: strongest and auditable, but
  ~10–12 Go files reaching into the freshness/hash machinery destabilized by
  recent issues (53/72/74/77) — highest risk for the least marginal benefit.

### Constraints (from source document)
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

## Selected Approach
**Part A (judgment skill):** a new `registry_b6.go` `testDesign()` record
(`domain=verification`, `tier=T1`, `procedure`, `artifact`, a technique-hint
binding → `wave-orchestration` only, five hydrate references), registered in
`defaultSkills()`, with a thin `test-design/SKILL.md` router whose frontmatter
mirrors the registry and `references/{test-doubles,
behavior-vs-implementation,case-enumeration,property-reasoning,test-data}.md`
carrying language-neutral depth; `test-design` added to the export allowlist;
`registry_test.go` ID list, `toolgen_test.go` count (22 → 23), and the
`skill_tree_inventory.codex.golden` manifest updated together.

**Part B (bridge):** extend `techniqueHint` with `kind/capability/language/optional`
and add `appendLanguageTestingHints` in `cmd/next_skill_view.go`, called for both
`next` and `run` after the catalog/profile hints. It emits one
`capability:language-testing` hint **per detected language** from
`ProjectContext.Languages`, falling back to `STACK.md` **only when
`ProjectContext.Languages` is empty** (never merged), only when a
`skill:test-design` hint is already present, never allowlist-gated, deduplicated
by language, and nothing when no language is detected. `disabled_controls` does
not gate it (it is not a control). CLAUDE.md documents both the host-resolution
contract and the `disabled_controls` non-interaction.

**Part C (structural isolation): Option A — Dispatch-contract isolation**
(user-confirmed 2026-06-05). Extend `wave-orchestration/SKILL.md.tmpl` with a
test/implementation isolation rule in the Dispatch Contract (anchored on the
existing `task_kind=test`→`code` + `depends_on` structure and the current
"one isolated executor context per task" / "pass file paths only" rules) and note
in `tdd-governance/SKILL.md.tmpl` that the frozen test-authoring commit is the RED
proof. No Go/model/hash/evidence change. This is **host-enforced** prose (same
trust model as the existing Dispatch-Contract rules), **not** engine-rejected;
acceptance is that the rendered SKILL.md carries the rule without over-claiming
engine enforcement. The Part C rule encodes **only orchestration mechanics**
(executor-context isolation + freeze ordering, layer ①); it does **not** restate
the behavior-vs-implementation judgment carried by the Part A `test-design`
references (layer ②), so the two layers stay complementary, not redundant.

## Interfaces and Data Flow
- **Capability registry → SKILL.md:** new `Skill{ID:"test-design", …}`; the
  binding-compare gate enforces frontmatter↔registry equality. `references/` keys
  are `test-design/<name>.md`.
- **Hint emission:** `populateNextSkillView` →
  `appendCatalogHints` (emits `skill:test-design` at the `wave-orchestration`
  host) → `appendWorkflowProfileTechniqueHints` → **new**
  `appendLanguageTestingHints` (reads `governedChange.ProjectContext.Languages`,
  falls back to `STACK.md` via the workspace root only when empty). Shared by
  `next` and `run` through `buildNextView`; `cloneTechniqueHints` carries the new
  scalar fields into the handoff.
- **No new persisted state.** No change to `change.yaml`, wave-plan model,
  `TaskPlanSemanticHash`, or evidence schemas. Part C is template prose only.

## Rollout and Rollback
- Single PR delivering A + B + C, executed in dependency-ordered waves (see
  `tasks.md`), test-first per part — each part's contract tests are authored RED
  before its implementation turns them GREEN. Additive only; no migrations.
- Rollback: revert the PR. Because no persisted schema, hash, or evidence format
  changes, reverting fully removes `test-design`, the language hint, and the
  template prose with no residual state. Per-part rollback is also clean
  (each part is isolated to its own files).
- Verification command: `go build ./...` && `go test -count=1 ./...`.

## Risk
- **Binding-compare drift (medium, mitigated):** author registry + frontmatter
  together; run `go test ./internal/engine/capability/...` first.
- **Fixture lag (low):** `toolgen_test.go` count, `registry_test.go` ID list, and
  the `skill_tree_inventory.codex.golden` manifest must move together with the
  code. The `tdd-governance` "no hints" test does NOT change (test-design binds to
  `wave-orchestration` only); the `wave-orchestration` host hint assertion does.
- **Per-language emission is the formal contract (not a deviation):** issue #73's
  "at most one language hint" wording is stale; per-language emission is intended —
  one hint per detected language, pinned by task-004's polyglot (Go+TS → two
  hints) test. Go-only changes still emit exactly one.
- **Part C over-claiming (low):** the SKILL.md prose must describe host-enforced
  isolation, not engine rejection; guard the wording in review.
- **STACK.md parsing (low):** tolerate scaffold/empty `- Languages:`; return no
  language rather than a bogus token.
