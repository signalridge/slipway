# S3 Review Repair Notes

Repair batch: `s3-review-repair:harden-release-supply-chain`

## Root Cause Summary

1. Release tag validation treated the tag as a line stream. The `grep -E`
   predicate succeeded when any line matched the semver pattern, so a
   multi-line manual input could pass and then be written raw to
   `GITHUB_OUTPUT`, creating output-file injection risk before the release job.
2. REST pagination validated only the initial GitHub API URL. Server-controlled
   `Link: rel="next"` URLs were used directly on the next loop iteration and
   authorized afterward, allowing an allowlisted endpoint to steer a bearer-token
   request to another host or outside the configured GitHub Enterprise base path.
3. Public `https://api.github.com/<path>` overrides were normalized and then
   compared against the allowlist. That allowed an explicit allowlist entry to
   override the categorical REQ-004 fail-closed rule for path-confused public
   GitHub API URLs.

## Files Changed

- `.github/workflows/release.yaml`
  - Rejects control characters before regex validation or output writes.
  - Uses Bash whole-scalar regex matching instead of line-oriented `grep`.
- `cmd/release_workflow_contract_test.go`
  - Adds executable coverage for valid scalar tags and multi-line/output
    injection attempts that must fail before writing `GITHUB_OUTPUT`.
- `cmd/tool_github.go`
  - Rejects non-root public `api.github.com` API overrides before allowlist
    evaluation.
  - Re-authorizes pagination `Link` URLs before use: absolute HTTPS URL, no
    userinfo, no fragment, same scheme/host, no encoded path, and path within
    the configured base path.
- `cmd/tool_test.go`
  - Adds regression coverage for allowlisted public-path override rejection.
  - Adds regression coverage for cross-host and base-path-escape pagination
    links, including proof that no token-bearing follow-up request is made.

## Tests Run

- `go test ./cmd -run 'TestReleaseWorkflow|TestGitHubAPIOverride|TestGitHubAPIOverrideRejectsUnsafeURLs|TestGitHubAPIOverrideRequiresOverrideTokenAndDoesNotUseAmbient|TestGitHubAPIOverrideRejectsUnsafePaginationLink' -count=1`
  - Result: passed (`ok github.com/signalridge/slipway/cmd 0.617s`).
- `actionlint .github/workflows/release.yaml`
  - Result: passed with no diagnostics.
- `go test ./cmd -run 'TestGitHub(APIOverride|BackendSelection|AutoGitHubBackend|ToolFetchPRChecksUsesTokenHTTP|ToolReplyToThreadConfirm|ToolFetchPRFeedback|ToolFetchReviewRequests)' -count=1`
  - Result: passed (`ok github.com/signalridge/slipway/cmd 0.320s`).
- `git diff --check -- .github/workflows/release.yaml cmd/release_workflow_contract_test.go cmd/tool_github.go cmd/tool_test.go artifacts/changes/harden-release-supply-chain/verification/s3-review-repair-notes.md`
  - Result: passed with no diagnostics.

## Residual Risk

- Bash/environment values cannot carry NUL bytes as ordinary shell scalars, so
  executable workflow coverage proves rejection for representable control
  characters such as LF, CR, and tab; the workflow guard uses `[[:cntrl:]]`
  before output to cover the scalar control-character boundary.
- Pagination link validation intentionally permits same-host links under the
  configured root public GitHub API base path. Non-root GitHub Enterprise bases
  are constrained to their configured path boundary.
- This repair did not rerun `slipway evidence` and did not edit verification
  YAML or lifecycle files, per the repair instructions.

## Second Repair Note: public-github-default-port-path-confusion

- Root cause: the public `api.github.com` path guard classified the host with
  `parsed.Host`, so explicit default ports such as `api.github.com:443` were not
  treated as the public GitHub API host.
- Repair: classify the public host with `parsed.Hostname()` before allowing
  non-root paths, preserving enterprise hosts with paths.
- Regression coverage: public `api.github.com` path overrides with an explicit
  default port, including mixed-case host input, now fail with
  `github_api_url_invalid` even when explicitly allowlisted.
