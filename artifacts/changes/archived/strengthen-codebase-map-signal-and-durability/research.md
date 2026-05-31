# Research

Source of truth: GitHub issue #27. Discovery for strengthening codebase-map
durability (git tracking) and freshness signal (handoff status + consume-time
advisory).

## Research Findings

### Architecture
- **Classification engine (reuse, do not change):** `artifact.AssessCodebaseMapDocs(root)`
  in `internal/engine/artifact/codebase_map.go:628-680` already computes a
  `CodebaseMapAssessment{Status, DocStates, MissingDocs, ScaffoldOnlyDocs,
  BaselineDocs, PopulatedDocs}` with status constants `missing/partial/
  scaffold_only/baseline/populated` (`codebase_map.go:95-110`). This is the
  single source of truth and is reused as-is.
- **Two view types + a projection (THREE edit points, not two):**
  - Handoff/run path: builds a **full** view whose `InputContext` is a
    `nextContext` (`cmd/next_handoff.go:64`); `buildNextHandoffContextByMode`
    (`next_handoff.go:147-185`) sets `view.InputContext.CodebaseMapDir/Docs` at
    `next_handoff.go:171-172`. It then **projects field-by-field** into the
    compact `nextHandoffContext` (`next_handoff.go:45-52`) at the literal in
    `next_handoff.go:208-228` (the codebase-map fields are copied at `:220-221`).
    ⚠️ The new status field must be added in BOTH the `nextHandoffContext` struct
    AND that projection literal (`:216-223`); adding it only to the struct/builder
    leaves `slipway run --json` silently omitting it. `view.Warnings` is already
    copied by the projection (`next_handoff.go:226`), so the advisory flows for
    free — only the InputContext status field needs the extra copy line.
  - Standard next path: `cmd/next_context_build.go:37-45` `buildNextContextByMode`
    populates `nextContext` (`cmd/next.go:136-152`); the standard view serializes
    `nextContext` directly (no projection), so setting the field on the builder
    suffices there.
  - Both resolve `paths.CodebaseMapDir` relative to `paths.WorkspaceRoot`
    (the worktree), so the assessment must run against `paths.WorkspaceRoot`.
- **Shared skill-view assembly (advisory injection point):** both next and run
  flow through `assembleSkillViewWithOptions` → `cmd/next_skill_view.go`. At
  `next_skill_view.go:229-235` there is an existing precedent that appends a
  `slipway codebase-map` `techniqueHint` when
  `progression.HasEmptyCodebaseMap(root, view.InputContext.CodebaseMapDocs)` is
  true during `S1_PLAN`. The `nextSkillName` is known at this point, so the
  advisory for research/plan-audit consumers belongs here. ⚠️ Two correctness
  notes: (1) **`HasEmptyCodebaseMap` does NOT line up with the advisory set —
  earlier drafts of this bundle had its truth value backwards.** Reading
  `skill_resolution.go:79`, it returns `false` only when it finds a non-scaffold
  doc with content; an all-`scaffold_only` map walks the whole loop without
  escaping and returns **`true`**, and `missing` also returns `true`, while
  `baseline`/`populated` return `false`. So the existing technique hint **already
  fires for `scaffold_only` and `missing`** but **not for `baseline`** — the case
  we most need to flag for consumers. The new advisory therefore CANNOT reuse
  `HasEmptyCodebaseMap`; it must branch on the new `codebase_map_status` field,
  fire for `scaffold_only` AND `baseline`, and avoid emitting a second
  contradictory "no durable docs" message for `scaffold_only` (where the
  technique hint is already present). Status matrix:

  | status | `HasEmptyCodebaseMap` | existing technique hint (S1_PLAN) | new advisory (research/plan-audit) |
  |---|---|---|---|
  | `missing` | `true` | fires | no |
  | `scaffold_only` (all scaffold) | `true` | fires | yes |
  | `baseline` (incl. scaffold+baseline; no populated/missing) | `false` | no | yes |
  | `partial` | depends | depends | no (`codebase_map_doc_states` only) |
  | `populated` | `false` | no | no |

  (2) The existing block is gated on
  `model.WorkflowState(nextState) == model.StateS1Plan` only; the advisory must
  additionally gate on `nextSkillName ∈ {progression.SkillResearchOrchestration
  ("research-orchestration"), progression.SkillPlanAudit ("plan-audit")}`
  (`internal/engine/progression/constants.go:6,8`; both resolve to `StateS1Plan`
  per `skill_resolution.go:55,60`, so the state gate is compatible). Reading
  `view.InputContext.CodebaseMapStatus` here means the **advisory code task
  depends on the status field already being set** — in the RED-first task graph,
  t-06 (advisory) ← t-05 (advisory RED test) ← t-04 (status field). The
  alternative is to re-call `AssessCodebaseMapDocs` in the advisory (a second
  ≤7-file read). Lean to reading the field to keep a single assessment.
