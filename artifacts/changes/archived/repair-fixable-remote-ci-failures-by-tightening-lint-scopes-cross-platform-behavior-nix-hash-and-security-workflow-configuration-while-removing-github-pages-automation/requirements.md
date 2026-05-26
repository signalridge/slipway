# Requirements

## Project Context
- Tech Stack: Go, GitHub Actions, Nix
- Conventions: repo-native Go tests/builds, checked-in GitHub Actions workflows, deterministic generated skill templates, governed Slipway artifacts
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, YAML, Shell, Nix

## Requirements

### Requirement: CI lint scope targets maintained source and config surfaces
REQ-001: YAML and Markdown lint jobs MUST avoid archived governance evidence, worktree-local bundles, and generated host skill templates that are not authored as standalone lint targets.

#### Scenario: YAML lint excludes governance/worktree artifacts
GIVEN the CI workflow runs `yamllint -c .yamllint.yaml .`
WHEN archived bundles or local worktree bundles are present in the checkout
THEN yamllint does not fail on those governance artifacts.

#### Scenario: Markdown lint keeps maintained docs in scope
GIVEN markdownlint runs in CI
WHEN maintained docs are changed
THEN maintained docs remain linted while generated host templates are excluded.

### Requirement: Cross-platform tests pass for repairable code/script defects
REQ-002: The Go test suite MUST avoid known Windows directory-sync failures and macOS Bash 3.2 incompatibilities without changing public CLI behavior.

#### Scenario: Windows directory sync is treated as unsupported
GIVEN atomic file writes and archive moves complete their file-level write, close, and rename steps
WHEN syncing the parent directory is unsupported on Windows
THEN the operation succeeds instead of failing the test suite with access-denied directory sync errors.

#### Scenario: Generated shell scripts run on macOS runner Bash
GIVEN rendered skill scripts are tested with the runner `bash`
WHEN the runner provides Bash 3.2
THEN the scripts avoid `mapfile` and associative arrays while preserving deterministic output.

### Requirement: Nix package metadata matches the current Go dependency graph
REQ-003: `flake.nix` MUST carry the vendor hash that matches the current `go.mod` and `go.sum`.

#### Scenario: Nix build evaluates current modules
GIVEN the Nix package build runs
WHEN Go dependencies are vendored
THEN the configured `vendorHash` matches the current module graph.

### Requirement: Security workflow uploads valid SARIF with checked-in permissions
REQ-004: Security workflow changes MUST be limited to checked-in workflow/config behavior and MUST NOT add token or secret dependencies.

#### Scenario: Trivy SARIF upload can read workflow run metadata
GIVEN the Trivy SARIF upload step runs
WHEN `github/codeql-action/upload-sarif` queries workflow run metadata
THEN the workflow permissions include the required read scope.

#### Scenario: govulncheck SARIF tags are upload-safe
GIVEN `govulncheck -format sarif ./...` writes `govulncheck.sarif`
WHEN the upload step consumes the file
THEN duplicate `properties.tags` entries are normalized before upload.

### Requirement: GitHub Pages automation is removed for now
REQ-005: Checked-in GitHub Pages deployment workflow automation MUST be removed while preserving documentation sources.

#### Scenario: Pages deployment no longer runs
GIVEN workflows are listed for the repository
WHEN docs or `mkdocs.yml` change
THEN no GitHub Pages deployment workflow is triggered from this change set.

### Requirement: external_api_contracts guardrail compliance
REQ-006: CI workflow and SARIF upload contract changes MUST remain bounded, reviewable, and reversible.

#### Scenario: external integration behavior is explicit
GIVEN the change alters GitHub Actions workflow behavior
WHEN implementation and review complete
THEN affected external integration surfaces, residual settings/token risks, and rollback steps are documented.
