# Decision
## Project Context
- Tech Stack: Go
- Conventions: Cobra CLI; cmd/ surfaces, internal/state durable state, internal/engine workflow logic
- Test Command: go test ./...
- Build Command: go build ./...
- Languages: Go

## Alternatives Considered

### Obs-1 - codebase-map content generation
- **(A) Language-agnostic fact detection + blank scaffold for semantic docs + new `baseline` status.**
  Tradeoffs: more detection code, but bootstraps real facts for any stack, never fabricates semantic
  prose, and `baseline` tells hosts the docs still await authored verification.
- (B) Pure scaffold generator (no fact detection at all). Tradeoffs: simplest, but discards useful
  deterministic facts (languages, build/test commands) the host would otherwise re-derive.
- (C) Keep generation but emit scaffold only for "unknown" stacks. Tradeoffs: still Go-biased and
  fails the language-agnostic contract for known-but-non-Go stacks unless fully generalized anyway.

### Obs-1b - existing docs polluted by old generated Go/Slipway baseline
- **(A) Targeted legacy generated-doc refresh.** Detect the old deterministic Slipway-generated
  Go/Slipway baseline shape and refresh it to the current baseline/scaffold. Tradeoffs: extra
  migration logic, but fixes the issue #17 rerun scenario for already-polluted worktrees.
- (B) Require operators to delete `artifacts/codebase/` manually before rerun. Tradeoffs: minimal code,
  but leaves the reported Lattice worktree broken unless the operator already knows the workaround.
- (C) Overwrite any content whose detected facts differ from the current repo. Tradeoffs: broader
  cleanup, but risks deleting hand-authored architecture analysis.

### Obs-2 - active-change discovery from the repo root
- **(A) Improve the diagnostic only; rely on existing `--change <slug>` routing.** Research confirms
  `next/run --change <slug>` already resolve context to the bound worktree (`ResolveChangePaths`
  uses `change.WorktreePath`) and from-root discovery already walks git worktrees. Tradeoffs:
  smallest change; keeps default cwd scoping; no new surfaces.
- (B) Auto-resolve a single bound change from root. Tradeoffs: changes default scoping behavior -
  rejected by the user (keep worktree scoping).
- (C) New configurable `worktree_root` + slug-derivation, or a separate registry file. Tradeoffs:
  new surfaces to maintain; redundant with git's worktree registry + `change.yaml` - rejected in
  favor of git+change.yaml discovery.

### Obs-3 - stale empty active bundle residue after done
- **(A) Treat empty orphan active bundle directories as safe residue: ignore them for active-change
  discovery and let `repair` remove them.** Tradeoffs: preserves corruption signaling for non-empty
  orphan bundles while preventing a post-archive empty `verification/` directory from blocking status.
- (B) Ignore every orphan bundle directory without `change.yaml`. Tradeoffs: hides real partial state
  corruption and could lose operator-visible recovery evidence.
- (C) Make `done` fail when any root duplicate exists. Tradeoffs: blocks successful archive because of
  stale non-authoritative residue and still does not help older workspaces already in this state.

### Obs-4 - validate --change archived slug
- **(A) Fail explicitly for archived terminal changes.** `validate` remains an active-governance
  readiness command, but exact archived slugs return `archived_change_not_validatable` with status and
  archive path. Tradeoffs: smallest contract fix and directly resolves the misleading empty no-active
  diagnostic.
- (B) Fully validate archived bundles. Tradeoffs: broader path-authority work because archived
  `change.yaml` snapshots intentionally scrub `WorktreePath` and active-bundle paths are no longer the
  authority.
- (C) Keep generic `no_active_change`. Tradeoffs: preserves current behavior but leaves the printed
  `--change <slug>` remediation misleading after archive.

### Guardrail execution shape
- **(A) Separate RED contract test tasks in wave 1, with production tasks depending on them.**
  Tradeoffs: one additional wave, but aligns with `slipway-tdd-governance` and gives auditable
  test-first evidence before guarded production changes.
- (B) Keep implementation waves first and write tests later. Tradeoffs: simpler task list, but violates
  the guardrail-domain TDD hard gate.
- (C) Put tests and implementation in the same task/commit. Tradeoffs: convenient, but same-commit
  evidence is not test-first proof.

## Selected Approach
- **Obs-1: (A)** Replace `inspectCodebaseMapFacts` with a manifest+extension-driven,
  language-agnostic detector and rewrite `codebaseMapBaselineDoc` to fill only detected fields.
  Add `CodebaseMapStatusBaseline`; `AssessCodebaseMapDocs` compares each doc to the freshly
  regenerated baseline to classify `baseline` vs `populated`; add `baseline_docs` to the `--json`
  view + text writer.
- **Obs-1b: (A)** Add targeted legacy generated-doc recognition for the old deterministic Go/Slipway
  baseline output and allow `EnsureCodebaseMapDocs` to refresh that generated content on rerun.
  Preserve hand-authored substantive docs.
- **Obs-2: (A)** Make `FindActiveChangeForWorktree` distinguish "no active change" from "bound
  elsewhere" and return a typed signal carrying the bound changes; translate it in
  `wrapResolutionError`/`resolveActiveChangeRef` into a `change_bound_to_other_worktree`
  precondition error. Lock the already-correct `--change <slug>`-from-root routing with tests.
