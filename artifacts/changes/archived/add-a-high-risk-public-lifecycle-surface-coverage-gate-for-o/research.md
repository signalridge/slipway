# Research

## Alternatives Considered

### Architecture
- Affected modules:
  - `internal/coverage/coverage.go` owns profile parsing, selected package
    baselines, fail-closed regression checks, and stable JSON marshaling
    (`internal/coverage/coverage.go:39`, `internal/coverage/coverage.go:117`,
    `internal/coverage/coverage.go:170`, `internal/coverage/coverage.go:239`).
  - `internal/coverage/cmd/covergate/main.go` is the CLI entry point. It has
    one baseline file, one kernel package set, and one validation path today
    (`internal/coverage/cmd/covergate/main.go:25`,
    `internal/coverage/cmd/covergate/main.go:31`,
    `internal/coverage/cmd/covergate/main.go:92`,
    `internal/coverage/cmd/covergate/main.go:164`).
  - `.github/workflows/ci.yml` has a single `Kernel Coverage Gate` job that
    measures only kernel packages and checks only `coverage-baseline.json`
    (`.github/workflows/ci.yml:94`, `.github/workflows/ci.yml:104`,
    `.github/workflows/ci.yml:111`, `.github/workflows/ci.yml:115`).
  - The existing committed baseline contains only the three governance-kernel
    packages (`coverage-baseline.json:1`, `coverage-baseline.json:3`,
    `coverage-baseline.json:8`).
  - Public lifecycle entry points are implemented under `cmd`, with concrete
    command factories for `status`, `next`, `validate`, `done`, and `evidence`
    (`cmd/status.go:155`, `cmd/next.go:314`, `cmd/validate.go:148`,
    `cmd/done.go:242`, `cmd/evidence.go:58`).
  - The required `internal/state` paths are load-bearing: verification path
    resolution fails closed on hidden sibling bundles
    (`internal/state/verification.go:62`, `internal/state/verification.go:86`,
    `internal/state/verification.go:131`), runtime state lives under the
    git-common runtime directory (`internal/state/store.go:52`,
    `internal/state/store.go:62`, `internal/state/store.go:109`), and worktree
    bindings are machine-local authority
    (`internal/state/worktree.go:64`, `internal/state/worktree.go:72`,
    `internal/state/store.go:556`).
- Dependency chains:
  - CI runs `go test -coverpkg=... -coverprofile=...`, then `covergate -check`.
  - `covergate` parses the profile via `coverage.ParseProfile`, loads a
    committed `coverage.Baseline`, validates required gated packages, and calls
    `Baseline.Check`.
  - Adding a new public-surface gate should reuse the same parser and baseline
    model, but it needs a distinct required package/surface catalog and a
    distinct baseline file.
- Blast radius:
  - Direct blast radius is `internal/coverage`, `internal/coverage/cmd/covergate`,
    the root coverage baseline artifact(s), docs for contributors, and the CI
    coverage job.
  - Indirect blast radius is PR acceptance: any change touching gated public
    surfaces can fail CI if tests no longer exercise the committed floors.
- Constraints:
  - Keep the existing governance-kernel gate and baseline in force.
  - Do not create compatibility shims or soft-pass paths.
  - Diagnostics must identify package/file/surface, not just an aggregate
    coverage percentage.

### Patterns
- Existing conventions:
  - Coverage data is package-level, union-deduplicated, rounded to one decimal,
    and encoded as reviewable JSON (`internal/coverage/coverage.go:39`,
    `internal/coverage/coverage.go:204`, `internal/coverage/coverage.go:260`).
  - `covergate` requires exactly one explicit mode, rejects write-only flags in
    check mode, and fails closed for invalid/missing baselines
    (`internal/coverage/cmd/covergate/main.go:92`,
    `internal/coverage/cmd/covergate/main.go:136`).
  - Baseline updates are visible PR diffs, not automatic CI mutations
    (`docs/contributing.md:111`, `docs/contributing.md:124`).
- Reusable abstractions:
  - `coverage.Baseline` and `coverage.Regression` can remain the core data
    model.
  - Required-package integrity helpers already exist for declared cover package
    sets and floors (`internal/coverage/coverage.go:177`,
    `internal/coverage/coverage.go:183`, `internal/coverage/coverage.go:189`).
- Convention deviations:
  - The current CLI text and docs are kernel-specific. This change must make
    the gate support a second named target without leaving a legacy kernel-only
    assumption in public behavior.
  - Package-level floors do not naturally name command-level surfaces. The new
    target needs a small static package-to-surface catalog for diagnostics.

### Risks
- Technical risks:
  - High: using one expanded baseline file could make kernel and public-surface
    floor changes hard to review independently.
  - Medium: public-surface `-coverpkg` over `cmd` plus `internal/state` can make
    the coverage job slower.
  - Medium: package-level Go coverage cannot prove every command entry point is
    behaviorally covered; it can only fail closed when the package/surface floor
    drops or disappears.
  - Low: adding a new CLI target can break existing tests if old messages are
    hard-coded.
