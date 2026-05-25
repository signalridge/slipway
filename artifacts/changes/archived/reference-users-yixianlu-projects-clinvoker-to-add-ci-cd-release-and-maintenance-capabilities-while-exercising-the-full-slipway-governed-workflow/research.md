# Research

## Research Findings

### Architecture
- Affected modules:
  - `.github/workflows/ci.yml`: existing single CI workflow already checks out the repo, uses `actions/setup-go` with `go-version-file: go.mod`, installs staticcheck, and runs unit tests, vet, staticcheck, and race tests (`.github/workflows/ci.yml:1-38`).
  - `go.mod`: Slipway is a Go module at `github.com/signalridge/slipway` with Go `1.25.5`, so workflow Go setup should read the module version instead of pinning a separate workflow variable (`go.mod:1-12`).
  - `cmd/root.go`: the CLI entrypoint is the Cobra root command, currently registering lifecycle and diagnostic subcommands but no version command or version flag metadata (`cmd/root.go:129-156`).
  - `internal/toolgen/toolgen.go`: public command metadata is centralized in `commandRegistry`; adding a new surfaced command would require updating generated command surfaces and contract tests (`internal/toolgen/toolgen.go:153-205`).
  - `README.md`: current verification docs already list `go test ./... -count=1`, `go vet ./...`, `staticcheck ./...`, and race tests (`README.md:144-153`).
- Dependency chains:
  - Release artifact verification needs a built binary entrypoint. `main.go` calls `cmd.Execute`, which delegates to the root Cobra command; version metadata therefore belongs in `cmd` or the root command setup rather than workflow-only scripts.
  - GitHub Actions workflows depend on `go.mod` for Go version selection and on stable CLI behavior for smoke checks.
  - Release-please depends on `release-please-config.json`, `.release-please-manifest.json`, and `CHANGELOG.md`; these files do not exist in Slipway yet.
- Blast radius:
  - Medium to high. Workflow/config files affect CI, release, security scanning, package/container publishing, docs publication, Nix builds, and maintenance automation; version support affects CLI output surface but should not touch lifecycle state, persistence, or governance transitions.
- Constraints:
  - External package/container publishing must be guarded by GitHub events, permissions, and secrets so local verification never performs real publishing.
  - Avoid adding a new public `version` subcommand unless command registry and generated surfaces are intentionally expanded.
  - Keep Go version source of truth in `go.mod`.

### Patterns
- Existing conventions:
  - Current CI is direct and deterministic: one `verify` job with setup, test, vet, staticcheck, and race tests (`.github/workflows/ci.yml:9-38`).
  - README build instructions use `go run . --help` and `go build ./...`, matching local Go workflows (`README.md:76-81`).
  - Root command registration is explicit through `cmd.AddCommand(...)`; visible command descriptions are contract-tested against toolgen (`cmd/root.go:138-156`, `cmd/command_description_contract_test.go:13-48`).
- Clinvoker patterns worth adapting:
  - Release-please workflow on `main` and `workflow_dispatch`, with config and manifest files (`clinvoker/.github/workflows/release-please.yaml:1-20`, `clinvoker/release-please-config.json:1-82`, `clinvoker/.release-please-manifest.json:1-3`).
  - Tag/manual release workflow that tests first, builds artifacts with GoReleaser, and verifies release outputs (`clinvoker/.github/workflows/release.yaml:1-37`, `clinvoker/.github/workflows/release.yaml:90-118`, `clinvoker/.github/workflows/release.yaml:224-247`).
  - Security workflow for govulncheck, Trivy filesystem scan, SBOM, and license report (`clinvoker/.github/workflows/security.yaml:30-129`).
  - PR title convention enforcement with semantic-pull-request (`clinvoker/.github/workflows/pr-title.yaml:1-42`).
  - Dependabot grouping for Go modules and GitHub Actions (`clinvoker/.github/dependabot.yml:1-41`).