- **Warnings surface on both views:** `view.Warnings` flows to the compact
  handoff (`nextHandoffView.Warnings`, `next_handoff.go:23`) and the standard
  next view (`nextView.Warnings`). `next_skill_view.go:172,189` already append
  to `view.Warnings`, confirming this is the sanctioned channel.
- **gitignore management:** `internal/state/local_ignore.go:19-25`
  `localStateGitIgnorePatterns` lists `/artifacts/codebase/` alongside
  `evidence/`, `events/`, `verification/`, `/.worktrees/`.
  `EnsureLocalStateGitIgnore` (`local_ignore.go:45-68`) rewrites the managed
  block in place whenever it differs, so removing the line auto-migrates
  existing repos on the next slipway command. Called from
  `cmd/codebase_map.go:40` and `cmd/new.go` (two sites).
- **Blast radius:** `cmd/next_handoff.go` (struct `:45-52`, builder `:171-172`,
  **and projection `:216-223`**), `cmd/next.go` (struct), `cmd/next_context_build.go`,
  `cmd/next_skill_view.go`, `internal/state/local_ignore.go`, plus templates and
  docs (`docs/commands.md`, `CLAUDE.md`, codebase-mapping SKILL, **`README.md:198`,
  `docs/operator-guide.md:15`** — the "local-only by default" lines). No changes
  to the state machine, gates, or `progression` API (Approach A).
- **Third consumer of the managed block (do not break):**
  `internal/engine/progression/readiness.go:675-681`
  (`scopeContractGeneratedOnlyGitIgnore`) compares the on-disk `.gitignore`
  against the **live** `state.LocalStateGitIgnoreBlock()` to decide whether a
  bare-managed-block `.gitignore` counts as an implementation-side changed file.
  Because it compares against the live block, removing the codebase line keeps it
  self-consistent — no edit required — but it is coupled to the block content and
  must not be overlooked (e.g. by any test asserting an exact pattern count).

### Patterns
- **Reusable assessment:** `artifact.AssessCodebaseMapDocs` and the
  `CodebaseMapStatus*` constants are already public and used by
  `cmd/codebase_map.go`. Reuse directly; add a small cmd helper so both
  builders call it once without duplication.
- **Display-doc helper precedent:** `artifact.CodebaseMapDisplayDocs`
  (`codebase_map.go:112-118`) is the existing pattern for deriving the
  per-doc map keyed by short name; the new `doc_states` field should key by the
  same short keys for caller symmetry.
- **Technique-hint / warning precedent:** `next_skill_view.go:229-235`
  (HasEmptyCodebaseMap → techniqueHint) is the template to extend for
  scaffold_only/baseline + a `view.Warnings` advisory.
- **Skill name constants:** `progression.SkillPlanAudit` (used at
  `next_skill_view.go:184`) and the research-orchestration skill id exist as
  constants — gate the advisory on these consumer skills.
- **gitignore block test precedent:** `internal/state/local_ignore_test.go`
  exists for asserting managed-block contents and migration.

### Risks
- **External contract change (medium):** `next`/`run` JSON is an
  `external_api_contracts` surface. Mitigation: new fields are additive and
  `omitempty`; no existing field changes. Backward compatible.