- Guardrail domains:
  - No credential, auth, financial, schema migration, or irreversible external
    API domain is introduced.
  - CI enforcement is affected, so failed PR checks are expected and desired for
    coverage regressions.
- Reversibility:
  - The change is reversible by reverting the CLI support, new baseline, docs,
    and CI additions. No persisted user data is migrated.

### Test Strategy
- Existing coverage:
  - `internal/coverage/coverage_test.go` covers profile parsing, package
    selection, baseline integrity helpers, regression checks, and baseline
    loading.
  - `internal/coverage/cmd/covergate/main_test.go` covers CLI mode selection,
    write/check behavior, invalid kernel baseline detection, and helper profile
    fixtures.
  - There is no current test proving a public-surface target exists, has
    required package/surface diagnostics, or is enforced by CI.
- Infrastructure needs:
  - Add table-driven tests for target selection, public-surface baseline
    validation, diagnostics that include surface names, and CI command coverage.
  - Generate the public-surface baseline from a local full-suite coverage run
    with `-coverpkg` over the declared public-surface package set.
- Verification approach:
  - Unit tests: `go test ./internal/coverage -count=1` and
    `go test ./internal/coverage/cmd/covergate -count=1`.
  - CI contract check: update existing release/workflow contract coverage or add
    a focused workflow test asserting both kernel and public-surface gates run.
  - Full suite: `go test ./... -count=1` and `golangci-lint run ./...`.

### Options
- Option 1: Expand `coverage-baseline.json` to include kernel plus
  public-surface packages.
  - Pros: smallest number of files.
  - Cons: mixes risk tiers, makes kernel baseline reviews noisier, and weakens
    the requirement that the existing kernel baseline stays clearly preserved.
- Option 2: Add a named target to `covergate` and a distinct
  `coverage-public-surface-baseline.json`.
  - Pros: keeps the existing kernel baseline intact, lets CI run two explicit
    gates, and enables target-specific package/surface diagnostics.
  - Cons: slightly more CLI branching and one additional committed baseline.
- Option 3: Build a changed-line coverage tool now.
  - Pros: closer to the broad long-term ideal of changed-line coverage.
  - Cons: much larger scope, higher false-positive risk, and unnecessary to
    satisfy opt.md 3.2 because a tiered public-surface gate is acceptable.
- Selected: Option 2. It is the smallest durable change that satisfies opt.md
  3.2 while preserving the current governance-kernel baseline and avoiding any
  compatibility layer.

## Unknowns
- Resolved: Is the current gate limited to the governance kernel? -> Yes. The
  current package set is only gate/governance/progression and CI only measures
  those packages.
- Resolved: Can the new public-surface gate reuse existing coverage parser and
  baseline comparison semantics? -> Yes. The parser and baseline types are
  package-generic; only CLI target selection and diagnostics are kernel-specific.
- Resolved: Should `internal/toolgen` be included now? -> No. The intake scope
  says include it only if this implementation modifies that surface, and the
  selected option does not require toolgen changes.
- Remaining: None.

## Assumptions
- The user-approved scope allows selecting the target design without another
  interrupt because the user explicitly authorized automatic decisions and
  follow-on permissions in this session.
- Package-level tiered coverage is acceptable for this change because opt.md 3.2
  asks for changed-line or tiered coverage, and a package/surface tier is enough
  to fail closed for high-risk public lifecycle surfaces.
- A separate public-surface baseline is preferable to changing the existing
  kernel baseline because opt.md 3.2 explicitly says not to replace the current
  governance-kernel baseline.

## Canonical References
- `opt.md:219` to `opt.md:234`
- `coverage-baseline.json:1` to `coverage-baseline.json:13`
- `internal/coverage/coverage.go:39` to `internal/coverage/coverage.go:111`
- `internal/coverage/coverage.go:170` to `internal/coverage/coverage.go:214`
- `internal/coverage/coverage.go:217` to `internal/coverage/coverage.go:257`
- `internal/coverage/cmd/covergate/main.go:25` to
  `internal/coverage/cmd/covergate/main.go:35`
- `internal/coverage/cmd/covergate/main.go:92` to
  `internal/coverage/cmd/covergate/main.go:157`
- `internal/coverage/cmd/covergate/main.go:164` to
  `internal/coverage/cmd/covergate/main.go:174`
- `.github/workflows/ci.yml:94` to `.github/workflows/ci.yml:115`
- `cmd/status.go:155`, `cmd/next.go:314`, `cmd/validate.go:148`,
  `cmd/done.go:242`, `cmd/evidence.go:58`
- `internal/state/verification.go:50` to `internal/state/verification.go:135`
- `internal/state/store.go:52` to `internal/state/store.go:113`
- `internal/state/store.go:526` to `internal/state/store.go:567`
- `internal/state/local_runtime_paths.go:44` to
  `internal/state/local_runtime_paths.go:90`
- `internal/state/worktree.go:64` to `internal/state/worktree.go:125`
