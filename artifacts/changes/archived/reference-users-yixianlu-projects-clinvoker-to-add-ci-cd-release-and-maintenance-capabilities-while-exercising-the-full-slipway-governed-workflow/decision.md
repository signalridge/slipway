# Decision

## Alternatives Considered

### Approach A: Workflow-only minimal release
- Add release-please, a tag/manual release workflow, Dependabot, PR title checks, and security scanning.
- Verify release binaries with `slipway --help` only and avoid CLI version metadata.
- Lowest implementation risk, but it does not satisfy the user's selected full clinvoker-style distribution breadth.

### Approach B: Release workflows plus root `--version` metadata
- Add release-please, GoReleaser GitHub Release archives, Dependabot, PR title checks, security scanning, and minimal root version metadata injected by ldflags.
- Keeps Slipway command registry stable while enabling artifact smoke checks.
- This was the recommended smaller implementation, but the user selected the broader C option.

### Approach C: Full clinvoker-style distribution
- Add or adapt release-please, GoReleaser, package manager outputs, container publishing, Nix checks, docs deployment, SBOM/signing/provenance, release verification, PR title checks, Dependabot, and security maintenance workflows.
- Add supporting repository files and version metadata so produced artifacts can be verified.
- Highest blast radius, but it is the selected scope and best matches the reference project.

## Selected Approach

Approach C is selected. Implement full clinvoker-style CI/CD, release, and maintenance capabilities for Slipway, adapted to this repository's Go module, Cobra entrypoint, binary name, package metadata, and governance workflow.

The implementation should preserve Slipway's lifecycle behavior and keep distribution side effects behind GitHub tag/manual events, permissions, and optional secrets. Local verification must use dry-run/check modes and repository-native build/test commands, not real external publishing.

## Interfaces and Data Flow

- Developer changes land on pull requests and run CI, lint, tests, race tests, build checks, PR title validation, security scanning, and package/config validation.
- Release-please reads conventional commits on `main`, updates `CHANGELOG.md` and `.release-please-manifest.json`, and opens release PRs using `release-please-config.json`.
- Tag/manual release runs Go tests, builds Slipway binaries through GoReleaser, injects version/commit/date metadata, uploads GitHub Release artifacts, and verifies downloaded artifacts with the Slipway version/help surfaces.
- GoReleaser owns cross-platform archives, checksums, SBOM/signing hooks, package definitions, container image definitions, and package-manager manifests where applicable.
- Maintenance workflows keep Go module and GitHub Actions dependencies current, scan source/container artifacts, and validate docs/Nix surfaces added by this change.

## Rollout and Rollback

Rollout:
- Add configuration and workflows in small, reviewable groups: CI/maintenance, version metadata, release config, package/container/Nix/docs surfaces, then verification evidence.
- Keep publish steps scoped to tag or manual release events and guarded by required secrets where external systems are involved.
- Document required repository secrets/settings in the implementation artifacts or README so maintainers can enable targets deliberately.

Rollback:
- Disable workflows by reverting `.github/workflows/*` additions or removing specific jobs.
- Remove GoReleaser/package/container/Nix/docs configs if a distribution target proves unsupported.
- Revert version metadata changes independently if CLI behavior regresses.
- Existing lifecycle state, command execution, and local Go tests should remain unaffected by workflow-only rollback.

## Risk

- High: Approach C touches external publishing and supply-chain automation. Misconfigured triggers or secrets can publish unexpected artifacts.
- High: copied clinvoker config can retain wrong names, paths, package IDs, or repository coordinates if not adapted carefully.
- Medium: adding a public version command can drift from Slipway's command registry and generated command surfaces if not updated consistently.
- Medium: GitHub-only checks such as SARIF upload, provenance, and package publishing cannot be fully proven locally.
- Mitigation: use explicit permissions, event guards, optional secret conditions, local dry-run/check commands, focused version tests, and final governance feedback for workflow limitations discovered during the run.
