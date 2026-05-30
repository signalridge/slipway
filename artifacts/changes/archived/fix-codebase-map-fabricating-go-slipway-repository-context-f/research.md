# Research

## Research Findings

### Architecture
- Affected modules:
  - `internal/engine/artifact/codebase_map.go` - `inspectCodebaseMapFacts` (Go-only),
    `codebaseMapBaselineDoc` (hardcoded Go/Slipway prose), `EnsureCodebaseMapDocs`
    (only refreshes scaffold-only files), `AssessCodebaseMapDocs` (classifies any
    non-scaffold content as `populated`), `CodebaseMapDocIsScaffoldOnly`.
  - `cmd/codebase_map.go` - `codebaseMapView` JSON contract (`status`, `doc_states`,
    `populated_docs`, `scaffold_only_docs`) + text writer.
  - `cmd/common.go` - `resolveActiveChangeRef`, `wrapResolutionError`, `currentWorktreeRoot`.
  - `internal/state/store.go` - `FindActiveChangeForWorktree`, `FindActiveChange`,
    `ListChanges`, `discoverActiveChangeSlugsAcrossRoots` (walks git worktrees).
  - `internal/engine/progression/skill_resolution.go` - `HasEmptyCodebaseMap` consumes
    `CodebaseMapDocIsScaffoldOnly`; drives the "run codebase-map" technique hint in
    `cmd/next_skill_view.go`.
  - `internal/state/stats.go` - separate coarse freshness surface; cannot import `artifact`
    due to import cycle.
- Key root causes:
  - New repo generation root cause: `codebaseMapBaselineDoc` emits Go/Slipway-specific content
    regardless of detected repo facts.
  - Rerun/rooted-pollution root cause: `EnsureCodebaseMapDocs` preserves any non-scaffold existing
    doc, so old buggy generated Go/Slipway docs are not refreshed; `AssessCodebaseMapDocs` then marks
    them as `populated`.
  - Root active-change concern: `FindActiveChangeForWorktree` collapses "no active change" and
    "active change exists but is bound elsewhere" into the same no-active diagnostic.
  - Closeout residue root cause: active-bundle discovery treats any directory under
    `artifacts/changes/<slug>` without `change.yaml` as missing authority, even when the directory
    contains only empty stale subdirectories outside the worktree-bound archive authority. `repair`
    reports the same empty residue as non-repairable because orphan detection has no safe-empty
    cleanup path.
  - Archived validate root cause: `validate --change` uses active-only explicit slug resolution.
    When the active lookup misses an archived slug, the command falls back to a generic empty
    no-active diagnostic instead of checking the archived terminal authority.
- Dependency chains:
  - `cmd/codebase_map.go` -> `artifact.EnsureCodebaseMapDocs` / `artifact.AssessCodebaseMapDocs`.
  - `cmd/next*.go` -> `state.ResolveChangePaths(root, change)` -> workspace_root from
    `change.WorktreePath` -> `artifact.CodebaseMapDisplayDocs`; and `progression.HasEmptyCodebaseMap`.
  - `artifact` imports `internal/state`; therefore `state` (stats) cannot import `artifact`.
- Blast radius: HIGH. This changes AI-host-facing CLI surfaces (`codebase-map`, next/run errors,
  validate explicit selector errors), generated advisory context, and guardrail-domain test/evidence
  requirements.
- Invariants to preserve:
  - `change.yaml` remains single current-state authority; bundles relocate into the worktree.
  - Deterministic, sorted output for docs/status/test assertions.
  - User-authored codebase-map analysis is not overwritten by generic fact drift.

### Patterns
- Existing conventions:
  - Codebase-map docs are an ordered fixed set (`codebaseMapDocNames`); doc->key map
    (`codebaseMapDocKeys`); blank templates (`codebaseMapDocTemplates`).
  - Facts are inspected from the filesystem then rendered into doc strings; status is derived by
    comparing rendered content against templates / substantive-content heuristics.
  - Worktree path is stored on the change and git maintains worktree roots; `.worktrees` is
    hardcoded in `DefaultWorktreePath`.
  - From-root discovery already walks all git worktrees:
    `discoverActiveChangeSlugsAcrossRoots` + `allWorkspaceRoots`/`candidateWorkspaceRoots`.
- Reusable abstractions to leverage:
  - `state.ResolveChangePaths(root, change)` already resolves workspace_root from `change.WorktreePath`;
    `next/run --change <slug>` should remain locked by tests.
  - `state.NormalizePath` for path comparison; typed CLI errors via `newPreconditionError`.
  - For status comparison: regenerate the current baseline once per assessment and compare normalized
    doc content.
