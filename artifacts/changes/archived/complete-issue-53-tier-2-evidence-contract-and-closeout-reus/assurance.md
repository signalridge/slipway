# Assurance
## Project Context
- Tech Stack: Go CLI, Cobra, Slipway governance runtime
- Conventions: command metadata in `internal/toolgen`, command wiring in `cmd/root.go`, compact YAML verification records, flat runtime task evidence JSON
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Issue #53 Tier 2 is complete within the approved scope. The shipped behavior adds a first-class `slipway evidence task` command for runtime task evidence, routes wave-orchestration guidance through that command, keeps repair from fabricating missing source task evidence, and adds explicit final-closeout reuse checks for fresh goal-verification proof. Tier 1 workflow fixes and Tier 3 stale-planning recovery remain outside this change.

## Verification Verdict
The change is ready for closeout short of the final `slipway done` archive step. Focused tests, full workspace tests, build, vet, diff whitespace checks, stub scan, scope-contract validation, spec-compliance review, code-quality review, and goal-verification all pass for run version 1. The only intentional stop line is the user's instruction to stop before final archive/close.

## Evidence Index
- Intake: `verification/intake-clarification.yaml`
- Research: `verification/research-orchestration.yaml`
- Plan audit: `verification/plan-audit.yaml`
- Wave plan: `verification/wave-plan.yaml`
- Wave orchestration: `verification/wave-orchestration.yaml`
- Execution summary: `verification/execution-summary.yaml`
- Spec compliance review: `verification/spec-compliance-review.yaml`
- Code quality review: `verification/code-quality-review.yaml`
- Goal verification: `verification/goal-verification.yaml`
- Runtime task ledger: `.git/slipway/runtime/changes/complete-issue-53-tier-2-evidence-contract-and-closeout-reus/evidence/tasks/*.json`

## Requirement Coverage
- REQ-001: covered by tasks `t-01`, `t-02`, and `t-07`; implemented by `cmd/evidence.go`, command wiring, toolgen metadata, docs, and command regression.
- REQ-002: covered by tasks `t-01`, `t-02`, and `t-07`; command validation computes `freshness_inputs` and rejects invalid verdicts without writing evidence.
- REQ-003: covered by tasks `t-03` and `t-07`; wave-orchestration guidance requires `slipway evidence task` and forbids manual runtime task JSON.
- REQ-004: covered by tasks `t-04` and `t-07`; repair reports missing source task evidence with the supported command hint and does not fabricate summaries.
- REQ-005: covered by tasks `t-05`, `t-06`, and `t-07`; final-closeout reuse is explicit, run-version-bound, freshness-bound, and timestamp-ordered against goal-verification.
- REQ-006: covered by tasks `t-02` through `t-07`; focused regressions, template checks, docs, full verification, and review records are present.

## Residual Risks and Exceptions
- Runtime task evidence refresh remains limited to S2 through the public command. If `tasks.md` changes after S2, the governed path is to rerun wave execution; the temporary local evidence-refresh helper used during this session was deleted and is not part of the diff.
- `go test` output contains cached package results for some unchanged packages during full-suite verification; focused changed-package tests were rerun after review fixes and a focused coverage profile was generated.
- Tier 3 stale-planning recovery and broader lifecycle rewind behavior are explicitly out of scope.

## Rollback Readiness
Rollback is a normal code revert of the command, template, authority, docs, and tests touched by this change. Runtime task evidence compatibility is preserved because `slipway evidence task` writes the same flat JSON shape already consumed by `progression.ParseTaskEvidence`; removing the new command does not require data migration for existing execution summaries.

## Archive Decision
Do not run `slipway done` in this session. The governed change should stop at close-ready or done-ready, with active `validate --json` proof captured before the final archive step. Archive can proceed later by running `slipway done` only after the user explicitly authorizes that final close action.
