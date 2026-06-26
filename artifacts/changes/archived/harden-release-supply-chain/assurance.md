# Assurance

## Scope Summary

This change delivers `opt.md` section 2 release and supply-chain hardening as
one governed scope:

- live GitHub repository protections for `main`, `refs/tags/v*`, and the
  `release-publish` environment;
- release workflow fail-closed tag validation before any secret-bearing or
  write-permission job;
- full-SHA workflow action pinning and fixed Go security tool versions;
- token-safe GitHub API override handling for REST and GraphQL helpers;
- governed `BaseRef` validation before `git worktree add`;
- CI release-config validation and release smoke wiring derived from generated
  `dist/` assets.

Out of scope remains `opt.md` section 3 and later architecture/coverage-gate
work.

## Verification Verdict

S2 implementation evidence is passing and fresh at run summary version `3`.
The task ledger recorded all six planned tasks with verdict `pass`, and
`wave-orchestration` recorded run_version `3` with real parallel executor
handles for wave 1 plus both S3 repair handles.

S3 peer review has converged: `spec-compliance-review`,
`code-quality-review`, `independent-review`, and `security-review` all recorded
fresh `pass` evidence at run_version `3` with distinct review-context handles.
Terminal ship-verification evidence gathering has also passed and produced the
authoritative suite, static-check, stub-scan, coverage, and closeout notes
under `verification/logs/` and `verification/ship-verification-notes.md`.

Current closeout status: the next active step is to stamp `ship-verification`
through `slipway evidence skill`, then capture final active `validate --json`
freshness/readiness proof before `done`. This assurance artifact is the final
evidence index and rollback record used by those gates; it is not an
archived-bundle revalidation claim.

## Evidence Index

- `verification/main-branch-ruleset-request.json`: live main branch ruleset
  request body.
- `verification/release-tag-ruleset-request.json`: live release tag ruleset
  request body.
- `verification/release-environment-request.json`: protected
  `release-publish` environment request body.
- `verification/task-results/t-01.json`: read-only GitHub API verification for
  branch, tag, and environment protections.
- `verification/task-results/t-02.json`: release workflow validation,
  permissions, environment gate, smoke-output, `actionlint`, and GoReleaser
  config evidence.
- `verification/task-results/t-03.json`: workflow full-SHA pinning and fixed
  security tool version evidence.
- `verification/task-results/t-04.json`: GitHub API override token isolation
  evidence and focused `cmd` tests.
- `verification/task-results/t-05.json`: `BaseRef` validation evidence and
  focused `internal/state` tests, including direct control-character coverage.
- `verification/task-results/t-06.json`: release workflow contract test and
  bounded GoReleaser snapshot dry-run evidence.
- `verification/wave-orchestration.yaml`: CLI-stamped S2 execution evidence.
- `verification/wave-orchestration-notes.md`: wave dispatch, integration gate,
  and scope-safety summary.
- `verification/spec-compliance-review.yaml` and
  `verification/spec-compliance-review-notes.md`: bidirectional spec trace,
  scope-contract pass, and negative-path pass evidence.
- `verification/code-quality-review.yaml` and
  `verification/code-quality-review-notes.md`: implementation quality and
  toolchain compatibility review evidence.
- `verification/independent-review.yaml` and
  `verification/independent-review-notes.md`: fresh diff-scoped independent
  review evidence.
- `verification/security-review.yaml` and
  `verification/security-review-notes.md`: secure-default review evidence,
  including closure of release-tag output injection, pagination token
  exfiltration, and public GitHub path-confusion blockers.
- `verification/ship-verification-notes.md`: terminal acceptance criteria,
  freshness, assurance, independence, and coverage evidence gathered for the
  ship gate.
- `verification/logs/ship-suite.txt`: authoritative `go test ./... -count=1`
  transcript.
- `verification/logs/ship-static-checks.txt`: `golangci-lint`, `actionlint`,
  GoReleaser `check`, diff whitespace, workflow pin/default scans, and live
  GitHub protection reads.
- `verification/logs/ship-stub-scan.txt`: planned-target stub and placeholder
  scan.
- `verification/logs/ship-coverage.txt` and
  `verification/logs/ship-coverage.out`: focused coverage report and profile
  for changed Go surfaces.

Additional executed checks:

