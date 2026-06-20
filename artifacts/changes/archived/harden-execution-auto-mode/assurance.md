# Assurance

## Scope Summary

This change hardens the existing `execution.auto` behavior without expanding the
public run or next JSON API surface. It addresses the confirmed audit,
fail-closed, regression-pin, and blocker-precedence gaps for auto mode:

- auto-resolved eligible `human_verify` checkpoints now carry an explicit
  `auto_checkpoint_acknowledged` lifecycle side effect that is derived from a
  non-user-controlled auto signal, not from response text alone
- `slipway learn` separates manual and auto checkpoint-resolution counts so
  auto acknowledgments no longer inflate the manual human-verification signal
- skill handoff auto-softening is now an explicit pure-pacing allowlist, with
  `security-review`, `worktree-preflight`, and unknown or unlisted skills
  remaining hard stops
- README, generated run-command, and run prompt safety redlines are pinned by
  tests
- auto-off non-pacing governance blockers are pinned to outrank review-batch and
  skill-handoff pacing prompts

No guardrail domain is active for this change, and no schema migration or data
backfill is required.

## Verification Verdict

Verdict: pass, pending final-closeout recording.

Fresh verification for run summary version 1 passed:

- `go test ./...`
- `go build ./...`
- `git diff --check`
- target-file placeholder scan, with only intentional template-test fixture
  markers found
- `go run . validate --json`, reporting fresh evidence and
  `scope_contract.status: pass`

S3 reviewer evidence is recorded and stamped for:

- `spec-compliance-review`: pass, `layer:R0=pass`
- `code-quality-review`: pass, `layer:IR1=pass`
- `independent-review`: pass
- `goal-verification`: pass, with
  `fresh:command_ref=artifacts/changes/harden-execution-auto-mode/verification/full-suite-proof.md`

## Evidence Index

- Full suite proof:
  `artifacts/changes/harden-execution-auto-mode/verification/full-suite-proof.md`
- Suite keystone:
  `artifacts/changes/harden-execution-auto-mode/verification/suite-result.yaml`
  with full suite digest
  `sha256:51b21885ae1d883786ed7807742a9f98dc91545a7b3a029962a77e3df5c46671`
- Spec compliance:
  `artifacts/changes/harden-execution-auto-mode/verification/spec-compliance-review.yaml`
  and
  `artifacts/changes/harden-execution-auto-mode/verification/spec-compliance-review-notes.md`
- Code quality:
  `artifacts/changes/harden-execution-auto-mode/verification/code-quality-review.yaml`
  and
  `artifacts/changes/harden-execution-auto-mode/verification/code-quality-review-notes.md`
- Independent review:
  `artifacts/changes/harden-execution-auto-mode/verification/independent-review.yaml`
  and
  `artifacts/changes/harden-execution-auto-mode/verification/independent-review-notes.md`
- Goal verification:
  `artifacts/changes/harden-execution-auto-mode/verification/goal-verification.yaml`
  and
  `artifacts/changes/harden-execution-auto-mode/verification/goal-verification-notes.md`
- Wave execution evidence:
  `artifacts/changes/harden-execution-auto-mode/verification/wave-orchestration.yaml`
  plus task evidence for `t-01` through `t-05`

## Requirement Coverage

- REQ-001, Auditable Auto Checkpoint Resolution: covered by
  `cmd/run.go`, `cmd/stage.go`, `cmd/next_context_build.go`, `cmd/next.go`, and
  `cmd/auto_mode_test.go`. The real `run --auto` path emits
  `auto_checkpoint_acknowledged`; manual response text equal to
  `auto-acknowledged` remains manual.
- REQ-002, Learn Separates Manual And Auto Checkpoints: covered by
  `cmd/learn.go` and `cmd/learn_test.go`. `checkpoint_resolved_manual` and
  `checkpoint_resolved_auto` are separate, while `checkpoint_resolved` remains
  the total.
- REQ-003, Skill Auto Softening Is Explicitly Allowlisted: covered by
  `internal/engine/progression/confirmation_boundaries.go`, `cmd/next.go`, and
  `cmd/auto_mode_test.go`. All current pure-pacing skills still soften under
  auto; `security-review`, `worktree-preflight`, and unlisted skills hard-stop.
- REQ-004, Auto Mode Safety Text Is Regression Pinned: covered by
  `internal/toolgen/toolgen_test.go` and `internal/tmpl/templates_test.go`.
  README, run registry, and generated run prompt surfaces are checked for the
  safety redlines around guardrail/sensitive confirmations, `security-review`,
  decision and human-action checkpoints, stale or unknown freshness, and
  evidence gates.
- REQ-005, Governance Blockers Precede Handoff Pacing: covered by
  `cmd/next.go` and `cmd/auto_mode_test.go`. Auto-off views with non-pacing
  blockers now assert `blocked_by_governance` ahead of skill-handoff and
  review-batch pacing prompts.

## Residual Risks and Exceptions

- Historical lifecycle events are not backfilled with the new auto side effect.
  The change is intentionally forward-only; old events remain readable and are
  still counted as they were recorded.
- Unknown future skills now hard-stop under auto until added to the pure-pacing
  allowlist. This is an intentional fail-closed behavior change for pacing, not
  an evidence-gate change.
- README text did not require an edit. It is included in scope because the new
  tests pin its existing redline semantics.
- No guardrail domain is active, so no SAST safety-baseline token is required.

## Rollback Readiness

Rollback is straightforward: revert the implementation, test, and governed
artifact changes from this branch, then rerun:

- `go test ./cmd ./internal/toolgen ./internal/tmpl`
- `go build ./...`

The lifecycle side effect is optional metadata on new events. Removing the code
does not require migration and does not make existing lifecycle logs
unreadable. The learn signal additions are additive JSON fields; rollback
returns the prior aggregation behavior.

## Archive Decision

Archive readiness decision: ready after final-closeout passes and the active
change gate reports no blockers.

Active `validate --json` proof was captured in S3 before final-closeout. At the
time this assurance was authored, it reported fresh evidence, valid
requirements/tasks/decision contracts, and `scope_contract.status: pass`; the
remaining blockers were the expected final-closeout and assurance attestations.
Final-closeout must refresh `validate --json` before `done` and record
`closeout:assurance_complete=pass` and
`closeout:reviewer_independence=pass`. Archived bundles should be treated as
frozen records after `done`, not as active validation inputs.
