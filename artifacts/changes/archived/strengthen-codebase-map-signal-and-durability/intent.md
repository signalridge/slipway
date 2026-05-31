# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack:
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions:

## Summary
strengthen codebase-map signal and durability
## Complexity Assessment
complex
<!-- Multi-surface change: gitignore behavior + next/run JSON handoff contract +
engine advisory warnings + skill templates + docs + tests. Touches the
externally-consumed handoff contract (external_api_contracts), so changes must
be additive/backward-compatible and coordinated across cmd/, internal/, and
templates. Source of truth: GitHub issue #27. -->

## Guardrail Domains
external_api_contracts

## In Scope
<!-- What is explicitly included -->
- **Git-track codebase maps by default** (`internal/state/local_ignore.go`):
  remove `/artifacts/codebase/` from `localStateGitIgnorePatterns`. Existing
  repos auto-migrate the next time `EnsureLocalStateGitIgnore` runs — i.e. on
  `slipway new` (`cmd/new.go:394,485`), `slipway codebase-map`
  (`cmd/codebase_map.go:40`), or `slipway init` (`internal/bootstrap/init.go:50`).
  `next`/`run`/`status`/`repair` do **not** reconcile the managed block today, so
  migration is not triggered by every slipway command. Keep `evidence/`,
  `events/`, `verification/`, `/.worktrees/` ignored.
- **Surface map freshness in the default compact handoff**: add
  `codebase_map_status` (and per-doc `codebase_map_doc_states`) to the
  `next`/`run` `input_context`. This needs **three** edit points, not two:
  (a) the `nextContext` struct (`cmd/next.go`) and `nextHandoffContext` struct
  (`cmd/next_handoff.go:45-52`); (b) the builders that populate them
  (`cmd/next_context_build.go`, `cmd/next_handoff.go:171-172`); and (c) the
  explicit handoff projection at `cmd/next_handoff.go:216-223`, which copies
  fields field-by-field from the full view into the compact `nextHandoffContext`
  — without adding the new field to that literal, `slipway run --json` (the
  handoff surface) silently drops it even though `slipway next --json` carries
  it. Populated by reusing `artifact.AssessCodebaseMapDocs()`. Not gated behind
  `--diagnostics`.
- **Engine-emitted advisory**: emit a non-blocking `warnings` entry from
  `next`/`run` when the resolved next skill consumes the map (research /
  plan-audit) and status is `scaffold_only` or `baseline`. Re-source the existing
  empty-map technique hint from `codebase_map_status` (same `WorkspaceRoot`
  assessment) so the hint, status, and advisory cannot diverge under a root
  `--change` invocation (REQ-009).
- **Template guidance**: update `research-orchestration` and `plan-audit`
  SKILL templates under `internal/tmpl/templates/skills/` to treat
  scaffold-only/baseline maps as non-durable and surface an advisory finding.
- **Docs**: update `docs/commands.md`, `CLAUDE.md`, and the codebase-mapping
  SKILL template to document the new status field and the git-tracked default.
- **Tests**: handoff status field, gitignore migration (block rewritten,
  evidence/events/verification retained), and the advisory warning path.