- New local patterns required:
  - Add a targeted predicate for the old deterministic Go/Slipway generated baseline shape. It should
    match combinations of known generated phrases/full normalized old output, not arbitrary "looks Go"
    prose, so authored docs are preserved.
  - Add `baseline` as a distinct assessment state for current generated facts.
  - Put RED contract test tasks before production code tasks in the wave plan.
  - Treat empty orphan bundle directories as safe residue only when a tree walk finds no files; keep
    non-empty orphan bundle directories as integrity findings.

### Risks
- Technical risks:
  - [medium] Legacy generated-doc detection could be too broad and overwrite authored content.
    Mitigation: match only the old deterministic generated output shape/phrase combinations and test
    authored-content preservation.
  - [medium] Existing command behavior changes from reporting fresh generated docs as `populated` to
    `baseline`. Mitigation: explicit contract tests and agent-facing docs about trust level.
  - [low] Multi-language detection false negatives/positives. Mitigation: deterministic manifest+
    extension detection with clean "not detected" -> blank scaffold fallback; unit tests per ecosystem.
  - [low] `stats` freshness stays modtime/coarse and may count baseline docs as present. Out of scope
    because it is not the planning-authority surface and avoiding it prevents an import-cycle refactor.
- Guardrail domains: `external_api_contracts`. TDD evidence, domain_review, independent_review, and
  rollback-required handling are required before closeout.
- Reversibility: HIGH. Pure code change; no data migration. Rollback = revert the commit. Generated
  `artifacts/codebase/*` docs are advisory and regenerated on demand.

### Test Strategy
- Existing coverage:
  - `cmd/codebase_map_command_test.go` (`TestCodebaseMapCommandCreatesDurableDocSet`,
    `TestCodebaseMapCommandWritesToInvocationWorktree`).
  - `cmd/codebase_map_context_test.go` (next includes durable map paths).
  - `internal/engine/progression/skill_resolution_test.go` (HasEmptyCodebaseMap).
  - `internal/state/stats_test.go` (codebase-map stats; unchanged boundary).
  - Worktree/active-change resolution tests in `cmd/*_test.go`.
- Required RED tests before production code:
  - Codebase-map RED tests: Rust/Cargo repo does not emit Go facts; no-manifest repo yields blank
    scaffold; legacy Go/Slipway generated docs in a Rust repo are refreshed on rerun; generated
    baseline status is `baseline`, not `populated`.
  - Active-change RED tests: bare root invocation returns `change_bound_to_other_worktree` with
    slug/path/remediation; `next --change <slug>` from root resolves to the bound worktree.
  - Closeout/repair RED tests: empty root active-bundle residue does not break active-change discovery;
    repair removes it and reports an applied repair; non-empty orphan bundles remain protected.
  - Archived validate RED tests: `validate --change <archived-slug>` returns
    `archived_change_not_validatable` with slug/status/archive path and does not emit the empty
    no-active diagnostic.
- Regression tests after implementation:
  - Node/TS, Python, Slipway/Go, and no-manifest cases.
  - authored-content preservation for codebase-map docs.
  - JSON/text view lists `baseline_docs`.
  - multiple active bound changes, zero active changes, and JSON error detail shape.
  - empty nested orphan bundle residue and non-empty orphan bundle preservation.
  - exact archived slug behavior for the shared explicit change resolver and validate command.
- Verification approach:
  - AC1 Rust repo -> Rust/cargo + baseline, no Go/Slipway prose.
  - AC2 legacy polluted Rust repo -> old generated phrases removed/refreshed, not `populated`.
  - AC3 no manifest -> blank scaffold and `scaffold_only`.
  - AC4 Slipway repo -> Go facts detected as baseline.
  - AC5 from-root diagnostic + `--change <slug>` works.
  - AC6 archived validate selector returns concrete archived-change diagnostic.
  - `go build ./...` && `go test ./...`.

## Alternatives Considered
- codebase-map content generation:
  - **(A) Language-agnostic fact detection + blank scaffold for semantic docs + new `baseline`
    status.** Tradeoff: more detection code, but bootstraps real facts for any stack and never
    fabricates semantic prose.
  - (B) Make codebase-map a pure scaffold generator. Tradeoff: simplest, but discards deterministic
    facts that are useful as a starting point.
  - (C) Keep generation but emit scaffold for "unknown" stacks only. Tradeoff: still Go-biased unless
    fully generalized.
  - **Selected: (A)**.
