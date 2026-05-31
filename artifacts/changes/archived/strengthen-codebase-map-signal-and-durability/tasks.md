# Tasks
## Project Context
- Tech Stack: Go
- Conventions: repo-native (`go build ./...`, `go test ./...`)
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Execution Discipline (TDD / RED-first)
This change is `guardrail_domain: external_api_contracts`, so plan-audit and
TDD governance require a **RED test plan before execution**: each production
(`task_kind: code`) task is preceded — in a strictly earlier wave — by a RED
test task whose failing test pins the new behavior, and each production task
must capture RED→GREEN→REFACTOR evidence (failing test recorded *before* the
production edit, minimal change that flips RED→GREEN, refactor while green).

- **Go compile-RED nuance:** a RED test that references a not-yet-added struct
  field (`codebase_map_status`) will fail to *compile*, which fails the whole
  package, not just the new assertion. That compile failure, scoped to the new
  test's reference to the unimplemented field/behavior, **counts as RED** when
  captured as evidence before the GREEN edit; do not present a green run of a
  pre-existing test as RED evidence for new behavior.
- **Docs-only exemption:** `t-09` is documentation only. It is not TDD-gated;
  the README invariant it must not break is already guarded by
  `internal/toolgen/toolgen_test.go:1001` (asserts the backtick string
  `` `artifacts/codebase/` `` still appears).

## Task List

- [x] `t-01` **[RED]** Gitignore-migration tests. In
  `internal/state/local_ignore_test.go`: (a) **MOVE** `artifacts/codebase/ARCHITECTURE.md`
  from the `ignored` set to the `trackable` set in
  `TestLocalStateGitIgnoreRulesHideProofDirsButNotGovernedRecords` (~:82,:95); and
  (b) add a migration test driving `EnsureLocalStateGitIgnore` over a `.gitignore`
  holding the legacy managed block with `/artifacts/codebase/` and asserting the
  rewritten block drops that line while keeping `evidence/`/`events/`/
  `verification/`/`/.worktrees/`. RED: both fail against current code (which still
  ignores `/artifacts/codebase/`). Capture the failing run before `t-02`.
  - wave: 1
  - depends_on: []
  - target_files: [internal/state/local_ignore_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-008]

- [x] `t-02` **[GREEN]** Git-track codebase maps by default: remove
  `/artifacts/codebase/` from `localStateGitIgnorePatterns` AND migrate this
  repository's checked-in managed `.gitignore` block; keep
  evidence/events/verification/worktrees ignored. Existing repos auto-migrate via
  the `EnsureLocalStateGitIgnore` block rewrite, which runs only on `slipway new`/
  `codebase-map`/`init` (not `next`/`run`/`status`/`repair`). Minimal change that
  flips `t-01` RED→GREEN; capture GREEN evidence.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [internal/state/local_ignore.go, .gitignore]
  - task_kind: code
  - covers: [REQ-001, REQ-008]

- [x] `t-03` **[RED]** Handoff status-field tests on **BOTH** surfaces. In
  `cmd/codebase_map_context_test.go`, for a `scaffold_only` fixture assert
  `input_context.codebase_map_status == "scaffold_only"` (and
  `codebase_map_doc_states`) on the standard `next` view **and** on the compact
  `run`/handoff projection — a `next`-only assertion would miss a forgotten
  projection copy at `next_handoff.go:216-221`. **No-map case (corrected round 3):**
  assert `codebase_map_status == "missing"` with `codebase_map_doc_states` PRESENT
  (each expected doc `missing`) — NOT an absent/empty field. `omitempty` is
  asserted only as an additive guarantee for genuinely unavailable context (e.g. a
  non-governed invocation), never for the valid `missing` assessment, which must
  surface so the default freshness signal works in the empty-map case #27 targets.
  RED via the compile-ref to the unimplemented field (documented above). Capture
  before `t-04`.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/codebase_map_context_test.go]
  - task_kind: test
  - covers: [REQ-002, REQ-007, REQ-008]

- [x] `t-04` **[GREEN]** Surface map freshness. Add a shared cmd helper that calls
  `artifact.AssessCodebaseMapDocs(paths.WorkspaceRoot)` and set
  `codebase_map_status` + `codebase_map_doc_states` (both `omitempty`). The helper
  MUST assess against `paths.WorkspaceRoot` (the bound worktree), and SHOULD
  short-circuit to `status: "missing"` when the worktree's `artifacts/codebase/`
  dir is absent — `AssessCodebaseMapDocs` otherwise runs a bounded repo walk
  (`codebaseMapBaselines` → `WalkDir`, codebase_map.go:392) on every `next`/`run`.
  A no-map result MUST serialize `codebase_map_status: "missing"` with doc states
  present (not an omitted field). THREE edit points: the structs (`nextContext` in
  cmd/next.go; `nextHandoffContext` at cmd/next_handoff.go:45-52), the builders
  (cmd/next_context_build.go; cmd/next_handoff.go:171-172), AND the field-by-field
  handoff projection at cmd/next_handoff.go:216-221 — without the projection copy
  line, `slipway run --json` omits the field even though `next --json` carries it.
  Minimal change that flips `t-03` RED→GREEN; capture GREEN evidence.
  - wave: 2
  - depends_on: [t-03]
  - target_files: [cmd/next.go, cmd/next_handoff.go, cmd/next_context_build.go]
  - task_kind: code
  - covers: [REQ-002, REQ-007, REQ-008, REQ-009]

