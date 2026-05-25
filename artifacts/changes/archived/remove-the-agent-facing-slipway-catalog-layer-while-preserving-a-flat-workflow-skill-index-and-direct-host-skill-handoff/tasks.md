# Tasks

## Project Context
- Tech Stack: Go CLI, generated Codex skills
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go

## Task List

- [x] `t-01` Replace the generated catalog manifest with a workflow-owned skill index.
  - wave: 1
  - depends_on: []
  - target_files: [`internal/toolgen/toolgen.go`, `internal/engine/capability/export.go`]
  - task_kind: code
  - evidence: artifact
  - covers: [REQ-001, REQ-002, REQ-003]
  - acceptance: `SkillIndexPath` is generated under `slipway/references/skill-index.md`; no `CatalogManifestPath` or top-level `using-slipway-catalog.md` generation remains.

- [x] `t-02` Remove agent-facing catalog route-card/support emission and stale cleanup gaps.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [`internal/toolgen/toolgen.go`]
  - task_kind: code
  - evidence: artifact
  - covers: [REQ-001, REQ-004, REQ-005]
  - acceptance: generator no longer emits `slipway/references/catalog/**`; refresh cleanup removes retired generated catalog paths.

- [x] `t-03` Update workflow skill wording and next-skill hint labels for direct host handoff.
  - wave: 2
  - depends_on: [t-01]
  - target_files: [`internal/tmpl/templates/skills/workflow/SKILL.md.tmpl`, `cmd/next_skill_view.go`, `internal/engine/capability/registry_b3.go`, `internal/engine/capability/registry_b5.go`, `internal/tmpl/templates/skills/incident-response/SKILL.md`, `internal/tmpl/templates/skills/threat-modeling/SKILL.md`]
  - task_kind: code
  - evidence: artifact
  - covers: [REQ-002, REQ-003]
  - acceptance: generated workflow guidance names `references/skill-index.md` as informational and directs agents to `.codex/skills/slipway-<name>/SKILL.md`; hint labels do not include catalog paths.

- [x] `t-04` Update regression tests and golden inventory for the new generated layout.
  - wave: 3
  - depends_on: [t-01, t-02, t-03]
  - target_files: [`internal/toolgen/toolgen_test.go`, `internal/engine/capability/export_test.go`, `cmd/next_skill_capability_hints_test.go`, `internal/toolgen/testdata/skill_tree_inventory.codex.golden`]
  - task_kind: code
  - evidence: verdict
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
  - acceptance: focused tests fail on old catalog paths and pass with the new skill-index/direct-handoff layout.

- [x] `t-05` Verify generated contract and Go suite.
  - wave: 4
  - depends_on: [t-04]
  - target_files: [`internal/toolgen`, `internal/engine/capability`, `cmd`]
  - task_kind: verification
  - evidence: verdict
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]
  - acceptance: focused tests, `go test -timeout=20m ./... -count=1`, and `go build ./...` pass; generated Codex inventory contains `slipway/references/skill-index.md` and no retired catalog paths.
