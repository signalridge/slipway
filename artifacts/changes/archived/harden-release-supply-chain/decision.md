# Decision

## Decision

Implement `opt.md` section 2 as one release and supply-chain hardening change:

- apply live GitHub rulesets for `main` and `v*` release tags,
- create the `release-publish` protected environment,
- add release tag validation before any release secret exposure,
- minimize release workflow permissions and scope write permissions to release
  jobs,
- pin workflow actions and security tools,
- fail closed for unsafe GitHub API override hosts and isolate override tokens,
- validate `BaseRef` before `git worktree add`,
- add PR release-config validation and runtime-derived release smoke manifests.

## Alternatives Considered

### Option A: Minimal YAML-only release hardening

Patch only `.github/workflows/release.yaml` and `.github/workflows/security.yaml`.

Tradeoffs:

- Lower implementation size.
- Leaves `main` and release tags unprotected in live GitHub settings.
- Leaves GitHub API override token routing and `BaseRef` worktree validation
  outside the change despite being explicit `opt.md` section 2 requirements.

### Option B: Cohesive section 2 hardening

Apply live repository protections, harden workflows, pin actions/tools, add API
override token safety, validate `BaseRef`, and add release-config/smoke checks.

Tradeoffs:

- Larger change, but it closes the coupled release trust boundary.
- Requires live GitHub API mutations and workflow changes in the same PR.
- Gives review and CI one coherent security story.

Selected: Option B.

## Selected Approach

Option B is selected: implement `opt.md` section 2 as one cohesive hardening
change covering live GitHub protections, release workflow safety, SHA/tool
pinning, API override token isolation, `BaseRef` validation, and release
config/smoke verification.

### Option C: Defer live GitHub settings to manual documentation

Only document required branch/tag/environment settings and merge code changes.

Tradeoffs:

- Avoids external mutation from this agent.
- Fails the live evidence requirement in `opt.md` section 2.1.
- Leaves the repo in a known unsafe state until a manual follow-up happens.

## Interfaces and Data Flow

- GitHub live settings:
  - Repository ruleset `18174607` targets `~DEFAULT_BRANCH` and requires exact
    always-running CI/security/title check contexts.
  - Repository ruleset `18174614` targets `refs/tags/v*` and restricts tag
    creation, update, deletion, and non-fast-forward changes.
  - Environment `release-publish` gates release publishing with required
    reviewer `signalridge`.
- Release workflow:
  - `validate-tag` is the no-secret first job.
  - `test` and `release` consume `needs.validate-tag.outputs.tag_name`.
  - `release` is the only job with release write/package/attestation
    permissions and the only job that reads `GH_PAT` and `AUR_SSH_PRIVATE_KEY`.
  - `release` emits actual smoke inputs from `dist/` through job outputs:
    `binary_matrix`, `deb_asset`, `rpm_asset`, and `apk_asset`.
  - verify jobs consume those outputs and run `slipway --version` and
    `slipway --help`.
- GitHub API HTTP backend:
  - Default public API flow: `https://api.github.com` plus ambient
    `GH_TOKEN`/`GITHUB_TOKEN`.
  - Override flow: exact HTTPS base URL in
    `SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS` plus
    `SLIPWAY_GITHUB_API_TOKEN`.
  - REST and GraphQL calls share `newGitHubHTTPClient`.
- Worktree provisioning:
  - `EnsureDefaultWorktreeForChange` validates `change.BaseRef` before it adds
    the ref as the final argv element to `git worktree add`.

## Rollout and Rollback

Rollout:

- Land the code/workflow/artifact change through a PR.
- Wait for all PR checks, including the new `Release Config` job, to pass.
- Merge by squash PR merge, then fast-forward local `main`.

Rollback:

- Revert the PR commit if workflow/API/worktree behavior regresses.
- For live GitHub settings, delete or edit rulesets `18174607` and `18174614`
  and environment `release-publish` only if they are proven to block valid
  repository operation.
- Verification commands:
  - `gh api repos/signalridge/slipway/branches/main --jq .protected`
  - `gh api repos/signalridge/slipway/rulesets`
  - `actionlint .github/workflows/*.yml .github/workflows/*.yaml`
  - `uvx yamllint -c .yamllint.yaml .github/workflows/*.yml .github/workflows/*.yaml`
  - `go test ./cmd ./internal/state -count=1`

## Risk

- Branch rulesets can block PRs if required status check names drift. This
  change uses actual current check names and avoids path-filtered docs/Nix
  checks.
- Tag rulesets can block release tag creation. This user-owned repo uses an
  explicit owner-user bypass so `v*` tags are restricted but not locked out.
- SHA-pinned actions can become stale. Original version tags remain as comments
  to make intentional refreshes straightforward.
- GitHub Enterprise users must add an exact HTTPS allowlist and override token.
  This is intentional to avoid ambient public GitHub token leakage.
- Release-config snapshot dry runs need Docker Buildx and syft in CI. The CI
  job installs both before running GoReleaser.

## Rationale

These requirements are coupled through release trust and repository control.
Splitting them into very small changes would leave windows where branch/tag
protection, workflow secrets, floating dependencies, and release smoke evidence
contradict each other. A single hardening scope makes the final state auditable.

## Tradeoffs

- The branch ruleset intentionally requires only always-running checks. This
  avoids blocking docs-only or Nix-unrelated PRs on path-filtered jobs that do
  not run.
- The tag ruleset uses an explicit owner-user bypass because this repository is
  user-owned, not organization-owned. That restricts arbitrary tag writes while
  avoiding a total release lockout.
- GitHub Enterprise API overrides require an exact HTTPS base URL allowlist and
  `SLIPWAY_GITHUB_API_TOKEN`. This is stricter than ambient token reuse but
  prevents public GitHub tokens from being sent to untrusted hosts.
- Local release dry-run evidence skips Docker/SBOM because this workstation's
  Docker daemon is unavailable and the aqua syft shim fails inside GoReleaser's
  child process. CI installs syft and sets up Docker Buildx to cover those paths.

## Consequences

- Future workflow action updates must intentionally refresh SHA pins.
- Release jobs now stop at `Validate Release Tag` for invalid manual input
  before release secrets or write permissions are reachable.
- The GitHub API HTTP backend remains compatible with the default public GitHub
  API but becomes intentionally stricter for overrides.
- Invalid `base_ref` values produce a product remediation instead of leaking raw
  git failure output after a partial worktree-add attempt.
