# Assurance

## Project Context
- Tech Stack: Go, GitHub Actions, shell
- Conventions: keep release and maintenance automation scoped to Slipway; avoid stale `clinvoker` identifiers in executable configuration; guard secret-dependent publishing paths.
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go, YAML, Shell

## Scope Summary

Implemented the selected full distribution approach:

- Added root-command release metadata (`version`, `commit`, `date`) with a focused `--version` regression test.
- Added CI, security, docs, Nix, lock-update, PR-title, Dependabot, Release Please, and release workflows.
- Added GoReleaser v2.16-compatible distribution config for archives, checksums, nFPM packages, GHCR images, Homebrew Casks, Scoop, AUR, SBOMs, and cosign signing.
- Added local tooling/config files: `justfile`, `.dockerignore`, `.golangci.yaml`, `.yamllint.yaml`, `.markdownlint.yaml`, `Dockerfile`, `Dockerfile.goreleaser`, `flake.nix`, `flake.lock`, `mkdocs.yml`, Release Please config, and `CHANGELOG.md`.
- Hardened container/security surfaces by pinning Docker base images by digest, bounding Docker build context, pinning Trivy Action by commit SHA, and avoiding Docker-socket Trivy scanning.
- Updated README release/maintenance docs with local commands and required external repository settings/secrets.
- Fixed the governed worktree-preflight progression deadlock found while exercising the workflow.
- Recorded workflow feedback for unclear intake/research routing, placeholder codebase maps, research/task schema friction, worktree-preflight deadlock, missing skills after worktree binding, read-lock contention, and full-suite timeout risk.

## Verification Verdict

Verdict: pass with documented external-environment exceptions.

Fresh local verification on 2026-05-25 UTC:

- `go test -timeout=20m ./... -count=1`: pass. Slowest package: `github.com/signalridge/slipway/cmd` at 720.749s.
- `go build ./...`: pass.
- `go vet ./...`: pass.
- `golangci-lint run --timeout 5m`: pass, `0 issues`.
- `actionlint .github/workflows/*.yml .github/workflows/*.yaml`: pass.
- `goreleaser check`: pass, `1 configuration file(s) validated`.
- `goreleaser release --snapshot --clean --skip=docker,homebrew,scoop,aur,sign,sbom`: pass; generated Slipway archives and deb/rpm/apk packages locally without publishing.
- `nix build .#slipway --no-link`: pass.
- `nix flake check --no-build`: pass, with expected dirty-tree, app meta, and incompatible-system warnings from local evaluation.
- `jq empty release-please-config.json .release-please-manifest.json`: pass.
- `go test -timeout=20m ./cmd ./internal/engine/progression -run 'TestRootVersionFlagPrintsReleaseMetadata|TestAdvanceGoverned_AppliesWorktreePreflightBeforeRequiredActionBlockers' -coverprofile=coverage.txt -count=1`: pass; change-surface tests exercised the new version surface and worktree-preflight ordering regression.
- Stub/placeholder scan over changed target files: pass for TODO/FIXME/HACK/placeholder markers. The secondary zero-value scan returned only benign pre-existing helpers plus the new `applyPendingWorktreePreflight` no-op/pass `return nil, nil` paths, not production stubs.
- `go run . --version`: pass; printed `slipway dev`, `commit: unknown`, `built: unknown`.
- `just --list`: pass.
- `rg -n "clinvoker|clinvk" ...`: only intentional README reference to the comparison source remained; executable configs use Slipway names.

The initial `go test ./... -count=1` run timed out at the Go default 10-minute package limit while `cmd` was executing under concurrent verification load. No assertion failure was surfaced. CI and local docs now use `-timeout=20m`, and the fresh full-suite run passed with that explicit timeout.

## Evidence Index

