# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `.github/workflows/security.yaml` currently provides security CI for
    `govulncheck`, Trivy, SBOM, and license checks, including SARIF upload for
    the vulnerability scanners, but no Go-source SAST job.
  - `cmd/pivot_execution.go`, `cmd/done.go`, and `internal/state/lifecycle.go`
    contain the issue-reported HIGH gosec findings.
  - The issue-reported changed-package baseline spans `cmd/`, `internal/state`,
    `internal/model`, `internal/toolgen`, and `internal/tmpl`.
- Dependency chains:
  - Security workflow -> GitHub Actions runner -> Go setup / SAST action ->
    SARIF artifact -> `github/codeql-action/upload-sarif`.
  - CLI command path resolution -> `state.ResolveChangePaths` -> governed bundle
    reads/writes -> gosec `G304`/`G703`/`G122` findings where gosec cannot infer
    Slipway's path authority.
- Blast radius:
  - Workflow-level blast radius is limited to the Security workflow.
  - Code-level blast radius includes every file currently reported by
    full-repository `gosec ./...`. Keep edits local to finding sites or shared
    helpers that remove repeated findings; do not perform unrelated rewrites.
- Constraints:
  - CodeQL and gosec must be additive security coverage.
  - `irreversible_operations` governance requires real SAST evidence at goal
    verification, no private attestation or force-pass.
  - User clarified that all current findings must be resolved, so full
    `gosec ./...` must be clean of unsuppressed findings before closeout.

### Patterns
- Existing conventions:
  - `.github/workflows/security.yaml` uses `actions/checkout@v6`,
    `actions/setup-go@v6`, SARIF artifacts via `actions/upload-artifact@v7`, and
    Code Scanning upload via `github/codeql-action/upload-sarif@v4`.
  - Go source verification uses `go test -count=1 ./...` and direct `go run .`
    lifecycle probes.
  - Runtime/governance path authority flows through `state.ResolveChangePaths`
    and artifact path helpers rather than ad hoc cwd-relative paths.
- Reusable abstractions:
  - Existing workflow SARIF upload steps can be mirrored for gosec.
  - Existing state/path helpers are the right place to justify or harden path
    reads/writes rather than adding new path parsing in command code.
- Convention deviations:
  - CodeQL does not produce a local SARIF file in normal GitHub Actions usage;
    its `analyze` action uploads results directly.
  - Full gosec SARIF should run without `-no-fail` after the baseline is fixed or
    locally suppressed, so CI can fail on future unsuppressed regressions.

### Risks
- Technical risks:
  - High: A gosec job that fails on the full current repository baseline would
    make the Security workflow red immediately on historical findings unless
    this change resolves every current finding first.
  - High: Suppressing `G122`/`G703` without a local rationale would preserve the
    untriaged baseline the issue exists to remove.
  - Medium: Broad permission changes may make tracked generated artifacts less
    usable; runtime/evidence files are safer candidates for private modes.
  - Medium: CodeQL autobuild can fail if the Go project needs a manual build
    path; current repo has a simple `go.mod` and should be compatible with Go
    setup plus CodeQL Go analysis.
  - Low: SARIF upload steps use `continue-on-error: true` elsewhere, so scanner
    result production and upload failures must be considered separately.
- Guardrail domains:
  - `irreversible_operations`, because this issue supports the governed
    `safety_baseline` requirement for irreversible-operation changes.
- Reversibility:
  - Workflow additions are reversible by removing the jobs.
  - Source suppressions/fixes are reversible by reverting the patch.
  - No data migration or archive finalization is part of this change.

### Test Strategy
- Existing coverage:
  - Go tests cover command and state behavior; they do not currently assert SAST
    workflow shape.
  - Existing Security workflow already proves the SARIF upload pattern for
    `govulncheck` and Trivy.
