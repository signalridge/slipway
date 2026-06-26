# Spec Compliance Review Notes

Verdict: pass

Reviewer handle: `codex-spec-compliance-review-20260627T021434JST`

Review role: `spec-compliance-review` only, fresh S3 peer review for governed
change `harden-release-supply-chain`.

## Evidence References To Record

- `layer:R0=pass`
- `scope_contract:pass`
- `negative_path:pass`
- `context_origin:stage=review=codex-spec-compliance-review-20260627T021434JST`
- `context_origin:stage=fix=019f04cc-6fed-74f0-9042-f92e767e0291`
- `context_origin:stage=fix=019f04e3-aa63-7ed2-a61f-467180f45be5`

## Lifecycle And Scope

- `SLIPWAY_HOST_CAPABILITIES=subagent go run . next --json --diagnostics`
  reports active state `S3_REVIEW`, review batch
  `spec-compliance-review`, `code-quality-review`, `independent-review`, and
  `security-review`, and locked decision Option B.
- `SLIPWAY_HOST_CAPABILITIES=subagent go run . validate --json` reports
  `scope_contract.status=pass`.
- `artifacts/changes/harden-release-supply-chain/verification/execution-summary.yaml`
  reports `run_summary_version: 3`, `overall_verdict: pass`, and completed
  task evidence for `t-01` through `t-06`.
- Scope contract changed files are the eight workflow files, three live
  GitHub request artifacts, `cmd/release_workflow_contract_test.go`,
  `cmd/tool_github.go`, `cmd/tool_test.go`,
  `internal/state/worktree.go`, and `internal/state/worktree_test.go`.
- `artifacts/codebase/ARCHITECTURE.md`, `CONCERNS.md`, `STRUCTURE.md`, and
  `TESTING.md` are reported as exempt context files by `scope_contract`.
  They do not add product behavior requiring a separate spec row.
- Remaining lifecycle blockers in `validate --json` are peer/terminal S3
  evidence blockers outside this role: stale or missing
  `code-quality-review`, `independent-review`, `security-review`, and
  `ship-verification` evidence. They are not spec-compliance blockers.

## Spec-To-Code Trace

