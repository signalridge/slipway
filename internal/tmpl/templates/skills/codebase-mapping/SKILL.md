---
skill_id: codebase-mapping
name: slipway-codebase-mapping
description: "Use when mapping repository architecture, dependency graphs, and module boundaries before planning. Triggers on brownfield discovery or whenever durable structural context is missing."
---

# Codebase Mapping

## Purpose
Systematically map repository architecture, dependency graphs, key
abstractions, and module boundaries before planning. This is a technique skill:
it produces durable context for research, scope confirmation, and plan audit,
but it does not own a governed route.

## When This Runs
Advisory hint during discovery or governed spec bundling within `S1_PLAN`. This is a technique skill — it does not produce governance evidence and missing output does not block any gate.

## Durable Output Contract
Use `slipway next --json` first and read these path fields:
- `input_context.codebase_map_dir`
- `input_context.codebase_map_docs`

Write a durable brownfield map under `artifacts/codebase/` using this fixed document set:
- `STACK.md`
- `INTEGRATIONS.md`
- `ARCHITECTURE.md`
- `STRUCTURE.md`
- `CONVENTIONS.md`
- `TESTING.md`
- `CONCERNS.md`

These documents are advisory, but they are the canonical reusable output of the technique skill. Downstream skills consume them by path instead of relying on transient chat context.

## Process

### 1. Resolve Output Paths
Extract `input_context.codebase_map_dir` and `input_context.codebase_map_docs`.
Create the map directory if needed.

### 2. Repository Structure Scan
Map the top-level directory structure and identify:
- Source code directories and their purposes
- Configuration files and build system
- Test directories and test infrastructure
- Documentation locations
- Generated vs. handwritten code (important for scope decisions)

### 3. Module Boundary Analysis
For each major module/package:
- Public API surface: exported types, functions, interfaces
- Implementation patterns: how the module organizes private logic
- Cross-module dependencies: who depends on whom
- Shared utilities and entry points

### 4. Dependency Graph
Build a dependency map:
- **Internal module dependencies**: Which modules depend on which (directional)
- **External dependencies**: Third-party libraries and their roles
- **Circular dependency detection**: Any A→B→A cycles
- **Layering violations**: Lower layers importing higher layers
- **Coupling hotspots**: Modules with high fan-in (many dependents) or fan-out (many dependencies)

### 5. Key Abstractions
Identify core domain concepts:
- **Primary data types**: Main structs/classes and their relationships
- **Interface contracts**: Abstractions and their implementations
- **State management patterns**: How state flows through the system
- **Error handling conventions**: Error types, propagation patterns, recovery mechanisms
- **Concurrency patterns**: Locks, channels, async patterns (if relevant)

### 6. Change Impact Analysis
For the current change scope:
- **Blast radius**: Which modules/files will be directly affected?
- **Transitive impact**: Which modules depend on affected modules?
- **Test impact**: Which tests cover the affected code?
- **Integration points**: Where does affected code interact with external systems?

### 7. Pattern Inventory
Document existing patterns that the change should follow:
- **Naming conventions**: How are similar entities named? (e.g., `FooManager`, `BarService`)
- **File organization**: Where do similar features live? (e.g., `cmd/` for CLI, `internal/` for private)
- **Error handling**: How do existing modules handle and propagate errors?
- **Testing patterns**: How are existing tests structured? (table-driven, subtests, fixtures)
- **Configuration patterns**: How is config loaded and injected?

### 8. Structured Output
Write the findings into the durable document set.

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
- Start broad (repository structure), then narrow to change-relevant areas
- Do NOT read entire files — use symbol overview tools or grep for specific patterns
- Limit dependency graph to 2 levels deep from the change scope
- Stop mapping when you have enough context for scope/planning decisions
- Total mapping should take < 15% of available context budget

## Relationship to Governance
Advisory only. Referenced in `technique_hints` during `S1_PLAN` discovery and bundle substeps but does NOT produce governance evidence. Missing codebase mapping SHALL NOT block any gate.

However, `slipway-research-orchestration`, `slipway-plan-audit`, and `slipway-wave-orchestration` SHOULD consume the durable `artifacts/codebase/` documents when present. A thorough codebase map improves scope validation, task targeting, and execution safety.
