# Assurance

## Scope Summary
GitHub issue #163 is implemented for Slipway decision artifacts. The change adds
a shared parsed decision contract in `internal/engine/artifact`, parses optional
decision lifecycle status, keeps status-free decisions compatible, rejects
explicit superseded/deprecated/rejected or unknown statuses, routes readiness
diagnostics through canonical reason and recovery surfaces, and prevents host
skill constraints plus the legacy locked-decision helper from building on dead
decisions. The parser now evaluates status-bearing sections in document order so
a live `## Status` section cannot mask a later dead or unknown `## Lifecycle` or
`## State` alias.

## Verification Verdict
PASS for the current post-review repair surface. The governed requirements,
tasks, and decision contracts validate; `scope_contract.status=pass`;
spec-compliance, code-quality, and goal-verification records are stamped passing
at run version 1 after the independent-review repair. The blocker was that
fixed heading-alias priority let an earlier live `## Status` value hide a later
dead or unknown `## Lifecycle` / `## State` value. The parser now scans all
status aliases in document order and fails closed when any explicit alias is
dead, unknown, or empty. Targeted parser/readiness/next-skill tests, affected
package tests, full `go test -count=1 ./...`, regenerated coverage, and
`git diff --check` pass after that repair.

## Evidence Index
- `go test -count=1 ./internal/engine/artifact`: pass after routing
  `ParseDecisionLockedDecisions` through `ParseDecisionContract`.
- `go test -count=1 ./internal/engine/artifact ./internal/engine/progression ./internal/model ./cmd`: pass.
- `go test -count=1 ./internal/engine/artifact ./internal/engine/progression ./cmd -run 'TestParseDecisionContractStatus|TestDecisionContractBlockers|TestParseDecisionItems|TestBuildSkillConstraintsLockedVsPending'`: pass with conflicting status aliases covered across parser, readiness, and next-skill constraints.
- `go test -count=1 ./internal/engine/artifact -run 'TestParseDecisionContractStatus|TestShouldRejectDecisionStatusNormalizationProperties|TestParseDecisionLockedDecisionsRejectsDeadStatus'`: pass with `Inactive`, `unaccepted`, `drafted`, empty status, mixed accepted/superseded, lowercase `## status`, punctuated `## Status:`, and uppercase closing-hash `## LIFECYCLE ##` regressions covered.
- `go test -count=1 ./internal/engine/progression -run TestDecisionContractBlockers`: pass with lowercase superseded status heading producing a readiness blocker.
- `go test -count=1 ./cmd -run 'TestParseDecisionItems|TestBuildSkillConstraintsIncludesDecisionState'`: pass with unusable decisions, including lowercase status headings, suppressed from pending and locked constraints.
- `go test -count=1 ./...`: pass after the post-review repair.
- `go test -count=1 -coverprofile=artifacts/changes/resolve-issue-163-decisions-gate/verification/coverage.out ./internal/engine/artifact ./internal/engine/progression ./internal/model ./cmd`: pass; total affected-package coverage 75.2%.
- `go tool cover -func=artifacts/changes/resolve-issue-163-decisions-gate/verification/coverage.out`: `ParseDecisionContract`, `parseDecisionStatus`, `isDecisionStatusHeading`, `selectDecisionStatus`, `DecisionContractBlockers`, and `parseDecisionItems` are 100.0% covered.
- `git diff --check`: pass.
- `go run . validate --json`: contracts valid, scope contract pass, freshness
  fresh; before final-closeout the only remaining blockers are the expected
  missing final-closeout record and assurance attestation.
- `verification/spec-compliance-review.yaml`: pass, `layer:R0=pass`.
- `verification/code-quality-review.yaml`: pass, `layer:IR1=pass`.
- `verification/goal-verification.yaml`: pass, AC-1 through AC-6 pass,
  coverage analysis pass, scope contract pass.
- `verification/execution-summary.yaml`: run summary version 1, tasks `t-01`
  through `t-04` pass.

## Requirement Coverage
REQ-001 is covered by `ParseDecisionContract`, selected decision extraction,
status explicitness fields, missing-status compatibility tests, and the host
constraint parser path.

REQ-002 is covered by rejected-status taxonomy, readiness blocker tests for
superseded decisions, next-skill tests for dead decision suppression, and the
legacy helper regression that returns no locked decisions for superseded status.

REQ-003 is covered by unknown explicit status parsing, canonical
`decision_status_unknown` diagnostics, recovery guidance, and tests proving
missing status remains non-blocking. The post-review repairs add explicit
coverage for `Inactive`, `unaccepted`, `drafted`, empty status, mixed
accepted/superseded status, and non-title-case status headings so live-status
substrings, heading casing variants, and conflicting status aliases cannot fail
open. Multiple live aliases remain compatible; any rejected, unknown, or empty
explicit alias wins over a live alias.

REQ-004 is covered by unit and property-style parser tests, readiness tests,
next-skill constraint tests, reason-code/recovery tests, targeted package tests,
heading-form regressions, and governed S3 review evidence.

## Residual Risks and Exceptions
No blocking exceptions remain. Missing status remains intentionally compatible
for existing authored decisions. Optional GSD-style issue-number decision
filenames and append-only dated amendments remain out of scope because they are
not required to prove the issue #163 fail-closed acceptance signal.

## Rollback Readiness
Rollback is a normal git revert of the decision parser, readiness tests,
next-skill constraint changes, reason-code/recovery additions, and governed
artifact updates. After rollback, rerun `go test -count=1 ./...`,
`git diff --check`, and `go run . validate --json` to confirm the previous
decision contract behavior and lifecycle gates are restored.

## Archive Decision
Ready for final-closeout and done-ready evaluation after this assurance artifact
validates. Active `validate --json` freshness/readiness proof must be captured
again before the change is considered done-ready; do not run `slipway done`
unless explicitly requested.
