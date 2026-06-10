# Assurance

## Scope Summary
Delivered issue #137 by adding Go-source SAST coverage to the existing Security
workflow and resolving the current full-repository gosec baseline. The workflow
now runs both gosec and CodeQL for Go while preserving the existing govulncheck,
Trivy, SBOM, and license jobs. Source changes include local gosec triage
comments plus direct-symlink rejection for governed artifact reads/writes where
review reproduced a real bundle-boundary escape.

## Verification Verdict
PASS for the closeout evidence set. The full repository gosec JSON scan reports
zero unsuppressed findings and no Go processing errors. The gosec SARIF scan
produced a non-empty SARIF file. The full Go test suite passed. Current
`go run . validate --json` reports fresh evidence and `scope_contract.status=pass`.
S3 spec-compliance and code-quality reviews pass, and S4 goal verification
records the required `irreversible_operations.safety_baseline` high-risk check.

## Evidence Index
- `verification/gosec-full.json`: full repository gosec JSON scan with
  `Stats.found=0`, `Stats.nosec=131`, empty `Golang errors`, and zero issues.
- `verification/gosec.sarif`: full repository gosec SARIF scan output,
  non-empty at 1088 bytes.
- `verification/go-test.txt`: full `go test -count=1 ./...` output, passing.
- `verification/workflow-static-inspection.md`: static workflow inspection
  confirming gosec and CodeQL jobs, SARIF artifact upload, and Code Scanning
  upload wiring.
- `verification/wave-orchestration.yaml`: S2 task execution summary for
  workflow, HIGH finding triage, MEDIUM finding triage, SAST evidence, and Go
  test evidence.
- `verification/spec-compliance-review.yaml`: S3 spec compliance review with
  required `layer:R0=pass`, `layer:R3=pass`, `scope_contract:pass`, and
  `negative_path:pass` tokens.
- `verification/code-quality-review.yaml`: S3 code quality review with required
  `layer:IR1=pass` and `layer:IR3=pass` tokens.
- `verification/goal-verification.yaml`: S4 acceptance verification with fresh
  gosec/test/workflow evidence, `scope_contract:pass`, and
  `high_risk_check:irreversible_operations.safety_baseline=pass`.
- `go run . validate --json`: current validation reports
  `scope_contract.status=pass`, requirements/tasks/decision contracts valid,
  and evidence freshness `fresh`.

## Requirement Coverage
- REQ-001 is covered by `.github/workflows/security.yaml` and
  `verification/workflow-static-inspection.md`; the workflow contains both the
  `gosec` job and the `codeql` Go analysis job while existing jobs remain.
- REQ-002 is covered by `.github/workflows/security.yaml`,
  `verification/gosec.sarif`, and `verification/workflow-static-inspection.md`;
  gosec runs `./...`, writes SARIF, uploads the SARIF artifact, uploads Code
  Scanning results, and exits non-zero on future unsuppressed gosec failures.
- REQ-003 is covered by `verification/gosec-full.json`; the current
  full-repository gosec baseline has no unsuppressed findings.
- REQ-004 is covered by the no-symlink file helper and regression tests for the
  reproduced `G122`/`G703` artifact-boundary issue, local `#nosec` triage for
  the remaining HIGH false positive, and the zero-finding gosec report.
- REQ-005 is covered by local `#nosec` comments for MEDIUM families (`G304`,
  `G301`, `G204`, `G306`) and the zero-finding gosec report.
- REQ-006 is covered by fresh gosec evidence, fresh Go test evidence, this
  assurance record, S3 review records for spec compliance and code quality, and
  S4 goal verification with the irreversible-operations safety baseline token.

## Residual Risks and Exceptions
CodeQL was verified by static workflow inspection rather than local CodeQL
execution; the GitHub Action remains the runtime authority after push. The
largest residual risk is over-suppression: 131 gosec suppressions are present
because gosec cannot infer Slipway's governed bundle, repository-root, and
git-helper authority boundaries. The mitigation is local, rule-specific
rationale at each suppression, targeted code fixes where review proves a real
boundary problem, no global gosec exclusions, and a CI gosec job that fails on
future unsuppressed findings.

## Rollback Readiness
Rollback is a source-only and workflow-only revert. Revert the commit containing
the Security workflow jobs, source triage comments, no-symlink file helper, and
governed artifacts, then rerun `go test -count=1 ./...` and
`go run . validate --json`. No data migration, archive mutation, or `slipway
done` operation is required for rollback. If GitHub Actions exposes a CodeQL or
SARIF-upload configuration problem after push, revert the workflow portion or
fix it in a follow-up governed change before relying on the new SAST gate.

## Archive Decision
Archive after final-closeout records the assurance attestation and
`go run . validate --json` reports `done-ready`. The rationale is that all
issue #137 acceptance signals are backed by fresh active-worktree evidence:
workflow lint, full Go tests, gosec JSON/SARIF, review records, goal
verification, and scope-contract pass. Active validation proof must be captured
immediately before `go run . done --json`; the archived bundle is then a frozen
record, not an input to the active ship gate.
