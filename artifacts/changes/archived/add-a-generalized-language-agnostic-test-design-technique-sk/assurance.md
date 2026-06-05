# Assurance
## Project Context
- Tech Stack: Go
- Conventions: catalog-skill binding-compare gate; deterministic toolgen; checkbox-native tasks.md
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Deliver issue #73 Parts A+B+C: (A) a generalized, language-agnostic `test-design`
catalog skill (registry + templates + allowlist); (B) a per-language
`capability:language-testing` technique hint that routes the host to its own
installed language testing skill; (C) a Dispatch-Contract rule in
`wave-orchestration` (plus a `tdd-governance` RED-proof note) that isolates the
test-authoring step from implementation. Non-goals: vendoring language-specific
testing skills; changing `tdd-governance`/`coverage-analysis` semantics; binding
the hints at `goal-verification`; any wave-plan model/hash/evidence change.

## Verification Verdict
Implementation, review, and S4 verification pass on the active worktree. The
change is additive and is verified by the Part A registry/template/toolgen gates,
the Part B `cmd` language-hint tests, the Part C rendered-template isolation
tests, fresh full-suite commands, and governed review records. Before `done`, the
final-closeout record must cite an active `go run . validate --json` result for
this same bundle so archive/ship readiness is based on live state, not archived
records.

## Evidence Index
Primary execution evidence:
- `verification/wave-orchestration.yaml` plus task evidence `task-001` through
  `task-007`, including the RED commits for template/content tests and Part B
  language-hint tests, the GREEN implementation commits, and the final
  `go build ./...` / `go test -count=1 ./...` task evidence.
- `verification/spec-compliance-review.yaml`: requirements-to-code trace passes,
  including scope-contract recovery for `internal/engine/capability/resolver_test.go`.
- `verification/code-quality-review.yaml`: independent quality review passes with
  focused tests, `git diff --check`, and placeholder/secrets scans classified.
- `verification/goal-verification.yaml`: S4 acceptance criteria verification
  against REQ-001 through REQ-012, fresh build/full-suite output, and target-file
  stub scan.
- `verification/final-closeout.yaml`: final ship-gate refresh, assurance
  attestation, active `validate --json` proof, and full-suite confirmation.

## Requirement Coverage
- REQ-002, REQ-008, REQ-009, REQ-010 → task-001 (RED: generalized-only guard +
  Part A content + Part C rendered-prose contract tests, authored before the
  templates/prose exist)
- REQ-001, REQ-002, REQ-003, REQ-006 → task-002 (atomic Part A registry +
  templates + allowlist + fixtures + golden; binding-compare gate satisfied
  in-task; turns task-001's Part A assertions GREEN)
- REQ-008, REQ-009 → task-003 (Part C host-template prose; turns task-001's
  Part C assertions GREEN)
- REQ-004, REQ-005, REQ-006, REQ-012 → task-004 (RED: Part B language-hint tests,
  incl. the polyglot two-hint case)
- REQ-004, REQ-005, REQ-012 → task-005 (Part B impl GREEN)
- REQ-007 → task-006 (CLAUDE.md contract, multi-hint resolution)
- REQ-011 → task-007 (final build + full test suite)
Every REQ maps to at least one task; each part is test-first (its contract tests
precede its implementation); verification evidence is attached per task.

## Residual Risks and Exceptions
- Part C is host-enforced (SKILL.md prose), not engine-rejected — accepted per
  the user-selected Option A; reviewers must confirm the wording does not
  over-claim enforcement, and that it encodes only orchestration isolation
  (layer ①) without duplicating the Part A behavior-vs-implementation judgment
  (layer ②).
- Per-language hint emission is the formal contract (issue #73's "at most one"
  wording is stale): one hint per detected language, pinned by task-004's polyglot
  two-hint test; Go-only changes still emit exactly one hint.
- The `test-design` technique hint binds to `wave-orchestration` only.
  `tdd-governance` is `ExportOnlyExtra` and never resolves as a next-skill, so a
  hint bound there would be a production-dead binding; scoping to
  `wave-orchestration` avoids it (research-decided). Part C's `tdd-governance`
  change ships as SKILL.md prose, reaching the host via SKILL.md export.
- Codebase map is stale advisory context (describes the prior create-guard
  change); the plan's source-backed context is research.md's file:line citations.
  Recorded as a non-blocking advisory gap in plan-audit; not relied upon.
- Deterministic toolgen: the new exported skill changes the generated skill tree;
  the `skill_tree_inventory.codex.golden` manifest is regenerated and re-asserted
  in task-002 (the atomic Part A code task).

## Rollback Readiness
Additive change with no persisted schema/hash/evidence-format change. Rollback =
revert the PR; no migration or data repair required. Per-part revert is also
clean (each part is isolated to its own files). Verification after rollback:
`go build ./...` && `go test -count=1 ./...`.

## Archive Decision
Archive after final-closeout passes and before/through `slipway done`; do not
archive from stale review or task evidence alone. The archive rationale is that
the change is additive, every REQ maps to a completed task and review record,
the fresh build/test suite is green, scope contract is pass, and final-closeout
must capture active `go run . validate --json` proof immediately before `done` so
the frozen bundle records live ship readiness.