## Out of Scope
<!-- What is explicitly excluded -->
- Making codebase-map a hard/blocking governance gate (stays advisory per #27).
- A `.slipway.yaml` opt-out toggle for local-only maps (rejected in favor of
  auto-migrate; possible follow-up, not this change).
- Changing the classification algorithm inside `AssessCodebaseMapDocs()` —
  reuse the existing status computation as-is.
- Auto-committing / `git add`-ing existing untracked map files (user git action).
- Backfilling map content quality (scaffold→populated authoring is agent work,
  not engine behavior).
- **Execution-summary staleness at closeout** (issue #27 comment 4584903982):
  `repair` rebuilding an `execution-summary.yaml` whose own `captured_at` makes
  older task evidence look `stale_execution_evidence`, with `repair` then
  returning `non_repairable_findings`. This is a distinct evidence-freshness /
  repair concern in a different subsystem (`internal/state/execution_summary.go`,
  `internal/engine/progression/evidence.go`, `cmd/repair.go`) and a different
  guardrail profile; it is intentionally deferred to its own governed change
  rather than folded into this codebase-map change. Tracked as GitHub issue #28.
- **Post-archive auditability of governed bundles** (issue #27 comment
  4585442105): after `done`, `slipway validate --json --change <slug>` returns
  `archived_change_not_validatable` (`cmd/common.go:363`, precondition_blocked —
  active governance commands only validate active changes), so a reviewer cannot
  rerun the validation a change recorded as final assurance. This is a `validate`
  command-scope / archived-evidence audit-semantics concern; this change does not
  touch `validate`. Deferred to its own governed change (a read-only archived
  audit surface, a non-advancing `validate --change` for archived evidence, or
  corrected final-assurance language). Tracked as GitHub issue #29.

## Constraints
<!-- Technical / business / time constraints -->
- Handoff JSON change must be additive and backward-compatible (new optional
  fields only); it is an `external_api_contracts` surface.
- No duplicate classification logic — reuse `artifact.AssessCodebaseMapDocs()`.
- `go build ./...` and `go test ./...` must pass.

## Acceptance Signals
<!-- What verifiable signals indicate completion -->
- `slipway next --json` **and** `slipway run --json` default output both include
  `codebase_map_status` matching `AssessCodebaseMapDocs` (e.g. `scaffold_only`
  for a fresh map) — covered by `cmd/` tests asserting **both** the standard
  next view and the compact handoff/run projection.
- After `slipway new` / `slipway codebase-map` / `slipway init` runs in a repo
  whose `.gitignore` had the old managed block (the only commands that call
  `EnsureLocalStateGitIgnore`), `/artifacts/codebase/` is gone from the block
  while `evidence/`/`events/`/`verification/` remain — covered by a
  `local_ignore` test that drives `EnsureLocalStateGitIgnore` directly.
- `git check-ignore artifacts/codebase/ARCHITECTURE.md` exits non-zero
  (no longer ignored) after migration.
- When the next skill is plan-audit/research and status is
  `scaffold_only`/`baseline`, `warnings` contains a codebase-map advisory —
  covered by a test.
- `go build ./...` and `go test ./...` pass.

## Resolved Clarifications
- [x] Can both `next` and `run` surface status/advisory from one place? Resolved in research.md (`next_skill_view.go`).
- [x] Does removing the gitignore line auto-migrate existing repos? Resolved in research.md (`EnsureLocalStateGitIgnore`).
- [x] Per-doc field shape resolved: emit both `codebase_map_status` and
  `codebase_map_doc_states`; missing maps report `"missing"` with per-doc missing
  states present. Wave-orchestration advisory coverage and repair migration are
  deferred follow-ups, not blockers for this change.

## Open Questions
- None.

## Deferred Ideas
<!-- Identified but postponed ideas -->
- `.slipway.yaml` policy toggle for projects that want local-only maps.
- Wire `slipway repair` to call `EnsureLocalStateGitIgnore` so it also
  reconciles (and migrates) the managed gitignore block, making `repair` a
  deterministic migration path. Currently only `new`/`codebase-map`/`init`
  reconcile it. (Realizes the research "repair note" open question.)
- Separate governed change(s) for the two deferred issue #27 concerns, now filed
  as standalone issues #28 (execution-summary self-staleness at closeout) and #29
  (post-archive `validate` can't re-audit archived bundles). Both are
  closeout/post-archive governance-evidence friction and could be bundled into
  one "closeout & audit hardening" change or split. See Out of Scope.

## Approved Summary
<!-- User-confirmed final summary + confirmation timestamp -->
Confirmed by user 2026-05-30T20:45:49Z.

Strengthen codebase-map signal and durability (GitHub issue #27) via three
coordinated changes:

1. **Git-track maps by default** — remove `/artifacts/codebase/` from the
   managed `.gitignore` block in `internal/state/local_ignore.go`; existing
   repos auto-migrate the next time `slipway new`/`codebase-map`/`init` rewrites
   the managed block (not `next`/`run`/`status`/`repair`). Keep `evidence/`,
   `events/`, `verification/`, and `/.worktrees/` ignored.
2. **Freshness in the default handoff** — add `codebase_map_status` (and
   per-doc states) to the `next`/`run` `input_context`, reusing
   `artifact.AssessCodebaseMapDocs()`; not hidden behind `--diagnostics`.
3. **Consume-time advisory** — engine emits a non-blocking `warnings` entry
   when the next skill (research / plan-audit) consumes a `scaffold_only` or
   `baseline` map, with matching SKILL template guidance.

Out of scope: blocking gate, `.slipway.yaml` opt-out toggle, changes to the
classification algorithm, auto-committing existing map files.

Primary acceptance: `slipway next --json` **and** `slipway run --json` default
output report the correct `codebase_map_status`; post-migration `git check-ignore
artifacts/codebase/ARCHITECTURE.md` returns non-zero; research/plan-audit
consuming a non-populated map yields a `warnings` advisory; `go build/test
./...` pass.

## Review Refinements (2026-05-31)
<!-- Post-approval accuracy refinements from plan review; approved scope unchanged. -->
The bundle was reviewed against source on 2026-05-31; the following were folded
in without changing approved scope (intent #1–#3):
1. **Handoff projection (was missing):** surfacing the status field requires
   editing the field-by-field projection at `cmd/next_handoff.go:216-223`, not
   only the struct + builder — else `slipway run --json` omits it.
2. **Migration trigger corrected:** only `new`/`codebase-map`/`init` call
   `EnsureLocalStateGitIgnore`; `repair` does **not** (the earlier "repair block
   rewrite" wording was wrong). Acceptance now names the triggering commands.
3. **Existing test flips:** `local_ignore_test.go`
   `TestLocalStateGitIgnoreRulesHideProofDirsButNotGovernedRecords` currently
   asserts `artifacts/codebase/ARCHITECTURE.md` is ignored — the RED gitignore
   test (t-01 in the restructured graph) must **move** it from the ignored set to
   the trackable set, not just add tests.
4. **Advisory condition (CORRECTED in round 2 — see below):** the advisory
   branches on the new `codebase_map_status` field and gates on `nextSkillName ∈
   {SkillResearchOrchestration, SkillPlanAudit}`. (The round-1 reason given here —
   "`HasEmptyCodebaseMap` is false for `scaffold_only`/`baseline`" — was wrong; the
   predicate is `true` for `scaffold_only`/`missing` and `false` for `baseline`.
   The conclusion stands; see round 2 #2.)
5. **Doc/test scope:** README narrative (asserted by `internal/toolgen`
   `toolgen_test.go:1001`) and the template-guidance assertions
   (`internal/tmpl/templates_test.go`) are in scope.
6. **Blast radius:** `internal/engine/progression/readiness.go:675-681`
   (`scopeContractGeneratedOnlyGitIgnore`) is a third consumer of
   `LocalStateGitIgnoreBlock()`; it stays self-consistent (compares the live
   block) but must not be broken.
7. **Issues #28 and #29 NOT in scope:** the execution-summary staleness concern
   (#28) and the post-archive `validate` re-audit limitation (#29) are separate
   subsystems, each deferred to its own change (see Out of Scope).

## Review Refinements (2026-05-31, round 2)
<!-- Second-pass review (independent + author). Approved scope (intent #1–#3)
unchanged; these fix governance evidence, execution shape, and a factual error.
Task IDs below refer to the restructured tasks.md. -->
1. **plan-audit evidence invalidated (blocker).** The prior
   `verification/plan-audit.yaml` `pass` (2026-05-30) predates these refinements,
   which change execution-affecting constraints (projection, migration trigger,
   test flip, advisory condition, RED-first tasks). Its verdict is set to `fail`
   (run_version 1) so it cannot gate; a fresh plan-audit must be run against this
   revised bundle via `slipway run` before execution.
2. **`HasEmptyCodebaseMap` truth value corrected (was inverted).** It returns
   **`true`** for `scaffold_only`/`missing` and **`false`** for
   `baseline`/`populated` (`skill_resolution.go:79`). So the existing technique
   hint already fires for `scaffold_only`/`missing` but misses `baseline`. The
   advisory branches on `codebase_map_status`, covers `scaffold_only` AND
   `baseline`, and must not double-signal `scaffold_only`. See the REQ-003 matrix.
3. **`partial` status is doc_states-only (explicit decision).** A `partial` map
   (some populated mixed with scaffold/missing) gets no whole-map advisory; its
   non-durable docs are visible via `codebase_map_doc_states`. Recorded so it is a
   conscious scope line, not an accidental gap.
4. **RED-first task restructure (external_api_contracts guardrail).** Every
   behavior-changing code task (gitignore t-02, status field t-04, advisory t-06,
   templates t-08) is now preceded by a RED test task in a strictly earlier wave,
   with RED→GREEN→REFACTOR evidence required (new REQ-008). This replaces the
   round-1 layout that placed production code in wave 1 and tests in wave 4.

## Review Refinements (2026-05-31, round 3)
<!-- Third-pass review (changes-requested). Approved scope (intent #1–#3)
unchanged; these fix two execution-affecting correctness issues, three quality
points, and add REQ-009. Verdict was changes-requested; plan-audit AND
research-orchestration evidence are invalidated and must be re-run. -->
1. **(major, new REQ-009) Workspace divergence on root `--change`.** The existing
   empty-map technique hint calls `HasEmptyCodebaseMap(root, …)` at
   `next_skill_view.go:230` with the **invocation** `root`, while the doc paths are
   relative to `paths.WorkspaceRoot`. Under `slipway next --change <slug>` from the
   root checkout (`invocation_workspace_path: "."` vs `bound_workspace_path:
   .worktrees/<slug>`) the hint reads the wrong checkout and can contradict the
   `WorkspaceRoot`-derived `codebase_map_status`. Fix: derive the hint, the status
   field, and the advisory from one `AssessCodebaseMapDocs(paths.WorkspaceRoot)`
   assessment; drop the divergent probe. New RED test for the root `--change` path
   (t-05) and a re-source of the hint (t-06).
2. **(major) Missing-map must report `"missing"`, not an omitted field.** Round-2
   t-03 said to assert "field empty/absent when no map exists", which lets the
   executor drop the most basic status and silently disable the default freshness
   signal #27 is about. `AssessCodebaseMapDocs` returns `Status: "missing"` for an
   empty map (`codebase_map.go:628-660`). REQ-002/t-03/t-10 now require
   `codebase_map_status == "missing"` with `codebase_map_doc_states` present;
   `omitempty` is an additive guarantee for genuinely unavailable context only.
3. **(quality) Assessment cost on the hot path corrected.** `AssessCodebaseMapDocs`
   triggers a bounded (`scanLimit=500`) repo `WalkDir` via `codebaseMapBaselines`
   on every `next`/`run`, not "≤7 file reads". The shared helper SHOULD
   short-circuit to `missing` when the worktree map dir is absent (t-04).
4. **(quality) `partial` needs guidance, not just signal.** `partial` gets no
   whole-map advisory (only `codebase_map_doc_states`), so REQ-004/t-07/t-08 now
   require the SKILL templates to direct consumers to inspect per-doc states.
5. **(quality) `scaffold_only` double-surface made explicit.** For `scaffold_only`
   both the technique hint (skill-attached) and the advisory (top-level `warnings`)
   fire by design; the advisory adds consume-time framing rather than restating
   "no durable docs". REQ-003 records this so it is intentional, and t-05 pins it.
6. **(minor) Research evidence invalidated.** `research-orchestration.yaml` (pass,
   2026-05-30) predates these round-3 facts (incl. the new workspace-divergence
   architecture finding); set to `fail` (run_version 1) so a fresh research/
   plan-audit pass is a precondition for execution. Mirrors the round-2 plan-audit
   invalidation.

## Review Refinements (2026-05-31, round 4)
<!-- Post-review hardening after implementation review. Scope remains issue #27:
make the durability signal true in the actual repo, remove retired signal paths,
and close the state-mutating run contract with direct coverage. -->
1. **Checked-in `.gitignore` must match the new default.** Removing
   `/artifacts/codebase/` from `LocalStateGitIgnoreBlock()` is insufficient if
   this repository's managed `.gitignore` block still ignores it. The current
   worktree must also be migrated and verified with `git check-ignore`.
2. **Remove the retired exported probe.** Once the hint/advisory are re-sourced
   from `codebase_map_status`, `progression.HasEmptyCodebaseMap` has no
   production caller and only its own test. Delete both to avoid a misleading
   public-looking orphan API.
3. **Exact status spelling.** Generated planning/discovery guidance must say
   `scaffold_only`, not the hyphenated prose spelling, because agent callers may
   compare the JSON value literally.
4. **Direct `run --json` coverage.** The compact projection is shared with
   `next --json`, but `run` is the state-mutating surface; add a direct
   `makeRunCmd` assertion covering a `baseline` map, advisory warning, status
   field, and absent empty-map hint.
5. **State change is intentional.** The local `change.yaml` state advanced to
   `S4_VERIFY`; keep and commit it with the hardening changes so the governed
   bundle reflects the actual lifecycle position.
