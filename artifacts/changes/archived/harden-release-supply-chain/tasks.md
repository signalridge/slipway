# Tasks

## Task List

- [x] `t-01` Apply and verify live GitHub repository protections.
  - depends_on: []
  - target_files: [artifacts/changes/harden-release-supply-chain/verification/main-branch-ruleset-request.json, artifacts/changes/harden-release-supply-chain/verification/release-tag-ruleset-request.json, artifacts/changes/harden-release-supply-chain/verification/release-environment-request.json]
  - task_kind: code
  - covers: [REQ-001, REQ-002]
  - acceptance:
    - `main` reports protected through GitHub API.
    - Active branch ruleset requires exact always-running check contexts.
    - Active tag ruleset restricts `refs/tags/v*`.
    - `release-publish` environment has required reviewers.

- [x] `t-02` Harden release workflow validation, permissions, and smoke.
  - depends_on: [t-01]
  - target_files: [.github/workflows/release.yaml, .github/workflows/ci.yml, cmd/release_workflow_contract_test.go]
  - task_kind: code
  - covers: [REQ-002, REQ-006]
  - acceptance:
    - Invalid release tags fail in `Validate Release Tag` before secret-bearing jobs.
    - Release write permissions and publishing secrets are scoped to the release job after validation.
    - Release job uses `release-publish` environment.
    - CI includes `Release Config` with `goreleaser check` and snapshot dry run.
    - Release smoke inputs are generated from actual `dist/` assets.

- [x] `t-03` Pin workflow actions and security tool versions.
  - depends_on: []
  - target_files: [.github/workflows/ci.yml, .github/workflows/docs.yml, .github/workflows/flake-lock-update.yaml, .github/workflows/nix.yaml, .github/workflows/pr-title.yaml, .github/workflows/release-please.yaml, .github/workflows/release.yaml, .github/workflows/security.yaml]
  - task_kind: code
  - covers: [REQ-003]
  - acceptance:
    - Workflow action refs are full commit SHA pins.
    - `DeterminateSystems/*@main` is absent.
    - `govulncheck@latest` and `go-licenses@latest` are absent.

- [x] `t-04` Implement GitHub API override token safety.
  - depends_on: []
  - target_files: [cmd/tool_github.go, cmd/tool_test.go]
  - task_kind: code
  - covers: [REQ-004]
  - acceptance:
    - HTTP, unknown-host, query/fragment, userinfo, and path-confused overrides fail closed.
    - Allowed override hosts require `SLIPWAY_GITHUB_API_TOKEN`.
    - Ambient `GH_TOKEN` and `GITHUB_TOKEN` are not sent to override hosts.
    - REST and GraphQL helpers share the same validated HTTP backend.

- [x] `t-05` Validate `BaseRef` before governed worktree creation.
  - depends_on: []
  - target_files: [internal/state/worktree.go, internal/state/worktree_test.go]
  - task_kind: code
  - covers: [REQ-005]
  - acceptance:
    - Option-like `base_ref` fails before `git worktree add`.
    - Unknown refs fail with product remediation and no raw git worktree error.
    - Valid tag refs still provision a default worktree.

- [x] `t-06` Add S2 release workflow contract verification.
  - depends_on: [t-01, t-02, t-03, t-04, t-05]
  - target_files: [cmd/release_workflow_contract_test.go]
  - task_kind: test
  - covers: [REQ-002, REQ-006]
  - acceptance:
    - Static Go tests parse `.github/workflows/release.yaml` and prove invalid tag validation happens before secret-bearing jobs.
    - Static Go tests prove `release` depends on `validate-tag`, uses `release-publish`, carries the only `GH_PAT`/`AUR_SSH_PRIVATE_KEY` secret references, and consumes the validated tag output.
    - Static Go tests prove release smoke inputs are generated from `dist/` outputs instead of a hand-written version matrix.
    - `actionlint`, `yamllint`, `goreleaser check`, focused Go tests, and local non-Docker/non-SBOM snapshot dry run pass before S3 review.