| Spec item | Realized by | Status | Reason |
| --- | --- | --- | --- |
| REQ-001: `main` and release tags must be protected by active GitHub rules matching project workflows (`requirements.md:7-22`). | Live `gh api` checks in this review: `main` returned `protected=true`; ruleset `18174607` is active for `~DEFAULT_BRANCH` and requires strict status checks including `Release Config`; ruleset `18174614` is active for `refs/tags/v*` and has `creation`, `update`, `deletion`, and `non_fast_forward` rules. Stored request artifacts: `verification/main-branch-ruleset-request.json`, `verification/release-tag-ruleset-request.json`, `verification/release-environment-request.json`; S2 evidence: `verification/task-results/t-01.json`. | covered | Direct live evidence and stored request artifacts match the requirement. |
| REQ-002: release workflow must validate manual release tag input before release secrets are available, and downstream release refs must use the validated tag output (`requirements.md:26-43`). | `.github/workflows/release.yaml:22-47` defines a read-only `validate-tag` job that rejects control characters and validates the whole scalar before writing `tag_name`. `.github/workflows/release.yaml:52-58` and `:84-106` make `test` and `release` depend on `validate-tag` and checkout `needs.validate-tag.outputs.tag_name`. `.github/workflows/release.yaml:81-164` confines write/package/attestation permissions and `GH_PAT` / `AUR_SSH_PRIVATE_KEY` to the `release` job behind `release-publish`. Tests: `cmd/release_workflow_contract_test.go:16-66` and `:68-91`. | covered | Static and executable workflow tests exercise the literal no-secret ordering, validated-tag use, and output-injection negative path. |
| REQ-003: workflow action and tool dependencies must not float (`requirements.md:47-61`). | Workflow `uses:` refs in all changed workflow files are full commit SHAs. `.github/workflows/security.yaml:23-24` pins `govulncheck@v1.5.0`; `.github/workflows/security.yaml:155-156` pins `go-licenses@v1.6.0`. Review `rg` scan found no active `uses:` refs using `@latest`, `@main`, `@master`, or version tags and no `govulncheck@latest` or `go-licenses@latest`; S2 evidence: `verification/task-results/t-03.json`. | covered | Action/tool dependency pinning matches the requirement; preserved original version tags are comments only. |
| REQ-004: token-backed GitHub REST/GraphQL backend must fail closed for unsafe API base URL overrides; allowed overrides require `SLIPWAY_GITHUB_API_TOKEN` and must not receive ambient `GH_TOKEN` / `GITHUB_TOKEN` (`requirements.md:65-81`). | `cmd/tool_github.go:337-371` resolves base URL and token policy. `cmd/tool_github.go:374-410` rejects non-HTTPS, userinfo, query/fragment, encoded path, non-canonical path, and public `api.github.com` path overrides before request construction. `cmd/tool_github.go:563-623` re-authorizes pagination `Link` URLs before token-bearing follow-up requests. Tests: `cmd/tool_test.go:424-481`, `:484-555`, and `:557-602`. | covered | Negative tests cover HTTP, unknown host, query, userinfo, public path confusion including explicit default port, override-token isolation, and unsafe pagination links. |
| REQ-005: governed `BaseRef` values must be validated before `git worktree add`; invalid values must not reach git, and valid branch/tag/SHA refs remain usable (`requirements.md:85-100`). | `internal/state/worktree.go:232-240` validates `change.BaseRef` before adding it to `git worktree add` argv. `internal/state/worktree.go:259-275` defaults empty refs to `HEAD`, rejects option-like and control-character refs, and verifies commit resolution with `git rev-parse --verify --quiet`. Tests: `internal/state/worktree_test.go:294-362`. | covered | Negative tests assert no `git worktree add failed` leakage and no worktree directory for invalid values; valid tag refs still create a worktree. |
| REQ-006: release config changes must be validated on PR and release smoke must derive from generated assets (`requirements.md:104-121`). | `.github/workflows/ci.yml:116-143` adds `Release Config` with GoReleaser v2.16.0 `check`, Docker Buildx, syft, and snapshot dry run. `.github/workflows/release.yaml:183-221` emits smoke outputs from `dist/`; `.github/workflows/release.yaml:318-396` consumes `deb_asset`, `rpm_asset`, `apk_asset`, and `binary_matrix` and runs `slipway --version` plus `slipway --help`. Test: `cmd/release_workflow_contract_test.go:94-132`; S2 evidence: `verification/task-results/t-02.json` and `t-06.json`. | covered | CI and release smoke wiring match the requirement and are covered by static workflow tests plus task evidence. |

No spec-to-code row is skipped, drifted, ambiguous, or uncheckable.

## Code-To-Spec Trace