- Infrastructure needs:
  - Use gosec v2.27.1 locally through `go run
    github.com/securego/gosec/v2/cmd/gosec@v2.27.1` so the baseline is
    reproducible without installing a global binary.
  - Store final SAST outputs under the governed verification directory for
    goal-verification evidence.
- Verification approach:
  - Re-run full-repository gosec after triage to prove every current finding is
    fixed or intentionally suppressed with rationale.
  - Re-run full gosec SARIF generation to prove CI can publish full visibility
    with no unsuppressed findings.
  - Run `go test -count=1 ./...`.
  - Validate `.github/workflows/security.yaml` contains both gosec and CodeQL
    jobs.
  - Run governed `validate`, `health --governance`, goal verification, and final
    closeout until `done-ready`.

### Options
- Option A: CodeQL only.
  - Tradeoff: GitHub-native Go analysis with direct code scanning upload, but it
    does not triage the gosec rule IDs reported in issue #137.
- Option B: gosec only.
  - Tradeoff: Directly addresses the reported baseline and SARIF workflow gap,
    but misses the user's confirmed request to do both and leaves CodeQL absent.
- Option C: Both CodeQL and gosec, with issue-scoped gosec triage and full-repo
  SARIF visibility.
  - Tradeoff: Adds both requested SAST surfaces while keeping the mandatory
    triage focused on the issue-reported changed-package baseline. Full-repo
    gosec remains visible in Code Scanning but is not allowed to broaden this
    governed change into a repository-wide security rewrite.
- Option D: Both CodeQL and gosec, with full-repository gosec baseline
  resolution before enabling a failing CI gate.
  - Tradeoff: Larger code-touch surface because full `./...` currently reports
    136 findings, but it matches the user's clarified requirement that all
    current findings be resolved.
- Selected: Option D. User confirmed `both` and clarified "all" findings must be
  resolved. Current evidence shows full `./...` has 136 findings, so completion
  requires fixing or locally suppressing every current finding and enabling a
  gosec job that can fail on future unsuppressed findings.

## Unknowns
- Resolved: Which SAST engines should be added? -> User confirmed both gosec and
  CodeQL.
- Resolved: Does the current Security workflow already include Go-source SAST?
  -> No. It has `govulncheck`, Trivy, SBOM, and license jobs, but no gosec or
  CodeQL source analysis job.
- Resolved: Is the issue's baseline reproducible? -> Yes. Current changed-package
  gosec v2.27.1 scan reports 72 findings: `G122` x2, `G703` x1, `G304` x36,
  `G301` x18, `G204` x11, and `G306` x4.
- Resolved: Does a full-repo gosec gate expose more work? -> Yes. Full `./...`
  reports 136 findings, including additional `G101`, `G122`, and `G703`
  findings outside the issue's changed-package baseline.
- Resolved: Should the full-repo findings be left as visibility/follow-up? -> No.
  User clarified that all current findings must be resolved in this change.
- Remaining: None.

## Assumptions
- CodeQL should use the current repository's existing `security-events: write`
  permission and GitHub's supported `github/codeql-action/*@v4` action family -
  Evidence: `.github/workflows/security.yaml`; GitHub CodeQL action docs.
- Gosec should publish SARIF through Code Scanning using the official gosec
  SARIF flow - Evidence: securego/gosec documentation and the existing SARIF
  upload pattern in `.github/workflows/security.yaml`.
- Full-repo gosec findings outside the initially reported issue baseline are
  in scope - Evidence: user clarified that all current findings must be
  resolved.

## Canonical References
- `.github/workflows/security.yaml`
- `cmd/pivot_execution.go`
- `cmd/done.go`
- `internal/state/lifecycle.go`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
- `/tmp/slipway-issue137-gosec-changed-packages.json`
- `/tmp/slipway-issue137-gosec-full.json`
- `https://github.com/securego/gosec`
- `https://github.com/github/codeql-action`
- `https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/manage-your-configuration/codeql-for-compiled-languages`
