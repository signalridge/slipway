# Requirements

## Requirements

### Requirement: Live Repository Protection

REQ-001: `main` and release tags MUST be protected by active GitHub rules
matching actual project workflows.

#### Scenario: Main requires current checks

GIVEN the repository default branch is `main`
WHEN GitHub branch protection or rulesets are queried
THEN `main` MUST report protected, and the active rule MUST require exact
always-running status check names from CI/security/title workflows.

#### Scenario: Release tags are restricted

GIVEN release tags match `v*`
WHEN GitHub rulesets are queried
THEN an active tag ruleset MUST restrict creation, update, deletion, and
non-fast-forward changes for `refs/tags/v*`.

### Requirement: Release Workflow Fails Closed Before Secret Exposure

REQ-002: The release workflow MUST validate manual release tag input before
release secrets are available.

#### Scenario: Invalid manual tag

GIVEN a `workflow_dispatch.inputs.tag` value that is not semver-like
WHEN the Release workflow runs
THEN it MUST fail in a no-secret validation job before any job can access
`GH_PAT`, `AUR_SSH_PRIVATE_KEY`, write permissions, package permissions,
attestation permissions, or release environment secrets; this ordering MUST be
covered by a static workflow contract test.

#### Scenario: Validated tag is the only release ref

GIVEN a valid release tag
WHEN test and release jobs checkout code, extract version, or run GoReleaser
THEN they MUST use the validated tag output, not raw workflow input or an
unvalidated expression.

### Requirement: Pinned Workflow and Tool Dependencies

REQ-003: Workflow action and tool dependencies MUST not float.

#### Scenario: Workflow actions

GIVEN any `.github/workflows/*` file uses an external action or reusable
workflow
WHEN the workflow is inspected
THEN the `uses:` ref MUST be a full commit SHA, with the original version tag
kept only as a comment when useful.

#### Scenario: Go security tools

GIVEN security workflows install `govulncheck` or `go-licenses`
WHEN the workflow is inspected
THEN the install target MUST use a fixed module version, not `@latest`.

### Requirement: GitHub API Override Token Safety

REQ-004: The token-backed GitHub REST/GraphQL backend MUST fail closed for
unsafe API base URL overrides.

#### Scenario: Unsafe override URL

GIVEN `SLIPWAY_GITHUB_API_URL` is `http://...`, an unknown HTTPS host, a
query/fragment URL, userinfo URL, or a path-confused public GitHub URL
WHEN the API backend is created
THEN it MUST fail closed before making a request.

#### Scenario: Allowed override host

GIVEN an exact HTTPS override base URL is listed in
`SLIPWAY_GITHUB_API_ALLOWED_BASE_URLS`
WHEN the API backend is created
THEN it MUST require `SLIPWAY_GITHUB_API_TOKEN` and MUST NOT send ambient
`GH_TOKEN` or `GITHUB_TOKEN` to that override host.

### Requirement: BaseRef Validation Before Worktree Creation

REQ-005: Governed `BaseRef` values MUST be validated before they reach
`git worktree add`.

#### Scenario: Invalid or option-like base ref

GIVEN `change.BaseRef` is empty, invalid, or starts with `-`
WHEN a default governed worktree is provisioned
THEN empty values MUST default to `HEAD`, option-like or invalid values MUST
fail with product-owned remediation, and the invalid value MUST NOT be passed
to `git worktree add`.

#### Scenario: Valid branch, tag, or SHA

GIVEN `change.BaseRef` resolves to a commit through Git
WHEN a default governed worktree is provisioned
THEN the worktree MUST still be created from that ref.

### Requirement: Release Config and Smoke Closure

REQ-006: Release configuration changes MUST be validated on PR and release
smoke MUST derive from actual generated assets.

#### Scenario: PR release config validation

GIVEN a PR changes release-related files
WHEN CI runs
THEN a `Release Config` check MUST run `goreleaser check` and a snapshot dry run
with fixed GoReleaser, syft, and Docker Buildx tooling.

#### Scenario: Tag release smoke

GIVEN a tag release generated actual archives and packages
WHEN smoke jobs run
THEN deb, rpm, apk, and representative binary archive smoke inputs MUST come
from a manifest generated from `dist/`, each smoke path MUST run
`slipway --version` and `slipway --help`, and this manifest wiring MUST be
covered by a static workflow contract test.
