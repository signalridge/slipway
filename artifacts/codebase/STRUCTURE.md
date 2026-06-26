# Structure

- `.github/workflows/release.yaml`
  - `validate-tag`: no-secret, read-only tag validation job.
  - `test`: checks out `needs.validate-tag.outputs.tag_name` and runs release
    metadata smoke before any release secret is available.
  - `release`: protected by `release-publish`, owns write/package/attestation
    permissions, optional publishing secrets, GoReleaser, and generated smoke
    outputs.
  - `verify-*`: post-release smoke jobs consume release job outputs and run
    `slipway --version` plus `slipway --help`.
- `.github/workflows/ci.yml`
  - `Release Config` installs fixed GoReleaser, syft, and Docker Buildx before
    `goreleaser check` and snapshot dry run.
  - `Build` depends on `Release Config`, so release-config breakage blocks PR
    success before merge.
- `.github/workflows/security.yaml`
  - `govulncheck` and `go-licenses` install pinned module versions.
  - security upload actions are full SHA-pinned like other workflows.
- `.github/workflows/nix.yaml` and
  `.github/workflows/flake-lock-update.yaml`
  - DeterminateSystems actions are pinned to full commit SHAs rather than
    `@main`.
- `cmd/tool_github.go`
  - Constants define default public GitHub API URL, allowlist env, override
    token env, and ambient token env names.
  - `resolveGitHubAPIConfigFromEnv` normalizes/validates base URL and chooses
    the correct token class.
  - `githubHTTPClient` serves both REST and GraphQL helper paths.
- `cmd/release_workflow_contract_test.go`
  - Reads the workflow from repo root.
  - Asserts validation-before-secret behavior and generated smoke outputs.
- `internal/state/worktree.go`
  - `validateWorktreeBaseRef` is local to the worktree provisioning boundary.
  - The helper defaults empty refs to `HEAD`, rejects option-like or malformed
    refs, and verifies commit-ish resolution through `git rev-parse`.
