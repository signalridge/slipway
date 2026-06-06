# Research

## Research Findings

### Architecture
- Affected modules:
  - `internal/toolgen/toolgen.go` — `commandRegistry[].Arguments` (source of
    `command-reference.md` + codex prompts); exported `CommandDescription` /
    `CommandClassification` (add `CommandArguments`).
  - `internal/tmpl/templates/_partials/command-*-body.tmpl` — 14 command-surface
    `## Flags` sections (handwritten).
  - `internal/tmpl/templates/skills/workflow/*.tmpl` — Slipway entry skill
    `SKILL.md` + `command-reference.md.tmpl`.
  - `internal/tmpl/templates/skills/<host>/SKILL.md.tmpl` — `slipway-*` host
    skills citing flags/usage.
  - `cmd/*.go` — Cobra commands; `Short`/`Long`/flag usage = the `--help`
    source of truth.
  - `docs/`, `README.md` — prose command/flag usage (enumerate in plan).
  - `cmd/template_flag_contract_test.go` — existing one-directional guard.
- Dependency chains: `cmd` (cobra) → imports `internal/toolgen`; `toolgen` →
  `internal/tmpl` + `internal/engine/capability`. `toolgen` cannot import `cmd`
  (cycle) → the reverse guard must live in the `cmd` test package (imports both).
- Blast radius: all generated AI-facing surfaces + `--help` text + entry skill.
  Docs / metadata / tests only; no runtime-behavior change.
- Constraints: `.claude/` etc. are gitignored generated output — fix in sources
  then regenerate, never hand-edit the generated tree.

### Patterns
- Exported registry accessors `CommandDescription` / `CommandClassification` →
  add `CommandArguments(id)` in the same shape.
- Existing guard `TestTemplateFlagsMatchCobraCommands` asserts template→cobra
  (phantom-flag) only; extend the same file with the reverse cobra→registry
  assertion.
- Generated-surface tests use `toolgen.Generate(tempdir)` then read `.md` and
  assert — reuse this harness.
- Body partials are named `command-<id>-body`; some (`new` / `status` / `init` /
  `repair`) are intentionally prose with no `## Flags` list.

### Risks
- Technical: LOW — alignment + tests, no runtime logic change. Reversible (git).
- Package cycle (toolgen↔cmd): mitigated by placing the reverse guard in the
  `cmd` test package.
- Entry-skill `description` change is a design/subjective edit → MEDIUM;
  acceptance leans on human review of the trigger language.
- `--help`-vs-logic audit may surface a genuine behavior bug → boundary set:
  record, do not fix here.
- Guardrail domains: NONE (no auth / credentials / PII / financial / migration /
  irreversible / external-API).
- Codebase-map note: `conventions` / `integrations` docs are `scaffold_only`
  (non-durable); this research relies on first-hand source inspection instead.

### Test Strategy
- Existing: `cmd/template_flag_contract_test.go`, `internal/toolgen/toolgen_test.go`,
  `internal/tmpl/templates_test.go`.
- Add a reverse flag-contract guard (cmd test pkg): enumerate every command
  constructor's non-hidden, non-help flags; assert each appears in
  `toolgen.CommandArguments(id)`; documented exemptions (`help`,
  `review:--artifact` unsupported-in-MVP).
- Verification: `go test ./cmd/... ./internal/toolgen/... ./internal/tmpl/...`
  plus a `--help`-vs-generated drift scan as an acceptance check.

## Alternatives Considered

### Approach A — Handwritten surfaces + reverse contract guard (RECOMMENDED)
Backfill the handwritten surfaces (registry Arguments, body `## Flags`, help
text, entry skill) and lock them with a reverse cobra→surface guard. Prose body
templates keep their design; only core action-flag omissions are added.
- Tradeoffs: preserves human-authored flag descriptions and prose design;
  focused, low-risk; guard prevents future missing-flag drift. Body surface
  stays curated (not exhaustively complete by construction); help audit is manual.

### Approach B — Generator single-source-of-truth (auto-derive from cobra FlagSet)
Extract flag metadata into a shared low-level package; toolgen and `--help` both
derive Arguments from it.
- Tradeoffs: most durable (never drifts), but a large refactor (break the
  toolgen↔cmd cycle, new shared package), loses handwritten flag-usage prose and
  the curated prose templates, higher risk, scope far beyond this objective.

### Approach C — Doc backfill only, no guard
Fix current gaps, add no reverse guard.
- Tradeoffs: smallest, but drifts again on the next added flag; fails the user's
  "comprehensive + don't-recur" intent.

**Recommendation: Approach A** — aligns now, guards against recurrence, keeps
authored quality, bounded scope.

**Selected approach (user-confirmed 2026-06-06): Approach A.** The locked
decision is recorded in `decision.md` under `## Selected Approach`.

## Unknowns
- None research-grade. docs/README/skill surface enumeration is a mechanical
  grep performed during planning.

## Assumptions
- `slipway init --refresh` is the canonical way to propagate source fixes to the
  generated surfaces (verified earlier: zero-diff regeneration from current binary).
- No hidden cobra flags currently exist (verified: no `Hidden`/`MarkHidden` in `cmd/`).

## Canonical References
- `artifacts/changes/align-generated-command-reference-with-real-cobra-flags/intent.md`
  — request + intake scope.
- `cmd/template_flag_contract_test.go` — existing guard to extend.
- `internal/toolgen/toolgen.go` — commandRegistry + exported accessors.
- `internal/tmpl/templates/_partials/command-*-body.tmpl` — body Flags sections.