- legacy polluted docs:
  - **(A) Targeted refresh of old deterministic Go/Slipway generated docs.** Tradeoff: extra matching
    logic, but directly fixes the issue #17 rerun scenario.
  - (B) Require manual deletion before rerun. Tradeoff: leaves existing Lattice worktrees broken unless
    the operator already knows the workaround.
  - (C) Generic overwrite when repo facts differ. Tradeoff: can destroy authored map analysis.
  - **Selected: (A)**.
- obs-2 active-change discovery from root:
  - **(A) Improve the diagnostic only.** `FindActiveChangeForWorktree` distinguishes "bound
    elsewhere"; bare `next/run` from root returns `change_bound_to_other_worktree` naming slug +
    worktree path + remediation. Rely on existing `--change <slug>` routing.
  - (B) Auto-resolve a single bound change from root. Tradeoff: changes default scoping behavior.
  - (C) New configurable `worktree_root` or a separate registry file. Tradeoff: new surfaces to maintain;
    redundant with git's worktree registry + `change.yaml`.
  - **Selected: (A)**.
- obs-4 archived explicit validate selector:
  - **(A) Fail explicitly for archived terminal changes.** Tradeoff: does not validate archived
    bundles, but fixes the misleading remediation with a narrow active-command diagnostic.
  - (B) Fully validate archived bundles. Tradeoff: broader path-authority work because archived
    snapshots intentionally scrub worktree runtime fields.
  - (C) Keep generic no-active. Tradeoff: leaves `--change <slug>` remediation misleading.
  - **Selected: (A)**.
- guardrail wave shape:
  - **(A) Separate RED contract tests into wave 1 and make production tasks depend on them.**
    Tradeoff: more tasks, but satisfies TDD governance.
  - (B) Keep test tasks after implementation. Tradeoff: violates the guardrail-domain hard gate.
  - (C) Same task/commit test+implementation evidence. Tradeoff: not auditable test-first proof.
  - **Selected: (A)**.

## Unknowns
- Resolved: Does `next --change <slug>` resolve context to the bound worktree from root? ->
  YES. `buildNextContextByMode` -> `state.ResolveChangePaths(root, change)` uses `change.WorktreePath`.
- Resolved: Is `change.yaml` readable from root after binding? -> YES. Bundles relocate into the
  worktree, but from-root discovery walks git worktrees.
- Resolved: Is `.worktrees` configurable today? -> NO; hardcoded in `DefaultWorktreePath`. We keep it
  hardcoded.
- Resolved: Ecosystem detection set -> manifest-driven (go.mod, Cargo.toml, package.json+tsconfig,
  pyproject/setup.py/requirements.txt, pom.xml, build.gradle[.kts], Gemfile, composer.json,
  *.csproj/*.sln) + extension scan; deps parsed for go/cargo/npm/pip, language+commands for the rest.
- Resolved: How to handle old generated Go/Slipway docs? -> targeted refresh of the old deterministic
  generated baseline shape; preserve authored content.
- Resolved: How to handle `validate --change <archived-slug>`? -> fail explicitly with
  `archived_change_not_validatable`, status, and archived authority path. Full archived validation is
  a separate path-authority change because validate remains active-readiness oriented.
- Resolved: How to satisfy TDD governance? -> wave 1 contains RED-only contract tests; production tasks
  depend on them and must cite separate RED evidence.
- Remaining: None blocking.

## Assumptions
- The `codebase-map --json` `status` and the docs read during plan-audit/wave-orchestration are the
  surfaces the issue targets - Evidence: issue #17 body/comments + command/skill references.
- Old generated Go/Slipway docs can be detected narrowly enough to avoid overwriting authored maps -
  Evidence: the old generator emitted deterministic phrases and fixed document shapes.
- git is available at runtime (already assumed: `currentWorktreeRoot`, `listGitWorktrees` shell out).

## Canonical References
- `internal/engine/artifact/codebase_map.go` (facts + baseline + refresh + assessment)
- `cmd/codebase_map.go` (JSON contract + text writer)
- `cmd/common.go` `resolveActiveChangeRef`, `wrapResolutionError`, `currentWorktreeRoot`
- `cmd/validate.go` `--change` active selector help and validate fallback behavior
- `internal/state/store.go` `FindActiveChangeForWorktree`, `discoverActiveChangeSlugsAcrossRoots`
- `internal/state/lifecycle.go` `LoadArchivedChange` / archived authority path lookup
- `internal/state/paths.go` `ResolveChangePaths`, `changeWorkspaceRoot`
- `internal/state/worktree.go` `DefaultWorktreePath`, `listGitWorktrees`
- `cmd/next_context_build.go` workspace_root resolution
- `internal/engine/progression/skill_resolution.go` `HasEmptyCodebaseMap`
- `internal/state/stats.go` `collectCodebaseMapStats` (separate freshness surface)