- Convention deviations:
  - Clinvoker uses fixed `GO_VERSION: "1.24"` in workflows; Slipway should keep `go-version-file: go.mod`.
  - Clinvoker release publishes many ecosystem artifacts and containers via GoReleaser; Slipway should preserve the breadth selected by the user while adapting names, package metadata, and secrets to this repository.
  - Clinvoker verifies `clinvk version`; Slipway lacks this surface, so release verification needs either root `--version` metadata or a new command.

### Risks
- Technical risks:
  - High: external distribution workflows can fail or publish unintentionally if event conditions, permissions, or secret checks are wrong. Use explicit tag/manual triggers, minimal permissions, and guarded steps.
  - High: signing, SBOM, provenance, containers, and package manager outputs introduce supply-chain surface. Prefer established actions and GoReleaser features and keep verification jobs explicit.
  - Medium: adding a new surfaced `version` command could require command registry, help grouping, generated prompts, and tests; using root `--version` is lower blast radius.
  - Medium: release workflows using GoReleaser can fail if config references non-existent files like `LICENSE`; verify repo file presence before copying archive file lists.
  - Medium: third-party GitHub Actions pinned by version tags are normal but supply-chain sensitive. Keep permissions minimal and avoid `pull_request_target`.
  - Low: release-please may create noisy PRs if changelog sections or version manifest are misconfigured.
  - Low: govulncheck/SARIF upload may require code scanning availability; configure it as a maintenance signal without hiding actual command failures where possible.
- Guardrail domains:
  - The implementation does not modify Slipway application credential handling, but Approach C references GitHub secrets and signing/publishing credentials operationally. Treat this as security-credential-sensitive during review even though the initial change classification did not set a guardrail domain.
- Reversibility:
  - Workflow/config additions and a minimal root version metadata addition are reversible by removing the files/variables.
  - Release tags, package pushes, container tags, and package-manager submissions are externally visible after merge. Tag/manual workflows must make publish boundaries explicit and avoid local publishing during verification.

### Test Strategy
- Existing coverage:
  - CI command coverage is already documented and enforced by the current workflow (`.github/workflows/ci.yml:28-38`, `README.md:144-153`).
  - Root help and command description surfaces have tests that would catch accidental drift if new commands are added to visible groups (`cmd/root_help_test.go:11-28`, `cmd/command_description_contract_test.go:13-48`).
- Coverage gaps:
  - No current tests validate release metadata or `--version` output.
  - No local test validates GitHub workflow syntax unless actionlint is available.
  - `artifacts/codebase/*` generated by `codebase-map` contains mostly headings and placeholders, so research still required direct source inspection.
- Infrastructure needs:
  - Unit test for root version output if root `--version` is added.
  - Local `goreleaser check` if GoReleaser is available; otherwise validate YAML structure and run Go build/test.
  - Optional actionlint check if installed or available through `gh`/local tooling.
- Verification approach:
  - Run `go test ./... -count=1`.
  - Run `go build ./...`.
  - Run focused tests for CLI version/root behavior if changed.
  - Run `go run . --version` or built binary version smoke if version metadata is added.
  - Run `goreleaser check` when available, otherwise document inability and validate config consistency by inspection.

### Unknowns Resolved
- Which implementation approach is selected? The user selected Approach C, so full clinvoker-style CI/CD, release, package/container, Nix, docs, and maintenance breadth is in scope.
- Should Slipway add version support? Yes, release artifact verification needs a version surface. Prefer root `--version` metadata over a new surfaced `version` subcommand to avoid expanding the governance command registry.
- Which maintenance checks are valuable? PR title enforcement, Dependabot for Go modules/actions, Go vulnerability/security scanning, release validation, docs deployment checks, and Nix flake maintenance are directly applicable once supporting files are added.

### Remaining Questions
- No research blocker remains. Repository owners will still need to configure secrets/settings before every external publishing target can succeed after merge.

