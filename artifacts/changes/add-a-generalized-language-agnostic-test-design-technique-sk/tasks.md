# Tasks
## Project Context
- Tech Stack: Go
- Conventions: catalog-skill binding-compare gate; deterministic toolgen; checkbox-native tasks.md
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [ ] `task-001` Author the Part A + Part C template-content contract tests (RED)
  - wave: 1
  - depends_on: []
  - target_files: [internal/tmpl/test_design_content_test.go, internal/tmpl/wave_isolation_content_test.go]
  - task_kind: test
  - covers: [REQ-002, REQ-008, REQ-009, REQ-010]
  - acceptance: "The language-neutral content contract is frozen BEFORE the templates/prose exist, so it is RED now (this is the RED proof for the Part A code in task-002 and the Part C prose in task-003). (1) Generalized-only guard scans test-design SKILL.md + references and FAILS on banned language-specific test syntax (t.Run, pytest, def test_, describe(, expect(, @Test). (2) Part A content asserts the five judgment topics are present across references (test-double strategy, behavior-vs-implementation, formal case enumeration with explicit oracles, property reasoning, test-data discipline). (3) Part C render content renders the wave-orchestration + tdd-governance host skills and asserts the Dispatch Contract carries a test/implementation isolation rule (test-authoring step scoped to spec + public API only, frozen before implementation) and that tdd-governance names the frozen test-authoring commit as the RED proof — asserted on stable concept tokens, NOT exact prose, so later wording edits do not break it. All three groups are RED until task-002/task-003 land the templates and prose."
  - evidence: "artifact: a STANDALONE RED commit carrying failing go test ./internal/tmpl/... that proves the content contract is unmet — committed BEFORE the task-002/task-003 implementation commits, so git history shows test-first rather than same-commit (this is the explicit RED proof for tdd-governance's Git History Verification Protocol, which rejects same-commit test+implementation)"

- [ ] `task-002` Register `test-design` atomically: registry + templates + references + allowlist + fixtures (GREEN for Part A)
  - wave: 2
  - depends_on: [task-001]
  - target_files: [internal/engine/capability/registry_b6.go, internal/engine/capability/registry_default.go, internal/tmpl/templates/skills/test-design/SKILL.md, internal/tmpl/templates/skills/test-design/references/test-doubles.md, internal/tmpl/templates/skills/test-design/references/behavior-vs-implementation.md, internal/tmpl/templates/skills/test-design/references/case-enumeration.md, internal/tmpl/templates/skills/test-design/references/property-reasoning.md, internal/tmpl/templates/skills/test-design/references/test-data.md, internal/toolgen/toolgen.go, internal/engine/capability/registry_test.go, internal/toolgen/toolgen_test.go, internal/toolgen/testdata/skill_tree_inventory.codex.golden]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003, REQ-006]
  - acceptance: "ATOMIC — the registry record and its templates land in this one task, so no intermediate state ever exposes a registry skill without a matching SKILL.md; the binding-compare gate is satisfied within this task, not deferred. testDesign() (verification/T1/procedure/artifact, technique-hint -> wave-orchestration ONLY, 5 hydrate refs) is in defaultSkills(); test-design/SKILL.md frontmatter mirrors the registry exactly (binding-compare + hydrate + T1 size-budget gates pass) and references/ carry the language-neutral depth that satisfies task-001's Part A guard + content assertions; test-design is in hostSkillExportAllowlist; registry_test ID list includes test-design; the EXPORTED host-skill count assertion in toolgen_test.go moves 22 -> 23 (this is the exported-skill count, which is one less than the allowlist member count because ExportOnlyExtra entries are excluded — do not edit by counting allowlist members); skill_tree_inventory.codex.golden regenerated with the slipway-test-design rows (UPDATE_GOLDEN=1, then re-assert without it). task-001's Part A assertions are now GREEN. go build ./... and go test ./internal/engine/capability/... ./internal/toolgen/... ./internal/tmpl/... green. This implementation lands in a commit AFTER task-001's RED commit (never squashed into it), so the Part A RED→GREEN transition is provable from git history per tdd-governance."
  - evidence: "artifact: go test output for capability + toolgen + tmpl packages (gates + golden + task-001 Part A assertions GREEN)"

