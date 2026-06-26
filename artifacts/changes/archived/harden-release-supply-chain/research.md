# Research

## Alternatives Considered

### Architecture

- Affected workflow files:
  - `.github/workflows/release.yaml`: release tag validation, release job
    permissions, protected environment usage, GoReleaser version pinning,
    release smoke manifest generation, and post-release smoke jobs.
  - `.github/workflows/ci.yml`: PR release-config validation through
    `goreleaser check` and snapshot dry run.
  - `.github/workflows/security.yaml`: pinned Go security tool installs.
  - `.github/workflows/nix.yaml` and
    `.github/workflows/flake-lock-update.yaml`: remove
    `DeterminateSystems/*@main`.
  - Other workflow files: pin action refs by commit SHA and use safer
    `$GITHUB_OUTPUT` writes where touched.
- Affected Go files:
  - `cmd/tool_github.go`: token-backed REST/GraphQL backend and
    `SLIPWAY_GITHUB_API_URL` override handling.
  - `cmd/release_workflow_contract_test.go`: static release workflow contract
    tests for tag validation before secret exposure and smoke manifest wiring.
  - `cmd/tool_test.go`: public GitHub helper regression tests.
  - `internal/state/worktree.go`: default governed worktree provisioning and
    `BaseRef` before `git worktree add`.
  - `internal/state/worktree_test.go`: worktree BaseRef regression tests.
- Live repository settings:
  - Before repair, `gh api repos/signalridge/slipway/branches/main --jq
    .protected` returned `false`.
  - Before repair, `gh api repos/signalridge/slipway/rulesets` returned `[]`.
  - Repo permissions for the active account include `admin: true`.
  - Owner type is `User`, so organization-admin bypass is not applicable.

### Patterns

- Existing workflows already use named jobs whose names are visible as GitHub
  status check contexts. Required checks should use those exact names instead
  of inferred workflow filenames.
- Existing release workflow already validates installed artifacts through
  `--version` and `--help`; this change should keep that smoke shape but derive
  asset names from actual release outputs.
- Existing GitHub helper tests use local HTTP servers; production should not
  allow HTTP override, so tests need TLS server transport injection rather than
  a production HTTP escape hatch.
- Existing worktree provisioning constructs `git` args without shell
  interpolation. The remaining risk is git option/ref confusion, so validation
  should happen before `git worktree add` and return product-owned remediation.

### Risks

- High: applying an over-strict branch ruleset could block legitimate PRs.
  Mitigation: require only always-running CI/security/title checks, avoid Nix
  and docs path-filtered checks, allow squash PR merge, and do not add branch
  bypass actors.
- High: tag protection with no bypass could lock out release tag creation on a
  user-owned repository. Mitigation: protect `refs/tags/v*` while allowing the
  current owner user as an explicit bypass actor.
- High: GitHub Enterprise API base URLs may need paths such as `/api/v3`.
  Mitigation: allow exact HTTPS base URLs with canonical paths only through
  `SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS`; default public GitHub remains the
  only ambient-token host.
- Medium: GoReleaser snapshot dry runs require external tools and Docker.
  Mitigation: CI release-config job installs syft and sets up Docker Buildx;
  local evidence records Docker/SBOM limits separately.
- Medium: pinning every action to SHA increases update friction.
  Mitigation: preserve original version/tag comments beside each SHA so a
  dependency refresh has an obvious source version.

### Test Strategy

- Use live `gh api` evidence for branch/ruleset/tag protection and protected
  environment state.
- Use `actionlint` and `yamllint` for workflow syntax.
- Use `goreleaser check` and snapshot dry-run evidence for release config.
- Use focused Go tests for:
  - release workflow secret-exposure ordering and smoke manifest output wiring,
  - unsafe API override URL rejection,
  - override host token isolation from ambient `GH_TOKEN`/`GITHUB_TOKEN`,
  - REST and GraphQL shared HTTP backend behavior through TLS test servers,
  - option-like and invalid `BaseRef` values failing before worktree creation,
  - valid tag `BaseRef` still provisioning a worktree.
- Use full `go test ./...` and `golangci-lint run ./...` before review.

### Options

- Option A: Only patch the two explicit `opt.md` floating dependencies.
  - Rejected because `opt.md` also requires GitHub Actions to use full commit
    SHA pinning and release workflow hardening.
- Option B: Pin all workflow action refs by full commit SHA and add fixed tool
  versions in workflows.
  - Selected. This directly satisfies the supply-chain pinning requirement and
    keeps original version tags as comments for update hygiene.
- Option C: Move all release smoke into a custom Go verifier.
  - Rejected for this change. The existing release workflow already owns the
    release channel smoke tests; deriving its inputs from actual release outputs
    is lower risk.

## Unknowns

- Resolved: Is `main` currently protected? -> It was not protected before this
  change. After applying ruleset `18174607`, `branches/main` reports
  `protected: true`.
- Resolved: Are release tags protected? -> There was no ruleset before this
  change. Ruleset `18174614` now targets `refs/tags/v*` with creation, update,
  deletion, and non-fast-forward rules plus an explicit owner-user bypass.
- Resolved: Is a release protected environment present? -> Only
  `github-pages` existed before this change. Environment `release-publish` now
  exists with required reviewer `signalridge` and `prevent_self_review: false`.
- Resolved: Which release tools must be installed for snapshot dry run? ->
  GoReleaser v2.16.0, syft, and Docker Buildx are needed to cover the configured
  SBOM and container artifact graph.
- Resolved: Where are GitHub API override and `BaseRef` boundaries? ->
  `cmd/tool_github.go` constructs the shared REST/GraphQL HTTP backend, and
  `internal/state/worktree.go` adds `change.BaseRef` to `git worktree add`.
- Remaining: None.

## Assumptions

- Requiring always-running CI/security/title checks on `main` is safer than
  requiring path-filtered docs or Nix jobs, because missing path-filtered checks
  can block unrelated PRs.
- `SLIPWAY_GITHUB_API_TOKEN` is the explicit override-host token. Ambient
  `GH_TOKEN` and `GITHUB_TOKEN` remain valid only for `https://api.github.com`.
- The release workflow should keep running the existing channel-specific smoke
  jobs, but asset filenames must be generated from actual release outputs rather
  than hard-coded from the version string.

## Canonical References

- `opt.md` section 2.1-2.6
- `.github/workflows/release.yaml`
- `.github/workflows/ci.yml`
- `.github/workflows/security.yaml`
- `.github/workflows/nix.yaml`
- `.github/workflows/flake-lock-update.yaml`
- `cmd/release_workflow_contract_test.go`
- `cmd/tool_github.go`
- `cmd/tool_test.go`
- `internal/state/worktree.go`
- `internal/state/worktree_test.go`
- `artifacts/changes/harden-release-supply-chain/verification/main-branch-ruleset-request.json`
- `artifacts/changes/harden-release-supply-chain/verification/release-tag-ruleset-request.json`
- `artifacts/changes/harden-release-supply-chain/verification/release-environment-request.json`
