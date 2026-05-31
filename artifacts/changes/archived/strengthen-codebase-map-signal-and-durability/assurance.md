# Assurance
## Project Context
- Tech Stack: Go
- Conventions: repo-native (`go build ./...`, `go test ./...`)
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Scope Summary
Strengthen codebase-map durability and freshness signal (issue #27): git-track
`artifacts/codebase` by default, surface `codebase_map_status` in the default
`next`/`run` handoff, and emit a non-blocking advisory when research/plan-audit
consume a `scaffold_only`/`baseline` map. Post-review hardening also migrates
the checked-in `.gitignore`, removes the retired `HasEmptyCodebaseMap` orphan,
uses the exact `scaffold_only` spelling in generated skill guidance, and adds a
direct `run --json` baseline advisory assertion. No state-machine or gate changes.

## Verification Verdict
Pending execution; **fresh plan-audit must pass before execution**. The stale
round-2 plan-audit remains invalidated (`fail`, run_version 1), while the
research-orchestration evidence has been refreshed against the round-3 bundle
(`pass`, run_version 2) after resolving the field-shape unknown and aligning the
cost / workspace-divergence findings across `research.md`, `decision.md`, and
`requirements.md`. Acceptance is met when the handoff reports
`codebase_map_status` on both `next` and `run`
(including `"missing"` with doc states present for the no-map case), a root
`--change` invocation is workspace-consistent (status and hint agree, no wrong-
checkout read), the gitignore migration drops the codebase line while keeping
evidence/events/verification ignored in both generated and checked-in blocks, the
advisory fires for `scaffold_only`/`baseline` consumed by research/plan-audit
(and not for `populated`/`partial`), the direct `run --json` surface carries the
baseline advisory/status contract, RED→GREEN evidence is on record per code
task, and `go build ./...` / `go test ./...` pass.

## Evidence Index
- `research.md` — discovery, file:line references, alternatives.
- `decision.md` — DEC-001..DEC-004 with REQ traceability.
- `verification/` — per-skill evidence (intake, research, plan-audit, reviews).
- Build/test: `go build ./...`, `go test ./...` (to be attached at goal
  verification).

## Requirement Coverage
(RED-first task graph; each behavior change is RED → GREEN → verify.)
- `REQ-001` (git-track maps) → DEC-001; t-01 (RED migration + ignored→trackable
  flip), t-02 (GREEN including checked-in `.gitignore` migration), t-11 (verify
  `git check-ignore` + retained patterns).
  Status: pending execution.
- `REQ-002` (handoff status field, both surfaces; no-map reports `"missing"`,
  not omitted) → DEC-002; t-03 (RED, asserts standard `next` view AND `run`/handoff
  projection next_handoff.go:216-221, plus the `missing` default), t-04 (GREEN),
  t-10 (verify). Status: pending execution.
- `REQ-003` (engine advisory matrix) → DEC-003; t-05 (RED matrix:
  missing/scaffold_only/baseline/partial/populated, branches on status, gated on
  research/plan-audit, no double-signal), t-06 (GREEN), t-12 (verify). Status:
  pending execution.
- `REQ-004` (template guidance) → DEC-003/DEC-004; t-07 (RED templates_test.go),
  t-08 (GREEN), t-12 (verify). Status: pending execution.
- `REQ-005` (docs) → DEC-004; t-09 (docs/commands.md, CLAUDE.md, codebase-mapping
  SKILL, plus README.md:198 and docs/operator-guide.md:15 "local-only by default"
  corrections; toolgen_test.go:1001 backtick string preserved). Status: pending
  execution.
- `REQ-006` (tests) → DEC-004; RED tasks t-01/t-03/t-05/t-07 (incl. the flipped
  `local_ignore_test.go` ignored→trackable assertion, both-surface status, and the
  `baseline` case `HasEmptyCodebaseMap` misses), direct `run --json` baseline
  advisory/status coverage, and t-13 (suite health). Status: pending execution.
- `REQ-007` (external_api_contracts guardrail) → DEC-004; additive-only
  `omitempty` contract, asserted by t-03/t-04 and verified by t-10, t-13. Status:
  pending execution.
- `REQ-008` (RED-first TDD discipline) → DEC-004; RED tasks t-01/t-03/t-05/t-07
  precede GREEN tasks t-02/t-04/t-06/t-08 in strictly earlier waves; t-13 verifies
  RED→GREEN evidence per code task. Status: pending execution.
- `REQ-009` (workspace-consistent single-source assessment) → DEC-002/DEC-003;
  t-04 (helper assesses `paths.WorkspaceRoot`), t-05 (RED: root `--change`
  consistency), t-06 (GREEN: re-source the hint from `codebase_map_status`, drop
  the `HasEmptyCodebaseMap(root, …)` probe and delete the retired helper/test),
  t-12 (verify). Status: pending execution.

## Residual Risks and Exceptions
- Auto-migration changes `git status` for repos with untracked maps (intended),
  and only fires on `slipway new`/`codebase-map`/`init` — not on every command.
  This repo's checked-in `.gitignore` is migrated directly by the change.
- New handoff fields must remain additive/`omitempty`; the `run`/handoff surface
  needs the field added to the projection literal (next_handoff.go:216-221), not
  only the struct/builder — verified by a `run --json` test (t-03/t-10).
- **Stale governance evidence (resolved):** both the round-1 plan-audit pass and
  the round-2 research-orchestration pass were invalidated after the bundle was
  revised; research has now been refreshed, and fresh plan-audit remains a
  precondition for execution (see Verification Verdict).
- **Advisory predicate hazard:** `HasEmptyCodebaseMap` is `true` for
  `scaffold_only`/`missing` and `false` for `baseline` — the advisory must branch
  on `codebase_map_status`, or it will miss `baseline` and/or double-signal
  `scaffold_only`. Covered by the t-05 matrix; the retired helper is removed after
  production no longer calls it.
- **Workspace divergence on root `--change` (REQ-009):** the legacy
  `HasEmptyCodebaseMap(root, …)` hint probe reads the invocation checkout, not the
  bound worktree, so a root `--change` invocation can contradict the
  `WorkspaceRoot`-derived status. Mitigation: re-source the hint from
  `codebase_map_status` (one assessment); covered by t-05/t-12.
- **Missing-map signal (REQ-002):** `omitempty` must not drop the valid
  `"missing"` status — that would silently disable the default freshness signal in
  the empty-map case. Covered by t-03/t-10.
- **Hot-path assessment cost (low):** `AssessCodebaseMapDocs` runs a bounded
  (`scanLimit=500`) repo walk on every `next`/`run`; the helper short-circuits to
  `missing` when the map dir is absent (t-04).
- No accepted exceptions to guardrail protections.
- **Out of scope (filed as issues #28 and #29):** the execution-summary
  self-staleness at closeout (#28) and the post-archive `validate` re-audit
  limitation (#29) are distinct governance-evidence concerns in other subsystems
  (`execution_summary.go`/`evidence.go`/`repair.go` and `cmd/common.go`/`validate`
  respectively); both are deferred to their own governed change(s) and are
  neither addressed nor verified here.

## Rollback Readiness
Revert the single change: restore `/artifacts/codebase/` to the managed gitignore
block and drop the new JSON fields. No data migration; callers tolerate field
absence via `omitempty`. Verify with `go build ./... && go test ./...` and
`git check-ignore artifacts/codebase/ARCHITECTURE.md` (expected ignored only
after rollback).

## Archive Decision
Not ready to archive — change is in planning. Archive only after goal
verification passes with fresh build/test evidence.