- **Auto-migration surprises operators (low-medium):** removing
  `/artifacts/codebase/` from the managed block changes `git status` for repos
  that had untracked maps — they will now show as untracked-and-not-ignored.
  This is the intended behavior (issue #27). Mitigation: keep evidence/events/
  verification ignored; consider a `repair` note (open question).
- **Assessment cost on the hot path (low, corrected round 3):** the earlier
  "≤7 small file reads" note understated it. `AssessCodebaseMapDocs` also calls
  `codebaseMapBaselines(root)` (`codebase_map.go:634`) → `inspectCodebaseMapFacts`
  → `inspectSourceExtensionFacts`, which runs `filepath.WalkDir(root, …)`
  (`codebase_map.go:392`) on **every** call — even when no map exists (baselines
  are computed before the doc loop, to distinguish `baseline` from `populated`).
  So wiring this into `next`/`run` (a high-frequency host path) adds a repo tree
  walk per invocation. It is **bounded** (`scanLimit = 500` files plus
  `shouldSkipCodebaseMapDir` dir pruning, `codebase_map.go:390-404`), so the cost
  is small, but it is a directory traversal, not 7 reads. Mitigation (SHOULD, in
  t-04): the shared cmd helper short-circuits to `status: "missing"` when the
  worktree's `artifacts/codebase/` dir is absent, skipping the walk in the common
  no-map case (an absent dir always assesses `missing`, so the result is identical).
- **Workspace divergence on root `--change` (NEW, round-3 finding, major):** the
  existing empty-map technique hint calls
  `HasEmptyCodebaseMap(root, view.InputContext.CodebaseMapDocs)` at
  `next_skill_view.go:230` with the **invocation** `root`
  (`assembleSkillViewWithOptions` param, `next_skill_view.go:69-70`), but the doc
  paths are display paths relative to `paths.WorkspaceRoot`
  (`CodebaseMapDisplayDocs(paths.WorkspaceRoot, …)`, `next_context_build.go:41` /
  `next_handoff.go:172`; `DisplayPath` is worktree-relative, `paths.go:114`).
  `HasEmptyCodebaseMap` re-joins them against `root` (`skill_resolution.go:83`).
  Empirically, `slipway next --change <slug> --json --diagnostics` from the
  **root checkout** reports `invocation_workspace_path: "."` vs
  `bound_workspace_path: ".worktrees/<slug>"`, so the hint reads the root
  checkout's map while the planned status field (off `paths.WorkspaceRoot`) reads
  the worktree map → contradictory output is possible (status `baseline`/
  `populated` while the hint says "no durable docs"). Fix folds into the design:
  derive BOTH the status and the hint from one
  `AssessCodebaseMapDocs(paths.WorkspaceRoot)` and drop the divergent probe
  (REQ-009). This is a latent pre-existing bug the change must not inherit.
- **Reversibility:** high. All changes are additive fields, one removed
  gitignore line, and advisory text; revertible without data loss.

### Test Strategy
- **Existing coverage to extend:** `cmd/codebase_map_context_test.go`
  (asserts `CodebaseMapDir`/`CodebaseMapDocs` on the next context) is the
  natural home for the new `codebase_map_status` assertions on both the
  handoff and standard next views. `internal/state/local_ignore_test.go` for
  migration. `cmd/codebase_map_command_test.go` already asserts `scaffold_only`
  freshness end-to-end (`cli_e2e_test.go:84`).
- **Existing test to FLIP (not just extend):**
  `internal/state/local_ignore_test.go` →
  `TestLocalStateGitIgnoreRulesHideProofDirsButNotGovernedRecords` currently
  lists `artifacts/codebase/ARCHITECTURE.md` in its `ignored` set and asserts it
  IS ignored (`:82`, `:92`). The RED test t-01 writes the flipped assertion and
  the GREEN code task t-02 makes it trackable, so this entry must MOVE from
  `ignored` to `trackable` — otherwise this existing test goes red.
- **New tests:**
  1. `next --json` **and** `run --json` default output both include
     `codebase_map_status` == `scaffold_only` for a fresh-mapped governed change
     (and a populated case if a fixture exists). The `run`/handoff assertion is
     load-bearing: it catches a forgotten projection copy at
     `next_handoff.go:216-223` that a `next`-only test would miss.
  2. `local_ignore`: after `EnsureLocalStateGitIgnore` rewrites a block that
     contained the legacy `/artifacts/codebase/` line, the line is gone while
     evidence/events/verification/worktrees remain.
  3. Advisory **matrix** (`cmd/next_skill_capability_hints_test.go`): drive
     `missing`, pure `scaffold_only`, mixed scaffold+baseline (= `baseline`
     status), `baseline`-only, and `populated`. When next skill is
     research-orchestration/plan-audit, assert the `warnings` advisory is present
     for `scaffold_only` AND `baseline`, absent for `populated`/`partial`, and
     that the existing `HasEmptyCodebaseMap` technique hint is not double-emitted
     as a second contradictory warning for `scaffold_only`. The `baseline` case is
     load-bearing: it is exactly what a `HasEmptyCodebaseMap`-based advisory would
     miss.
  4. Template guidance (`internal/tmpl/templates_test.go`): the regenerated
     research-orchestration and plan-audit SKILL surfaces contain the
     non-durable scaffold/baseline handling guidance, including the directive to
     inspect per-doc `codebase_map_doc_states` for `partial` maps (REQ-004).
  5. Workspace consistency (REQ-009): a root-checkout `next --change <slug>`
     against a bound worktree whose map is `baseline`/`populated` asserts the
     status reflects the worktree map AND no "no durable docs" hint fires. This
     is the case the old `HasEmptyCodebaseMap(root, …)` probe gets wrong; the
     test must drive the root `--change` path (not in-worktree invocation).
  6. Missing-status default (REQ-002): with no map, `codebase_map_status ==
     "missing"` and `codebase_map_doc_states` is present (not absent/empty) on
     BOTH the standard `next` view and the `run`/handoff projection — guards the
     `omitempty` from dropping the valid `missing` assessment.
- **Infrastructure:** existing governed-change test helpers
  (`createGovernedRequest`) suffice; no new harness.
- **Verification of acceptance:** `git check-ignore artifacts/codebase/ARCHITECTURE.md`
  exits non-zero post-migration; `go build ./...` and `go test ./...` pass.

## Alternatives Considered
- **Approach A — cmd-layer localized assembly (SELECTED):** status filled by
  the two cmd next-context builders via a shared helper calling
  `AssessCodebaseMapDocs`; advisory emitted by extending the existing
  `next_skill_view.go` technique-hint block plus a `view.Warnings` entry, gated
  on research/plan-audit consumers. Tradeoffs: smallest blast radius, reuses
  existing patterns, no `progression` API change; advisory logic lives in cmd
  (acceptable — that is where next-skill resolution already happens).
- **Approach B — progression engine layer:** push assessment + advisory into
  `EvaluateGovernanceReadiness` so the advisory rides `readiness.Diagnostics`
  (already merged into `view.Warnings`). Tradeoffs: uniform across surfaces and
  auto-deduped, but widens the progression API/tests and status still must be
  threaded onto the cmd context structs anyway.
- **Approach C — hybrid:** shared cmd helper for status + advisory via
  progression readiness. Tradeoffs: cleanest separation but spans two layers
  with the most moving parts.
- **Selected: Approach A** — confirmed by user 2026-05-30. It is additive,
  localized, reuses the `HasEmptyCodebaseMap`/technique-hint precedent at the
  exact point where the next skill name is known, and avoids broadening the
  governance readiness contract. A tiny shared cmd helper removes A's only
  downside (duplicate assessment call across the two builders).

## Unknowns
- Resolved: "Can both next and run surface the status/advisory from one place?"
  -> Yes; both flow through `assembleSkillViewWithOptions` →
  `next_skill_view.go`, and `view.Warnings` is on both view types.
- Resolved: "Does removing the gitignore line migrate existing repos?" -> Yes;
  `EnsureLocalStateGitIgnore` rewrites the managed block in place when it
  differs (`local_ignore.go:56-67`).
- Resolved: exact field shape -> include both `codebase_map_status` and
  `codebase_map_doc_states`, keyed by the same short doc keys as
  `codebase_map_docs`. Missing maps report `codebase_map_status: "missing"`
  with every expected doc state present as `missing`.
- Resolved / out of scope: advisory coverage stays limited to
  `research-orchestration` and `plan-audit` for this change. `wave-orchestration`
  can consume `codebase_map_doc_states`, and a whole-map advisory for that host is
  a possible follow-up rather than a blocker here.
- Resolved (review 2026-05-31): `cmd/repair.go` does **not** reconcile the
  managed gitignore block at all (only `new`/`codebase-map`/`init` call
  `EnsureLocalStateGitIgnore`). So "repair prints a note when it drops the line"
  is moot today. Reframed and deferred: whether to wire `repair` to reconcile
  (and thereby migrate) the block as a deterministic migration path — captured
  as a Deferred Idea in intent.md, not in this change's scope.

## Assumptions
- The next/run JSON handoff is the contract AI callers consume for map
  freshness - Evidence: issue #27 ("especially when `slipway next` includes
  `codebase_map_docs` in handoff/read refs"); `cmd/next_handoff.go:45-52`.
- Reusing `AssessCodebaseMapDocs` keeps a single classification source -
  Evidence: `internal/engine/artifact/codebase_map.go:628-680`,
  `cmd/codebase_map.go:47`.
- Both next and run share the skill-view assembly that owns warnings -
  Evidence: `cmd/next_handoff.go:141`, `cmd/next_skill_view.go:172,189,229`.

## Canonical References
- `internal/engine/artifact/codebase_map.go` (AssessCodebaseMapDocs, status
  constants, CodebaseMapAssessment, CodebaseMapDisplayDocs)
- `cmd/next_handoff.go` (nextHandoffContext, buildNextHandoffContextByMode)
- `cmd/next.go` (nextContext struct)
- `cmd/next_context_build.go` (buildNextContextByMode)
- `cmd/next_skill_view.go` (technique-hint/warning injection point)
- `internal/state/local_ignore.go` (managed gitignore block)
- `internal/state/local_ignore_test.go`, `cmd/codebase_map_context_test.go`
  (test homes)
- GitHub issue #27 (problem statement)
