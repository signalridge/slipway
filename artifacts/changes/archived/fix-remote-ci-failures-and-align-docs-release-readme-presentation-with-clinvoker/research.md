# Research

## Research Findings

### Architecture
- Affected modules: GitHub Actions workflows (`.github/workflows/ci.yml`, `.github/workflows/docs.yml`), README presentation (`README.md`), Release Please configuration (`release-please-config.json`), and slug identity helpers (`internal/model/identity.go`, `cmd/new.go`).
- Dependency chains: `cmd/new.go` calls `model.SlugifyTitle`; generated slugs determine governed artifact and worktree path lengths; remote CI consumes tracked paths during checkout and test execution.
- Blast radius: low to medium. The behavior change is limited to generated slug length, docs workflow deployment gating, Windows checkout configuration, and presentation copy.
- Constraints: preserve existing release artifact names, GitHub Actions workflow intent, and current package distribution channels.

### Patterns
- Existing conventions: Go tests cover CLI/model behavior; workflows use least-privilege permissions and matrix jobs; README already lists install and documentation surfaces.
- Reusable abstractions: `model.SlugifyTitle` is the single low-level place to cap generated slug length; `cmd.generateUniqueChangeSlug` owns collision suffix behavior.
- Convention deviations: the docs workflow now validates docs even when GitHub Pages is disabled, and conditionally uploads/deploys only when the repository Pages API reports Pages enabled.

### Risks
- Technical risks: Windows checkout can fail before project tests run when tracked paths are too long; docs deployment can fail when Pages is disabled; overly long slugs can recreate long-path failures later.
- Guardrail domains: none. This change does not modify auth, secrets handling, privacy data, payments, schema migration, destructive operations, or external API contracts.
- Reversibility: high. The workflow and docs changes can be reverted by reverting commits `48f3c92` and `00f1397`; slug truncation affects future generated slugs only.

### Test Strategy
- Existing coverage: Go model/CLI tests cover slug generation; repo-native validation covers workflows, docs, release config, build, and full test suite.
- Infrastructure needs: no new external services. Remote GitHub Actions are the authority for CI failure remediation.
- Verification approach: local validation with `actionlint`, `mkdocs build --strict`, `goreleaser check`, `jq`, `go build ./...`, `go test -timeout=20m ./... -count=1`; remote validation with latest GitHub Actions runs on `main`.

## Alternatives Considered
- Workflow-only fix: configure Windows checkout and docs deploy conditions only. Tradeoff: fixes immediate failures but leaves future generated long slugs able to recreate path-length risk.
- Repository-settings fix: enable GitHub Pages in GitHub settings and keep docs deploy unconditional. Tradeoff: changes remote repository configuration outside the code change and still does not make docs validation robust for disabled Pages.
- Selected: combine scoped workflow hardening, slug-length prevention, and clinvoker-inspired presentation updates. This addresses the immediate CI failures, prevents recurrence from future slugs, and completes the requested README/release polish.

## Unknowns
- Resolved: GitHub Pages was not enabled for this repository during triage, so docs deployment had to be conditional while docs build remains required.
- Resolved: latest `main` commit `00f13975d154bfd110f9d129c1d7bd7daba8d21d` has successful CI, Docs, Security, and Release Please runs.
- Remaining: whether the operator wants GitHub Pages enabled as a repository setting later. That is intentionally outside this code change.

## Assumptions
- The clinvoker reference is a presentation pattern source, not a request for a file-for-file copy. Evidence: operator wording asks to reference clinvoker release docs and README badges.
- Docs should remain validated even when Pages deployment is skipped. Evidence: remote CI failure was caused by deployment conditions, not by MkDocs content failure.
- Slug truncation should preserve collision suffixes. Evidence: `cmd.generateUniqueChangeSlug` appends numeric suffixes during collision handling.

## Canonical References
- `.github/workflows/ci.yml`
- `.github/workflows/docs.yml`
- `README.md`
- `release-please-config.json`
- `internal/model/identity.go`
- `cmd/new.go`
- `internal/model/identity_test.go`
- `cmd/new_test.go`
- `/Users/yixianlu/Projects/clinvoker/README.md`
- `/Users/yixianlu/Projects/clinvoker/release-please-config.json`
