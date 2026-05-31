# Decision
## Project Context
- Tech Stack: Go
- Conventions: repo-native (`go build ./...`, `go test ./...`)
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

Three implementation routes were weighed for where the freshness status is
computed and where the consume-time advisory is emitted (the gitignore change is
identical in all three). Full detail in `research.md` → `## Alternatives
Considered`.

- **Approach A — cmd-layer localized assembly (SELECTED):** the two cmd
  next-context builders call `artifact.AssessCodebaseMapDocs` via a shared helper
  to set the status field; the advisory extends the existing
  `cmd/next_skill_view.go` technique-hint block plus a `view.Warnings` entry,
  gated on research/plan-audit consumers. Smallest blast radius; reuses existing
  patterns; no `progression` API change.
- **Approach B — progression engine layer:** push assessment + advisory into
  `EvaluateGovernanceReadiness` so the advisory rides `readiness.Diagnostics`.
  Uniform across surfaces but widens the governance readiness API/tests, and the
  status field must still be threaded onto the cmd context structs anyway.
- **Approach C — hybrid:** shared cmd helper for status + advisory via
  progression readiness. Cleanest separation but the most moving parts.

## Selected Approach

**Approach A**, confirmed by the user on 2026-05-30.

- `DEC-001` — **Git-track maps by default.** Remove `/artifacts/codebase/` from
  `localStateGitIgnorePatterns` in `internal/state/local_ignore.go` and migrate
  this repository's checked-in `.gitignore` managed block in the same change.
  Existing repos auto-migrate because `EnsureLocalStateGitIgnore` rewrites the
  managed block in place whenever it differs. `evidence/`, `events/`,
  `verification/`, and `/.worktrees/` stay ignored. Implements `REQ-001`.
