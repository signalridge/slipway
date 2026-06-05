# Requirements
## Project Context
- Tech Stack: Go
- Conventions: catalog-skill binding-compare gate; deterministic toolgen; checkbox-native tasks.md
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Requirements

### Requirement: test-design catalog skill registration (Part A)
REQ-001: The system MUST register a `test-design` catalog skill in the Go
capability registry (`registry_b6.go`, wired into `defaultSkills()`), classified
`domain=verification`, `tier=T1`, `primary_attachment=procedure`,
`evidence_contract=artifact`, with a `technique-hint` binding to
`wave-orchestration` (only) and a hydrate-reference set for its `references/`
files.

#### Scenario: Registry exposes test-design
GIVEN the default capability registry
WHEN `test-design` is looked up
THEN it resolves with domain `verification`, tier `T1`, a technique-hint binding
to `wave-orchestration`, and a non-empty hydrate set, and
`go test ./internal/engine/capability/...` (registry + binding-compare gates)
passes.

### Requirement: test-design templates mirror the registry and stay generalized (Part A)
REQ-002: The system MUST ship `internal/tmpl/templates/skills/test-design/SKILL.md`
(a thin router whose frontmatter mirrors the registry record exactly and whose
body is within the T1 size budget) plus `references/*.md` carrying the judgment
depth — test-double strategy, behavior-vs-implementation, formal case
enumeration with explicit oracles, property reasoning, and test-data discipline.

#### Scenario: Frontmatter matches registry and content is language-neutral
GIVEN the test-design templates
WHEN the binding-compare and size-budget gates run
THEN frontmatter bindings and hydrate_references equal the registry record, the
body is within the T1 budget, and the references teach the five judgment topics
without naming any concrete language or framework.

### Requirement: test-design exported as a host skill with fixtures updated (Part A)
REQ-003: The system MUST add `test-design` to `hostSkillExportAllowlist` and keep
the gate fixtures consistent: the `registry_test.go` ID list includes
`test-design`, the exported host-skill count in `toolgen_test.go` is updated
(22 → 23), and the `internal/toolgen/testdata/skill_tree_inventory.codex.golden`
manifest is regenerated to include the `slipway-test-design/SKILL.md` and one
`slipway-test-design/references/*.md` row per reference file.

#### Scenario: Allowlist, counts, and golden manifest agree
GIVEN the toolgen and registry tests
WHEN `go test ./internal/toolgen/... ./internal/engine/capability/...` runs
THEN `ShouldExportAsHostSkill("test-design")` is true, the exported host-skill
count assertion passes at 23, the registry ID-list test includes test-design, and
`TestGeneratedSkillTreeInventoryManifest` passes against the regenerated golden.

### Requirement: technique-hint schema carries capability routing fields (Part B)
REQ-004: The system MUST extend the `techniqueHint` type (`cmd/next.go`) with
optional `kind`, `capability`, `language`, and `optional` fields, propagated
through the compact handoff projection used by `run`.

#### Scenario: Capability hint round-trips through next and run
GIVEN a `capability:language-testing` hint
WHEN it is emitted on `next` and cloned into the `run`/handoff projection
THEN the `kind`/`capability`/`language`/`optional` fields are preserved verbatim.

### Requirement: language-aware capability hint emitted per detected language (Part B)
REQ-005: The system MUST emit one `capability:language-testing` technique hint
per detected language. Language source MUST be `Change.ProjectContext.Languages`
when non-empty, and MUST fall back to codebase-map `STACK.md` ONLY when
`ProjectContext.Languages` is empty (the two sources MUST NOT be merged). The
emission MUST be gated on the presence of the `skill:test-design` hint, MUST NOT
be allowlist-gated, MUST be deduplicated by language, MUST emit nothing when no
language is detected (including a scaffold/empty `STACK.md` `- Languages:` line),
and MUST behave identically on `next` and `run`.

#### Scenario: Per-language emission and the empty case
GIVEN a change whose `ProjectContext.Languages` is `[Go, TypeScript]` at a host
that hints test-design
WHEN `next`/`run` is queried
THEN `technique_hints` include `skill:test-design` and exactly two
`capability:language-testing` hints (`Go`, `TypeScript`); AND GIVEN a change with
no detected language THEN only `skill:test-design` is present with no language
hint.

#### Scenario: ProjectContext languages take precedence over STACK.md
GIVEN a change whose `ProjectContext.Languages` is `[Go]` while codebase-map
`STACK.md` lists `Go, Python`
WHEN `next`/`run` is queried
THEN exactly one `capability:language-testing` hint is emitted (`Go`) and none
for `Python` (STACK.md is not consulted while `ProjectContext.Languages` is
non-empty); AND GIVEN `ProjectContext.Languages` is empty while `STACK.md` lists
`Go, Python` THEN two hints (`Go`, `Python`) are emitted from the fallback.

