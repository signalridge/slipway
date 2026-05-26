# Assurance

## Project Context
- Tech Stack: Go, GitHub Actions, Nix
- Conventions: repo-native Go tests/builds, deterministic generated skill templates, checked-in workflow permissions, governed Slipway evidence
- Test Command: `go test -timeout=20m ./... -count=1`
- Build Command: `go build ./...`
- Languages: Go, YAML, Shell, Nix

## Scope Summary
Delivered repo-local CI repair scope:
- CI lint scope now excludes worktree-local/governance artifacts and generated host skill templates where they were not intended as standalone lint targets.
- Generated shell scripts no longer use Bash 4-only `mapfile` or associative arrays.
- Windows directory fsync errors are treated as unsupported only for parent-directory sync after file-level write/rename work is complete.
- `internal/engine/context` uses a non-conflicting package name while preserving the import path and caller aliases.
- Nix `vendorHash` matches the current Go dependency graph.
- Security workflow grants `actions: read` and normalizes duplicate SARIF `properties.tags` before govulncheck upload.
- `.github/workflows/docs.yml` was removed so GitHub Pages deployment no longer runs.

## Verification Verdict
Pass for repo-local verification.

Commands run:
- `go test ./internal/toolgen -run TestScriptFixtureContracts -count=1` passed.
- `go test ./internal/fsutil ./internal/state ./internal/engine/context ./internal/engine/progression -count=1` passed.
- `go test ./cmd -run TestStatusJson -count=1` passed as a compile-focused package slice with no matching tests.
- Static grep for `mapfile|declare -A` under generated skill script directories found no matches.
- Line-length scan for edited workflow/lint YAML found no lines over 120 characters.
- `golangci-lint run --timeout=5m` passed with 0 issues.
- `actionlint` passed.
- `nix flake check` passed.
- `nix build .#slipway` passed.
- `go build ./...` passed.
- `go test -timeout=20m ./... -count=1` passed.

Local verification limits:
- `yamllint` was not installed locally.
- `markdownlint-cli2` was not installed locally.
- I did not install missing lint tools because dependency-install policy requires explicit preflight when the manager is ambiguous; the checked-in scope was inspected and other repo-native checks were run.

## Evidence Index
- `verification/intake-clarification.yaml`
- `verification/research-orchestration.yaml`
- `verification/plan-audit.yaml`
- `verification/wave-plan.yaml`
- `verification/wave-orchestration.yaml`
- `verification/execution-summary.yaml`
- `verification/spec-compliance-review.yaml`
- `verification/code-quality-review.yaml`
- `verification/coverage-analysis.yaml`
- `verification/goal-verification.yaml`
- `evidence/tasks/rv1/t-01.json` through `evidence/tasks/rv1/t-10.json`

## Requirement Coverage
- REQ-001: `.github/workflows/ci.yml`, `.yamllint.yaml`, `docs/command-contract-matrix.md`; verified by actionlint, YAML line-length scan, and full Go regression.
- REQ-002: `internal/fsutil/atomic.go`, `internal/state/lifecycle.go`, generated skill scripts, `internal/engine/context`; verified by focused Go tests, full Go regression, and golangci-lint.
- REQ-003: `flake.nix`; verified by `nix flake check` and `nix build .#slipway`.
- REQ-004: `.github/workflows/security.yaml`; verified by actionlint and workflow inspection.
- REQ-005: `.github/workflows/docs.yml` removed; verified by git diff/status.
- REQ-006: External API contract scope remained bounded to checked-in GitHub Actions/SARIF behavior; token/settings gaps remain explicit residual risks.

## Residual Risks and Exceptions
- Release Please PR creation/token/repository-setting failures remain out of scope by user instruction.
- GitHub Pages deployment remains removed until repository Pages settings are intentionally restored.
- Remote CI still needs to validate Ubuntu-hosted `yamllint` and `markdownlint-cli2` behavior because those tools were absent locally.

## Rollback Readiness
Rollback is a normal git revert of this change. Restoring Pages automation requires a separate explicit decision after Pages settings are enabled.

## Archive Decision
Ready to finalize. Execution summary, domain-aware review, independent review, coverage analysis, goal verification, and ship gate approval are recorded in the verification index. Remaining token/settings and Pages enablement items are explicit out-of-scope follow-up work, not blockers for this repair change.
