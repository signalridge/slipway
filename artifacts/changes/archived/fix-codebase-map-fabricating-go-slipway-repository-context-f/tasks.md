# Tasks
## Project Context
- Tech Stack: Go
- Conventions: Cobra CLI; cmd/ surfaces, internal/state durable state, internal/engine workflow logic
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-00a` RED contract tests for codebase-map behavior
  - wave: 1
  - depends_on: []
  - target_files: [internal/engine/artifact/codebase_map_test.go, cmd/codebase_map_command_test.go]
  - task_kind: test
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-007]
  - evidence: verdict (tests fail against the current Go-only / populated implementation before production changes)
  - acceptance: add failing behavioral tests for Rust/Cargo baseline facts, no-manifest blank scaffold,
    legacy Go/Slipway generated map refresh, no fabricated semantic prose, and `baseline` /
    `baseline_docs` status. Capture RED command output before any production file changes.

- [x] `t-00b` RED contract tests for from-root active-change behavior
  - wave: 1
  - depends_on: []
  - target_files: [internal/state/store_test.go, cmd/active_change_resolution_test.go]
  - task_kind: test
  - covers: [REQ-005, REQ-006, REQ-007]
  - evidence: verdict (tests fail against the current `no_active_change` diagnostic / root routing contract)
  - acceptance: add failing behavioral tests for bare root invocation returning
    `change_bound_to_other_worktree` with slug/path/remediation and for
    `next --change <slug>` from root targeting the bound worktree. Capture RED command output before
    production file changes.

- [x] `t-00d` RED contract tests for archived explicit validate behavior
  - wave: 1
  - depends_on: []
  - target_files: [cmd/common_test.go]
  - task_kind: test
  - covers: [REQ-007, REQ-013]
  - evidence: verdict (tests fail against the current empty no-active diagnostic for archived explicit validate)
  - acceptance: add failing command/helper tests proving `validate --change <archived-slug>`
    returns `archived_change_not_validatable` with slug/status/archive path and does not emit
    the empty-slug `no active change or ambiguous` diagnostic.

- [x] `t-00c` RED contract tests for stale empty bundle closeout behavior
  - wave: 2
  - depends_on: []
  - target_files: [internal/state/store_test.go, cmd/repair_test.go]
  - task_kind: test
  - covers: [REQ-011, REQ-012]
  - evidence: verdict (tests fail against the current missing-authority status / non-repairable repair behavior)
  - acceptance: add failing behavioral tests proving empty root active-bundle residue is ignored by
    active-change discovery and removed by repair, while non-empty orphan bundles remain protected.

- [x] `t-01` Language-agnostic fact detection + legacy generated map refresh
  - wave: 2
  - depends_on: [t-00a]
  - target_files: [internal/engine/artifact/codebase_map.go]
  - task_kind: code
  - covers: [REQ-001, REQ-002, REQ-003]
  - evidence: artifact (regenerated STACK/STRUCTURE/TESTING reflect detected stack; legacy Go/Slipway baseline is replaced; semantic docs blank)
  - acceptance: inspectCodebaseMapFacts detects languages/build-test/deps via manifests + bounded
    extension scan; codebaseMapBaselineDoc fills only detected fields; zero detections == blank
    template; hardcoded Slipway/Go prose is removed; known old deterministic Go/Slipway baseline docs
    are recognized and refreshed without overwriting authored content.

- [x] `t-02` Bound-elsewhere active-change diagnostic
  - wave: 2
  - depends_on: [t-00b]
  - target_files: [internal/state/store.go, cmd/common.go]
  - task_kind: code
  - covers: [REQ-005]
  - evidence: verdict (CLI returns change_bound_to_other_worktree from root)
  - acceptance: FindActiveChangeForWorktree returns a typed ChangeBoundElsewhereError listing bound
    {slug, worktreePath} when active changes exist but none match the current dir / are unbound;
    wrapResolutionError maps it to a self-explanatory precondition error with remediation.

- [x] `t-09` Safe empty orphan bundle handling
  - wave: 3
  - depends_on: [t-00c]
  - target_files: [internal/state/store.go, internal/state/health.go, cmd/repair.go]
  - task_kind: code
  - covers: [REQ-011, REQ-012]
  - evidence: verdict (status/active-change discovery ignore empty residue; repair removes it)
  - acceptance: active bundle discovery skips directories without change.yaml only when they contain
    no files; repair removes those empty orphan bundle directories and records an applied repair;
    non-empty orphan bundle directories continue to be surfaced as non-repairable integrity findings.

- [x] `t-03` Add `baseline` status + `baseline_docs` (assessment + cmd view + text)
  - wave: 3
  - depends_on: [t-01]
  - target_files: [internal/engine/artifact/codebase_map.go, cmd/codebase_map.go]
  - task_kind: code
  - covers: [REQ-004]
  - evidence: verdict (codebase-map --json reports status baseline + baseline_docs)
  - acceptance: CodebaseMapStatusBaseline added; AssessCodebaseMapDocs classifies baseline vs populated
    by comparing to regenerated baseline; aggregate baseline status; codebaseMapView.BaselineDocs +
    text writer.

- [x] `t-04` Lock `--change <slug>` worktree resolution from any directory
  - wave: 3
  - depends_on: [t-02]
  - target_files: [cmd/common.go, cmd/next.go, cmd/run.go]
  - task_kind: code
  - covers: [REQ-006]
  - evidence: verdict (next --change <slug> from root targets worktree)
  - acceptance: confirm/harden resolveExplicitChange -> ResolveChangePaths path so --change resolves the
    bound worktree from root; minimal code change only if a gap is found (behavior is expected to be
    already correct, but must remain covered by the RED test from t-00b).

- [x] `t-05` Complete codebase-map regression suite
  - wave: 4
  - depends_on: [t-01, t-03]
  - target_files: [internal/engine/artifact/codebase_map_test.go, cmd/codebase_map_command_test.go, cmd/cli_e2e_test.go]
  - task_kind: test
  - covers: [REQ-008]
  - evidence: checklist (Rust/Node/Python/no-manifest/Slipway cases; legacy generated map refresh; baseline status; updated TestCodebaseMapCommandCreatesDurableDocSet)
  - acceptance: extend the initial RED tests into full regression coverage for Node, Python, Slipway/Go,
    existing command tests, authored-content preservation, and aggregate/doc state classification.

- [x] `t-06` Complete active-change resolution regression suite
  - wave: 4
  - depends_on: [t-02, t-04]
  - target_files: [internal/state/store_test.go, cmd/active_change_resolution_test.go, cmd/common_test.go]
  - task_kind: test
  - covers: [REQ-008, REQ-013]
  - evidence: checklist (bound-elsewhere error from root; --change <slug> from root succeeds and targets worktree; archived explicit validate diagnostic)
  - acceptance: extend the initial RED tests with edge cases for zero active changes, multiple active
    bound changes, unbound active changes if applicable, JSON error details, root/worktree path
    display, and archived explicit validate behavior.

- [x] `t-10` Complete stale empty bundle regression suite
  - wave: 5
  - depends_on: [t-09]
  - target_files: [internal/state/store_test.go, cmd/repair_test.go, internal/state/health_test.go]
  - task_kind: test
  - covers: [REQ-008, REQ-011, REQ-012]
  - evidence: checklist (empty residue ignored; repair removes empty residue; non-empty orphan preserved)
  - acceptance: cover empty nested directory residue, non-empty orphan protection, and repair JSON
    `applied_repairs` shape for removed empty orphan bundles.

- [x] `t-07` Documentation for the `baseline` status
  - wave: 4
  - depends_on: [t-03]
  - target_files: [CLAUDE.md, docs/commands.md, internal/tmpl/templates/skills/context-assembly/references/codebase-map.md, internal/tmpl/templates/skills/codebase-mapping/SKILL.md]
  - task_kind: doc
  - covers: [REQ-009, REQ-013]
  - evidence: artifact (docs and agent-facing skill guidance describe the baseline status; command docs note active-only validate selector behavior)
  - acceptance: describe `baseline` as CLI-detected facts awaiting authored verification/citations;
    ensure codebase-mapping and context-assembly guidance do not treat baseline docs as completed
    brownfield analysis; document that `validate --change` selects active changes and explicit
    archived slugs fail with a concrete archived-change diagnostic.

- [x] `t-11` Archived explicit selector diagnostic
  - wave: 4
  - depends_on: [t-00d]
  - target_files: [cmd/common.go, cmd/validate.go, internal/state/lifecycle.go]
  - task_kind: code
  - covers: [REQ-013]
  - evidence: verdict (`validate --change <archived-slug>` returns archived_change_not_validatable)
  - acceptance: explicit slug resolution detects archived changes after active lookup misses,
    returns a precondition error with terminal status and archived authority path, and the
    validate flag help names an explicit active change slug.

- [x] `t-08` Full verification + guardrail compliance
  - wave: 6
  - depends_on: [t-05, t-06, t-07, t-10, t-11]
  - target_files: [internal/engine/artifact/codebase_map.go, cmd/codebase_map.go, internal/state/store.go, cmd/common.go, cmd/cli_e2e_test.go, artifacts/changes/fix-codebase-map-fabricating-go-slipway-repository-context-f/verification/tdd-governance.yaml, artifacts/changes/fix-codebase-map-fabricating-go-slipway-repository-context-f/verification/wave-orchestration.yaml]
  - task_kind: verification
  - covers: [REQ-007, REQ-008, REQ-010, REQ-013]
  - evidence: verdict (`go build ./...` && `go test ./...` green; TDD/domain/independent review evidence present)
  - acceptance: build + full test suite pass; tdd-governance verifies separate RED evidence before
    production changes for t-01/t-02/t-03/t-04; domain_review and independent_review evidence are ready
    before closeout.

## Non-Goals
- No semantic auto-authoring of ARCHITECTURE/CONVENTIONS/CONCERNS/INTEGRATIONS.
- No change to default cwd-based worktree scoping of next/run.
- No new `worktree_root` config field or separate registry file.
- No exhaustive language coverage; common ecosystems with a clean "not detected" fallback.
- No generic overwrite of user-authored codebase-map analysis when repository facts later change.
- `internal/state/stats.go` freshness model is unchanged (out of scope; import-cycle boundary).
