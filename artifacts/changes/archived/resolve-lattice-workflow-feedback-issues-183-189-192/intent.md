# Intent

## Summary
Resolve the three current open Lattice-reported Slipway workflow feedback issues:
#183, #189, and #192.

## Complexity Assessment
complex

Rationale: the change touches public CLI JSON/error/remediation surfaces and
governed evidence path semantics across bound worktrees, S0 handoff, and
post-S2 verification workflows. The intended implementation remains narrow and
test-led.

## Guardrail Domains
External API contracts: additive/behavioral CLI JSON and error-remediation
surface changes.

## In Scope
- `cmd/evidence.go`: make `slipway evidence skill --notes-file artifacts/...`
  resolve workspace-relative notes against the active change's authoritative
  workspace/bound worktree, not always the root checkout.
- `cmd/evidence.go`: make post-S2 wrong-state evidence errors for task evidence
  and `wave-orchestration` evidence point to the correct S3/S4 verification
  surfaces instead of implying S2 evidence can or should be refreshed.
- `cmd/next.go` and `cmd/next_context_build.go`: make skill-handoff hard stops
  and `--resume-response` without an active checkpoint state clearly that
  `--resume-response` is only for active checkpoints and is not a bridge for
  missing governance skill evidence.
- Focused regression tests covering #183, #189, and #192 user-visible behavior.

## Out of Scope
- Do not modify the Lattice repository.
- Do not weaken fresh confirmation requirements for `intake-clarification` or
  any other governance skill handoff.
- Do not add a new lifecycle state, evidence type, bypass flag, or force-close
  path.
- Do not redesign the broader checkpoint/resume mechanism beyond the precise
  remediation and action-surface clarity needed for these issues.

## Constraints
- Preserve existing JSON field names and error codes unless the change is
  additive or strictly improves remediation prose.
- Keep the fix fail-closed: post-review repairs must be captured through S3/S4
  review and verification evidence, not by mutating stale S2 task evidence.
- Keep path handling workspace-relative and reject absolute paths or traversal.

## Acceptance Signals
- A regression test proves `--notes-file artifacts/...` reads from the bound
  worktree/authoritative workspace for a worktree-bound change.
- Regression tests prove S3/S4 wrong-state errors mention the correct
  replacement verification surfaces (`spec-compliance-review`,
  `code-quality-review`, `goal-verification`, `final-closeout`) for post-S2
  repairs.
- Regression tests prove `confirmation_requirement` / `--resume-response`
  surfaces distinguish skill handoff from checkpoint resume.
- Focused package tests for the touched command behavior pass.
- Repository-level Go tests pass before claiming done-ready.

## Open Questions
None

## Deferred Ideas
- A first-class post-review evidence refresh command can be reconsidered later,
  but this change only fixes the current CLI guidance and accepted replacement
  evidence path.
- Delegated-autonomy policy for S0 intake can be reconsidered later, but this
  change does not alter hard-stop confirmation semantics.

## Approved Summary
User confirmed continuation on 2026-06-12 after reviewing the proposed scope.
This change fixes Slipway CLI/evidence workflow surfaces reported from Lattice
issues #183, #189, and #192. It is limited to bound-worktree notes-file path
authority, post-review evidence remediation clarity, and checkpoint-vs-skill
handoff guidance. It explicitly excludes Lattice code changes, bypass paths, and
weakening fresh confirmation requirements. Primary acceptance is focused
regression coverage plus passing Go tests.
