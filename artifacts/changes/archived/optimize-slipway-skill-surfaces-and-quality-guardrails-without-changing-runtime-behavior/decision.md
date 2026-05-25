# Decision

## Project Context
- Tech Stack: Go CLI, generated Agent Skills templates
- Conventions: Go-owned capability metadata, deterministic toolgen output, generated surfaces tested through `internal/toolgen`.
- Test Command: `go test ./internal/toolgen ./cmd ./internal/engine/capability`
- Build Command: `go build ./...`
- Languages: Go, Markdown, Shell, Python

## Alternatives Considered

### Alternative A: Import or generate a broad external skill catalog
- Pros: maximizes breadth and mirrors popular marketplace/awesome-list trends.
- Cons: expands prompt surface area, increases third-party quality/security burden, risks non-deterministic catalog drift, and conflicts with the current slim exported host surface.
- Decision: rejected as over-engineered for this project.

### Alternative B: Add a new public skill-quality command or diagnostics mode
- Pros: creates an explicit operator-facing quality surface.
- Cons: changes product behavior, adds a new CLI contract, and exceeds the user's "no functionality change" boundary.
- Decision: rejected.

### Alternative C: Tighten generated lookup surfaces and mechanical guardrails
- Pros: improves discoverability and quality protection using existing metadata, templates, and tests; preserves runtime semantics; easy to verify and roll back.
- Cons: narrower than a marketplace and does not solve arbitrary third-party skill curation.
- Decision: selected.

## Selected Approach
Use Alternative C.

Implementation will:
- render existing public `--focus` aliases from `capability.surfacePolicy` into the workflow command reference;
- add a compact public-focus section to the generated workflow-owned skill index;
- add focused tests that fail when public focus aliases are not rendered;
- add a high-threshold long-reference navigation guard and add `## Quick Navigation` only to references that currently exceed it;
- add concise workflow prose clarifying that lookup aids do not replace CLI authority.

Implementation will not:
- add or change lifecycle states, gates, command flags, JSON fields, or CLI command semantics;
- export non-allowlisted support skills as host `SKILL.md` files;
- recreate retired `references/catalog/` artifacts;
- import external skills or build a registry/marketplace.

## Interfaces and Data Flow
- New internal read-only helper: a sorted view of explicit public focus records in `internal/engine/capability/surfaces.go`.
- Updated internal rendering data: `workflowSkillData` / `commandEntry` carry public focus metadata into the command-reference template.
- Updated informational rendering: `capability.BuildSkillIndexWithPaths` includes a compact focus-alias table. This file remains generated reference material only and is not read back by the kernel.
- No public CLI, JSON, state, or persisted state interfaces change.

## Rollout and Rollback
- Rollout is a normal source change to Go renderers, templates, references, tests, and governed artifacts.
- Verification:
  - `go test ./internal/engine/capability ./internal/toolgen ./cmd`
  - `go test ./...`
  - `go build ./...`
- Rollback is a revert of the source/template/test edits. No data migration or archive repair is required.

## Risk
- Public focus alias drift: mitigated by rendering from `surfacePolicy` and testing generated output.
- Host export boundary regression: mitigated by preserving existing allowlist/no-catalog tests and not adding host paths for non-exported focus-backed skills.
- Reference bloat: mitigated by adding navigation only for very long files and retaining existing byte-budget tests.
- Misleading authority prose: mitigated by explicitly saying the CLI remains authoritative and generated lookup aids are informational.
