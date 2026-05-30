# Intent

## Project Context
<!-- Auto-filled by InferProjectContext(); .slipway.yaml overrides -->
- Tech Stack: Go
- Languages: Go
- Test Command: go test ./...
- Build Command: go build ./...
- Conventions: Cobra CLI; cmd/ owns surfaces, internal/state owns durable state, internal/engine owns workflow logic; JSON surfaces are consumed contracts.

## Summary
Resolve GitHub issue #17 (Lattice feedback). Five execution-relevant problems
are in scope:
(1) CONFIRMED DEFECT - `slipway codebase-map` fabricates Go/Slipway repository
context (`Languages: Go`, `go build ./...`, `cmd/`, `internal/state`) for any
repo regardless of the detected stack, and marks the generated docs as
`populated`, misleading AI hosts that consume them as authored brownfield
context.
(2) CONFIRMED RERUN GAP - repositories already polluted by the old Go/Slipway
generated docs are not repaired by rerunning `slipway codebase-map`, because
existing non-scaffold docs are preserved and assessed as `populated`.
(3) CONCERN - `slipway next`/`run` return a misleading `no_active_change` when
invoked from the repo root while the active change is bound to a worktree,
whereas `slipway status` discovers it. The diagnostic is not self-explanatory.
(4) CONFIRMED CLOSEOUT DEFECT - after `slipway done`, an empty stale root
active-bundle directory can make status fail and repair report non-repairable
state even though the worktree archive is intact.
(5) CONCERN - `slipway validate --change <archived-slug>` returns the same
empty no-active diagnostic after archive, even though the remediation suggests
using an explicit slug.

## Complexity Assessment
complex
<!-- Rationale: multi-file change across cmd/, internal/engine/artifact, and
internal/state; changes externally-consumed JSON/text surfaces (`baseline` status
+ field; active-change resolution diagnostic; validate explicit selector error);
requires multi-language fixtures, legacy-generated-doc fixtures, closeout
residue fixtures, and guardrail-domain RED evidence before production changes. -->

## Guardrail Domains
external_api_contracts
<!-- The codebase-map `--json` view and next/run/validate resolution errors are
contracts consumed by AI host callers. domain_review + rollback_required apply. -->

## In Scope
Obs-1 - codebase-map content generation (`internal/engine/artifact/codebase_map.go`, `cmd/codebase_map.go`):
- Replace the Go-only `inspectCodebaseMapFacts` with language-agnostic detection:
  manifest-driven ecosystem detection (`go.mod`, `Cargo.toml`, `package.json`/`tsconfig.json`,
  `pyproject.toml`/`setup.py`/`requirements.txt`, `pom.xml`, `build.gradle[.kts]`, `Gemfile`,
  `composer.json`, `*.csproj`/`*.sln`, ...) -> detected languages, build/test commands, key
  dependencies; plus language-agnostic directory layout, entry points, and test layout.
- Rewrite `codebaseMapBaselineDoc` so STACK/STRUCTURE/TESTING contain only detected facts
  (undetected fields stay as blank scaffold lines; with zero detections the doc equals the
  blank template) and INTEGRATIONS/ARCHITECTURE/CONVENTIONS/CONCERNS are emitted as blank
  scaffold (no hardcoded Slipway/Go prose).
- Add targeted recognition of old deterministic Go/Slipway generated map content and refresh it
  to the current baseline/scaffold on rerun, without overwriting hand-authored substantive maps.
- Add `CodebaseMapStatusBaseline`; `AssessCodebaseMapDocs` classifies CLI-generated baseline
  docs (non-blank content equal to the regenerated baseline) as `baseline` (not `populated`),
  with a `baseline` aggregate status. Add `baseline_docs` to the `--json` view + text writer.

Obs-2 - active-change discovery from the root checkout (`cmd/common.go`, `internal/state/store.go`):
- Resolve a change's worktree from its recorded `change.yaml` `WorktreePath` (written at creation
  and already discoverable from the repo root via `ListChanges`), cross-checked against git's own
  worktree registry (`git worktree list`). No new hand-maintained registry and no new config field
  - git + `change.yaml` are the source of truth.
