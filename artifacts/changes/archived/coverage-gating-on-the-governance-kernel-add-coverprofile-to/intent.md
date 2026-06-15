# Intent

## Summary
Coverage gating on the governance kernel: add `-coverprofile` to the CI test run, gate a **no-regression coverage floor** on the governance-kernel packages (gate / readiness / progression logic) that fails closed on any drop, and document an explicit exclusion list. The gate is self-contained in-repo (no external service). Mutation testing is deferred. Issue #162.

## Complexity Assessment
complex
<!-- Rationale -->
Touches CI workflow config, introduces a new fail-closed gate mechanism (baseline + checker), and requires identifying the exact kernel package set via discovery. Multi-file, multi-step, with a safety-critical gate whose contract must fail closed.

## Guardrail Domains
none — coverage gating is CI/test infrastructure, not Auth/AuthZ, Credentials/PII, Financial, Schema migration, Irreversible ops, or External API contracts. The gate is itself a safety mechanism, so its public CI behavior is reviewed as a contract surface, but no sensitive guardrail domain applies.

## In Scope
- `.github/workflows/ci.yml`: add `-coverprofile` (with `-coverpkg` scoped to the kernel packages) to the test run so CI emits per-package coverage for the governance kernel.
- A committed baseline thresholds file recording current per-kernel-package coverage (the ratchet floor).
- A self-contained in-repo checker (Go tool under `cmd/` or equivalent) that parses the coverage profile and fails CI if any kernel package drops below its baseline. No external service, token, or network.
- An explicit, documented exclusion list (generated / vendored / test-only packages).
- Docs: a brief operator/contributing note describing the coverage gate and how a legitimate baseline change is made (ratchet up).

## Out of Scope
- Mutation testing (gremlins / go-mutesting) — explicitly deferred per the issue's long-horizon note.
- Repo-wide coverage number or gating non-kernel packages.
- External coverage services (Codecov etc.) — self-contained gate chosen.
- Backfilling tests to raise actual coverage — the ratchet only blocks regressions; writing new tests to hit a higher target is separate work.

## Constraints
- Self-contained and deterministic: baseline committed in-repo, no external dependency; the gate fails closed with no skip/force/soft-pass escape hatch.
- Coverage instrumentation overhead must fit the existing CI test timeout (20m).
- Exact kernel package set is determined in the discovery stage (see Open Questions).

## Acceptance Signals
- CI emits a coverage profile for the kernel packages on every run.
- A simulated coverage drop in a kernel package fails CI (gate proven to fail closed); an unchanged/improved run passes.
- The exclusion list is documented and applied by the checker.
- No skip / force / soft-pass path exists in the gate.

## Open Questions
- [x] Exact governance-kernel package set to gate — RESOLVED in research.md: `internal/engine/{gate,governance,progression}` (the readiness resolver is `internal/engine/progression/readiness.go`).
- [x] Coverage attribution mechanism — RESOLVED in research.md: full-suite `go test -coverpkg=<kernel>` with **union** dedup in the checker (naive per-block summing inflates denominators ~3× and is wrong).
- [x] Baseline storage format and ratchet-update workflow — RESOLVED in research.md/decision.md: committed JSON baseline (`coverage-baseline.json`) + `covergate -check`/`-write` mirroring `gen-surface-manifest`; a legitimate baseline change appears in the PR diff and is reviewed, never auto-lowered.

## Deferred Ideas
- Incremental mutation testing scoped to changed kernel packages, revisited once a coverage baseline exists and the Go mutation toolchain is worth the CI budget.
- An aspirational absolute coverage target to ratchet the floor toward over time.

## Approved Summary
Add `-coverprofile`/`-coverpkg` coverage measurement for the governance-kernel packages to CI, plus a self-contained in-repo gate that fails CI closed if any kernel package's coverage drops below a committed baseline (no-regression ratchet), and ship a documented exclusion list.

- **In scope:** CI coverage emission, committed baseline file, in-repo checker, exclusion list, brief docs.
- **Out of scope:** mutation testing (deferred), external coverage services (Codecov), repo-wide coverage, backfilling tests to raise coverage.
- **Primary acceptance signal:** a simulated coverage drop in a kernel package fails CI; an unchanged/improved run passes; no skip/force/soft-pass path exists.
- **Resolved in discovery:** exact kernel package set, coverage-attribution mechanism, baseline format + ratchet-update workflow.

Confirmed by user: 2026-06-15T06:46:57Z
