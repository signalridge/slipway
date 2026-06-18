# Research

## Alternatives Considered

### Architecture

- Affected modules:
  - `cmd/evidence.go`: public evidence write guard and replacement policy for
    skill evidence.
  - `internal/engine/progression/authority.go`: context-origin fail-closed
    authority and selected-review participant parsing.
  - `internal/toolgen/toolgen.go`: adapter generation, cleanup, skill export, and
    command/router surface generation.
  - `internal/fsutil/transaction.go`: existing rollback-capable multi-file
    transaction primitive.
  - `internal/toolgen/surface_manifest.go` and `docs/SURFACE-MANIFEST.json`:
    public surface inventory, distinct from a generated-file ownership manifest.
  - `docs/`, `mkdocs.yml`, `.golangci.yaml`, and a new Go analyzer package for
    documentation/test-quality surfaces. For #170, GSD Core is the local
    reference for Diataxis grouping and tutorial/how-to/reference/explanation
    index shape; Trellis is the external reference for Start Here onboarding,
    first-task setup, task/spec/memory terminology, platform-aware setup, and
    real-world-scenario pages.
- Dependency chains:
  - `slipway evidence skill` -> `validateEvidenceSkillActionable` -> selected
    review state/readiness -> verification file write.
  - `authority.go` -> model context-origin parser -> reason-code/recovery output.
  - `toolgen.Registry` / skill registries / command registry -> generated
    adapter files -> sentinel/cleanup -> surface manifest/docs.
  - analyzer package -> test command/CI/docs policy, without becoming a runtime
    dependency of Slipway commands.
- Blast radius: CLI evidence recovery, adapter refresh output, generated skill
  listing, documentation navigation, and test-quality checks. Runtime lifecycle
  authority remains unchanged except for a narrower public repair path.
- Constraints:
  - Preserve fail-closed standard/strict guardrails.
  - Do not create general evidence overwrite or force-pass behavior.
  - Do not delete unknown or modified user-adjacent generated files without
    ownership evidence.
  - Keep toolgen ownership metadata separate from public docs surface inventory.

### Patterns

- Existing conventions:
  - Evidence writes are engine-stamped through CLI commands, not hand-edited
    verification YAML.
  - File safety uses `fsutil.WriteFileAtomic` and the existing
    `fsutil.ApplyFileTransaction` primitive.
  - Public command and skill surfaces are generated from Go-owned registries and
    templates.
  - Surface/docs contracts are protected by deterministic manifest tests.
  - Tests are package-local and table-driven around CLI and internal seams.
- Reusable abstractions:
  - Reuse `ApplyFileTransaction` rather than building another rollback layer.
  - Reuse selected-review context-origin validation to identify repairable
    passing evidence.
  - Reuse toolgen registries to compute install-profile closure and router
    membership.
  - Reuse docs manifest generation to make new Diataxis pages visible, while
    shaping #170 content from GSD Core's docs taxonomy and Trellis's
    task-centered onboarding/scenario model.
- Convention deviations:
  - Toolgen will need a planning/apply split so generated operations can be
    collected before mutation.
  - A private generated-file ownership manifest is new, but it should stay
    toolgen-owned and not alter the public `SurfaceManifestRow` contract unless
    a public docs inventory change is explicitly needed.

### Risks

- High: evidence replacement could become too broad. Mitigation: only allow
  replacement when existing passing selected-review evidence is invalid under
  context-origin requirements or explicit fix/review alignment is active; retain
  current rejection for normal passing evidence.
- High: generated cleanup can lose user changes. Mitigation: classify files with
  a path/checksum ownership manifest, preserve unknown files, and back up or
  refuse managed-modified files.
- Medium: profile/router optimization can hide mandatory governance skills.
  Mitigation: hardcode/prove lifecycle-critical and sensitive-domain skills are
  always included in every profile closure.
- Medium: docs tutorials can drift or become generic. Mitigation: use GSD Core
  only for the Diataxis structure, use Trellis only for concrete onboarding and
  scenario patterns, keep snippets tied to current Slipway command surfaces, and
  update surface-manifest checks.
- Medium: analyzers can block legitimate golden/contract tests. Mitigation:
  explicit allow rules plus fixture tests for exceptions.
