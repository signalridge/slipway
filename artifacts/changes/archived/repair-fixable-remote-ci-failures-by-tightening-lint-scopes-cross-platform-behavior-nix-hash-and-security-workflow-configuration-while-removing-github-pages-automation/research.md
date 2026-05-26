# Research

## Research Findings

Question: Which remote CI failures are repairable in checked-in code/config, and which are intentionally out of scope because they require GitHub Pages, token, secret, or repository-setting changes?

### Architecture
- Affected modules:
  - `.github/workflows/ci.yml:18` runs `yamllint -c .yamllint.yaml .`, and `.github/workflows/ci.yml:36` runs markdownlint over `**/*.md`.
  - `.github/workflows/security.yaml:25` installs `govulncheck@latest`; `.github/workflows/security.yaml:28` writes `govulncheck.sarif`; `.github/workflows/security.yaml:54` uploads Trivy SARIF.
  - `.github/workflows/docs.yml:18` grants Pages permissions and `.github/workflows/docs.yml:66` deploys with `actions/deploy-pages@v5`.
  - `flake.nix:29` pins the Go vendor hash used by the Nix build.
  - `internal/fsutil/atomic.go:65` fsyncs parent directories after atomic writes, and `internal/state/lifecycle.go:277` has the archive directory sync helper.
  - `internal/tmpl/templates/skills/sast-orchestration/scripts/merge-sarif.sh:48`, `internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh:78`, and `internal/tmpl/templates/skills/gha-security-review/scripts/pin-actions.sh:90` use Bash 4-only features exercised by `internal/toolgen/toolgen_test.go:1785`.
  - `internal/engine/context/context.go:1` declares package `context`; call sites import it as `ctxpack` from `cmd/common.go:14` and `internal/engine/progression/readiness.go:13`.
- Dependency chains:
  - GitHub Actions CI -> workflow YAML -> repository lint configs -> maintained source, docs, and generated-template surfaces.
  - GitHub Actions matrix tests -> `go test ./...` -> template script fixture tests -> rendered host skill scripts.
  - GitHub Actions Windows tests -> state persistence/archive helpers -> filesystem directory sync behavior.
  - Nix workflow -> `flake.nix` derivation -> Go module vendor hash.
  - Security workflow -> govulncheck/Trivy SARIF production -> CodeQL SARIF upload API.
- Blast radius:
  - CI/workflow YAML, lint config, Nix metadata, generated skill shell scripts, and narrowly scoped Go portability/lint cleanup.
  - No public CLI behavior, model schema, user data, or networked product API should change.
- Constraints:
  - `artifacts/codebase/ARCHITECTURE.md` identifies `cmd/`, `internal/state/`, and `internal/engine/` as load-bearing boundaries; state changes must preserve `change.yaml` authority.
  - `artifacts/codebase/TESTING.md` says focused regression tests should precede full-suite verification.
  - User scope excludes GitHub Pages settings and missing token/secret repair, so workflow changes must not introduce new credential dependencies.

### Patterns
- Existing conventions:
  - Workflow lint/build/test jobs live under `.github/workflows/` with explicit permissions.
  - Generated skill scripts are offline and deterministic; `pin-actions.sh` documents no network lookups, and `merge-sarif.sh` only requires `jq`.
  - Tests for rendered scripts execute generated files rather than only template sources, via `generatedSkillsRoot` and `scriptPathForTest` in `internal/toolgen/toolgen_test.go:1688` and `internal/toolgen/toolgen_test.go:1697`.
  - Internal package imports already alias the engine context package as `ctxpack`, so a non-conflicting package name can preserve caller readability.
- Reusable abstractions:
  - Keep existing atomic write and archive helpers; only adjust unsupported Windows directory fsync handling.
  - Keep existing shell script interfaces and test fixtures; replace Bash 4 constructs with Bash 3-compatible indexed arrays and read loops.
  - Keep the existing GitHub Actions security workflow; add missing checked-in permissions and normalize SARIF before upload.
- Convention deviations:
  - Removing `.github/workflows/docs.yml` is intentional and temporary because Pages enablement is outside the checked-in repo.
  - Excluding `artifacts/**`, `.worktrees/**`, and generated host templates from broad markdown/YAML lint is a scope correction, not a formatter waiver for maintained docs.

### Risks
- Technical risks:
  - Medium: ignoring unsupported directory sync failures on Windows could weaken durability semantics if applied too broadly. Limit it to directory sync errors on Windows after file sync/rename work is already complete.
  - Medium: SARIF normalization could hide malformed scanner output if it is too broad. Limit it to deterministic tag de-duplication before upload.
  - Low: Bash 3-compatible rewrites can alter script ordering. Preserve sorted input and existing command-line interfaces.
  - Low: lint scope tightening could miss real documentation/config issues. Keep maintained docs and workflow YAML in scope; exclude generated/archive surfaces that are not authored as standalone lint targets.
  - Low: Nix vendor hash update can drift again when Go modules change; verification should run the Nix build or at least validate the checked hash against the remote failure.
- Guardrail domains:
  - `external_api_contracts` applies because GitHub Actions workflows and SARIF upload contracts are external CI integration surfaces.
  - Security scanning workflow is touched, but no secrets, credentials, or auth behavior are added or modified.
