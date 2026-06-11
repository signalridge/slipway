# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `internal/toolgen/toolgen.go`: adapter registry, command registry, governance skill descriptors, generated skill/command paths, deterministic `Generate`.
  - `internal/tmpl/templates/`: authored command/skill templates and support files embedded by `internal/tmpl/templates.go`.
  - `internal/engine/capability/`: public focus/skill surface metadata used by skill indexes and route surfaces.
  - `cmd/`: Cobra command/flag definitions and JSON-emitting command implementations.
  - `README.md`, `docs/ai-tools.md`, `docs/commands.md`, `docs/operator-guide.md`: public docs rows for generated and JSON surfaces.
- Dependency chains:
  - `commandRegistry` -> command descriptions/arguments -> command reference and adapter prompt surfaces.
  - `governanceSurfaceDescriptors` plus capability registry -> generated host skills and skill index.
  - Cobra command constructors -> generated template flag contracts and JSON command behavior.
  - docs tables/prose -> public operator contract for generated adapter paths and JSON contracts.
- Blast radius: medium, bounded to `internal/toolgen`, a small regeneration entrypoint, docs, and tests if the generator reads existing registries rather than changing runtime lifecycle code.
- Constraints:
  - Do not weaken existing README/token tests.
  - Do not introduce a second command or skill registry.
  - Keep regenerated output deterministic and diffable.

### Patterns
- Existing conventions:
  - `internal/toolgen` is the local source for generated adapter surfaces and already owns package-local tests for registry drift.
  - `internal/toolgen/support_files_test.go` already uses a generated inventory/golden pattern for the Codex skill tree.
  - `cmd/template_flag_contract_test.go` already enforces generated-template/Cobra drift in both directions.
  - Docs already centralize generated adapter paths in `README.md` and `docs/ai-tools.md`, and JSON contracts in `docs/operator-guide.md`.
- Reusable abstractions:
  - `Registry`, `GeneratedAdapterMarkerPath`, `SkillPath`, `commandIDs`, command metadata, governance descriptors, and capability registry helpers can supply manifest rows.
  - Existing generated-output tests can continue proving renderability while the new manifest proves inventory and docs coverage.
- Convention deviations:
  - A committed JSON manifest is new for this repo, but it matches the existing golden-manifest testing style and the issue's requested fail-closed behavior.

### Risks
- Technical risks:
  - Medium: duplicate source lists could drift from command/skill registries. Mitigation: derive rows from existing registries and expose package-local helpers only where needed.
  - Medium: docs-row checks can become brittle if they parse broad prose. Mitigation: use stable row/table tokens and path references, not whole-document snapshots.
  - Low: generated JSON ordering/dates can create noisy diffs. Mitigation: deterministic sort and avoid volatile timestamps in check comparisons.
- Guardrail domains: none detected.
- Reversibility: high. The change adds generator/test/docs artifacts and can be reverted without data migration or runtime state changes.

### Test Strategy
- Existing coverage:
  - `internal/toolgen/toolgen_test.go` checks command registry completeness, generated command surfaces, generated skill command references, and README command descriptions.
  - `internal/toolgen/adapter_contract_test.go` freezes per-tool command path contracts.
  - `internal/toolgen/support_files_test.go` freezes generated Codex skill tree structure.
  - `cmd/template_flag_contract_test.go` checks generated template flag usage against real Cobra commands and the reverse flag-to-registry coverage.
- Infrastructure needs:
  - A Go-native manifest builder in `internal/toolgen`.
  - A small regeneration entrypoint with explicit check and write modes that produces the same JSON as the tests.
  - A committed public manifest under `docs/` because the inventory represents operator-facing product surfaces, not only package internals.
- Verification approach:
  - Test stale manifest by rebuilding live rows and comparing against the committed file.
  - Test check/write behavior through the regeneration entrypoint or shared builder path.
  - Test docs coverage by making each manifest row include the stable docs reference/token expected for that surface family.
  - Preserve the existing README token/description test and add stable README/docs coverage derived from the manifest where practical.
  - Run targeted `go test ./internal/toolgen ./cmd`, then full `go test ./...`.

