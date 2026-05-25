# Tasks

## Project Context
- Tech Stack: Go, GitHub Actions, shell
- Conventions: keep implementation scoped to CI/CD, release, maintenance, and required version support; preserve Slipway lifecycle behavior; guard secret-dependent publication paths.
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go, YAML, Shell

## Task List

- [x] `t-01` Add Slipway version metadata for release verification.
  - wave: 1
  - depends_on: []
  - target_files: [cmd/root.go, cmd/root_version_test.go]
  - task_kind: code
  Acceptance detail: `go test ./cmd -run Version -count=1` or equivalent focused root-command test passes, and a built or `go run` binary can print version metadata.
  - covers: [REQ-003, REQ-004]

- [x] `t-02` Add baseline repository tooling and distribution scaffolding.
  - wave: 1
  - depends_on: []
  - target_files: [.dockerignore, .golangci.yaml, .yamllint.yaml, .markdownlint.yaml, justfile, Dockerfile, flake.nix, mkdocs.yml]
  - task_kind: code
  Acceptance detail: configuration files use Slipway names/paths, avoid clinvoker identifiers, and keep container build context bounded.
  - covers: [REQ-002, REQ-005, REQ-006]

- [x] `t-03` Add release metadata files for automated changelog and release PRs.
  - wave: 1
  - depends_on: []
  - target_files: [release-please-config.json, .release-please-manifest.json, CHANGELOG.md]
  - task_kind: code
  Acceptance detail: release-please config targets `github.com/signalridge/slipway`, uses the `go` release type, and updates the Slipway changelog/manifest.
  - covers: [REQ-003, REQ-005]

- [x] `t-04` Add CI, maintenance, security, docs, and Nix workflows.
  - wave: 2
  - depends_on: [t-01, t-02]
  - target_files: [.github/workflows/ci.yml, .github/workflows/security.yaml, .github/workflows/pr-title.yaml, .github/workflows/docs.yml, .github/workflows/nix.yaml, .github/workflows/flake-lock-update.yaml, .github/dependabot.yml]
  - task_kind: code
  Acceptance detail: workflows cover tests/vet/lint/race/build, PR title validation, Dependabot, security scanning, docs checks/deploy, and Nix checks without stale Go version pins.
  - covers: [REQ-002, REQ-005, REQ-006]

- [x] `t-05` Add GoReleaser and tag/manual release workflow for full distribution breadth.
  - wave: 2
  - depends_on: [t-01, t-02, t-03]
  - target_files: [.goreleaser.yaml, Dockerfile.goreleaser, .github/workflows/release.yaml, .github/workflows/release-please.yaml]
  - task_kind: code
  Acceptance detail: release workflow tests first, builds Slipway artifacts, verifies version/help output, and guards external package/container/signing publication with event and secret conditions.
  - covers: [REQ-003, REQ-004, REQ-005, REQ-006]

- [x] `t-06` Update operator documentation and workflow feedback for release/maintenance enablement.
  - wave: 2
  - depends_on: [t-01, t-02, t-03]
  - target_files: [README.md, artifacts/changes/reference-users-yixianlu-projects-clinvoker-to-add-ci-cd-release-and-maintenance-capabilities-while-exercising-the-full-slipway-governed-workflow/workflow-feedback.md]
  - task_kind: code
  Acceptance detail: docs identify local verification commands and required external repository secrets/settings without claiming they are configured locally.
  - covers: [REQ-001, REQ-005, REQ-006, REQ-007]

- [x] `t-07` Run local implementation verification.
  - wave: 3
  - depends_on: [t-04, t-05, t-06]
  - target_files: [artifacts/changes/reference-users-yixianlu-projects-clinvoker-to-add-ci-cd-release-and-maintenance-capabilities-while-exercising-the-full-slipway-governed-workflow/assurance.md]
  - task_kind: verification
  Acceptance detail: `go test -timeout=20m ./... -count=1`, `go build ./...`, focused version checks, and available config checks such as `goreleaser check` or actionlint are recorded with pass/fail rationale.
  - covers: [REQ-002, REQ-003, REQ-004, REQ-005, REQ-006, REQ-007]

- [x] `t-08` Complete governance verification and closeout artifacts.
  - wave: 4
  - depends_on: [t-07]
  - target_files: [artifacts/changes/reference-users-yixianlu-projects-clinvoker-to-add-ci-cd-release-and-maintenance-capabilities-while-exercising-the-full-slipway-governed-workflow/tasks.md, artifacts/changes/reference-users-yixianlu-projects-clinvoker-to-add-ci-cd-release-and-maintenance-capabilities-while-exercising-the-full-slipway-governed-workflow/assurance.md]
  - task_kind: verification
  Acceptance detail: Slipway `validate`, task completion state, and assurance requirement coverage support entering governed review; downstream review evidence is produced by the S3/S4 review and closeout skills.
  - covers: [REQ-001, REQ-007]