- Guardrail domains: `irreversible_operations` due generated-file cleanup and
  public evidence recovery. All paths fail closed to review/evidence.
- Reversibility: code/docs/test edits are git-revertible; generated adapter
  cleanup must itself be rollback-capable.

### Test Strategy

- Existing coverage:
  - `cmd/evidence_skill_test.go` covers evidence command guard behavior.
  - `internal/engine/progression/authority_test.go` covers context-origin
    authority blockers.
  - `internal/fsutil/transaction_test.go` covers rollback behavior.
  - `internal/toolgen/*_test.go` covers generated adapter and surface manifest
    contracts.
  - `internal/tmpl/templates_test.go` covers generated skill content.
- Infrastructure needs:
  - Toolgen operation planning hooks for deterministic transaction failure tests.
  - Ownership manifest fixtures covering pristine, modified, and unknown files.
  - Install-profile closure tests over generated skill definitions.
  - Analyzer fixture tests under a new `internal/testlint` package.
- Verification approach:
  - Focused tests for each issue seam first, then `go test ./...`.
  - `go run ./internal/toolgen/cmd/gen-surface-manifest --check` after docs and
    surface changes.
  - Fresh `gh issue list` before final tracker/issue closeout decisions.

### Options

- Option 1: sequential single-issue changes. Lower per-change blast radius but
  violates the user's request to resolve the current open set together and leaves
  #169 stale while child issues remain half-done.
- Option 2: integrated batch with shared safety boundaries. Implement the
  confirmed open issues in one governed change, but keep each issue behind a
  focused test seam and shared fail-closed rules. This best matches the user
  request and avoids duplicate toolgen/docs churn.
- Option 3: docs/toolgen-only slice now, defer evidence recovery and analyzers.
  Lower implementation risk, but leaves a confirmed public CLI dead end (#263)
  and test-quality gap (#161) unresolved.
- Selected: Option 2. The user explicitly asked to confirm the open issues are
  real developer-experience or optimization concerns and continue with a single
  batch. The selected approach keeps one governed worktree, issue-specific tests,
  and a shared safety rule: no bypass, no unsafe deletion, no hidden pruning of
  required governance skills.

## Unknowns

- Resolved: Are all current open issues real enough to act on? -> Yes. The live
  open list contains #263, #170, #169, #168, #167, and #161, and local source/docs
  checks confirmed each issue's described gap still exists.
- Resolved: Is #258 in scope through #169? -> No. `gh issue view 258` reports it
  closed; #169 is stale and must be updated/resolved against the live open set.
- Remaining: None.

## Assumptions

- The bound worktree `.worktrees/resolve-current-open-issues` is the change
  authority. Evidence: `slipway next --json --diagnostics` path authority.
- The user approved the integrated option by asking to continue after a second
  confirmation that each open issue has real developer-experience or optimization
  impact.
- Toolgen ownership metadata can be private to adapter refresh because the
  public surface manifest's current row schema is docs-oriented, not an ownership
  contract.

## Canonical References

- `cmd/evidence.go:1078-1110`
- `internal/engine/progression/authority.go:828-845`
- `internal/engine/progression/authority.go:1002-1006`
- `internal/fsutil/transaction.go:47-52`
- `internal/fsutil/transaction.go:149-172`
- `internal/toolgen/toolgen.go:824-1072`
- `internal/toolgen/toolgen.go:1102-1125`
- `internal/toolgen/surface_manifest.go:21-29`
- `.golangci.yaml:7-12`
- `mkdocs.yml:33-41`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/docs/README.md`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/docs/tutorials/your-first-project.md`
- `/Users/yixianlu/ghq/github.com/open-gsd/gsd-core/docs/tutorials/onboarding-an-existing-codebase.md`
- `https://github.com/mindfold-ai/Trellis`
- `https://docs.trytrellis.app/`
- `https://docs.trytrellis.app/start/install-and-first-task`
- `https://docs.trytrellis.app/start/how-it-works`
- `https://docs.trytrellis.app/start/real-world-scenarios`
- `artifacts/codebase/ARCHITECTURE.md`
- `artifacts/codebase/TESTING.md`
- `artifacts/codebase/CONCERNS.md`
