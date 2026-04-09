---
name: codebase-mapping
description: "Systematically map repository architecture, dependency graphs, and module boundaries"
---

# Codebase Mapping

## Purpose
Systematically map repository architecture, dependency graphs, key abstractions, and module boundaries before planning. Provides essential structural context for research, scope confirmation, and plan audit.

## When This Runs
Advisory hint during discovery or governed spec bundling within `S1_PLAN`. This is a technique skill — it does not produce governance evidence and missing output does not block any gate.

## Durable Output Contract
Run `slipway next --json` first and use these context fields:
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

### 0. Resolve Output Paths
Run:

```bash
slipway next --json
```

Extract `input_context.codebase_map_dir` and create it if needed.

### 1. Repository Structure Scan
Map the top-level directory structure and identify:
- Source code directories and their purposes
- Configuration files and build system
- Test directories and test infrastructure
- Documentation locations
- Generated vs. handwritten code (important for scope decisions)

**Concrete commands**:
```bash
# Top-level structure
ls -la
# Source files by type
find . -name "*.go" -o -name "*.ts" -o -name "*.py" | head -50
# Build system
cat go.mod  # or package.json, Cargo.toml, etc.
# Test infrastructure
find . -name "*_test.go" -o -name "*.test.ts" | head -20
```

### 2. Module Boundary Analysis
For each major module/package:
- **Public API surface**: Exported types, functions, interfaces
- **Internal implementation patterns**: How the module organizes its private logic
- **Cross-module dependencies**: Import graph (who depends on whom)
- **Shared utilities**: Common abstractions used across modules
- **Entry points**: Where external calls enter this module

**Concrete commands**:
```bash
# Go: package-level overview
go list ./...
# Import graph
grep -r "import" --include="*.go" <module_dir> | sort
# Exported symbols
grep -rn "^func [A-Z]\|^type [A-Z]" --include="*.go" <module_dir>
```

### 3. Dependency Graph
Build a dependency map:
- **Internal module dependencies**: Which modules depend on which (directional)
- **External dependencies**: Third-party libraries and their roles
- **Circular dependency detection**: Any A→B→A cycles
- **Layering violations**: Lower layers importing higher layers
- **Coupling hotspots**: Modules with high fan-in (many dependents) or fan-out (many dependencies)

**Output format**:
```
module_a → module_b (uses: TypeX, FuncY)
module_a → module_c (uses: InterfaceZ)
module_b → external/lib (uses: Client)
```

### 4. Key Abstractions
Identify core domain concepts:
- **Primary data types**: Main structs/classes and their relationships
- **Interface contracts**: Abstractions and their implementations
- **State management patterns**: How state flows through the system
- **Error handling conventions**: Error types, propagation patterns, recovery mechanisms
- **Concurrency patterns**: Locks, channels, async patterns (if relevant)

### 5. Change Impact Analysis
For the current change scope:
- **Blast radius**: Which modules/files will be directly affected?
- **Transitive impact**: Which modules depend on affected modules?
- **Test impact**: Which tests cover the affected code?
- **Integration points**: Where does affected code interact with external systems?

### 6. Pattern Inventory
Document existing patterns that the change should follow:
- **Naming conventions**: How are similar entities named? (e.g., `FooManager`, `BarService`)
- **File organization**: Where do similar features live? (e.g., `cmd/` for CLI, `internal/` for private)
- **Error handling**: How do existing modules handle and propagate errors?
- **Testing patterns**: How are existing tests structured? (table-driven, subtests, fixtures)
- **Configuration patterns**: How is config loaded and injected?

### 7. Structured Output
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

However, research-orchestration, plan-audit, and wave-orchestration SHOULD consume the durable `artifacts/codebase/` documents when present. A thorough codebase map improves scope validation, task targeting, and execution safety.

## Rationalization Red Flags
| Rationalization | Counter-rule |
|---|---|
| "I already know this codebase" | Document it anyway. Undocumented knowledge is unverifiable. |
| "Mapping takes too long" | Use the efficiency guidelines. Broad scan, then narrow. |
| "The change is small, no mapping needed" | Small changes in coupled modules have large blast radius. Map the coupling. |
| "I'll discover the architecture during implementation" | Discovery during implementation causes scope drift. Map first. |
| "Dependency graph is obvious" | Obvious dependencies still need documentation for scope confirmation. |
