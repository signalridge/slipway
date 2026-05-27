# Decision

## Project Context
- Tech Stack: Go, GitHub Actions, MkDocs, GoReleaser, Release Please
- Conventions: local-first Slipway governance, repo-native validation, least-privilege CI permissions, deterministic release automation
- Test Command: go test -timeout=20m ./... -count=1
- Build Command: go build ./...
- Languages: Go, YAML, Markdown

## Alternatives Considered

### DEC-001: Workflow-only remediation

Fix `.github/workflows/ci.yml` and `.github/workflows/docs.yml` only. This would satisfy the immediate CI parts of REQ-001, REQ-002, and REQ-006, but would not satisfy REQ-003, REQ-004, or REQ-005. Rejected because the operator also requested clinvoker-inspired presentation improvements and the long-path failure had an obvious future-prevention point in slug generation.

### DEC-002: Repository-setting remediation

Enable GitHub Pages in repository settings and leave docs deployment unconditional. This could avoid the current Docs failure for REQ-002, but it would make the code path depend on mutable remote settings and would not improve docs validation behavior for forks or disabled Pages environments. Rejected because repository settings are outside the code-change boundary.

### DEC-003: Scoped code, workflow, and presentation hardening

Fix the failing workflows, make docs deployment conditional on Pages status, cap future generated slugs, and update README/release presentation using clinvoker as a style reference. This satisfies REQ-001 through REQ-006 with targeted code/config/docs changes and clear rollback. Selected.

## Selected Approach

Use DEC-003. The implemented fix is scoped to:
- `.github/workflows/ci.yml` for Windows long-path checkout behavior.
- `.github/workflows/docs.yml` for Pages-aware docs deployment.
- `internal/model/identity.go` and `cmd/new.go` for future slug length control.
- `README.md` and `release-please-config.json` for clinvoker-inspired presentation.
- Focused tests and repo-native validation for proof.

## Interfaces and Data Flow

`go run . new` and related change creation flows generate slugs through `model.SlugifyTitle`; `cmd.generateUniqueChangeSlug` appends collision suffixes while preserving the max length. GitHub Actions consume tracked repository paths during checkout, build, docs, and tests. Docs workflow now queries repository Pages status before upload/deploy and always runs the MkDocs build.

## Rollout and Rollback

Rollout was two commits on `main`: `48f3c92` for workflow/README/release/slug hardening and `00f1397` for Pages API permissions in the docs workflow. Rollback is a normal Git revert of those commits. GitHub Pages enablement remains a separate repository settings decision.

## Risk

Residual risk is low. Slug truncation changes future generated slug display, but keeps uniqueness via suffix handling and does not migrate existing archives. Docs deployment is skipped while Pages is disabled, so publishing still requires repository settings alignment. Security workflow annotations about code scanning settings are repository configuration warnings, not failing checks.
