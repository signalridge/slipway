# Distillation Schema (frozen at B0)

This document freezes the authoring contract that every catalog skill must
satisfy. It is consumed by authors, by the Go-side capability registry in
`internal/engine/capability/`, and by the CI gates scheduled for B8
(`schema-lint`, `size-lint`, `binding-compare`, `provenance-coverage-scan`).

The schema is descriptive and export-facing. Runtime binding authority lives in
the Go registry, not in frontmatter.

## 1. Source layout

```
internal/tmpl/templates/skills/<skill-id>/
  SKILL.md              # required
  provenance.yaml       # required
  PROSE.tmpl            # optional
  CHECKLIST.tmpl        # optional
  VERDICT.tmpl          # optional
  references/           # optional, one-hop hydration shelf
  scripts/              # optional, deterministic helpers
```

Rules:

1. `SKILL.md` and `provenance.yaml` are required for every catalog skill.
2. Typed templates (`PROSE.tmpl`, `CHECKLIST.tmpl`, `VERDICT.tmpl`) are opt-in
   and consumed only when a binding or evidence contract needs them.
3. `references/` is a one-hop support shelf for examples, anti-patterns, and
   source notes. It is never routing authority.
4. `scripts/` is reserved for deterministic helpers with explicit I/O contracts.
5. A reference or script file must be named from the skill body or resolver
   output before it is considered in scope.

## 2. SKILL.md frontmatter contract

```yaml
---
skill_id: <stable slipway skill identifier>
domain: <one of: intake, execution, debugging, review-quality,
         review-security, review-change-shape, verification,
         repair-ci, ops-diagnostics>
function: <single-sentence description of the one job>
tier: <T1 | T2 | T3>
primary_attachment: <posture | procedure | checklist | tool-recipe | report-schema>
summary: "Use when <trigger>. Triggers on <signal>."
size_rationale: <optional string, required only when body exceeds tier hard-max>
trigger_signals:
  - <clause>
evidence_contract: <verdict | artifact | checklist>
bindings:
  - type: <host-embedded | command-auto | command-manual
           | technique-hint | command-view | export-only>
    target: <host name or command mode/view id>
    attachment: <posture | procedure | checklist | tool-recipe | report-schema>
provenance_ref: provenance.yaml
---
```

Rules:

1. `skill_id` must match the directory name under `templates/skills/`.
2. `summary` must use the `Use when ... / Triggers on ...` phrasing so
   description-as-dispatcher adapters can rely on it.
3. `tier` encodes semantic role, not binding count. T1 = reusable core method,
   T2 = specialist route (typically tool-recipe), T3 = diagnostic view.
4. `primary_attachment` is mandatory. Additional attachment modes may be
   declared per binding entry.
5. `trigger_signals[]` entries are DSL clauses drawn from the bounded operator
   set (§4). Free prose is rejected by schema-lint.
6. `bindings[]` must mirror the Go-owned capability registry 1:1.
   `binding-compare` enforces this.
7. `evidence_contract` names the evidence shape the skill produces.
8. `size_rationale` is optional by default. It is required only when
   `SKILL.md` body size exceeds the tier hard-max band (6/8/3 KB).

## 3. Attachment modes (frozen set)

| Mode | Meaning | Typical carrier |
|------|---------|-----------------|
| `posture` | Persistent stance injected at prompt top | "enforce TDD" |
| `procedure` | Ordered steps | `RED -> GREEN -> REFACTOR` |
| `checklist` | Discrete check items | security review items |
| `tool-recipe` | Tool or command invocation pattern | semgrep config |
| `report-schema` | Structured output constraint | verdict shape |

Typical template mapping:

- `PROSE.tmpl` → `posture` or `procedure`
- `CHECKLIST.tmpl` → `checklist`
- `VERDICT.tmpl` → `report-schema`
- `tool-recipe` → `scripts/` or inline body

The resolver uses attachment mode to decide prompt injection position.

## 4. Trigger DSL (frozen operator set)

