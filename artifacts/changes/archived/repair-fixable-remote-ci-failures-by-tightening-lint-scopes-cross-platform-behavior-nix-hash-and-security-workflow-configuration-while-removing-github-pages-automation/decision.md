# Decision

## Project Context
- Tech Stack: Go, GitHub Actions, Nix
- Conventions: repo-native Go tests/builds, deterministic generated skill templates, checked-in workflow permissions, governed Slipway evidence
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, YAML, Shell, Nix

## Alternatives Considered

### Minimal source/config repair
- Design: patch only checked-in workflow/config files, Bash 3-compatible generated scripts, Go portability/lint issues, and Nix metadata; delete only the GitHub Pages deployment workflow.
- Tradeoffs: preserves CI signal and matches the requested scope; leaves Release Please and repository-setting/token failures as residual non-code issues.

### CI topology reset
- Design: disable or broadly rewrite failing workflows to reduce immediate red checks.
- Tradeoffs: lower implementation effort, but hides useful CI coverage and does not repair root causes.

### External setting/token repair
- Design: enable GitHub Pages, add or rotate release tokens, or change repository Actions settings.
- Tradeoffs: may address external red runs, but it is outside the user-requested scope and cannot be audited as a repo-local code change.

## Selected Approach
Selected: Minimal source/config repair.

Rationale:
- It repairs all identified failures that are attributable to checked-in code/config.
- It preserves the existing CI coverage intent instead of suppressing checks.
- It honors the explicit exclusions: GitHub Pages deployment is removed for now, and missing token/secret/repository-setting gaps are not fixed in this change.

## Interfaces and Data Flow
- `.github/workflows/ci.yml` keeps the existing lint/test/build flow but narrows lint inputs to maintained surfaces.
- `.github/workflows/security.yaml` keeps govulncheck, Trivy, SBOM, and license jobs, adds required checked-in permissions, and normalizes govulncheck SARIF before upload.
- `.github/workflows/docs.yml` is removed so GitHub Pages deployment no longer runs.
- `flake.nix` updates only the Go package `vendorHash`.
- `internal/fsutil/atomic.go` and `internal/state/lifecycle.go` retain file-level durability steps and treat unsupported Windows directory sync as non-fatal.
- Generated skill scripts preserve existing command-line interfaces while replacing Bash 4-only constructs.
- `internal/engine/context` keeps its import path but changes package naming to avoid a standard-library package name conflict.

## Rollout and Rollback
- Rollout: land the checked-in code/config changes after local targeted tests, full Go tests/build, lint checks, Nix verification where available, and Slipway validation.
- Rollback: revert this branch's commit(s). Restoring Pages automation later is a separate explicit decision after Pages settings are enabled.
- Verification after rollback: `go test -timeout=20m ./... -count=1`, `go build ./...`, and workflow/lint checks for any reverted surfaces.

## Risk
- Directory sync portability: low to medium. Mitigated by limiting the behavior change to Windows directory sync errors after file-level writes/renames have completed.
- SARIF normalization: low to medium. Mitigated by only de-duplicating `properties.tags` arrays rather than rewriting scanner findings.
- Lint scope: low. Mitigated by keeping maintained docs and workflow/config files in scope and excluding generated/archive surfaces only.
- Residual CI: Release Please or other token/settings failures may remain red. This is accepted by scope and must be called out in closeout.
