# Requirements

## Project Context
- Tech Stack: Go, GitHub Actions, shell
- Conventions: prefer repo-native Go commands, Slipway governed lifecycle, concrete workflow evidence, and Slipway-specific metadata over literal clinvoker copying.
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go, YAML, Shell

## Requirements

### Requirement: Clinvoker parity analysis and governed feedback
REQ-001: The change MUST preserve evidence of the clinvoker-to-Slipway comparison and MUST record unreasonable Slipway workflow friction discovered during the governed run.

#### Scenario: Reference comparison remains auditable
GIVEN the implementation adapts workflows and distribution files from `/Users/yixianlu/Projects/clinvoker`
WHEN a reviewer inspects the governed artifacts
THEN `research.md`, `decision.md`, and `workflow-feedback.md` identify the selected Approach C, key reference files, scope choices, and workflow feedback.

### Requirement: CI coverage for the Go CLI
REQ-002: The repository MUST have CI automation covering Go tests, vet, static analysis or equivalent linting, race tests, and build checks using the Go version from `go.mod`.

#### Scenario: Pull request CI verifies core quality gates
GIVEN a pull request or push triggers CI
WHEN `.github/workflows/ci.yml` runs
THEN it executes `go test -timeout=20m ./... -count=1`, `go vet ./...`, static analysis or equivalent linting, `go test -timeout=20m ./... -race -count=1`, and `go build ./...` without pinning a separate stale Go version.

### Requirement: Release automation and metadata
REQ-003: The repository MUST include release-please and tag/manual release automation for Slipway that builds cross-platform binaries, produces release metadata, verifies artifacts, and supports SBOM/signing/provenance/package outputs where practical.

#### Scenario: Release tag builds and verifies artifacts
GIVEN a version tag or manual release dispatch
WHEN the release workflow runs
THEN it tests the repository, invokes GoReleaser with Slipway-specific metadata, creates release artifacts/checksums, verifies the built binary version/help surface, and exposes guarded package/container/SBOM/signing/provenance paths.

### Requirement: CLI version self-verification
REQ-004: Slipway MUST expose version metadata suitable for release artifact smoke checks without changing lifecycle semantics.

#### Scenario: Built binary reports injected metadata
GIVEN Slipway is built with release ldflags
WHEN `slipway --version` or an equivalent version surface is executed
THEN the output includes version information and can be asserted by release verification tests or scripts.

### Requirement: Distribution support files
REQ-005: The repository MUST include Slipway-specific distribution support files for the selected Approach C surfaces: GoReleaser, Docker/container, Nix, docs, linting, dependency maintenance, and release helper configuration.

#### Scenario: Distribution configuration uses Slipway metadata
GIVEN a reviewer inspects distribution configuration
WHEN they compare it to clinvoker
THEN copied surfaces are adapted to `github.com/signalridge/slipway`, the `slipway` binary, repository files that actually exist, and guarded optional secrets for external publication.

### Requirement: Maintenance and security automation
REQ-006: The repository MUST include directly applicable maintenance/security automation adapted from clinvoker, including PR title checks, Dependabot grouping, vulnerability/security scanning, docs/Nix upkeep where supporting files exist, and release configuration validation.

#### Scenario: Maintenance workflows are meaningful for Slipway
GIVEN scheduled, pull request, or manual maintenance workflows run
WHEN they execute
THEN they validate Go dependencies/actions, security scan outputs, docs/Nix surfaces, and release configuration without requiring local secret provisioning.

### Requirement: End-to-end verification and lifecycle closeout
REQ-007: The final change MUST be verified with repository-native build/test commands and Slipway lifecycle validation before completion is claimed.

#### Scenario: Completion evidence proves the goal
GIVEN implementation is complete
WHEN local verification and Slipway governance commands are run
THEN `go test -timeout=20m ./... -count=1`, `go build ./...`, relevant focused checks, and final Slipway validation/closeout evidence are recorded in governed artifacts.