| Changed file or hunk group | Plan/spec mapping | Status | Reason |
| --- | --- | --- | --- |
| `artifacts/changes/harden-release-supply-chain/verification/main-branch-ruleset-request.json` | REQ-001; task `t-01`; decision live settings (`decision.md:62-68`). | covered | Stored request payload for the active main branch ruleset. |
| `artifacts/changes/harden-release-supply-chain/verification/release-tag-ruleset-request.json` | REQ-001; task `t-01`; decision live settings (`decision.md:62-68`). | covered | Stored request payload for the active `refs/tags/v*` ruleset. |
| `artifacts/changes/harden-release-supply-chain/verification/release-environment-request.json` | REQ-002; task `t-01`; decision protected release environment (`decision.md:67-68`). | covered | Stored request payload for `release-publish` required reviewer protection. |
| `.github/workflows/release.yaml` | REQ-002, REQ-003, REQ-006; tasks `t-02`, `t-03`, `t-06`; release data flow (`decision.md:69-77`). | covered | Implements tag validation, release permissions/secrets, SHA-pinned release actions, GoReleaser, and smoke outputs. |
| `.github/workflows/ci.yml` | REQ-003, REQ-006; tasks `t-02`, `t-03`; PR release config check (`decision.md:91-95`). | covered | Adds always-running `Release Config` and SHA pins. |
| `.github/workflows/docs.yml` | REQ-003; task `t-03`. | covered | Workflow dependency pinning only. |
| `.github/workflows/flake-lock-update.yaml` | REQ-003; task `t-03`. | covered | Workflow dependency pinning and removal of active moving refs. |
| `.github/workflows/nix.yaml` | REQ-003; task `t-03`. | covered | Workflow dependency pinning and removal of active moving refs. |
| `.github/workflows/pr-title.yaml` | REQ-001 required check context and REQ-003 pinning; task `t-03`. | covered | Keeps title check available as a required main-branch status and pins its action. |
| `.github/workflows/release-please.yaml` | REQ-003; task `t-03`. | covered | Workflow dependency pinning only. |
| `.github/workflows/security.yaml` | REQ-003; task `t-03`. | covered | Pins security actions and fixed Go security tool versions. |
| `cmd/release_workflow_contract_test.go` | REQ-002 and REQ-006; tasks `t-02`, `t-06`. | covered | Static/executable workflow contract tests for validation order and smoke manifest wiring. |
| `cmd/tool_github.go` | REQ-004; task `t-04`; GitHub API HTTP backend (`decision.md:78-84`). | covered | Shared REST/GraphQL HTTP client, override-token policy, URL normalization, and pagination reauthorization. |
| `cmd/tool_test.go` | REQ-004; task `t-04`. | covered | Negative-path and token-isolation regression tests for the API override boundary. |
| `internal/state/worktree.go` | REQ-005; task `t-05`; worktree provisioning (`decision.md:85-87`). | covered | Validates `BaseRef` before worktree creation. |
| `internal/state/worktree_test.go` | REQ-005; task `t-05`. | covered | Negative and positive regression coverage for BaseRef validation. |
| `artifacts/codebase/ARCHITECTURE.md`, `CONCERNS.md`, `STRUCTURE.md`, `TESTING.md` | `scope_contract.exempt_context_files`. | covered | Context-map updates only; no product behavior outside scope. |

No code-to-spec row is skipped, drifted, ambiguous, or uncheckable.

## Negative And Error Path Evidence

- `negative_path:pass`
- Release invalid/manual tag path: `.github/workflows/release.yaml:37-47`
  rejects control characters and non-semver scalar tags before
  `GITHUB_OUTPUT`; `cmd/release_workflow_contract_test.go:68-91` executes
  multiline, CR, and tab cases and asserts output remains empty.
- API override unsafe URL path: `cmd/tool_github.go:374-410` rejects unsafe
  base URLs before request construction; `cmd/tool_test.go:424-481` covers
  HTTP, unknown HTTPS host, query, userinfo, and public path-confused URLs.
- API override allowlisted public-path path: `cmd/tool_github.go:403-405`
  uses `parsed.Hostname()` to reject non-root public `api.github.com` paths
  before allowlist evaluation; `cmd/tool_test.go:484-517` covers exact,
  explicit default-port, and mixed-case public path spellings even when
  allowlisted.
- API override token-isolation path: `cmd/tool_github.go:352-371` requires
  `SLIPWAY_GITHUB_API_TOKEN` for overrides; `cmd/tool_test.go:520-555`
  proves ambient tokens are not sent and only the override token is used.
- API pagination token-leak path: `cmd/tool_github.go:563-623` validates
  `Link` next URLs before use; `cmd/tool_test.go:557-602` proves cross-host
  and base-path-escape links fail after only the first allowlisted request.