- [x] `t-05` **[RED]** Advisory **matrix** test in
  `cmd/next_skill_capability_hints_test.go`. Cover every status the classifier can
  return, because `HasEmptyCodebaseMap` does NOT line up with the advisory set
  (see REQ-003 / research.md matrix): `missing`, pure `scaffold_only`, mixed
  scaffold+baseline (= `baseline` status), `baseline`-only, and `populated`. For
  next skill ∈ {`research-orchestration`, `plan-audit`}: assert a `warnings`
  codebase-map advisory is present for `scaffold_only` AND `baseline`, and ABSENT
  for `populated` (and for `partial`, per the REQ-003 decision). Assert NO
  duplicate/contradictory signal: the existing `HasEmptyCodebaseMap` technique
  hint ("No durable codebase-map documents found") still fires for `missing`/
  all-`scaffold_only` and is not re-emitted as a second warning. **Workspace
  consistency (round 3, REQ-009):** also add a RED test driving a root-checkout
  `slipway next --change <slug> --json` against a bound worktree whose map is
  `baseline`/`populated` while the root checkout's map differs (e.g. empty);
  assert the status reflects the **worktree** map AND no "no durable docs" hint
  fires (status and hint agree). This exercises the path where the old
  `HasEmptyCodebaseMap(root, …)` probe reads the wrong checkout; it must drive the
  root `--change` invocation, not in-worktree. RED before `t-06`.
  Post-review hardening: include a direct `run --json` assertion for the
  state-mutating command surface, covering a `baseline` map consumed by
  research-orchestration so the advisory and `codebase_map_status` contract are
  not only tested through the shared compact `next --json` projection.
  - wave: 3
  - depends_on: [t-04]
  - target_files: [cmd/next_skill_capability_hints_test.go]
  - task_kind: test
  - covers: [REQ-003, REQ-008, REQ-009]

- [x] `t-06` **[GREEN]** Engine-emitted advisory in `cmd/next_skill_view.go`.
  Branch on the new `view.InputContext.CodebaseMapStatus ∈ {scaffold_only,
  baseline}` (NOT `HasEmptyCodebaseMap`, which is `true` for `scaffold_only`/
  `missing` and `false` for `baseline`), gated additionally on `nextSkillName ∈
  {progression.SkillResearchOrchestration, progression.SkillPlanAudit}` (the
  existing `StateS1Plan` gate is compatible), and append a non-blocking
  `view.Warnings` advisory. **Re-source the hint (round 3, REQ-009):** change the
  existing empty-map technique hint at next_skill_view.go:230 to branch on
  `view.InputContext.CodebaseMapStatus` (`missing`/`scaffold_only`) instead of its
  own `HasEmptyCodebaseMap(root, view.InputContext.CodebaseMapDocs)` probe, so the
  hint, the status field, and the advisory all read the one `WorkspaceRoot`
  assessment and cannot diverge under a root `--change` invocation. Preserve the
  hint's existing wording/behavior; only swap its signal source. Ensure
  `scaffold_only` is not double-signalled within a channel: the hint
  ("No durable…") and the advisory (consume-time framing) are complementary, not
  duplicate text. The projection already copies `view.Warnings`
  (next_handoff.go:226), so no handoff-projection edit is needed. Remove the
  retired exported `progression.HasEmptyCodebaseMap` helper and its self-only
  test after the hint is re-sourced, so the old divergent probe cannot survive as
  a public-looking orphan API. Minimal change that flips `t-05` RED→GREEN.
  - wave: 4
  - depends_on: [t-05]
  - target_files: [cmd/next_skill_view.go, internal/engine/progression/skill_resolution.go, internal/engine/progression/skill_resolution_test.go]
  - task_kind: code
  - covers: [REQ-003, REQ-008, REQ-009]

