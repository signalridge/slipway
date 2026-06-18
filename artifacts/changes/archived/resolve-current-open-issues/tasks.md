# Tasks

## Task List

- [x] `t-01` Implement recoverable invalid selected-review context-origin evidence
  - depends_on: []
  - target_files: [`cmd/evidence.go`, `cmd/evidence_skill_test.go`, `internal/engine/progression/authority.go`, `internal/engine/progression/authority_test.go`]
  - task_kind: code
  - covers: [REQ-001]

- [x] `t-02` Make toolgen refresh transactional and ownership-safe
  - depends_on: []
  - target_files: [`cmd/init_test.go`, `internal/bootstrap/init_test.go`, `internal/toolgen/toolgen.go`, `internal/toolgen/toolgen_test.go`, `internal/toolgen/ownership_manifest.go`, `internal/toolgen/ownership_manifest_test.go`, `internal/fsutil/transaction.go`, `internal/fsutil/transaction_test.go`]
  - task_kind: code
  - covers: [REQ-002]

- [x] `t-03` Add fail-closed generated skill install profiles and namespace routers
  - depends_on: [`t-02`]
  - target_files: [`internal/toolgen/toolgen.go`, `internal/toolgen/install_profiles.go`, `internal/toolgen/install_profiles_test.go`, `internal/toolgen/testdata/skill_tree_inventory.codex.golden`, `internal/tmpl/templates/skills/surface/SKILL.md.tmpl`]
  - task_kind: code
  - covers: [REQ-003]

- [x] `t-04` Reorganize docs into Diataxis and add guided tutorials
  - depends_on: []
  - target_files: [`mkdocs.yml`, `docs/index.md`, `docs/start-here.md`, `docs/real-world-scenarios.md`, `docs/tutorials/first-governed-change.md`, `docs/tutorials/onboarding-existing-codebase.md`, `docs/how-to/install-and-refresh-adapters.md`, `docs/how-to/recover-and-troubleshoot.md`, `docs/reference/commands.md`, `docs/reference/ai-tools.md`, `docs/explanation/design.md`, `docs/explanation/workflow.md`, `docs/SURFACE-MANIFEST.json`, `internal/toolgen/surface_manifest.go`, `internal/toolgen/surface_manifest_test.go`]
  - task_kind: doc
  - covers: [REQ-004]

- [x] `t-05` Add delete-bad-tests policy and Go test-lint analyzer
  - depends_on: []
  - target_files: [`.github/workflows/ci.yml`, `docs/contributing.md`, `.golangci.yaml`, `go.mod`, `go.sum`, `internal/engine/progression/freshness_guard_test.go`, `internal/testlint/analyzer.go`, `internal/testlint/analyzer_test.go`, `internal/testlint/cmd/testlint/main.go`, `internal/testlint/testdata/src/bad/bad_test.go`, `internal/testlint/testdata/src/good/good_test.go`, `internal/testlint/testdata/src/github.com/stretchr/testify/assert/assert.go`, `internal/testlint/testdata/src/github.com/stretchr/testify/require/require.go`]
  - task_kind: code
  - covers: [REQ-005]

- [x] `t-06` Refresh GitHub tracker state for the open issue batch
  - depends_on: [`t-01`, `t-02`, `t-03`, `t-04`, `t-05`]
  - target_files: [`artifacts/changes/resolve-current-open-issues/verification/github-open-issue-recheck.md`]
  - task_kind: verification
  - covers: [REQ-006]

- [x] `t-07` Run integrated verification and governed readiness closeout prep
  - depends_on: [`t-06`]
  - target_files: [`artifacts/changes/resolve-current-open-issues/assurance.md`, `artifacts/changes/resolve-current-open-issues/verification/final-verification.md`, `artifacts/changes/resolve-current-open-issues/verification/s3-review-repair-notes.md`]
  - task_kind: verification
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]