### Requirement: test-design hint surfaces at the authoring host (Part B)
REQ-006: The system MUST surface `skill:test-design` in `technique_hints` at the
`wave-orchestration` execution host (the single live next-skill authoring host).
It MUST NOT bind a technique hint to `tdd-governance`, which is `ExportOnlyExtra`
and never resolves as a next-skill; consequently the existing
`TestAppendCatalogHintsVerifyHostsDoNotEmitRetiredFreshEvidence` assertion
(covering `tdd-governance`) stays valid unchanged.

#### Scenario: Authoring host emits test-design; tdd-governance does not
GIVEN host `wave-orchestration`
WHEN catalog hints are resolved
THEN `skill:test-design` is present among the emitted hints; AND GIVEN host
`tdd-governance` WHEN catalog hints are resolved THEN no `skill:test-design` hint
is emitted.

### Requirement: host-resolution contract documented (Part B)
REQ-007: The system MUST document, in CLAUDE.md, the
`capability:language-testing` host-resolution contract — that Slipway emits a
vendor-neutral capability + language descriptor (one per detected language) and
the host resolves EACH such hint, by its language, to its own installed language
testing skill. The contract MUST cover the multi-hint polyglot case (N detected
languages → N hints, each resolved independently), not merely the single-language
case, and MUST note that `capability:`-kind hints are not Slipway host skills.

#### Scenario: CLAUDE.md describes the contract
GIVEN CLAUDE.md
WHEN the Routed Commands / handoff section is read
THEN it states that `capability:`-kind hints are not Slipway host skills and that
the host resolves each emitted `capability:language-testing` hint (one per
detected language, including the polyglot N-hint case) to its own installed
language testing skill.

### Requirement: wave-orchestration encodes test/implementation isolation (Part C)
REQ-008: The system MUST extend the `wave-orchestration` host SKILL.md Dispatch
Contract so a test-bearing unit is executed as an isolated test-authoring step
(scoped to spec/acceptance + public API signatures, never the implementation)
producing frozen tests, followed by an implementation step (a later
`depends_on` task) that must satisfy those frozen tests — anchored on the
existing `task_kind=test`→`task_kind=code` + `depends_on` structure. The rule
MUST encode only the orchestration mechanics (executor-context isolation + freeze
ordering); it MUST NOT restate the behavior-vs-implementation judgment taught by
the Part A `test-design` references, so layer ① (orchestration) and layer ②
(judgment) stay non-overlapping.

#### Scenario: Dispatch contract describes isolation
GIVEN the rendered wave-orchestration host skill
WHEN the Dispatch Contract section is read
THEN it instructs isolating the test-authoring step from the implementation and
freezing the tests before implementation, without claiming engine-level rejection,
and without restating the Part A behavior-vs-implementation judgment content.

### Requirement: tdd-governance treats frozen tests as the RED proof (Part C)
REQ-009: The system MUST note in the `tdd-governance` host SKILL.md that, for an
isolated test-authoring step, the frozen test-authoring commit is the canonical
RED proof consumed by the existing git-history verification.

#### Scenario: tdd-governance references the frozen RED proof
GIVEN the rendered tdd-governance host skill
WHEN its RED evidence section is read
THEN it identifies the frozen test-authoring commit as the RED proof for
isolation-structured tasks.

### Requirement: generalized-only guard test (cross-cutting)
REQ-010: The system MUST add a guard test asserting the Slipway-owned
`test-design` templates contain no language-specific test syntax (e.g. `t.Run`,
`pytest`, `describe(`).

#### Scenario: Guard rejects language syntax
GIVEN the test-design SKILL.md and references
WHEN the guard test scans them
THEN no banned language-specific test token is found and the test passes.

### Requirement: build and full test suite green (cross-cutting)
REQ-011: The system MUST keep `go build ./...` and `go test -count=1 ./...`
passing after all changes.

#### Scenario: Clean build and tests
GIVEN the final working tree
WHEN `go build ./...` and `go test -count=1 ./...` run
THEN both succeed with no failures.

### Requirement: disabled_controls does not gate the language hint (Part B)
REQ-012: The `capability:language-testing` hint MUST be independent of
`disabled_controls`: it is a vendor-neutral advisory technique hint, not a
governance control, and maps to no control ID. Setting any `disabled_controls`
value MUST NOT change whether the hint is emitted.

#### Scenario: disabled_controls leaves the hint unchanged
GIVEN a Go change that emits one `capability:language-testing` hint
WHEN `disabled_controls` is set to any control ID (e.g. `["research"]`)
THEN the same `capability:language-testing` hint is still emitted, unchanged.