- [ ] `task-003` Encode Part C test/implementation isolation in the host templates (GREEN for Part C)
  - wave: 2
  - depends_on: [task-001]
  - target_files: [internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl, internal/tmpl/templates/skills/tdd-governance/SKILL.md.tmpl]
  - task_kind: code
  - covers: [REQ-008, REQ-009]
  - acceptance: "wave-orchestration Dispatch Contract gains a test/implementation ISOLATION rule encoding ONLY the orchestration mechanics: isolate the test-authoring step (spec + public API only, never the implementation) from implementation and freeze the tests first, anchored on the existing task_kind=test->code + depends_on and the current 'one isolated executor context per task' / 'pass file paths only' rules. It MUST NOT restate the behavior-vs-implementation judgment content that lives in Part A's references — layer 1 (orchestration) and layer 2 (judgment) stay non-overlapping. tdd-governance names the frozen test-authoring commit as the RED proof for isolation-structured tasks. Host-enforced wording only; no engine-rejection over-claim. task-001's Part C render assertions are now GREEN. Part C edits file content (no new/removed files), so the structural golden is unaffected; keep go test ./internal/tmpl/... ./internal/toolgen/... green. This prose lands in a commit AFTER task-001's RED commit, preserving git-history test-first for the Part C assertions."
  - evidence: "artifact: go test ./internal/tmpl/... green (task-001 Part C assertions GREEN)"

- [ ] `task-004` Author the Part B language-hint tests against the frozen public API (RED)
  - wave: 3
  - depends_on: [task-002]
  - target_files: [cmd/next_skill_language_hint_test.go, cmd/next_skill_capability_hints_test.go]
  - task_kind: test
  - covers: [REQ-004, REQ-005, REQ-006, REQ-012]
  - acceptance: "Tests encode REQ-004/005/006/012 from the spec + the public emitter signature only: skill:test-design present at wave-orchestration AND absent at tdd-governance (REQ-006, GREEN against task-002's registration); per-language emission; no-language empty case; dedupe; not allowlist-gated; next/run parity; ProjectContext.Languages precedence over STACK.md (Go-only context yields only Go, never STACK's Python); STACK.md fallback only when ProjectContext.Languages is empty; an explicit polyglot case (e.g. Go+TypeScript -> exactly two capability:language-testing hints, each resolved independently) so the per-language contract is pinned, not just the single-language path; disabled_controls does not change emission (REQ-012). The wave-orchestration / tdd-governance assertions pass against task-002; the language-hint assertions are RED (fail/compile-fail) until task-005."
  - evidence: "artifact: a STANDALONE RED commit whose failing go test ./cmd/... proves the language-hint behavior is unimplemented — committed BEFORE task-005 implements it, so git history shows test-first (not same-commit) per tdd-governance"

- [ ] `task-005` Implement the Part B schema + per-language emitter to satisfy the frozen tests (GREEN)
  - wave: 4
  - depends_on: [task-004]
  - target_files: [cmd/next.go, cmd/next_skill_view.go, cmd/next_handoff.go]
  - task_kind: code
  - covers: [REQ-004, REQ-005, REQ-012]
  - acceptance: "techniqueHint gains kind/capability/language/optional (cloned in the handoff projection); appendLanguageTestingHints emits one capability:language-testing hint per detected language from ProjectContext.Languages, falling back to STACK.md only when ProjectContext.Languages is empty (never merged), gated on skill:test-design, not allowlist-gated, deduped, none when no language, independent of disabled_controls, wired for next and run. task-004 tests now pass; go test ./cmd/... green. Implemented in a commit AFTER task-004's RED commit (not same-commit), so git history shows test-first."
  - evidence: "artifact: go test ./cmd/... green"

- [ ] `task-006` Document the capability:language-testing host-resolution contract
  - wave: 5
  - depends_on: [task-005]
  - target_files: [CLAUDE.md]
  - task_kind: doc
  - covers: [REQ-007]
  - acceptance: "CLAUDE.md states that capability:-kind hints are vendor-neutral capability+language descriptors that are NOT Slipway host skills, and that the host resolves EACH emitted capability:language-testing hint (one per detected language) to its own installed language testing skill — explicitly covering the multi-hint polyglot case (N languages -> N hints, each resolved independently), not just a single hint. It also states disabled_controls does not gate capability:language-testing."
  - evidence: "checklist: CLAUDE.md section reviewed against REQ-007"

- [ ] `task-007` Final build and full test-suite verification
  - wave: 5
  - depends_on: [task-002, task-003, task-005]
  - target_files: [artifacts/changes/add-a-generalized-language-agnostic-test-design-technique-sk/verification/wave-orchestration.yaml]
  - task_kind: verification
  - covers: [REQ-011]
  - acceptance: "go build ./... and go test -count=1 ./... both pass on the final tree. (task-006 is doc-only — CLAUDE.md is not compiled or tested — so it does not gate this verification and runs in parallel in the same wave.)"
  - evidence: "artifact: build + full go test output"