- `go test ./... -count=1`
- `go test ./cmd ./internal/state -count=1`
- `go test ./cmd -run TestReleaseWorkflow -count=1`
- `go test ./internal/state -run 'TestEnsureDefaultWorktreeForChange(Rejects|Accepts|_Provisions)' -count=1`
- `golangci-lint run ./...`
- `actionlint .github/workflows/*.yml .github/workflows/*.yaml`
- `uvx yamllint -c .yamllint.yaml .github/workflows/ci.yml .github/workflows/docs.yml .github/workflows/flake-lock-update.yaml .github/workflows/nix.yaml .github/workflows/pr-title.yaml .github/workflows/release-please.yaml .github/workflows/release.yaml .github/workflows/security.yaml`
- `go run github.com/goreleaser/goreleaser/v2@v2.16.0 check`
- `go run github.com/goreleaser/goreleaser/v2@v2.16.0 release --snapshot --clean --skip=publish,sign,sbom,docker`

## Requirement Coverage

- REQ-001 Live Repository Protection: covered by `t-01` and the stored GitHub
  ruleset/environment request bodies. Live reads confirmed `main` is protected,
  the active main ruleset requires the exact always-running check contexts, the
  `refs/tags/v*` tag ruleset restricts tag mutations, and `release-publish`
  has required reviewers.
- REQ-002 Release Workflow Fails Closed Before Secret Exposure: covered by
  `t-02`, `t-06`, and `cmd/release_workflow_contract_test.go`. Static tests
  assert no-secret tag validation precedes test/release jobs, the release job
  consumes the validated tag output, and `GH_PAT` / `AUR_SSH_PRIVATE_KEY` are
  confined to the protected release job.
- REQ-003 Pinned Workflow and Tool Dependencies: covered by `t-03`. Workflow
  `uses:` refs are full commit SHAs, moving DeterminateSystems refs are only
  comments, and `govulncheck` / `go-licenses` installs are version-pinned.
- REQ-004 GitHub API Override Token Safety: covered by `t-04`,
  `cmd/tool_github.go`, and `cmd/tool_test.go`. Unsafe override URLs fail
  closed, allowlisted override URLs require `SLIPWAY_GITHUB_API_TOKEN`, ambient
  tokens stay limited to the default public GitHub API, and REST/GraphQL share
  the same validated backend path.
- REQ-005 BaseRef Validation Before Worktree Creation: covered by `t-05`,
  `internal/state/worktree.go`, and `internal/state/worktree_test.go`. Empty
  values default to `HEAD`, option-like and invalid refs fail before
  `git worktree add`, unknown refs fail with product remediation, and valid tag
  refs still provision a worktree.
- REQ-006 Release Config and Smoke Closure: covered by `t-02`, `t-06`,
  `.github/workflows/ci.yml`, `.github/workflows/release.yaml`, and
  `cmd/release_workflow_contract_test.go`. CI includes `Release Config` with
  GoReleaser check and snapshot dry-run coverage, and smoke jobs consume
  manifest outputs generated from actual `dist/` assets.

## Residual Risks and Exceptions

- Local bounded GoReleaser snapshot verification skips publish, sign, SBOM, and
  Docker because local Docker/syft availability is environment-dependent. CI
  `Release Config` installs syft and Docker Buildx to cover the fuller path.
- Live GitHub rulesets and environment settings are external repository state.
  Rollback must account for both git changes and the live ruleset/environment
  IDs recorded in task evidence.
- Repository branch protection now requires the new `Release Config` status.
  Future PRs must keep that check always-running or update rulesets and
  workflow requirements together.
- The GoReleaser dry run generates ignored `dist/` output. It is not intended
  for commit or archive evidence.

## Rollback Readiness

Rollback requires two coordinated actions:

1. Revert or supersede this PR's git changes for workflows and Go code.
2. Restore or replace the live GitHub repository controls:
   - main branch ruleset `protect main required checks`;
   - release tag ruleset `protect release tags`;
   - `release-publish` protected environment reviewers.

The request payloads in `verification/*-request.json` and `t-01` live-read
evidence identify the applied live controls. If rollback removes the
`Release Config` job, the main ruleset required checks must be updated in the
same rollback window to avoid blocking all future PR merges.

## Archive Decision

Archive after the terminal ship gate accepts this run. The active change has
fresh run_version `3` task, wave, and S3 peer-review evidence. Final archive
readiness now requires only the terminal `ship-verification` evidence stamp with
`closeout:assurance_complete=pass` and
`closeout:reviewer_independence=pass`, followed by a final active
`validate --json` freshness/readiness proof captured before `slipway done`.

Do not archive if that final active validation reports stale evidence, scope
contract drift, missing ship-verification attestations, or new blockers. After
`done`, the archived bundle should be treated as an archive surface, not as
something revalidated through the active validate gate.
