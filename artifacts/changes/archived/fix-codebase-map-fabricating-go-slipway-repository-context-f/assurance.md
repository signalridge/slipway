# Assurance
## Project Context
- Tech Stack: Go
- Conventions: Cobra CLI; cmd/ surfaces, internal/state durable state, internal/engine workflow logic
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Resolve issue #17: (1) make `slipway codebase-map` language-agnostic and stop
fabricating Go/Slipway context; (2) refresh old deterministic Go/Slipway generated
map docs already written into non-Go repos without overwriting authored maps; (3)
add a `baseline` status so generated facts are not mistaken for authored analysis;
(4) make next/run active-change discovery from the repo root self-explanatory and
lock `--change <slug>` worktree resolution from any directory; (5) make status and
repair handle empty stale root active-bundle residue after a worktree-bound `done`
without blocking closeout; (6) make `validate --change <archived-slug>` fail with
a concrete archived-change diagnostic instead of an empty no-active view. No new
config field; git + change.yaml are the source of truth.

## Verification Verdict
Pass. S2 execution, S3 spec/code-quality review, and S4 goal-verification completed
with fresh evidence. Final signals: separate RED contract evidence before production
changes, a late S4 RED/green micro-loop for Go single-line `require` parsing,
`go build ./...` green, `go test -count=1 ./...` green, focused cross-package coverage
green, and issue #17 regression tests pass.

## Evidence Index
- intent.md (approved scope), research.md (evidence-backed findings), decision.md (selected approach)
- verification/intake-clarification.yaml, verification/research-orchestration.yaml, verification/plan-audit.yaml
- verification/tdd-governance.yaml, verification/wave-orchestration.yaml,
  verification/spec-compliance-review.yaml, verification/code-quality-review.yaml,
  verification/goal-verification.yaml

## Requirement Coverage
- REQ-001, REQ-002, REQ-003, REQ-004 -> t-00a, t-01, t-03, t-05
- REQ-005, REQ-006 -> t-00b, t-02, t-04, t-06
- REQ-007 -> t-00a, t-00b, t-08
- REQ-008 -> t-05, t-06, t-10, t-08
- REQ-009 -> t-07
- REQ-010 -> t-08
- REQ-011, REQ-012 -> t-00c, t-09, t-10
- REQ-013 -> t-00d, t-06, t-07, t-11, t-08

## Residual Risks and Exceptions
- `internal/state/stats.go` freshness stays coarse/modtime-based (import-cycle boundary) - accepted exception.
- Existing generated baseline docs are refreshed only when they match the old deterministic
  Go/Slipway generated shape. Generic authored map refresh remains out of scope.

## Rollback Readiness
Pure code change; no data migration. Rollback = revert the PR/commit. Verification after rollback:
`go build ./... && go test ./...`. Generated `artifacts/codebase/*` are advisory and regenerated on demand.

## Archive Decision
Pending explicit `slipway done`. Goal-verification passes and the change is ready for
the final lifecycle transition.
