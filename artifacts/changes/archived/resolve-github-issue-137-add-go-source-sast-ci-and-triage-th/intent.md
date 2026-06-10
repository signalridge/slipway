# Intent

## Summary
Resolve GitHub issue #137: add Go-source SAST CI and triage the gosec baseline findings until done ready

## Complexity Assessment
complex

This is complex because it touches CI security gates, repository-wide source
analysis, and the `irreversible_operations` safety-baseline evidence path. It
also requires distinguishing real security fixes from intentional baseline
suppressions with written rationale.

## Guardrail Domains
irreversible_operations

## In Scope
- Add static application security testing gates for Go source to
  `.github/workflows/security.yaml` using both gosec and CodeQL, including SARIF
  artifact upload and Code Scanning upload behavior consistent with the existing
  security jobs.
- Establish a triaged baseline for the issue #137 gosec findings, prioritizing
  the reported HIGH findings and the current full-repository gosec baseline:
  - `G703` in `cmd/pivot_execution.go`.
  - `G122` in `internal/state/lifecycle.go`.
  - `G122` in `cmd/done.go`.
- Resolve every current unsuppressed `gosec ./...` finding by fixing the code or
  adding a local, auditable suppression with rationale. Current full-repository
  baseline is 136 findings across `G101`, `G122`, `G703`, `G304`, `G301`,
  `G204`, and `G306`.
- Add source comments, `#nosec` suppressions, helper APIs, tests, or workflow
  checks only where they make the triage auditable and keep the CI gate useful.
- Record fresh governed evidence through planning, execution, security/domain
  review, goal verification, final closeout, and stop at `done-ready`.

## Out of Scope
- Do not run `slipway done` or archive/finalize the change unless the user
  explicitly asks after `done-ready`.
- Do not block unrelated issue #129 work on this pre-existing baseline.
- Do not broaden into unrelated linting, formatting, dependency update, SBOM, or
  Trivy/govulncheck redesign unless required to add the SAST gate safely.
- Do not suppress gosec findings without a local rationale tied to the specific
  call site or controlled input boundary.

## Constraints
- The workflow must remain useful on pull requests and `main`, and should retain
  SARIF upload parity with existing `govulncheck` and Trivy jobs.
- `irreversible_operations` governance must fail closed to explicit review,
  rollback notes, and fresh evidence; no force-pass or private attestation.
- Baseline triage must be reproducible from repo commands or CI configuration,
  not only from prose.
- Existing unrelated worktrees and archived bundles are not part of this change.

## Acceptance Signals
- `go test -count=1 ./...` passes from the governed worktree.
- The selected SAST commands run from the governed worktree or CI-equivalent
  configuration, and full-repository gosec has no unsuppressed findings.
- `.github/workflows/security.yaml` contains Go-source SAST jobs for both gosec
  and CodeQL that emit or upload SARIF/code scanning results.
- All current gosec findings from `gosec ./...` are fixed or carry auditable
  local suppressions with rationale, including all HIGH and MEDIUM families.
- `go run . validate --json` and `go run . health --governance --json` show the
  change is ready through `G_ship`, with `final-closeout` pass evidence and
  lifecycle state advanced to `done-ready`.

## Open Questions
- [x] User confirmation: implement the CI SAST gate with gosec, CodeQL, or both?
  The recommended intake choice is gosec because issue #137 reports gosec rule
  IDs and baseline counts. User confirmed `both`; subsequent blockers should be
  resolved by best engineering judgment when the local evidence is sufficient.

## Deferred Ideas
- None. User clarified that all current gosec findings should be resolved in
  this change, including full-repository findings outside the initially reported
  changed-package baseline.

## Approved Summary
Approved 2026-06-09T17:12:00Z. Resolve issue #137 under the
`irreversible_operations` guardrail by adding both gosec and CodeQL Go-source
SAST coverage to `.github/workflows/security.yaml`, resolving every current
unsuppressed full-repository gosec finding with a code fix or auditable local
suppression, and keeping the SAST gate useful for future regressions. Keep
unrelated security workflow redesign, dependency/SBOM changes, issue #129
finalization, and `slipway done` out of scope. Completion means fresh
tests/SAST evidence, full-repository gosec clean/triaged output, domain review,
rollback notes, governance validation/health proof, and lifecycle advancement to
`done-ready`.
