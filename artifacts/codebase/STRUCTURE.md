# Structure

Re-authored for change `resolve-github-issue-137-add-go-source-sast-ci-and-triage-th`
(issue #137). This map scopes to Go-source SAST CI and full-repository gosec
baseline triage.

- Directory layout:
  - `.github/workflows/` owns repository CI workflow definitions. The Security
    workflow is the only workflow expected to change.
  - `cmd/` contains Cobra command implementations that resolve governed paths,
    run git commands, and read/write lifecycle artifacts.
  - `internal/state/` owns Slipway state path resolution, archive/copy helpers,
    worktree/git subprocess helpers, and local runtime path handling.
  - `internal/engine/` owns governance, artifact, intake, progression, status,
    and skill-loading flows that intentionally read repository/governed paths.
  - `internal/model/`, `internal/toolgen/`, `internal/tmpl/`, and `internal/fsutil/`
    contain additional full-repository gosec findings that must be triaged.
- Entry points:
  - `main.go` and `cmd/*` are CLI execution entry points.
  - `.github/workflows/security.yaml` is the CI entry point for SAST coverage.
  - `go.mod` controls the Go module used by Actions setup and local tests.
- Generated versus handwritten boundaries:
  - `internal/tmpl/templates/**` are source templates and may carry findings in
    test fixture/template loading code; generated copies outside these sources
    are not the authoring surface for this change.
  - `artifacts/changes/*` and `artifacts/codebase/*` are governed planning,
    review, and evidence artifacts. Verification outputs for this change live
    under the active bundle's `verification/` directory.
- Ownership hints:
  - Workflow SARIF upload conventions live in `.github/workflows/security.yaml`
    and should be mirrored for gosec.
  - Runtime path authority lives in `internal/state` helpers; call-site
    suppressions should cite that authority instead of introducing parallel path
    parsing.
  - Git subprocess findings should stay local to `cmd/`, `internal/state/`, and
    `internal/fsutil/` call sites with bounded executable/argument rationale.
