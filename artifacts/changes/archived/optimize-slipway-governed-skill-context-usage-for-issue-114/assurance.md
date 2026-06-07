# Assurance
## Project Context
- Tech Stack: Go
- Test Command: `go test ./...`
- Build Command: `go build ./...`
- Languages: Go

## Scope Summary
This change optimizes Slipway governed skill context usage for issue #114 by
making the heavy `goal-verification`, `worktree-preflight`, and
`wave-orchestration` host surfaces summary-first/thin-host while preserving their
hard gates, freshness/run-version contracts, runtime task evidence contract, and
guardrail safety-baseline fail-closed behavior.

During execution, the S2 Scope Contract surfaced a false-positive loop because
durable `artifacts/codebase/` discovery-context files were treated as execution
changed-file drift. The final scope includes the narrow internal sampler repair:
exclude `artifacts/codebase/` only from S2 execution-drift sampling while keeping
real implementation/test dirt and `done` dirty-worktree visibility intact.

A review-driven follow-up tightened the `worktree-preflight` template prose by
removing a redundant "In short:" restatement of the bounded-baseline contract,
with the matching `thin_host_content_test.go` assertion updated to the surviving
authoritative instruction. The edit invalidated the S3 review and S4
goal-verification certifications, which were re-run through the public Slipway
flow (auto-reopen to S3_REVIEW); both review stages and goal-verification were
re-certified fresh at the current run_version before this closeout.

## Verification Verdict
Current S4 verification state is execution-fresh after the review-driven prose
delta. Both review stages and goal-verification were re-certified at run_version
2; the remaining ship gate input is final-closeout evidence:
- `go vet ./...`: clean
- `git diff --check`: clean
- `go test ./internal/tmpl -count=1`: pass
- `go test ./internal/engine/progression -count=1`: pass
- `go test -count=1 ./...`: pass (23/23 test-bearing packages green, 0 failures)
- `slipway validate --json` (current-worktree binary): fresh execution evidence,
  `scope_contract.status=pass`, valid requirements/tasks contracts, and only the
  final-closeout/assurance attestation blockers before ship can advance.

## Evidence Index
- `verification/intake-clarification.yaml`: refreshed scope confirmation after
  adding REQ-007.
- `verification/research-orchestration.yaml`: refreshed research covering
  template context reduction and the Scope Contract sampler repair.
- `verification/plan-audit.yaml`: refreshed plan audit for seven requirements and
  six execution tasks.
- `verification/wave-plan.yaml`: generated six-task, four-wave execution plan
  with tasks_plan_hash
  `2605bfe020c1f7653917c6f706d8e845f8ce0b01ac4b30d35ed183bcb972c833`.
- `verification/wave-orchestration.yaml`: run_summary_version 2, all six tasks
  passed with runtime task evidence recorded through `go run . evidence task`.
- `verification/execution-summary.yaml`: overall verdict pass, completed
  t-01..t-06, evidence freshness fresh.
- `verification/spec-compliance-review.yaml`: Stage 1 review pass with
  `layer:R0=pass`, `scope_contract:pass`, and `negative_path:pass`.
- `verification/code-quality-review.yaml`: Stage 2 review pass with
  `layer:IR1=pass`.
- `verification/goal-verification.yaml`: S4 acceptance proof pass for the
  context-optimization criteria, Scope Contract status, stub scan, and fresh
  command references.

## Requirement Coverage
- REQ-001: `goal-verification` thin-host delegation plus fail-closed
  safety-baseline command-reference contract, covered by
  `TestThinHostGoalVerificationDelegatesBulkyEvidence`.
- REQ-002: `worktree-preflight` bounded baseline summary/output reference
  contract, covered by
  `TestThinHostWorktreePreflightKeepsOnlyBoundedBaselineSummary`.
- REQ-003: `wave-orchestration` coordinator no longer reads broad codebase-map
  bodies and preserves PR #112 self-check through executor-owned map metadata,
  covered by `TestThinHostWaveOrchestrationDelegatesCodebaseMapReads`.
- REQ-004/REQ-005/REQ-006: portable "when supported; otherwise" fallback,
  preserved hard-gate/run-version/evidence-task language, and regression tests
  against full-output-heavy host behavior are covered by
  `internal/tmpl/thin_host_content_test.go`.
- REQ-007: Scope Contract sampler excludes `artifacts/codebase/` only from S2
  execution drift while preserving real implementation dirt and `done` dirty
  advisories, covered by progression tests in
  `readiness_optimization_test.go` and `advance_test.go`.

## Residual Risks and Exceptions
- Generated `.codex/`, `.claude/`, `.cursor/`, or `.gemini/` surfaces are
  `.gitignore`d and not regenerated in this state. The source templates and
  embedded-template tests are the authority; generated-surface refresh can be
  performed via the repo-native init/refresh command if repository policy
  requires the per-tool copies updated.
- The Scope Contract exemption is intentionally narrow. It does not exempt
  implementation/test files, active governed bundles, or dirty-worktree advisory
  reporting at `done`.
- No runtime token-attribution telemetry or model-routing feature is included;
  those remain deferred ideas from the approved intent.

## Rollback Readiness
Rollback is straightforward: revert the template/test edits and the narrow
`readiness.go` sampler exemption. There is no data migration, artifact schema
change, CLI contract change, or persisted state migration.

## Archive Decision
Closeout-ready. The change is in S4_VERIFY with both review stages and
goal-verification re-certified fresh at run_version 2 after the prose delta.
Active `slipway validate --json` was captured fresh from the current-worktree
binary immediately before `done` and reports `scope_contract.status=pass` with no
remaining blockers other than this final-closeout attestation. Recording
final-closeout (`closeout:assurance_complete=pass`) satisfies the standard-preset
ship gate; `slipway done` then archives the bundle.