- BaseRef invalid path: `internal/state/worktree.go:259-275` rejects
  option-like, control-character, and unresolved refs before `git worktree add`;
  `internal/state/worktree_test.go:294-362` asserts no worktree directory is
  created for invalid refs and valid tag refs still work.

## Decision Fidelity

Locked decision fidelity: pass.

The locked decision from `next --json --diagnostics` requires Option B as one
cohesive `opt.md` section 2 change covering live GitHub protections, release
workflow safety, SHA/tool pinning, API override token isolation, BaseRef
validation, and release config/smoke verification.

- `decision.md:45-48` selects Option B with that scope.
- `decision.md:62-68` requires live rulesets and the `release-publish`
  environment. Fresh `gh api` checks during this review confirm the active main
  ruleset, active release tag ruleset, and required reviewer environment.
- `decision.md:69-77` requires no-secret validation, validated tag downstream
  use, secret confinement, and smoke outputs from `dist/`; the workflow and
  contract tests implement those clauses.
- `decision.md:78-84` requires default public ambient-token flow, exact
  allowlisted override flow with `SLIPWAY_GITHUB_API_TOKEN`, and shared
  REST/GraphQL client creation; `cmd/tool_github.go` implements that boundary
  and `cmd/tool_test.go` covers its negative paths.
- `decision.md:85-87` requires `BaseRef` validation before adding the ref to
  `git worktree add`; `internal/state/worktree.go` implements that boundary.
- Option C is not implemented; live settings were applied and verified rather
  than deferred to documentation.

## Commands Run

- `SLIPWAY_HOST_CAPABILITIES=subagent go run . next --json --diagnostics`
- `SLIPWAY_HOST_CAPABILITIES=subagent go run . validate --json`
- `git status --short`
- `git diff --stat -- . ':!dist'`
- `git diff --name-only -- . ':!dist'`
- `rg -n "uses: [^#\\n]+@(v[0-9]|main|master|latest)|@(latest|main|master)\\b|govulncheck@latest|go-licenses@latest|DeterminateSystems/[^@]+@main" .github/workflows`
  - result: no matches
- `gh api repos/signalridge/slipway/branches/main --jq '{protected:.protected, protection_url:.protection_url}'`
- `gh api repos/signalridge/slipway/rulesets --jq '[.[] | {id:.id,name:.name,target:.target,source_type:.source_type,enforcement:.enforcement}]'`
- `gh api repos/signalridge/slipway/rulesets/18174607 --jq '{id,name,enforcement,target,conditions:.conditions, rules:[.rules[] | {type:.type, parameters:.parameters}]}'`
- `gh api repos/signalridge/slipway/rulesets/18174614 --jq '{id,name,enforcement,target,conditions:.conditions, rules:[.rules[] | {type:.type, parameters:.parameters}]}'`
- `gh api repos/signalridge/slipway/environments/release-publish --jq '{name:.name, protection_rules:.protection_rules}'`
- `go test ./cmd ./internal/state -count=1`
- `go test ./cmd -run 'TestReleaseWorkflow|TestGitHub' -count=1`
- `go test ./cmd -run 'TestReleaseWorkflow|TestGitHubAPIOverrideRejectsUnsafeURLs|TestGitHubAPIOverrideRejectsAllowlistedPublicPathConfusion|TestGitHubAPIOverrideRequiresOverrideTokenAndDoesNotUseAmbient|TestGitHubAPIOverrideRejectsUnsafePaginationLink' -count=1`
- `go test ./internal/state -run 'TestEnsureDefaultWorktreeForChange' -count=1`
- `actionlint .github/workflows/*.yml .github/workflows/*.yaml`
- `git diff --check -- . ':!dist'`

All commands above passed, except the `rg` scan intentionally returned exit 1
with no matches for forbidden floating refs.

## Blockers

None for spec compliance.

Peer and terminal S3 lifecycle blockers remain as reported by `validate --json`
and must be handled by their owning roles before ship.