### Options
- Option A: filesystem-scanning script outside the Go package.
  - Tradeoffs: Simple to run, but adds a non-Go toolchain path and duplicates Slipway registries by filesystem scanning.
- Option B: Go-native `internal/toolgen` manifest builder, committed public JSON, check/write regeneration entrypoint, and package tests.
  - Tradeoffs: Best fit for Slipway because it reads the existing Go registries, remains deterministic in normal `go test`, keeps generator logic near adapter generation, and still gives maintainers an explicit regeneration surface.
- Option C: Runtime `slipway surface-manifest` CLI command.
  - Tradeoffs: More discoverable for operators, but wider public CLI surface and command docs burden before the internal contract is proven.
- Selected: Option B in full. This satisfies issue #158 while keeping source authority in the package that already owns generated surfaces and giving CI and maintainers the same deterministic manifest contract.

## Unknowns
- Resolved: Which existing Slipway files own generated skills, command metadata, JSON/user-facing contracts, and docs rows? -> `internal/toolgen/toolgen.go`, `internal/tmpl/templates/`, `internal/engine/capability/`, `cmd/`, `README.md`, `docs/ai-tools.md`, `docs/commands.md`, and `docs/operator-guide.md`.
- Resolved: What inventory-manifest mechanics are worth adapting? -> Build live inventory, support deterministic write/check behavior, commit JSON, and make tests fail with useful additions/removals. Do not import an external script implementation or surface family model.
- Resolved: Where should the committed manifest live, and what generator/test entrypoint matches Slipway? -> Use `docs/SURFACE-MANIFEST.json` because the file describes public/operator-facing product surfaces. Implement a Go-native builder in `internal/toolgen` plus a deterministic check/write regeneration entrypoint used by tests.
- Resolved: How should S2 wave evidence be recorded when `execution-summary.yaml` does not exist yet? -> `wave-orchestration` is the bootstrap stage that creates execution-summary evidence, so its `slipway evidence skill` path derives the run version from a single-version task evidence ledger. Later run-summary-bound skills continue to require an existing execution summary.
- Remaining: None.

## Assumptions
- A manifest row is useful only if it names the source authority and expected docs representation for the surface. Evidence: issue #158 asks for generated skills, CLI commands, JSON contracts, and docs to stay one product surface.
- `internal/toolgen` is the lowest-risk implementation home. Evidence: `toolgen.Generate` and the existing toolgen tests already own generated adapter outputs and contract drift checks.
- Codebase-map docs are semantically stale for this change and are treated as advisory only during this recovery audit. Evidence: `artifacts/codebase/ARCHITECTURE.md`, `artifacts/codebase/STRUCTURE.md`, `artifacts/codebase/TESTING.md`, and `artifacts/codebase/CONCERNS.md` still name issue #151, while the issue #158 plan authority lives in this governed bundle and the current implementation diff.
- S2 wave-orchestration evidence is the only run-summary-bound skill evidence that can be recorded before `execution-summary.yaml`; its run version comes from runtime task evidence. Evidence: `cmd/evidence.go` and `cmd/evidence_task_test.go`.
- User confirmation for selecting the recommended approach and full implementation scope is covered by the objective's instruction to make the best choice when blocked plus the later instruction to fully implement the issue rather than a narrow subset. Evidence: active thread objective and follow-up scope refinement.

## Canonical References
- `https://github.com/signalridge/slipway/issues/158`
- `internal/toolgen/toolgen.go`
- `internal/toolgen/toolgen_test.go`
- `internal/toolgen/adapter_contract_test.go`
- `internal/toolgen/support_files_test.go`
- `cmd/template_flag_contract_test.go`
- `README.md`
- `docs/ai-tools.md`
- `docs/commands.md`
- `docs/operator-guide.md`
- `cmd/evidence.go`
- `cmd/evidence_task_test.go`
- `artifacts/codebase/ARCHITECTURE.md` (advisory; stale issue #151 map)
- `artifacts/codebase/TESTING.md` (advisory; stale issue #151 map)
- `artifacts/codebase/CONCERNS.md` (advisory; stale issue #151 map)
