# Tasks

## Project Context
- Tech Stack: Go, GitHub Actions, Nix
- Conventions: repo-native Go tests/builds, deterministic generated skill templates, checked-in workflow permissions, governed Slipway evidence
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, YAML, Shell, Nix

## Task List

- [x] `t-01` Tighten CI lint scope and maintained Markdown lint cleanliness.
  - wave: 1
  - depends_on: []
  - target_files: [".github/workflows/ci.yml", ".yamllint.yaml", ".markdownlint.yaml", "docs/command-contract-matrix.md"]
  - task_kind: code
  - evidence: verdict
  - acceptance: YAML lint excludes runtime/governance artifacts, Markdown lint excludes generated host templates while maintained docs stay in scope, and the unlabeled docs fence is language-tagged.
  - covers: [REQ-001]

- [x] `t-02` Fix cross-platform Go tests and generated Bash scripts.
  - wave: 1
  - depends_on: []
  - target_files: ["internal/fsutil/atomic.go", "internal/state/lifecycle.go", "internal/tmpl/templates/skills/sast-orchestration/scripts/merge-sarif.sh", "internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh", "internal/tmpl/templates/skills/gha-security-review/scripts/pin-actions.sh", "internal/engine/context/context.go", "internal/engine/context/context_test.go", "cmd/common.go", "internal/engine/progression/readiness.go"]
  - task_kind: code
  - evidence: verdict
  - acceptance: Windows directory sync errors no longer fail atomic/archive tests, generated Bash scripts avoid Bash 4-only constructs, and Go lint no longer reports the internal engine context package name conflict.
  - covers: [REQ-002]

- [x] `t-03` Update Nix package metadata.
  - wave: 1
  - depends_on: []
  - target_files: ["flake.nix"]
  - task_kind: code
  - evidence: verdict
  - acceptance: vendorHash matches the current Go dependency graph and Nix verification is run or its unavailability is recorded.
  - covers: [REQ-003]

- [x] `t-04` Repair checked-in security workflow defects.
  - wave: 1
  - depends_on: []
  - target_files: [".github/workflows/security.yaml"]
  - task_kind: code
  - evidence: verdict
  - acceptance: Security workflow grants SARIF upload run-metadata permission, normalizes govulncheck duplicate tags before upload, and adds no token or secret dependency.
  - covers: [REQ-004, REQ-006]

- [x] `t-05` Remove GitHub Pages workflow automation.
  - wave: 1
  - depends_on: []
  - target_files: [".github/workflows/docs.yml"]
  - task_kind: code
  - evidence: verdict
  - acceptance: The Pages deployment workflow is removed, documentation sources remain present, and Release Please is not altered for token/settings issues.
  - covers: [REQ-005, REQ-006]

- [x] `t-06` Run focused regression verification.
  - wave: 2
  - depends_on: [t-01, t-02, t-03, t-04, t-05]
  - target_files: ["internal/toolgen/toolgen_test.go", "internal/fsutil/atomic_test.go", "internal/state/lifecycle_test.go", "internal/engine/context/context_test.go", "cmd/common.go", "internal/engine/progression/readiness.go"]
  - task_kind: verification
  - evidence: verdict
  - acceptance: Focused Go tests for generated scripts, fsutil, state lifecycle, and renamed package callers pass, and static greps find no mapfile or declare -A in generated skill scripts.
  - covers: [REQ-001, REQ-002, REQ-006]

- [x] `t-07` Run full local Go verification.
  - wave: 3
  - depends_on: [t-06]
  - target_files: ["go.mod", "go.sum", "cmd", "internal"]
  - task_kind: verification
  - evidence: verdict
  - acceptance: `go test -timeout=20m ./... -count=1` and `go build ./...` pass locally.
  - covers: [REQ-001, REQ-002, REQ-003, REQ-004, REQ-005, REQ-006]

- [x] `t-08` Run lint and workflow-scope verification.
  - wave: 3
  - depends_on: [t-06]
  - target_files: [".github/workflows/ci.yml", ".github/workflows/security.yaml", ".yamllint.yaml", ".markdownlint.yaml", "docs/command-contract-matrix.md"]
  - task_kind: verification
  - evidence: verdict
  - acceptance: YAML lint and Markdown lint are run locally where available or tool absence is recorded, and workflow YAML changes are inspected for permissions and trigger scope.
  - covers: [REQ-001, REQ-004, REQ-006]

- [x] `t-09` Run Nix verification.
  - wave: 3
  - depends_on: [t-03]
  - target_files: ["flake.nix"]
  - task_kind: verification
  - evidence: verdict
  - acceptance: Nix build/check for the package is run where available, or tool absence/timeout is recorded with the updated hash.
  - covers: [REQ-003]

- [x] `t-10` Record residual external CI risks and governed closeout evidence.
  - wave: 4
  - depends_on: [t-06, t-07, t-08, t-09]
  - target_files: ["artifacts/changes/repair-fixable-remote-ci-failures-by-tightening-lint-scopes-cross-platform-behavior-nix-hash-and-security-workflow-configuration-while-removing-github-pages-automation/assurance.md", "artifacts/changes/repair-fixable-remote-ci-failures-by-tightening-lint-scopes-cross-platform-behavior-nix-hash-and-security-workflow-configuration-while-removing-github-pages-automation/verification"]
  - task_kind: verification
  - evidence: verdict
  - acceptance: Assurance records verification commands and residual token/settings risks, and Slipway validation/closeout checks pass or any out-of-scope blocker is explicit.
  - covers: [REQ-006]