- `next`/`run` (and peers) with `--change <slug>` resolve and operate against that change's bound
  worktree from any directory, including the repo root.
- Self-explanatory diagnostic: when invoked from a directory that matches no bound worktree and
  >=1 active change is bound elsewhere, return an error naming the slug(s) + bound worktree path
  + remediation (`--change <slug>` or cd into the worktree) instead of `no_active_change`.

Obs-3 - post-archive empty bundle residue (`internal/state/store.go`, `internal/state/health.go`, `cmd/repair.go`):
- Active-change discovery and status ignore active bundle directories that lack `change.yaml` only
  when the directory tree contains no files.
- `repair --json` removes those empty orphan active-bundle directories and reports an applied
  `empty_orphan_bundle` repair. Non-empty orphan bundles remain integrity findings.

Obs-4 - archived explicit validate selector (`cmd/common.go`, `cmd/validate.go`, `internal/state/lifecycle.go`):
- `validate --change <archived-slug>` fails explicitly with `archived_change_not_validatable`,
  terminal status, and archived authority path, instead of returning an empty-slug no-active
  diagnostic. Full archived-bundle validation is out of scope for this change.

TDD + tests + docs:
- Author separate RED contract tests before production changes:
  codebase-map behavior tests first, active-change behavior tests first, archived validate behavior
  tests first, and empty-bundle residue behavior tests first.
- Multi-language codebase-map regression tests (Rust/Cargo, Node, Python, no-manifest, Slipway/Go)
  plus legacy generated Go/Slipway map refresh and authored-content preservation.
- Tests for the obs-2 diagnostic + slug-derived worktree resolution from git + `change.yaml`.
- Tests for the obs-3 empty residue behavior and obs-4 archived explicit validate diagnostic.
- Update `CLAUDE.md`, docs, context-assembly guidance, and `slipway-codebase-mapping` guidance for
  the new `baseline` status and its trust level; update command docs for active-only validate
  selector behavior.

## Out of Scope
- Deep semantic auto-authoring of ARCHITECTURE/CONVENTIONS/CONCERNS/INTEGRATIONS - those remain
  AI/human-authored scaffolds; the CLI only bootstraps deterministically detectable facts.
- Changing the default cwd-based worktree scoping of `next`/`run`.
- A separate stateful registry file, a `worktree_root` config field, or slug-derived path guessing.
- Exhaustive coverage of every programming language/build tool; cover the common ecosystems with
  a clean "not detected" fallback.
- Generic overwrite of user-authored codebase-map analysis when repository facts later change.
- Full validation of archived terminal bundles through `validate --change`; the command now fails
  explicitly for archived slugs and names the archived authority path.

## Constraints
- Deterministic, sorted output for testability.
- Guardrail domain `external_api_contracts`: RED-test-first, domain_review, and rollback_required
  are enforced.
- Work stays within the governed worktree; `go build ./...` and `go test ./...` must pass.

## Acceptance Signals
- `slipway codebase-map --json` in a Rust/Cargo-only repo: STACK lists `Rust` + cargo build/test
  commands (not Go / `go build ./...`); ARCHITECTURE/STRUCTURE contain no Slipway `cmd/`/
  `internal/` prose; aggregate status is `baseline` (not `populated`).
- `slipway codebase-map --json` in a Rust/Cargo repo already containing old Go/Slipway generated
  codebase-map docs removes or replaces the old generated phrases and does not report the old
  generated content as authored `populated` analysis.
- `slipway codebase-map --json` in a repo with no recognizable manifest: no fabricated `Go`;
  docs are blank scaffold; status `scaffold_only`.
- `slipway codebase-map --json` in the Slipway repo itself: detects Go + go commands as baseline
  facts and reports them as `baseline`.
- From the repo root with a change bound to a worktree: bare `slipway next`/`run` return a
  self-explanatory error naming the bound worktree + slug; `slipway next --change <slug>` from
  the root resolves and operates against that worktree.