- [x] `t-07` **[RED]** Template-guidance assertions in
  `internal/tmpl/templates_test.go`: the regenerated research-orchestration and
  plan-audit SKILL surfaces must contain the non-durable `scaffold_only`/
  `baseline` handling guidance using the exact JSON status value, **and** an
  instruction to inspect per-doc
  `codebase_map_doc_states` (not only whole-map `codebase_map_status`) so a
  `partial` map stays actionable (REQ-004). RED: fails before the templates are
  updated. Capture before `t-08`.
  - wave: 1
  - depends_on: []
  - target_files: [internal/tmpl/templates_test.go]
  - task_kind: test
  - covers: [REQ-004, REQ-008]

- [x] `t-08` **[GREEN]** Update research-orchestration and plan-audit SKILL
  templates to treat `scaffold_only`/`baseline` maps as non-durable and to
  surface an advisory finding rather than relying on them as durable context,
  using the exact underscore status value, AND to direct consumers to inspect
  per-doc `codebase_map_doc_states` so a `partial` map (which gets no whole-map
  advisory) is still actionable. Minimal change that flips `t-07` RED→GREEN.
  - wave: 2
  - depends_on: [t-07]
  - target_files: [internal/tmpl/templates/skills/research-orchestration/SKILL.md, internal/tmpl/templates/skills/plan-audit/SKILL.md]
  - task_kind: code
  - covers: [REQ-004, REQ-008]

- [x] `t-09` Docs (documentation-only, not TDD-gated): document the new
  `codebase_map_status` handoff field and the git-tracked default for
  `artifacts/codebase`. Correct the now-false "local-only by default" narrative at
  README.md:198 and the `artifacts/codebase/**` row in docs/operator-guide.md:15.
  Preserve the backtick string `` `artifacts/codebase/` `` so
  toolgen_test.go:1001 stays green.
  - wave: 3
  - depends_on: [t-02, t-04]
  - target_files: [docs/commands.md, CLAUDE.md, internal/tmpl/templates/skills/codebase-mapping/SKILL.md, README.md, docs/operator-guide.md]
  - task_kind: doc
  - covers: [REQ-005]

- [x] `t-10` Verify BOTH `next --json` (standard view) AND `run --json` (compact
  handoff projection) default output include `codebase_map_status` matching
  `AssessCodebaseMapDocs` (e.g. `scaffold_only` for a fresh map). The `run --json`
  check specifically guards the handoff projection at next_handoff.go:216-221.
  Also verify the no-map case reports `codebase_map_status: "missing"` with
  `codebase_map_doc_states` present (not omitted) on both surfaces.
  - wave: 5
  - depends_on: [t-03, t-04]
  - target_files: [cmd/codebase_map_context_test.go, cmd/next_context_build.go, cmd/next_handoff.go]
  - task_kind: verification
  - covers: [REQ-002]

- [x] `t-11` Verify the gitignore migration: after the managed block is rewritten,
  `/artifacts/codebase/` is gone while evidence/events/verification/worktrees
  remain, and `git check-ignore artifacts/codebase/ARCHITECTURE.md` exits
  non-zero in both the migration fixture and this repository worktree.
  - wave: 5
  - depends_on: [t-01, t-02]
  - target_files: [internal/state/local_ignore.go, internal/state/local_ignore_test.go, .gitignore]
  - task_kind: verification
  - covers: [REQ-001]

- [x] `t-12` Verify the advisory matrix: for `missing`/pure-`scaffold_only`/mixed
  scaffold+baseline/`baseline`/`populated`, the `warnings` advisory is present
  exactly for `scaffold_only` and `baseline` consumed by research/plan-audit,
  absent for `populated`/`partial`, with no duplicate "No durable codebase-map
  documents found" signal. Also verify workspace consistency (REQ-009): a
  root-checkout `next --change <slug>` against a bound worktree with a
  `baseline`/`populated` map reports that status with no contradicting "no durable
  docs" hint.
  - wave: 5
  - depends_on: [t-05, t-06]
  - target_files: [cmd/next_skill_capability_hints_test.go, cmd/next_skill_view.go, internal/tmpl/templates/skills/research-orchestration/SKILL.md, internal/tmpl/templates/skills/plan-audit/SKILL.md]
  - task_kind: verification
  - covers: [REQ-003, REQ-004, REQ-009]

- [x] `t-13` Verify RED→GREEN evidence is captured for every code task
  (t-02/t-04/t-06/t-08 — RED recorded before each production edit), additive-
  contract compliance (new fields are `omitempty`, no existing field changed), and
  suite health: `go build ./...` and `go test ./...` pass.
  - wave: 6
  - depends_on: [t-09, t-10, t-11, t-12]
  - target_files: [artifacts/changes/strengthen-codebase-map-signal-and-durability/verification/tdd-governance.yaml, artifacts/changes/strengthen-codebase-map-signal-and-durability/verification/wave-orchestration.yaml, artifacts/changes/strengthen-codebase-map-signal-and-durability/assurance.md]
  - task_kind: verification
  - covers: [REQ-006, REQ-007, REQ-008]
