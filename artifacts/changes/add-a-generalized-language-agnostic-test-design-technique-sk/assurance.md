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
Pending S2 execution and S4 verification. Planned gates: binding-compare +
size-budget + golden-manifest gates (Part A), `cmd` hint tests incl. per-language
emission, ProjectContext-over-STACK precedence, disabled_controls independence,
and next/run parity (Part B), template-content + generalized-only guard tests
(Part A/C), and a final `go build ./...` + `go test -count=1 ./...`.

## Evidence Index
To be populated during execution from `verification/*.yaml` and task evidence:
- `verification/wave-orchestration.yaml` + per-task evidence (task-001..task-007)
- `verification/spec-compliance-review.yaml`, `code-quality-review.yaml`
- `verification/goal-verification.yaml`, `final-closeout.yaml`

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
Pending. Archive readiness will be recorded at closeout after an active
`validate --json` freshness/readiness proof is captured before `done`. This
bundle is not yet revalidated through the active validate gate.