Owned by `internal/engine/capability/trigger.go`. No operators outside this
list are permitted in `trigger_signals[]`:

- `all_of` — every nested clause must match
- `any_of` — at least one nested clause must match
- `not` — inverts the nested clause
- `command` — current command surface (e.g. `review`, `validate`, `repair`,
  `status`, `health`)
- `host` — current governed host (e.g. `intake-clarification`)
- `blocker_reason` — matches a normalized blocker reason code
- `changed_files_include` — glob over the change's file list (`*`, `?`, `**`,
  and brace alternatives such as `*.{yml,yaml}`)
- `path_includes` — substring match on a referenced path
- `user_text_matches` — case-insensitive substring match on user text

Each top-level clause carries a mandatory `reason` field used by the resolver
to explain the match in `TechniqueHints` output.

Scoring is Go-owned. The resolver returns at most one routed route and up to
three ranked support attachments per invocation.

## 5. provenance.yaml contract

```yaml
sources:
  - source: <vendor>/<source-skill>
    absorbed_as: <standalone | posture-only | partial-only>
    extracted:
      - <rule|procedure kept in this skill>
    dropped:
      - <narrative content or rule intentionally discarded>
    conflicts_with: []
```

Rules:

1. Every source whose `by-source.md` disposition is `standalone` or
   `partial-only` must appear in `extracted`, `dropped`, or `conflicts_with`
   for exactly one catalog skill's provenance file. Missing coverage is
   blocked by `provenance-coverage-scan`. `posture-only`, `absorbed`,
   `view-only`, `route-only`, and `deferred` rows remain documented in
   `by-source.md` but are not provenance-gated.
2. `absorbed_as: standalone` means the source substantively feeds the catalog
   skill. `posture-only` means only stance is preserved. `partial-only` means
   only a subsection or template partial is consumed.
3. Conflict resolution default is conservative merge. Record the conflict and
   the chosen rule under `conflicts_with` with a short `reason`.

## 6. Assembler order (fixed)

Toolgen's multi-file assembler compiles a catalog skill in this exact order
and no other:

1. Frontmatter
2. `SKILL.md` body
3. Conditional typed-template injection
   1. `PROSE.tmpl` when a binding declares `posture` or `procedure`
   2. `CHECKLIST.tmpl` when a binding declares `checklist`
   3. `VERDICT.tmpl` when a binding declares `report-schema`
4. `references/<name>.md` files named from the body or resolver
   (`hydrate_references[]`)

Rules:

1. Catalog skills use this assembler today. Existing non-catalog governed
   hosts may still use their single-file or `.tmpl` sources directly.
2. Hydration never reaches beyond the skill's own `references/` directory.
3. Scripts are never inlined at assembly time; they remain file-system
   artifacts invoked by user tooling.

## 7. CI gates (implemented in Go tests and expected in CI)

| Gate | Scope |
|------|-------|
| `schema-lint` | Parses frontmatter and typed-template references; asserts operator whitelist |
| `size-lint` | Measures `SKILL.md` body size against tier budget (T1 ≤ 2 KB, T2 ≤ 3 KB, T3 ≤ 1.5 KB; warnings 2-6 / 3-8 / 1.5-3 KB; above top bound requires rationale) |
| `binding-compare` | Diff between frontmatter `bindings[]` and Go-owned registry; must be 1:1 |
| `provenance-coverage-scan` | Enforces `standalone` / `partial-only` by-source rows are covered by `provenance.yaml`, plus reverse check that each provenance source appears in `by-source.md` |

## 8. Extension outputs (optional)

The resolver may return, beyond the primary route or support list:

- `hydrate_references[]` — `references/*.md` paths recommended for injection.
  Contract is declared from B2 (`context-assembly` frontmatter); resolver
  emission is currently reserved and returns empty.
- `llm_tiebreak` — candidate ids plus a decision criterion when DSL scoring
  ties. First exercised at B7. Never implemented or tested at B1.
