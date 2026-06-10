# Architecture

Re-authored for change `resolve-github-issue-137-add-go-source-sast-ci-and-triage-th`
(issue #137). This map scopes to adding Go-source SAST coverage and triaging the
reported gosec baseline.

- Module responsibilities:
  - `.github/workflows/security.yaml` — existing security CI authority. It
    currently runs `govulncheck`, Trivy filesystem scan, SBOM generation, and
    license reporting with SARIF uploads for vulnerability scanners, but it has
    no Go-source static application security testing job.
  - `cmd/pivot_execution.go` — contains the reported `G703`/`G306` path/write
    finding in `clearApprovedSummaryForPivot`, which rewrites `intent.md` under
    the governed bundle resolved by `state.ResolveChangePaths`.
  - `internal/state/lifecycle.go` — contains archive/copy helpers and the
    reported `G122`/`G301`/`G304` findings around recursive bundle copy and
    archive migration.
  - `cmd/done.go` — contains the reported `G122`/`G304` finding while scanning
    governed-bundle remediation references during finalization.
  - `internal/state`, `cmd`, `internal/model`, `internal/toolgen`, and
    `internal/tmpl` — the issue's changed-package gosec baseline scope.
- Dependency flow:
  - CLI commands resolve project/change paths through `state.ResolveChangePaths`
    before reading/writing governed artifacts.
  - Security workflow jobs use GitHub Actions permissions at the workflow level
    (`actions: read`, `contents: read`, `security-events: write`) and upload
    SARIF through `github/codeql-action/upload-sarif@v4`.
  - Gosec and CodeQL should remain CI/reporting additions; they must not alter
    Slipway runtime lifecycle behavior.
- Coupling hotspots:
  - Path-handling fixes in archive/finalization code can affect `slipway done`,
    archive migration, and governed bundle recovery.
  - Broad permission changes from `0755`/`0644` to private modes can change
    whether generated user-facing artifacts remain inspectable; permission
    triage must distinguish repository artifacts from git-scoped runtime state.
  - Adding CodeQL to the existing Security workflow shares the same
    `security-events: write` permission used by current SARIF upload jobs.
- Current change blast radius:
  - Expected implementation files: `.github/workflows/security.yaml`, targeted
    Go files carrying full-repository gosec findings, and local triage comments
    or narrow suppressions for every current unsuppressed full-repository
    finding.
  - Expected governed files: this codebase map, `research.md`,
    `requirements.md`, `decision.md`, `tasks.md`, `assurance.md`, and
    verification evidence under the active bundle.
- Notes / source references:
  - `.github/workflows/security.yaml`
  - `cmd/pivot_execution.go`
  - `cmd/done.go`
  - `internal/state/lifecycle.go`
  - `/tmp/slipway-issue137-gosec-changed-packages.json`
  - `/tmp/slipway-issue137-gosec-full.json`
