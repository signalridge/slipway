# Testing

- Workflow syntax:
  - `actionlint .github/workflows/*.yml .github/workflows/*.yaml`
  - `uvx yamllint -c .yamllint.yaml .github/workflows/release.yaml
    .github/workflows/ci.yml .github/workflows/security.yaml
    .github/workflows/nix.yaml .github/workflows/flake-lock-update.yaml
    .github/workflows/docs.yml .github/workflows/pr-title.yaml
    .github/workflows/release-please.yaml`
- Release config:
  - `go run github.com/goreleaser/goreleaser/v2@v2.16.0 check`
  - Local bounded dry run:
    `go run github.com/goreleaser/goreleaser/v2@v2.16.0 release --snapshot
    --clean --skip=publish,sign,sbom,docker`
  - CI dry run covers Docker/SBOM because it installs syft and sets up Docker
    Buildx on a runner with Docker available.
- Release workflow contract:
  - `go test ./cmd -run TestReleaseWorkflow -count=1`
  - The test proves `validate-tag` is no-secret/read-only, release/test jobs
    consume `needs.validate-tag.outputs.tag_name`, only `release` references
    `GH_PAT`/`AUR_SSH_PRIVATE_KEY`, and smoke inputs come from generated
    release outputs.
- GitHub API override:
  - `go test ./cmd -run 'TestGitHub(APIOverride|BackendSelection|AutoGitHubBackend|ToolFetchPRChecksUsesTokenHTTP|ToolReplyToThreadConfirm|ToolFetchPRFeedback|ToolFetchReviewRequests)' -count=1`
  - TLS test servers exercise the override path without allowing production
    HTTP base URLs.
- BaseRef validation:
  - `go test ./internal/state -run 'TestEnsureDefaultWorktreeForChange(Rejects|Accepts|_Provisions)' -count=1`
  - Tests cover option-like refs, unknown refs, valid tag refs, and default
    provisioning.
- Full implementation verification before S3:
  - `go test ./cmd -count=1`
  - `go test ./internal/state -count=1`
  - `go test ./... -count=1`
  - `golangci-lint run ./...`