- Focused version test: `go test ./cmd -run TestRootVersionFlagPrintsReleaseMetadata -count=1`
- Worktree-preflight regression: `go test ./internal/engine/progression -run TestAdvanceGoverned_AppliesWorktreePreflightBeforeRequiredActionBlockers -count=1`
- Full suite: `go test -timeout=20m ./... -count=1`
- Build/static checks: `go build ./...`, `go vet ./...`, `golangci-lint run --timeout 5m`
- Workflow/release checks: `actionlint .github/workflows/*.yml .github/workflows/*.yaml`, `goreleaser check`
- Snapshot packaging: `goreleaser release --snapshot --clean --skip=docker,homebrew,scoop,aur,sign,sbom`
- Nix checks: `nix build .#slipway --no-link`, `nix flake check --no-build`
- Metadata/config checks: `go run . --version`, `jq empty release-please-config.json .release-please-manifest.json`, `just --list`
- Change-surface coverage/stub checks: focused `go test ... -coverprofile=coverage.txt`, marker scans over changed target files
- Governed feedback: `workflow-feedback.md`

## Requirement Coverage

| Requirement | Coverage |
| --- | --- |
| REQ-001 | README and `workflow-feedback.md` record Clinvoker parity decisions and workflow friction discovered during execution. |
| REQ-002 | CI workflow covers multi-OS tests, race tests, vet, Go lint, YAML lint, markdown lint, actionlint, module hygiene, release-smoke build, and cross-compilation. Local `go test`, `go build`, `go vet`, `golangci-lint`, and `actionlint` passed. |
| REQ-003 | Release Please config, changelog, tag/manual release workflow, and GoReleaser config target `signalridge/slipway`; `goreleaser check` and snapshot release passed. Container image publication uses version tags rather than a mutable `latest` tag. |
| REQ-004 | `--version` metadata support is implemented and verified by focused test plus `go run . --version`. |
| REQ-005 | Docker, Nix, GoReleaser archives/packages/images, Homebrew Cask, Scoop, AUR, docs, and README setup were added; Docker base images are digest-pinned, build context is bounded, and Nix build/check plus snapshot packaging passed. |
| REQ-006 | Dependabot, flake-lock update, PR-title check, security workflow, SBOM generation, govulncheck/Trivy setup, and guarded optional secrets are present; Trivy Action is pinned by commit SHA and workflow lint passed. |
| REQ-007 | Full local verification and this assurance record document completion evidence, known exceptions, and rollback/readiness. |

## Residual Risks and Exceptions

- External publishing paths were not exercised against live GitHub Releases, GHCR, Homebrew tap, Scoop bucket, or AUR. They are guarded by tag/manual release triggers and documented secrets/settings.
- `goreleaser healthcheck` is not a local pass because this machine lacks a usable cosign binary through aqua and Docker buildx reports an invalid local driver. The release workflow installs cosign and configures Docker Buildx before publishing.
- Local snapshot packaging skipped Docker, Homebrew, Scoop, AUR, signing, and SBOM. Docker/signing/SBOM are verified structurally by `goreleaser check`, Dockerfile review, and release workflow setup; real publication requires repository credentials and tag context.
- The repository has no `LICENSE` file. Package metadata is marked `UNLICENSED`, and archives intentionally include `README.md` only.
- Full-suite runtime is high. The documented and CI commands now use an explicit 20-minute timeout, but the slow `cmd` package remains a future maintenance target.
- README intentionally mentions `clinvoker` as the comparison source; executable configuration was checked to avoid stale `clinvoker`/`clinvk` names.

## Rollback Readiness

Rollback is file-scoped and does not require data migration:

- Remove `.github/workflows/{ci.yml,security.yaml,pr-title.yaml,docs.yml,nix.yaml,flake-lock-update.yaml,release.yaml,release-please.yaml}` and `.github/dependabot.yml` to disable automation.
- Remove `.goreleaser.yaml`, `.dockerignore`, Dockerfiles, Nix files, lint configs, `justfile`, MkDocs config, Release Please files, and `CHANGELOG.md` to roll back distribution scaffolding.
- Revert `cmd/root.go` and remove `cmd/root_version_test.go` to roll back version metadata.
- Revert the worktree-preflight progression fix only if the governed workflow no longer needs worktree binding; the regression test documents the bug it prevents.

No irreversible external side effects were performed locally. No release, container, package-manager, or docs deployment publication was executed from this worktree.

## Archive Decision

Archive decision: ready after Slipway consumes goal-verification and final-closeout evidence and reports `done_ready`. Implementation, review, and final verification evidence are complete for the current run version.
