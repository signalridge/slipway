# Assurance

## Scope Summary
This change completes the remaining `opt.md` section 1 lifecycle route-surface gap by adding the existing invocation route contract to successful single-change mutating JSON surfaces:

- `done --json`
- `evidence skill --json`
- `evidence task --json`
- `evidence task --result-file ... --result-file ... --json` batch output

The implementation reuses the existing `invocationRouteView` and command-local `stateReadContext`. It does not add compatibility shims for the previous missing-route output.

## Verification Verdict
Verdict: PASS.

Fresh ship-verification evidence for `run_summary_version: 1` was gathered after the selected S3 peer reviews converged. The terminal proof shows:

- Targeted mutating route/fail-closed tests passed, including root unscoped `done --json` against a change bound to another worktree.
- Existing P0 route, freshness, action, and host capability regression tests passed.
- `go test ./cmd -count=1` passed.
- `go test ./... -count=1` passed.
- `golangci-lint run ./... --timeout=5m` passed.
- `git diff --check` passed.
- Placeholder/stub scan found no hits in the touched command and test files.

## Evidence Index
- `verification/intake-clarification.yaml`
- `verification/research-orchestration.yaml`
- `verification/plan-audit.yaml`
- Runtime task evidence for `t-01` and `t-02`
- `verification/wave-orchestration.yaml`
- `verification/spec-compliance-review.yaml`
- `verification/code-quality-review.yaml`
- `verification/independent-review.yaml`
- `verification/lifecycle-surface-route-tests.md`
- `verification/logs/00-status-json.txt`
- `verification/logs/01-validate-json.txt`
- `verification/logs/02-go-test-cmd-mutating-routes.txt`
- `verification/logs/03-go-test-cmd-existing-route-regressions.txt`
- `verification/logs/04-go-test-cmd-full.txt`
- `verification/logs/05-go-test-all.txt`
- `verification/logs/06-golangci-lint.txt`
- `verification/logs/07-git-diff-check.txt`
- `verification/logs/08-placeholder-scan.txt`

## Requirement Coverage
- REQ-001: Covered by route fields added to `doneView`, `evidenceSkillView`, `evidenceTaskView`, and `evidenceTaskBatchView`, plus targeted route-output tests.
- REQ-002: Covered by explicit missing slug, root bound-elsewhere, archived target, no-active, and wrong-state fail-closed regression tests.
- REQ-003: Covered by direct top-level `invocation_route` fields; no compatibility envelope or legacy adapter was introduced.
- REQ-004: Covered by targeted command tests, existing P0 contract tests, `go test ./cmd -count=1`, `go test ./... -count=1`, lint, diff check, and placeholder scan.

## Residual Risks and Exceptions
- The legacy top-level `evidence_freshness` field remains because earlier changes already introduced split freshness fields without removing legacy output. This change does not add a new compatibility layer.
- The new `invocation_route.next_command` for successful `done` reflects pre-archive target context. Consumers should treat it as route context, not a post-archive action recommendation.
- Verification logs under `verification/logs/` are runtime proof and are intentionally not part of the final committed source/archive surface.

## Rollback Readiness
Rollback is a normal source revert of the command/view/test changes plus rerunning:

```bash
go test ./cmd -count=1
go test ./... -count=1
```

No schema migration, credential, irreversible state mutation, or external API contract change is introduced.

## Archive Decision
Ready to archive after recording ship-verification evidence and capturing a final active `validate --json --change repair-lifecycle-surface-contracts` proof.

Rationale: selected S3 peer reviews passed, fresh terminal verification passed, scope contract is pass, and no blocking residual risk remains.
