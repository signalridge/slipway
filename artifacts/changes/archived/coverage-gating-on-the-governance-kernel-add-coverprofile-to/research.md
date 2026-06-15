# Research

## Research Findings

### Architecture
- **Affected modules:**
  - `.github/workflows/ci.yml` — add a coverage job (current test job: line 79 `go test -timeout=20m ./... -count=1`; race job line 91; no `-cover` anywhere).
  - New checker package + tool, e.g. `internal/coverage/` (core) + `internal/coverage/cmd/covergate/main.go` (CLI), mirroring the existing `internal/toolgen/cmd/gen-surface-manifest/main.go` `-check`/`-write` tool.
  - New committed baseline file (the ratchet floor).
  - `justfile` already has a `coverage:` recipe (`go test ./... -coverprofile=coverage.txt` + `go tool cover -func`); extend with a kernel-scoped gate recipe.
  - Docs: `docs/operator-guide.md` / `docs/contributing.md` note.
- **Governance-kernel package set (resolves Open Question #1):** `internal/engine/gate`, `internal/engine/governance`, `internal/engine/progression`. The "readiness resolver" the issue names is `internal/engine/progression/readiness.go` (`EvaluateGovernanceReadiness`, `GovernanceReadiness`, `ArtifactReadiness`) plus `authority.go`/`advance_governed.go`/`autopass.go`/`validation.go` — all inside `internal/engine/progression`. So gate/readiness/progression collapse to these three packages.
- **Dependency chains:** the kernel packages are imported by `cmd/`, `internal/engine/action`, `internal/state`, `internal/engine/status` etc., so integration tests in those packages also exercise kernel branches.
- **Blast radius:** additive only — a new CI job, a new tool package, a committed baseline file, doc text. No change to runtime/product code or existing gates.
- **Constraints / invariants:** the gate must fail closed (no skip/soft-pass); baseline committed in-repo; deterministic across runs.

### Patterns
- **Reusable pattern (the key anchor):** `internal/toolgen/cmd/gen-surface-manifest/main.go` is a standalone `package main` tool with `-check` (fail if the committed `docs/SURFACE-MANIFEST.json` is stale) and `-write` (regenerate). Staleness is enforced by a Go test (`internal/toolgen/surface_manifest_test.go:140` fatals with "run `… --write`"). This is the exact pattern to mirror for a coverage baseline: committed file + `-check`/`-write` + clear remediation message.
- **CI pattern:** dedicated single-purpose jobs already exist (`race` is ubuntu-only, separate from the 3-OS `test` matrix). A new ubuntu-only `coverage` job fits this convention.
- **Go tooling:** Go 1.26.4; `go test -coverpkg=<list> -coverprofile`, `go tool cover -func`.

### Risks
- **[medium] `-coverpkg` emits duplicate block entries.** Running `go test ./... -coverpkg=<kernel>` instruments the kernel in *every* test binary, so the merged profile contains the same code block once per test binary. The checker MUST union blocks (covered if ANY entry hits), exactly as `go tool cover` does. Naive summing inflates the denominator ~3× and badly under-reports coverage (observed: naive sum gave progression 23.9% vs correct union 71.8%). This is the single most important implementation detail; the checker's parser needs a dedup test.
- **[medium] OS/float determinism.** Kernel packages have **no** OS-tagged files (verified: no `_unix/_windows/_darwin/_linux/_other.go` in the three packages), so coverage is OS-stable — but the gate must still pin to a single OS (ubuntu) to avoid matrix divergence and toolchain drift. Coverage is a rational number; compare with `current < baseline` (fail on any drop), no negative tolerance.
- **[low] Baseline gaming.** A regression could be hidden by lowering the committed baseline. Mitigation: the baseline file is committed, so any downward edit shows in the PR diff and is caught in review — never auto-lowered by the gate.
- **[low] CI cost.** A full-suite coverage run is roughly as long as the existing test job (progression tests alone ~108s); acceptable as a parallel job that does not block the matrix.
- **Guardrail domains:** none (CI/test infrastructure). The gate is itself a safety mechanism, reviewed as a CI contract surface.
- **Reversibility:** fully reversible (delete job + tool + baseline).

### Test Strategy
- **Existing coverage (kernel-tests-only baseline, union semantics, measured this run):**
  - `internal/engine/gate` — **84.4%** (65/77 stmts)
  - `internal/engine/governance` — **88.0%** (746/848 stmts)
  - `internal/engine/progression` — **71.8%** (2442/3399 stmts)
  - kernel aggregate — **75.2%** (matches `go tool cover -func` total). Full-suite numbers (incl. cmd/state/action integration tests) will be ≥ these.
- **Infrastructure needs:** the checker's coverage-profile parser + per-package aggregator + baseline comparator must be unit-tested, including: (a) a duplicate-block/union test, (b) a fail-closed test where a simulated drop returns non-zero, (c) an exclusion-list test, (d) a missing/unreadable baseline test.
- **Verification approach per acceptance signal:**
  - "CI emits a coverage profile" → the coverage job uploads/produces `coverage.out`.
  - "a simulated drop fails CI" → a checker unit test feeds a below-baseline profile and asserts non-zero exit; optionally a documented manual demo.
  - "exclusion list documented + applied" → checker test with an excluded package present in the profile but not gated.
  - "no skip/soft-pass path" → review the checker for the absence of any env/flag bypass.

## Alternatives Considered
- **Approach A — Dedicated ubuntu coverage job + in-repo checker with committed baseline; holistic full-suite `-coverpkg` (RECOMMENDED).** New ubuntu-only `coverage` CI job runs the full suite with `-coverpkg` scoped to the 3 kernel packages, then runs the checker (`-check`) against `coverage-baseline.json`; `-write` ratchets the baseline. Checker core unit-tested incl. fail-closed + union dedup.
  - Tradeoffs: + truest safety signal (credits integration tests exercising the kernel); + deterministic single-OS; + self-contained; + reuses the proven gen-surface-manifest pattern; + the gate logic is itself testable. − adds a CI job ~as long as the test job; − one new tool package + baseline file to maintain.
- **Approach B — Kernel-tests-only coverage, same checker/baseline.** Coverage job runs only the 3 kernel packages' tests (~2 min).
  - Tradeoffs: + faster; + isolates the kernel's own test quality. − under-counts (ignores integration tests that exercise the kernel) so the floor is pessimistic; − a refactor moving coverage into integration tests could falsely trip the gate; − less representative of real exercised safety.
- **Approach C — Minimal: add `-coverprofile` to the existing matrix test job + assert a hardcoded floor inside a Go test.** No new tool.
  - Tradeoffs: + smallest change. − the matrix produces 3 OS profiles with possible divergence; − a normal test can't cleanly read its own full-suite profile (chicken-and-egg); − a hardcoded floor in test code mixes baseline data with code and has no clean ratchet-update workflow. Rejected as the weakest design.
- **Selected:** Approach A — pending user confirmation in the clarification round; rationale recorded in `decision.md`.

## Unknowns
- Resolved: Exact kernel package set -> `internal/engine/{gate,governance,progression}` (readiness resolver is `progression/readiness.go`).
- Resolved: Coverage attribution mechanism -> full-suite `-coverpkg=<kernel>` with **union** dedup in the checker (Approach A); naive per-block summing is wrong.
- Resolved: Baseline storage + ratchet workflow -> committed JSON baseline + `-check`/`-write` tool mirroring `gen-surface-manifest`; legitimate baseline changes appear in the PR diff and are reviewed.
- Remaining: Final baseline file path/name (`coverage-baseline.json` at repo root vs `docs/COVERAGE-BASELINE.json`) and exact tool package path — minor placement decisions for `slipway-plan-audit`/planning to settle.

## Assumptions
- The full suite already passes within the 20m CI timeout - Evidence: `.github/workflows/ci.yml:79` runs `go test -timeout=20m ./... -count=1` today.
- Kernel coverage is OS-deterministic - Evidence: no OS-tagged files in `internal/engine/{gate,governance,progression}` (verified via filename scan this run).
- `go tool cover` union semantics are authoritative - Evidence: recomputed per-package union totals reconcile exactly with `go tool cover -func` aggregate (75.2%).

## Canonical References
- `.github/workflows/ci.yml` (test job line 79, race job line 91) — CI surface to extend.
- `internal/toolgen/cmd/gen-surface-manifest/main.go` — `-check`/`-write` tool pattern to mirror.
- `internal/toolgen/surface_manifest.go` (`SurfaceManifestPath`) and `internal/toolgen/surface_manifest_test.go:140` — committed-file + Go-test enforcement pattern.
- `internal/engine/progression/readiness.go` — the readiness resolver (kernel membership).
- `justfile` (`coverage:` recipe) — existing coverage measurement to extend.
