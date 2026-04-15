# Optimization Strategies

Mutation testing is embarrassingly parallel and embarrassingly slow. A naive run mutates every statement
in the codebase and runs the full test suite against each mutant; the cost is (number of mutants) x (test
runtime), which at even modest repository sizes is hours to days. The goal of this note is to get useful
mutation signal inside a bounded time budget — routinely 30 minutes for local work, 2 hours for CI.

## Scope Restriction

The single biggest lever. Most mutants are uninteresting — they live in code paths no one is currently
changing.

- **Diff-only.** Mutate only files changed in the current branch versus the main branch. Matches the
  review cadence and keeps runs well under 10 minutes for normal PRs.
- **Package-only.** Mutate only one module or package per run. Use when a subsystem is being overhauled.
- **Excluded dirs.** Unconditionally skip generated code, vendored code, test fixtures, migrations, and
  anything that is not the behavioral core. Exclusions live in the config file, not in CI scripts, so
  they survive runner changes.

Diff-only mutation testing is the default; everything else is a deliberate deviation.

## Incremental Runs

Re-mutating unchanged files wastes cycles. Most mature frameworks can cache per-mutant results keyed on
the source file hash plus the test selection.

- **mutmut** persists mutant status in `.mutmut-cache` — commit it on long-lived branches, or wire it to
  the CI cache.
- **PIT** supports incremental analysis via `--withHistory`.
- **Stryker** has an incremental mode backed by a report file; re-runs skip unchanged mutants.

An incremental mutation run on a warm cache is usually 5-10x faster than a cold run on the same diff.

## Parallelism Caps

Mutation runs burn cores. Uncapped parallelism starves the rest of the machine and can cause flaky tests
to fail under contention (timing-sensitive tests, locks, port binding).

- Local: cap at physical cores minus two. Laptops stay responsive, thermal throttling stays bounded.
- CI: cap at the runner's advertised cores, but also watch for noisy-neighbor tests (tests that pass
  serially and fail in parallel). Fix the tests rather than disabling parallelism globally.

## Mutant Sampling

When the mutant population is too large even for diff-only, sample.

- **Uniform random sampling.** Pick N mutants uniformly; cheapest to implement, produces an unbiased
  mutation-score estimator with known variance.
- **Stratified sampling.** Sample by mutator class (e.g. 20% of boundary mutants, 10% of conditional
  mutants) to ensure all mutator kinds are represented in the report.
- **Risk-weighted sampling.** Weight mutants in critical paths (auth, payments, data integrity) higher.
  Requires annotating the critical paths once.

Sampling reports a confidence interval instead of a point score. Accept that; a 75% ± 3% score in 30
minutes beats an exact 75% score in 8 hours.

## Test Selection

Do not run every test against every mutant.

- **Coverage-aware selection.** For each mutant, run only tests whose coverage includes the mutated line.
  Requires a recent coverage trace; refresh on every full test-suite run.
- **Test-impact analysis.** Go further: for each mutant, run only tests that cover the mutated line *and*
  have a dependency graph path to the mutant's module. Higher precision, higher setup cost.
- **Test ordering.** When running multiple tests per mutant, order by historical fail-first: tests that
  have killed mutants in this file before go first. Kills shorten mutant lifetimes dramatically.

A coverage-aware run typically reduces wall time by 60-90% at the cost of a build-time coverage pass.

## Timeout Tuning

A mutant that hangs is a mutant that blocks the run. Every mutation framework has a per-test timeout; the
default is almost always too generous.

- Set the timeout to `k * median_test_time`, with `k` in `[3, 10]`. Start at 5, tune down once you see the
  true test-time distribution.
- Treat timeouts as kills. An infinite loop introduced by a mutant is a caught mutation.
- Log timeout rate per mutator. If a single mutator (often "replace loop condition with true") produces
  an outsized share of timeouts, either tighten the timeout or disable that mutator.

## Caching Unchanged Mutants

Incremental runs (above) cache the full mutant verdict. Separately, cache:

- **Compilation artifacts.** Re-compiling for every mutant dominates runtime in compiled languages.
  Frameworks like PIT and mull patch bytecode/IR directly and avoid re-compilation.
- **Test fixture setup.** Long-running DB migrations, schema creation, or test-data seeds should live in
  a session-scoped fixture, not a function-scoped one.
- **Docker image pulls in CI.** Cache the mutation runner's container image; it does not change across
  PRs.

## CI vs. Local Profiles

Local and CI have different objectives and should not share a config.

- **Local profile.** Diff-only, fast, incremental cache on, moderate parallelism, timeout aggressive.
  Target: < 10 minutes on the mean PR.
- **CI profile (PR).** Diff-only, sampling if diff is huge, incremental cache on via shared artifact.
  Target: < 30 minutes, blocks merge on regression beyond a configured threshold.
- **CI profile (nightly / weekly).** Full-repo, no sampling, cold cache allowed, slowest timeouts.
  Writes the authoritative mutation-score baseline that PR runs compare against.

Run the nightly at a low priority and accept that it may take hours. Its output is the calibration for the
fast PR run, not a merge gate.

## A 30-Minute Budget Playbook

Given a typical PR in a typical mid-sized repo, the following knobs usually land a run inside 30 minutes
while preserving meaningful signal.

1. **Scope:** diff-only against the PR base. Exclude generated code, migrations, and tests.
2. **Cache:** enable incremental mode, backed by the CI cache keyed on the PR branch.
3. **Parallelism:** `min(cores, 8)` workers. Higher hurts more than it helps on standard CI runners.
4. **Test selection:** coverage-aware. Rebuild coverage once per PR on the first test run, reuse for
   mutation.
5. **Timeout:** `5 * median_test_time`, floor 5 seconds. Revisit per quarter.
6. **Sampling:** off by default. Turn on stratified sampling if the diff exceeds ~300 mutants; aim for
   150-200 sampled mutants.
7. **Mutator set:** language default minus the noisy ones (see `configuration.md`).
8. **Threshold:** fail the run on a >3% drop in mutation score versus the latest nightly baseline for the
   changed files. Do not gate on absolute score on PRs; gate on regression.

This configuration trades theoretical completeness for operable speed. The nightly run closes the loop.
