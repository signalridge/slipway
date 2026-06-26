# Architecture

- Question: Which release and supply-chain seams must change so `opt.md`
  section 2 is actually closed rather than documented?
- GitHub repository protection is external state, but this change stores the
  applied request bodies under
  `artifacts/changes/harden-release-supply-chain/verification/`. The live
  authority is GitHub API rulesets `18174607` for `main` and `18174614` for
  `refs/tags/v*`, plus environment `release-publish`.
- `.github/workflows/release.yaml` owns tag-release and manual-release control
  flow. The hardened flow is `validate-tag` -> `test` -> `release` -> smoke
  jobs. `validate-tag` is intentionally no-secret/read-only; `release` is the
  only job that carries write/package/attestation permissions and publishing
  secrets.
- `.github/workflows/ci.yml` owns PR verification. The new `Release Config`
  job validates the GoReleaser config and runs a snapshot dry run before the
  normal build job can pass.
- `.github/workflows/security.yaml`, `.github/workflows/nix.yaml`, and
  `.github/workflows/flake-lock-update.yaml` are supply-chain entry points for
  scanner/tool installs and Nix setup. Floating action refs and `go install
  ...@latest` are not acceptable there.
- `cmd/tool_github.go` owns token-backed REST/GraphQL helper construction for
  GitHub tools. `newGitHubHTTPClient` is the shared choke point for
  `SLIPWAY_GITHUB_API_URL`, token selection, and HTTP client setup.
- `cmd/release_workflow_contract_test.go` is the static workflow policy test
  for REQ-002 and REQ-006. It parses `.github/workflows/release.yaml` and
  asserts the secret-exposure ordering and smoke-manifest wiring.
- `cmd/tool_test.go` covers GitHub API override and token isolation behavior
  through TLS test servers and an injected HTTP transport.
- `internal/state/worktree.go` owns governed default worktree provisioning.
  `EnsureDefaultWorktreeForChange` now validates `change.BaseRef` before the
  value is passed to `git worktree add`.
