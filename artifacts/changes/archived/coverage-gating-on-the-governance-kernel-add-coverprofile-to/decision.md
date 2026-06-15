# Decision

## Alternatives Considered
Three approaches were evaluated in `research.md` (`## Alternatives Considered`):
- **A — Dedicated ubuntu coverage CI job + in-repo checker with committed baseline, holistic full-suite `-coverpkg`.** Truest signal (credits integration tests exercising the kernel), deterministic single-OS, self-contained, reuses the `gen-surface-manifest` `-check`/`-write` pattern, gate logic is itself unit-tested. Cost: one CI job ~as long as the test job, plus a new tool package and baseline file.
- **B — Same checker but kernel-tests-only coverage.** Faster (~2 min) but under-counts because it ignores integration tests in `cmd/`/`state`/`action` that exercise the kernel; the floor is pessimistic and a refactor that moves coverage into integration tests could falsely trip the gate.
- **C — Minimal: `-coverprofile` on the existing matrix test job + a hardcoded floor asserted inside a Go test.** Smallest change but produces 3-OS profile divergence, cannot cleanly read its own full-suite profile, and a hardcoded floor has no clean ratchet-update workflow. Rejected as the weakest design.

## Selected Approach
**Approach A**, confirmed by the user on 2026-06-15T06:56:18Z.

A new ubuntu-only `coverage` CI job runs the full suite with `-coverpkg` scoped to the governance-kernel packages — `internal/engine/{gate,governance,progression}` (the readiness resolver is `internal/engine/progression/readiness.go`) — producing one coverage profile. A self-contained in-repo checker (`internal/coverage` core + `internal/coverage/cmd/covergate` CLI, mirroring `internal/toolgen/cmd/gen-surface-manifest`) parses that profile with **union** semantics (a block is covered if any test binary hit it), computes per-kernel-package coverage, and compares it to a **committed baseline file**. `covergate -check` fails closed (non-zero exit) if any kernel package drops below its baseline; `covergate -write` regenerates the baseline (the no-regression ratchet). The baseline is committed, so any downward change is visible in the PR diff and reviewed. There is no skip/force/soft-pass path.

Rationale: it is the only option that gives a representative safety signal (whole-suite exercise of the kernel), stays deterministic and self-contained, and reuses a proven repo pattern whose gate logic can itself be tested.

## Interfaces and Data Flow
- **New dev/CI tool** `internal/coverage/cmd/covergate` with flags `-check` (compare profile to committed baseline; non-zero on regression) and `-write` (regenerate baseline). Mirrors the `gen-surface-manifest` `-check`/`-write` contract and its remediation message style.
- **New committed data file**: a coverage baseline (per-kernel-package floor), JSON. Final path (`coverage-baseline.json` at repo root vs `docs/COVERAGE-BASELINE.json`) settled in planning.
- **CI data flow**: `coverage` job → `go test ./... -coverpkg=<3 kernel pkgs> -coverprofile=coverage.out -count=1` → `covergate -check -profile coverage.out -baseline <file>` → pass/fail.
- **No change** to runtime/product interfaces, the engine, or existing governance gates. Additive only.

## Rollout and Rollback
- **Rollout**: add the `internal/coverage` package + `covergate` tool, generate the baseline via `covergate -write`, add the ubuntu-only `coverage` job to `.github/workflows/ci.yml`, and document the gate + ratchet workflow. The job becomes a required check once green.
- **Rollback**: remove the `coverage` job from `ci.yml` (and optionally delete the tool package and baseline file). Verification command: `go build ./...` and a CI run that is green without the coverage job. Fully reversible because the change is additive and touches no product code.

## Risk
- **[medium] Duplicate-block under-count.** `-coverpkg` over a multi-package run emits the same block once per test binary; summing instead of unioning inflates denominators ~3× (observed: progression 23.9% naive vs 81.8% union in the committed full-suite baseline). Mitigation: a dedicated union/dedup unit test on the checker's parser.
- **[low] Baseline gaming.** A regression could be masked by editing the committed baseline down. Mitigation: the baseline is committed and any downward edit shows in the PR diff for review; the gate never auto-lowers it.
- **[low] OS / toolchain nondeterminism.** Mitigation: pin the job to ubuntu; the kernel packages contain no OS-tagged files (verified), so coverage is stable.
- **[low] CI time.** A full-suite coverage run is ~as long as the existing test job; it runs as a parallel job and does not block the OS matrix.
- **Guardrail domains**: none (CI/test infrastructure). The gate is itself a safety mechanism and its CI/CLI behavior is reviewed as a contract surface.
