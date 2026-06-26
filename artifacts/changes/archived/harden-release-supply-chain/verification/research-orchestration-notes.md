# Research Orchestration Notes

## Verdict Basis

`research.md` contains concrete findings for architecture, patterns, risks, test
strategy, alternatives, unknowns, assumptions, and canonical references.

## Live Evidence

- Before repair: `branches/main` returned `protected: false`.
- Before repair: `repos/signalridge/slipway/rulesets` returned `[]`.
- After repair: `branches/main` returned `protected: true`.
- Created branch ruleset `18174607`, `protect main required checks`, target
  `branch`, enforcement `active`.
- Created tag ruleset `18174614`, `protect release tags`, target `tag`,
  enforcement `active`, conditions include `refs/tags/v*`.
- Created environment `release-publish` with required reviewer `signalridge`
  (`id: 52877870`).

## Repository Evidence

- `.github/workflows/release.yaml` now has a no-secret `Validate Release Tag`
  job before release secrets or write permissions.
- `.github/workflows/ci.yml` now has `Release Config` running fixed GoReleaser
  check and snapshot dry run.
- `cmd/tool_github.go` centralizes API override URL and token selection.
- `internal/state/worktree.go` validates `BaseRef` before `git worktree add`.
- `cmd/release_workflow_contract_test.go` provides static workflow policy
  coverage for tag validation before secret exposure and release smoke manifest
  wiring.
- `artifacts/codebase/{ARCHITECTURE,STRUCTURE,TESTING,CONCERNS}.md` has been
  reauthored for this release/supply-chain scope.

## Verification Evidence

- `actionlint .github/workflows/*.yml .github/workflows/*.yaml`: pass.
- `uvx yamllint -c .yamllint.yaml ...`: pass.
- `go run github.com/goreleaser/goreleaser/v2@v2.16.0 check`: pass.
- `go run github.com/goreleaser/goreleaser/v2@v2.16.0 release --snapshot --clean --skip=publish,sign,sbom,docker`: pass for local archives/packages/tap/scoop/AUR manifest graph.
- Full Docker/SBOM snapshot validation is configured in CI via Docker Buildx
  and downloaded syft; local Docker daemon was unavailable and local syft is an
  aqua shim that fails inside GoReleaser's child process.
- Focused Go tests:
  - `go test ./cmd -run TestReleaseWorkflow -count=1`: pass.
  - `go test ./internal/state -run 'TestEnsureDefaultWorktreeForChange(Rejects|Accepts|_Provisions)' -count=1`: pass.
  - `go test ./cmd -run 'TestGitHub(APIOverride|BackendSelection|AutoGitHubBackend|ToolFetchPRChecksUsesTokenHTTP|ToolReplyToThreadConfirm|ToolFetchPRFeedback|ToolFetchReviewRequests)' -count=1`: pass.

## Selected Approach

Selected approach: one cohesive `opt.md` section 2 hardening change. The active
thread goal grants auto-mode decision authorization, and this option closes the
coupled protection, workflow, token, ref-validation, and smoke-evidence gaps
without mixing in `opt.md` section 3 or section 4.