- **Obs-3: (A)** Add a narrow empty-orphan test for active bundle directories without `change.yaml`.
  Active-change discovery skips only fileless orphan bundle directories; `repair` removes those empty
  directories and records an applied repair. Non-empty orphan bundle directories remain integrity
  findings requiring operator review.
- **Obs-4: (A)** Extend explicit slug resolution to detect archived changes when the active lookup
  misses. Return a concrete precondition error with terminal status and archived `change.yaml` path,
  and document `validate --change` as an active-change selector.
- **Guardrail execution: (A)** Wave 1 owns RED contract tests only. Production code tasks depend on
  the relevant RED tasks and must cite separate RED evidence before implementation.

Honors documented constraints: deterministic sorted output; fail-closed domain_review + rollback_required;
all work inside the governed worktree; green build/test.

## Interfaces and Data Flow
- `internal/engine/artifact/codebase_map.go`:
  - `inspectCodebaseMapFacts(root)` -> language-agnostic `codebaseMapFacts` (languages,
    build/test cmds, deps, top dirs, entry points, test layout). New table-driven ecosystem
    detection + bounded extension scan.
  - New baseline builder computes facts once and renders only detected values into the fixed doc set.
  - New legacy generated-doc predicate recognizes the old deterministic Go/Slipway baseline shape.
    `EnsureCodebaseMapDocs` refreshes files that are blank scaffold or recognized old generated
    baseline docs; it does not overwrite authored content.
  - `AssessCodebaseMapDocs(root)`: regenerate baselines once; per doc -> missing | scaffold_only |
    baseline | populated. Aggregate status becomes `baseline` when generated baseline docs are present
    and no authored populated docs are required to consider the map complete.
  - New exported `CodebaseMapStatusBaseline`; assessment gains `BaselineDocs []string`.
- `cmd/codebase_map.go`: `codebaseMapView` gains `BaselineDocs []string`
  (`baseline_docs,omitempty`); text writer prints a Baseline section.
- `internal/state/store.go`: `FindActiveChangeForWorktree` returns a new typed
  `*ChangeBoundElsewhereError` (carrying []{slug, worktreePath}) instead of `ErrNoActiveChange`
  when active changes exist but are all bound to non-matching worktrees. `FindActiveChange`
  semantics unchanged.
- `cmd/common.go`: `wrapResolutionError` maps `*ChangeBoundElsewhereError` ->
  `newPreconditionError` (`change_bound_to_other_worktree`, details: bound_changes[]).
- `internal/state/store.go` / `internal/state/health.go`: empty orphan bundle directories are recognized
  by checking that the directory tree contains no files; discovery and health ignore those safe residues.
- `cmd/repair.go`: calls the state cleanup helper before orphan reporting, exposes
  `removed_empty_orphan_bundles`, and includes an `empty_orphan_bundle` applied repair entry.
- `internal/state/lifecycle.go`: exposes the archived `change.yaml` read path selected for an exact
  archived slug so command errors can name the actual terminal authority.
- `cmd/common.go` / `cmd/validate.go`: explicit slug resolution returns
  `archived_change_not_validatable` for archived changes; validate help says the flag selects an
  explicit active change.
- Data flow unchanged: `change.yaml` remains current-state authority; worktree paths read from
  `change.WorktreePath`; discovery via `git worktree list`.

## Rollout and Rollback
- Rollout: single PR on `feat/fix-codebase-map-fabricating-go-slipway-repository-context-f`;
  reviewable in six waves:
  1. RED contract tests for codebase-map, active-change, and archived validate behavior.
  2. RED contract tests for stale empty bundle residue plus core production fixes for fact
     detection/legacy refresh and bound-elsewhere diagnostics.
  3. Baseline status/view plumbing, `--change` routing hardening, and empty orphan bundle handling.
  4. Codebase-map / active-change regression tests, archived validate diagnostic, and documentation.
  5. Stale empty bundle regression tests.
  6. Full verification + guardrail review evidence.
- Verification command: `go build ./... && go test ./...`.
- Rollback: pure code change, no data migration. Revert the commit/PR. Generated
  `artifacts/codebase/*` docs are advisory and regenerated on demand.

## Risk
- [medium] Existing polluted codebase-map docs need targeted migration. Mitigation: restrict
  auto-refresh to the known old deterministic generated Go/Slipway baseline shape and test authored
  content preservation.
- [medium] Contract behavior changes: freshly mapped repos move from `populated` to `baseline`.
  Mitigation: explicit command tests and docs explaining the new trust level.
- [low] Multi-language detection false positives/negatives. Mitigation: deterministic manifest+
  extension detection with clean "not detected" -> blank scaffold fallback; per-ecosystem tests.
- [low] `stats` freshness stays modtime/coarse (cannot import `artifact`; import cycle). `baseline`
  is scoped to the codebase-map/next planning surface. Documented boundary.
- Guardrail (external_api_contracts): RED-test-first with separate pre-implementation RED evidence.
  domain_review + rollback_required run at S3/S4 review.