## Alternatives Considered

### Approach A: Workflow-only minimal release
- Design: add release-please, tag release workflow, Dependabot, PR title, and security workflow. Verify release binaries with `slipway --help` only. Do not add CLI version metadata.
- Tradeoffs:
  - Smallest code diff.
  - Release artifacts lack reliable version/commit/date self-verification.
  - Less aligned with clinvoker's release verification pattern.

### Approach B: Release workflows plus root `--version` metadata
- Design: add release-please, GoReleaser-based GitHub Release archives, Dependabot, PR title, security workflow, and minimal root `--version` metadata injected by ldflags. Verify binaries with `slipway --version` and `slipway --help`.
- Tradeoffs:
  - Slight code change in root command setup and focused tests.
  - Keeps lifecycle command registry stable by avoiding a new surfaced `version` subcommand.
  - Provides a real release artifact smoke check.
- Recommendation: choose this approach.

### Approach C: Full clinvoker-style distribution
- Design: copy/adapt release-please, GoReleaser, package managers, container publishing, Nix checks, docs deployment, SBOM/signing/provenance, and package verification jobs.
- Tradeoffs:
  - Most complete parity with clinvoker.
  - Highest blast radius and requires careful adaptation of names, paths, secrets, and repository settings.
  - Cannot be fully proven locally because external publishing and GitHub repository settings remain outside the workspace.

## Selected Approach

Selected Approach C after explicit user selection (`c`): implement full clinvoker-style distribution breadth for Slipway, including CI hardening, release-please, GoReleaser, package/container outputs, docs/Nix maintenance, security scanning, SBOM/signing/provenance, release verification, and supporting version metadata.

## Unknowns

- External package publishing ownership, credentials, registry policy, and GitHub repository settings remain unknown. The implementation must make these prerequisites explicit and guarded rather than silently assuming they exist.
- GoReleaser availability in local developer environments is unknown; release config should be validated with `goreleaser check` when available and otherwise by repository inspection plus CI.
- GitHub repository settings for CodeQL/SARIF upload are unknown; security workflow should be useful without becoming the only local verification gate.

## Assumptions

- User selection `c` is approval to add package/container/Nix/docs distribution configuration, but not approval to create secrets, accounts, or run real publishing locally.
- `go.mod` is the authoritative Go version source for Actions setup.
- Root `--version` metadata is sufficient for release artifact smoke checks and lower risk than adding a new surfaced command.
- `GITHUB_TOKEN` should be enough for in-repository release creation; package managers, signing, AUR, Homebrew/Scoop taps, and container registry targets may require optional secrets documented in workflow/configuration comments.

## Canonical References

- Slipway current CI: `.github/workflows/ci.yml:1-38`
- Slipway Go module version: `go.mod:1-12`
- Slipway root command registration: `cmd/root.go:129-156`
- Slipway command metadata registry: `internal/toolgen/toolgen.go:153-205`
- Slipway verification docs: `README.md:144-153`
- Clinvoker CI reference: `/Users/yixianlu/Projects/clinvoker/.github/workflows/ci.yaml:1-301`
- Clinvoker release workflow reference: `/Users/yixianlu/Projects/clinvoker/.github/workflows/release.yaml:1-247`
- Clinvoker release-please reference: `/Users/yixianlu/Projects/clinvoker/.github/workflows/release-please.yaml:1-20`
- Clinvoker release config reference: `/Users/yixianlu/Projects/clinvoker/release-please-config.json:1-82`
- Clinvoker security workflow reference: `/Users/yixianlu/Projects/clinvoker/.github/workflows/security.yaml:1-129`
- Clinvoker Dependabot reference: `/Users/yixianlu/Projects/clinvoker/.github/dependabot.yml:1-41`
- User selection: 2026-05-25 continuation message `c`, interpreted as Approach C from the presented alternatives.