- Reversibility:
  - All changes are checked-in text/code changes and can be reverted cleanly.
  - Pages workflow removal can be restored later when repository Pages settings are enabled.

### Test Strategy
- Existing coverage:
  - `internal/toolgen/toolgen_test.go:1785` exercises the generated shell scripts that failed on macOS.
  - `internal/fsutil/atomic_test.go:13` covers atomic file replacement; `internal/state/lifecycle_test.go:17` and related archive tests cover archive/move behavior.
  - `go test ./...` is the repo-native full regression suite per `AGENTS.md` and `artifacts/codebase/TESTING.md`.
- Infrastructure needs:
  - No new service credentials or external GitHub settings.
  - `yamllint`, `markdownlint-cli2`, `golangci-lint`, Nix, `jq`, and Go tests for local verification where available.
- Verification approach:
  - Run focused tests for `internal/toolgen`, `internal/fsutil`, `internal/state`, `internal/engine/context`, and affected `cmd`/`internal/engine/progression` imports.
  - Run local lint checks that match CI where installed or install via repo/toolchain conventions only when needed.
  - Run `go test -timeout=20m ./... -count=1` and `go build ./...`.
  - Run or validate Nix build metadata; if full Nix build is unavailable or expensive, record the exact hash update and any attempted build output.
  - Run `go run . validate --json` and Slipway repair/closeout checks before finalizing.

## Alternatives Considered
- Minimal source/config repair:
  - Design: patch only checked-in CI workflow/config, Bash compatibility, Go portability/lint issues, and Nix metadata; remove only the Pages deployment workflow; leave token/settings failures untouched.
  - Tradeoffs: lowest blast radius and directly matches user scope, but remote Release Please may remain red until repo settings or tokens are fixed later.
- CI topology reset:
  - Design: disable or rewrite broad CI/release/security workflows to reduce failures immediately.
  - Tradeoffs: faster to get green by suppression, but loses coverage and violates the requirement to fix what is actually repairable.
- External setting/token repair:
  - Design: configure GitHub Pages, add/rotate `GH_PAT`, or change repository Actions settings.
  - Tradeoffs: may address remaining red runs, but it is outside the requested scope and not auditable as a repo-local code change.
- Selected: Minimal source/config repair, because it preserves CI signal, is fully reviewable in git, removes only the explicitly out-of-scope Pages automation, and leaves token/settings work as a documented residual external dependency.

## Unknowns
- Resolved: Why did YAML lint fail? -> CI runs `yamllint -c .yamllint.yaml .`, so archived governance artifacts are in scope unless ignored.
- Resolved: Why did Markdown lint fail? -> CI scans `**/*.md`; generated host templates and one maintained doc fence currently violate markdownlint expectations.
- Resolved: Why did macOS tests fail? -> Rendered Bash scripts use Bash 4-only `mapfile` and associative arrays; macOS system Bash is 3.2 on hosted runners.
- Resolved: Why did Windows tests fail? -> Directory fsync calls return access-denied style errors on Windows temp directories.
- Resolved: Why did Nix fail? -> `flake.nix:29` has a stale Go vendor hash.
- Resolved: Which security failures are repo-fixable? -> Trivy SARIF upload needs checked-in workflow permission, and govulncheck SARIF needs deterministic normalization before upload.
- Resolved: Which failures remain intentionally out of scope? -> GitHub Pages enablement and Release Please PR-creation/token/settings failures.
- Remaining: None for implementation planning.

## Assumptions
- The GitHub Pages workflow, not docs content or `mkdocs.yml`, is the surface to remove. Evidence: user asked to delete GitHub Pages-related automation for now; `.github/workflows/docs.yml:18` and `.github/workflows/docs.yml:66` are the Pages-specific surfaces.
- Release Please should not be altered in this change. Evidence: user explicitly said missing token-like issues should not be fixed for now, and `.github/workflows/release-please.yaml:18` depends on the release-please PR creation path.
- Windows directory sync can be treated as unsupported while preserving file-level fsync/rename semantics. Evidence: failing calls are directory sync helpers at `internal/fsutil/atomic.go:65` and `internal/state/lifecycle.go:282`.

## Canonical References
- `artifacts/changes/repair-fixable-remote-ci-failures-by-tightening-lint-scopes-cross-platform-behavior-nix-hash-and-security-workflow-configuration-while-removing-github-pages-automation/intent.md`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `.github/workflows/ci.yml`
- `.github/workflows/security.yaml`
- `.github/workflows/docs.yml`
- `.github/workflows/release-please.yaml`
- `.yamllint.yaml`
- `.markdownlint.yaml`
- `flake.nix`
- `docs/command-contract-matrix.md`
- `internal/fsutil/atomic.go`
- `internal/state/lifecycle.go`
- `internal/tmpl/templates/skills/sast-orchestration/scripts/merge-sarif.sh`
- `internal/tmpl/templates/skills/root-cause-tracing/scripts/find-polluter-go.sh`
- `internal/tmpl/templates/skills/gha-security-review/scripts/pin-actions.sh`
- `internal/toolgen/toolgen_test.go`
- `internal/engine/context/context.go`
- `cmd/common.go`
- `internal/engine/progression/readiness.go`