- After a worktree-bound `done`, empty stale root active-bundle residue does not break status and
  `repair --json` removes it; non-empty orphan bundle dirs remain protected.
- `slipway validate --json --change <archived-slug>` returns
  `archived_change_not_validatable` with terminal status and archived `change.yaml` path, not an
  empty-slug no-active diagnostic.
- Separate RED evidence exists before production changes for the codebase-map and active-change
  contract changes.
- `go build ./...` and `go test ./...` pass.

## Open Questions
<!-- Unresolved questions -> consumed by S1_PLAN/research; all resolved during S0/S1 research -->
- [x] Does `next --change <slug>` resolve context to the bound worktree from root? Resolved: yes - ResolveChangePaths uses change.WorktreePath (research.md).
- [x] Is `change.yaml` WorktreePath readable from the repo root after binding/relocation? Resolved: yes - discovery walks git worktrees (research.md).
- [x] Which ecosystems to detect deterministically without overreach? Resolved: manifest+extension set, deps for go/cargo/npm/pip (research.md).
- [x] Should old buggy Go/Slipway generated codebase-map content be refreshed automatically? Resolved: yes - targeted refresh is in scope; manual deletion is not an adequate issue #17 fix.
- [x] How should post-archive empty active-bundle residue be handled? Resolved: ignore/remove only fileless orphan dirs; preserve non-empty orphan bundles.
- [x] How should `validate --change <archived-slug>` behave? Resolved: fail explicitly with `archived_change_not_validatable`; full archived validation is deferred.
- [x] How is RED-first enforced? Resolved: separate wave-1 RED contract test tasks must precede production code tasks.

## Deferred Ideas
- A first-class `slipway worktree` discovery/listing surface.
- Auto-suggesting `--change <slug>` in shell completion.

## Approved Summary
<!-- User-confirmed final summary + confirmation timestamp -->
Confirmed by user: 2026-05-30T10:26:41Z
Artifact review corrections accepted: 2026-05-30T11:47:40Z

Resolve GitHub issue #17 with the following fixes:

1. **codebase-map generation.** Replace the hardcoded Go/Slipway baseline
   generator with language-agnostic, manifest-driven fact detection (Go, Rust,
   Node/TS, Python, Java/Kotlin, Ruby, PHP, .NET, ...). STACK/STRUCTURE/TESTING
   are populated only with detected facts. ARCHITECTURE/CONVENTIONS/CONCERNS/
   INTEGRATIONS are emitted as blank scaffold; the CLI never fabricates semantic
   prose.

2. **legacy polluted map refresh.** Detect the old deterministic Go/Slipway
   generated docs that prior Slipway versions wrote into non-Go repos and refresh
   those generated docs on rerun. Preserve hand-authored substantive maps.

3. **baseline status.** Add a `baseline` doc/aggregate status (+ `baseline_docs`
   field) so AI hosts treat CLI-detected facts as awaiting authored verification,
   not as completed brownfield analysis.

4. **next/run active-change discovery.** Keep default cwd-based worktree scoping.
   Locate a change's worktree from git's worktree registry plus `change.yaml`
   `WorktreePath`. `next/run --change <slug>` resolves against the bound
   worktree from any directory; a bare root invocation returns a
   self-explanatory `change_bound_to_other_worktree` diagnostic.

5. **TDD execution shape.** Separate RED contract test tasks run before any
   production code changes. Implementation tasks depend on those RED tasks.

6. **post-archive empty residue.** Empty root active-bundle residue left after a
   worktree-bound `done` no longer breaks status; repair removes it and keeps
   non-empty orphans protected.

7. **archived validate selector.** `validate --change <archived-slug>` now fails
   with a concrete archived-change diagnostic instead of an empty no-active view.

**Out of scope:** semantic auto-authoring of architecture/conventions docs;
changing default cwd scoping; a `worktree_root` config field or separate
registry file; exhaustive language coverage; generic overwrite of authored
codebase maps after repository facts change; full archived-bundle validation
through `validate --change`.