- `DEC-002` — **Surface freshness in the default handoff.** Add a
  `codebase_map_status` string and `codebase_map_doc_states` map to both
  `nextHandoffContext` (`cmd/next_handoff.go:45-52`) and `nextContext`
  (`cmd/next.go`), populated from a shared cmd helper that calls
  `artifact.AssessCodebaseMapDocs(paths.WorkspaceRoot)`. **The handoff/`run`
  surface is a field-by-field projection** (`cmd/next_handoff.go:216-223`), so
  the new field must also be added to that projection literal — adding it only to
  the struct + builder makes `slipway run --json` silently omit it while
  `slipway next --json` carries it. Not behind `--diagnostics`. Reuses the
  existing classification — no duplicate logic. **Missing-map semantics
  (round 3):** when no map exists the field reports the valid assessment value
  `"missing"` (with each doc `missing` in `codebase_map_doc_states`), NOT an
  omitted/empty field — `AssessCodebaseMapDocs` initializes `Status: "missing"`
  (`codebase_map.go:628-660`). `omitempty` is only an additive-contract tag for
  genuinely unavailable context, never a substitute for `missing` (else the
  default freshness signal disappears in exactly the empty-map case #27 targets).
  **Cost guard (round 3):** the shared helper SHOULD short-circuit to `missing`
  when the worktree's `artifacts/codebase/` dir is absent, since
  `AssessCodebaseMapDocs` otherwise runs a bounded repo walk
  (`codebaseMapBaselines` → `WalkDir`, `codebase_map.go:392`) on every
  `next`/`run`. Implements `REQ-002`.
- `DEC-003` — **Consume-time advisory + template guidance.** Extend the
  `cmd/next_skill_view.go:229-235` block that already appends a `slipway
  codebase-map` technique hint for an empty map, adding a non-blocking
  `view.Warnings` advisory for non-populated maps consumed by
  `research-orchestration` or `plan-audit`. **Correctness note (inverted in an
  earlier draft):** `HasEmptyCodebaseMap` is **`true`** for `scaffold_only` and
  `missing`, **`false`** for `baseline`/`populated` (`skill_resolution.go:79`).
  So the existing technique hint already fires for `scaffold_only`/`missing` but
  NOT for `baseline` — the case the advisory most needs to add. The advisory
  therefore branches on the new `codebase_map_status` field (set by DEC-002),
  fires for `scaffold_only` AND `baseline`, gates on `nextSkillName ∈
  {SkillResearchOrchestration, SkillPlanAudit}` in addition to the existing
  `StateS1Plan` gate, and must NOT double-emit a second "no durable docs" message
  for `scaffold_only` (where the hint already fires); `partial` is surfaced via
  `codebase_map_doc_states` only — no whole-map advisory (REQ-003 matrix). The
  advisory code task depends on the status field — RED-first order is
  t-06 ← t-05 ← t-04. `view.Warnings` is copied by the handoff projection
  (`next_handoff.go:226`), so the advisory reaches both surfaces with no extra
  projection edit. **Workspace-consistency fix (round 3, REQ-009):** the existing
  hint must NOT keep its own `HasEmptyCodebaseMap(root, view.InputContext.CodebaseMapDocs)`
  probe — under a root-checkout `slipway next --change <slug>` invocation, `root`
  is the invocation checkout while the doc paths are worktree-relative, so the
  probe reads the wrong checkout and can contradict the `WorkspaceRoot`-derived
  status. The hint is therefore re-derived from `codebase_map_status` (the same
  single `AssessCodebaseMapDocs(paths.WorkspaceRoot)` assessment), so status,
  advisory, and hint never diverge. This revises the earlier "keep the hint
  intact" wording into "keep the hint's behavior but re-source it from the status
  field." **scaffold_only double-surface (round 3):** for `scaffold_only` the
  hint (skill-attached) AND the advisory (top-level `warnings`) both fire by
  design — different consumers — but the advisory wording adds consume-time
  framing rather than restating "no durable docs", so they are complementary, not
  contradictory. Update those two SKILL templates to treat non-populated maps as
  non-durable, to use the exact `scaffold_only` JSON value, and to direct
  consumers to inspect per-doc `codebase_map_doc_states` so a `partial` map (no
  whole-map advisory) stays actionable. Remove the retired exported
  `progression.HasEmptyCodebaseMap` helper and its self-only test after the hint
  is re-sourced, so the old divergent probe cannot survive as a public-looking
  orphan API. Implements `REQ-003`, `REQ-004`, and `REQ-009`.
- `DEC-004` — **Docs + tests + guardrail compliance + RED-first discipline.**
  Document the new field and the git-tracked default in `docs/commands.md`,
  `CLAUDE.md`, and the codebase-mapping SKILL template; **correct the stale
  "local-only by default" narrative at `README.md:198` and the
  `artifacts/codebase/**` row in `docs/operator-guide.md:15`**. Because this is an
  `external_api_contracts`/guardrail change, every behavior-changing code task is
  authored RED-first (REQ-008): a failing test in a strictly earlier wave precedes
  each of the gitignore removal, the status field on **both** the standard `next`
  view and the `run`/handoff projection, the advisory **matrix** (incl. the
  `baseline` case `HasEmptyCodebaseMap` misses, and the flipped existing
  `TestLocalStateGitIgnoreRulesHideProofDirsButNotGovernedRecords` assertion so
  `artifacts/codebase/ARCHITECTURE.md` is trackable), direct `run --json`
  coverage for the state-mutating surface, and the template guidance
  (`internal/tmpl/templates_test.go`). Keep the handoff JSON change
  additive/backward compatible per the `external_api_contracts` guardrail.
  Implements `REQ-005`, `REQ-006`, `REQ-007`, and `REQ-008`.

## Interfaces and Data Flow

- **`next`/`run` JSON handoff (external contract):** two new optional,
  `omitempty` fields on `input_context` — `codebase_map_status` (string) and
  `codebase_map_doc_states` (map of short doc key → status). Additive only; no
  existing field changes, so callers that ignore them are unaffected.
- **Data flow:** `paths.WorkspaceRoot` → `artifact.AssessCodebaseMapDocs` →
  `CodebaseMapAssessment{Status, DocStates}` → cmd context structs → JSON. For
  the standard `next` view, `nextContext` serializes directly. For the `run`/
  handoff view, the field must additionally be copied in the field-by-field
  projection at `cmd/next_handoff.go:216-223` into `nextHandoffContext`. The
  advisory flows `assembleSkillViewWithOptions` → `next_skill_view.go` →
  `view.Warnings` → both views (the projection already copies `view.Warnings` at
  `next_handoff.go:226`). The empty-map technique hint at `next_skill_view.go:230`
  is re-sourced from `view.InputContext.CodebaseMapStatus` (the same
  `WorkspaceRoot` assessment), replacing its independent
  `HasEmptyCodebaseMap(root, …)` filesystem read so all three signals share one
  worktree-bound source (REQ-009). An empty map yields `codebase_map_status:
  "missing"` (not an omitted field).
- **gitignore:** `LocalStateGitIgnoreBlock()` output loses one line; the on-disk
  managed block is reconciled in place by `EnsureLocalStateGitIgnore`, which runs
  only on `slipway new`/`codebase-map`/`init` (not `next`/`run`/`status`/`repair`).
  This repository's checked-in `.gitignore` managed block is migrated immediately
  so Slipway no longer ignores its own `artifacts/codebase/` records.
  `internal/engine/progression/readiness.go:675-681`
  (`scopeContractGeneratedOnlyGitIgnore`) compares the on-disk block against the
  **live** block, so it stays self-consistent after the line is removed (no edit,
  but a coupling to keep in mind).
- No state-machine or gate changes. The only `progression` package surface
  change is deleting the retired `HasEmptyCodebaseMap` helper after production
  callers were moved to `codebase_map_status`.

## Rollout and Rollback

- **Rollout:** ship as one change. The handoff begins reporting
  `codebase_map_status` immediately on the next `slipway next`/`run` (those read
  the map fresh). The checked-in Slipway `.gitignore` is migrated in this change.
  Existing downstream repositories auto-migrate the managed block the next time
  `slipway new`/`codebase-map`/`init` runs (the commands that call
  `EnsureLocalStateGitIgnore`), since `next`/`run`/`status`/`repair` do not
  reconcile it. No dedicated migration command; in practice the relevant paths
  (`new`, `codebase-map`) are exactly the ones an operator hits when creating a
  change or (re)generating a map.
- **Rollback:** revert the change. Restoring `/artifacts/codebase/` to
  `localStateGitIgnorePatterns` re-ignores maps on the next command; the new
  JSON fields disappear (callers tolerate their absence via `omitempty`). No data
  migration, so rollback is clean. Verify with `go build ./... && go test ./...`
  and `git check-ignore artifacts/codebase/ARCHITECTURE.md`.

## Risk

- **External contract drift (medium → mitigated):** additive `omitempty` fields
  only; covered by a handoff test asserting the field is present and correct.
- **Auto-migration changes `git status` in existing repos (low-medium):**
  intended per issue #27; maps become visible/committable. Evidence/events/
  verification remain ignored, verified by a `local_ignore` migration test.
- **Hot-path assessment cost (low):** one shared `AssessCodebaseMapDocs` source
  is used for the status, doc states, advisory, and technique hint. The helper
  short-circuits to `missing` when `artifacts/codebase/` is absent; otherwise the
  existing classifier may run its bounded repo walk (`scanLimit=500`) while
  deriving baseline docs.
- **Reversibility:** high — additive fields, one removed ignore line, advisory
  text only.
