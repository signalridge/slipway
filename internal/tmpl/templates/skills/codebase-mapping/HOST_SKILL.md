---
skill_id: codebase-mapping
name: slipway-codebase-mapping
description: "Use when mapping repository architecture, dependency graphs, and module boundaries before planning. Triggers on brownfield discovery or whenever durable structural context is missing."
---

# Codebase Mapping

## Purpose
Map repository architecture, dependency graphs, key abstractions, and module
boundaries before planning. This technique produces durable context for
research, scope confirmation, and plan audit; it does not own a governed route.

## When This Runs
Advisory hint during discovery or governed spec bundling within `S1_PLAN`. This is a technique skill — it does not produce governance evidence and missing output does not block any gate.

## Durable Output Contract
Resolve the canonical map paths from `slipway codebase-map --json` — read
`codebase_map_dir`, `codebase_map_docs`, `status`, and `doc_states`. (During
standalone discovery `slipway next --json` returns `no_active_change`; only
*inside* an active change can you instead read
`input_context.codebase_map_dir`/`input_context.codebase_map_docs` from
`slipway next --json`.) The path fields are:
- `codebase_map_dir`
- `codebase_map_docs`

Write a durable brownfield map under `artifacts/codebase/` using this fixed document set:
- `STACK.md`
- `INTEGRATIONS.md`
- `ARCHITECTURE.md`
- `STRUCTURE.md`
- `CONVENTIONS.md`
- `TESTING.md`
- `CONCERNS.md`

These documents are advisory but canonical. Downstream skills consume them by
path instead of relying on transient chat context.

Before authoring each doc, read its authoring contract with `slipway
instructions <doc> --json` (e.g. `slipway instructions architecture --json`;
both the key `architecture` and the file name `ARCHITECTURE.md` resolve). It
returns the same instructions->author payload the governed bundle artifacts use:
- `template` — the doc's structure to fill in.
- `guidance` — the quality bar (file:line-cited findings, module boundaries,
  change-relevant risks).
- `resolved_output_path` — the repo-scoped path under `artifacts/codebase/` to
  write to. Codebase-map docs are repo-scoped and advisory, so this resolves
  without an active change and does not gate any stage.
- `context` — when the CLI's baseline scan detected real facts, the machine-
  extracted baseline content. These are real detected facts to preserve and
  extend, NOT a placeholder seed to delete.

`slipway codebase-map --json` may report `status: "baseline"` and
`baseline_docs` when the CLI has written only detected repository facts. Baseline
docs are not finished mapping work; refine them with file:line citations,
module-boundary findings, and change-specific risks before treating the map as
reviewed context.

These documents are git-tracked by default: durable brownfield context is meant
to be reviewed and shared, not hidden as local-only state. `next`/`run` surface
`input_context.codebase_map_status` (and per-doc
`input_context.codebase_map_doc_states`) so downstream hosts can tell whether the
map is durable; a `scaffold_only` or `baseline` status means the map is
non-durable and should be refined before it is consumed as reviewed context. A
`populated` status only means the documents differ from the scaffold/baseline —
not that they describe the current change. When a populated map predates the
change in scope, re-author the change-relevant documents in place rather than
treating presence as freshness; the assessment re-reads them, so the inline edit
is the refresh.

## Process

### 1. Resolve Output Paths
Run `slipway codebase-map --json` to scaffold the canonical map directory and the
fixed document set, then read `codebase_map_dir`/`codebase_map_docs` from its
output. Do not hand-create the directory or guess the doc set. (Inside an active
change you may instead read the `input_context.codebase_map_*` fields from
`slipway next --json`.) For each doc you author, read `slipway instructions
<doc> --json` to get its template, quality bar, resolved output path, and
baseline facts (see Durable Output Contract).

### 2. Repository Structure Scan
Map top-level directories, build/config files, tests, docs, and
generated-vs-handwritten boundaries.

### 3. Module Boundary Analysis
For each major module/package:
- Public API surface: exported types, functions, interfaces
- Implementation patterns: how the module organizes private logic
- Cross-module dependencies: who depends on whom
- Shared utilities and entry points

### 4. Dependency Graph
Build a dependency map covering internal directionality, external libraries,
cycles, layering violations, and high fan-in/fan-out hotspots.

### 5. Key Abstractions
Identify core domain concepts:
- primary data types and relationships
- interface contracts and implementations
- state flow, error handling, and concurrency patterns

### 6. Change Impact Analysis
For the current change scope:
- direct blast radius and transitive dependents
- tests covering affected code
- integration points with external systems

### 7. Pattern Inventory
Document existing patterns that the change should follow:
- naming, file organization, error handling, tests, and configuration

### 8. Structured Output
Write the findings into the durable document set. Author each doc against its
`slipway instructions <doc>` contract — fill the returned `template`, meet the
`guidance` quality bar, preserve and extend any baseline `context`, and write to
the `resolved_output_path`.

```markdown
artifacts/codebase/STACK.md
- languages, frameworks, build/test tooling, key dependencies, file counts

artifacts/codebase/INTEGRATIONS.md
- external APIs, infra bindings, queues, databases, file formats, protocols

artifacts/codebase/ARCHITECTURE.md
- module responsibilities, dependency flow, coupling hotspots, change blast radius

artifacts/codebase/STRUCTURE.md
- directory layout, entry points, generated-vs-handwritten boundaries, ownership hints

artifacts/codebase/CONVENTIONS.md
- naming, file organization, error handling, config, state-management conventions

artifacts/codebase/TESTING.md
- existing test layout, coverage hotspots/gaps, verification commands, fixture patterns

artifacts/codebase/CONCERNS.md
- risks, architectural pressure points, brittle areas, migration traps, deferred concerns
```

## Efficiency Guidelines
- Start broad, then narrow to change-relevant areas.
- Do NOT read entire files; use symbol overview tools or targeted grep.
- Limit dependency graph to 2 levels from the change scope.
- Stop when there is enough context for scope/planning decisions.
- Keep mapping under 15% of available context budget.

## Relationship to Governance
Advisory only. Referenced in `technique_hints` during `S1_PLAN` discovery and bundle substeps but does NOT produce governance evidence. Missing codebase mapping SHALL NOT block any gate.

However, `slipway-research-orchestration`, `slipway-plan-audit`, and `slipway-wave-orchestration` SHOULD consume the durable `artifacts/codebase/` documents when present. A thorough codebase map improves scope validation, task targeting, and execution safety.
