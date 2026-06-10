# Decision

## Alternatives Considered
- CodeQL only: simple GitHub-native SAST coverage, but it does not address the
  issue's gosec-specific baseline and rule IDs.
- Gosec only: directly addresses the reported baseline, but does not satisfy the
  user's confirmed request to add both gosec and CodeQL.
- Both engines with gosec SARIF only: adds visibility but leaves the current
  baseline as non-blocking, which does not satisfy the user's clarification that
  all findings must be resolved.
- Both engines with full-repository gosec baseline resolution: larger code-touch
  surface, but it gives the repository a real gosec gate and satisfies the
  clarified scope.

## Selected Approach
Add both gosec and CodeQL to `.github/workflows/security.yaml`, and resolve every
current full-repository gosec finding before enabling gosec as a failing CI job.

The implementation will prefer real fixes where they reduce risk without
changing public behavior. Where gosec cannot infer Slipway's bounded authority
model, the implementation will use local `#nosec` suppressions with rule IDs and
specific rationales rather than global exclusions. This keeps future gosec
findings actionable while documenting why the current baseline is safe.

## Interfaces and Data Flow
- `.github/workflows/security.yaml` gains:
  - A gosec job that checks out the repo, sets up Go, runs gosec across `./...`,
    writes `gosec.sarif`, uploads the SARIF artifact, and uploads Code Scanning
    results.
  - A CodeQL job for Go using `github/codeql-action/init@v4` and
    `github/codeql-action/analyze@v4`.
- Go source data flow does not gain a new runtime interface. Existing CLI path
  flows remain rooted in `state.ResolveChangePaths`, state path helpers, and
  git/worktree helpers.
- Suppression data flow is local to source comments. Each suppression documents
  why that exact call site is safe.

## Rollout and Rollback
- Rollout:
  - Merge workflow additions and source triage together so the new gosec job is
    not red on the historical baseline.
  - Verify with `go test -count=1 ./...` and full-repository gosec JSON/SARIF
    commands before final closeout.
  - Let GitHub Actions run CodeQL on pull request and main events.
- Rollback:
  - Revert the commit that adds the Security workflow jobs and source
    suppressions/fixes.
  - Re-run `go test -count=1 ./...` and `go run . validate --json` to confirm
    the rollback returns the worktree to the prior behavior.
  - No data migration or archive mutation is required; `slipway done` remains
    out of scope.

## Risk
- Full baseline cleanup touches many files. The main implementation risk is
  noisy suppressions that hide real problems; mitigation is local, rule-specific
  rationale at each suppression and tests for any real code fixes.
- Permission tightening can change artifact readability. The implementation must
  only tighten permissions when the file is runtime/private, or otherwise
  explain why the broader permission is intentional.
- Symlink/WalkDir high findings must not be dismissed silently. They require
  either a safe root-preserving implementation or a clear explanation that the
  path is already bounded by Slipway's governed bundle authority.
- CodeQL local execution is not available in the same way as gosec; local
  verification is workflow inspection plus CI after push, while gosec and Go
  tests provide local closeout evidence.
