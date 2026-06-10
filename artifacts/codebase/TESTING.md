# Testing

Re-authored for change `resolve-github-issue-137-add-go-source-sast-ci-and-triage-th`
(issue #137). This map scopes to SAST workflow coverage and gosec baseline
triage.

- Test layout:
  - Go unit tests live in package-local `*_test.go` files.
  - Workflow changes are YAML-only and should be validated by static inspection,
    current GitHub Actions conventions, and branch/PR CI after push.
- Baseline security scans:
  - Changed-package baseline command used during research:
    `go run github.com/securego/gosec/v2/cmd/gosec@v2.27.1 -fmt=json -out=/tmp/slipway-issue137-gosec-changed-packages.json ./cmd ./internal/state ./internal/model ./internal/toolgen ./internal/tmpl`
  - Current changed-package result: 72 findings across `G122`, `G703`, `G304`,
    `G301`, `G204`, and `G306`.
  - Full-repo visibility command used during research:
    `go run github.com/securego/gosec/v2/cmd/gosec@v2.27.1 -fmt=json -out=/tmp/slipway-issue137-gosec-full.json ./...`
  - Current full-repo result: 136 findings. User clarified that all current
    findings must be resolved in this change, so final gosec verification is
    full-repository.
- Verification commands expected before closeout:
  - `go test -count=1 ./...`
  - `go run github.com/securego/gosec/v2/cmd/gosec@v2.27.1 -fmt=json -out=<path> ./...`
  - `go run github.com/securego/gosec/v2/cmd/gosec@v2.27.1 -fmt=sarif -out=<path> ./...`
  - `go run . validate --json`
  - `go run . health --governance --json`
- Coverage hotspots:
  - `cmd/done.go` and `internal/state/lifecycle.go` for symlink/WalkDir
    high-severity findings.
  - `cmd/pivot_execution.go` and `internal/engine/intake/intake.go` for gosec
    taint/path write findings in full-repo scans.
  - `.github/workflows/security.yaml` for gosec/CodeQL job configuration and
    SARIF/code-scanning upload behavior.
- Residual risk:
  - Local CodeQL analysis is not normally runnable without the GitHub CodeQL
    action environment; verification is workflow-structure inspection plus CI
    execution after push.
