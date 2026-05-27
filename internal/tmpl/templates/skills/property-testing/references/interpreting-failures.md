# Interpreting Failures

A property test that fails delivers two things: a counterexample and a seed. Read both carefully. Most
wasted debugging time on property tests comes from mistrusting the library's output or chasing the
original draw instead of the shrunk one.

## Shrinking vs. Minimization

**Shrinking** is the library reducing a failing input toward a local minimum that still fails. It is
heuristic: the library does not prove minimality, it tries smaller variants until no smaller variant
fails. **Minimization** in the mathematical sense — the provably smallest failure — is rarely what
property-testing libraries deliver, and rarely what you need.

Practical consequence: a shrunk counterexample is a useful bug report, not a proof of the simplest case.
Do not spend hours trying to shrink it further by hand unless the reported input is genuinely too large to
reason about.

## Trust the Shrunk Input

The original failing draw was typically much larger than the shrunk result. The original is informative
only in one way: it confirms that the bug is reachable from realistic-shaped inputs. Otherwise, debug
against the shrunk input exclusively.

Reasons to go back to the original draw:

- The shrunk input no longer fails when you copy it into a unit test (→ see "property-statement bug"
  below).
- The shrunk input fails for a different reason than the original (→ you may have two bugs; file both).
- The property has side effects and shrinking interacts with them (→ your property is probably impure;
  fix that first).

## Reproducing with Seeds

Every mainstream property-testing library records a seed and exposes a way to replay. Replay is the
fastest path from a flake report to a deterministic failure.

- **Hypothesis:** the example database auto-replays most regressions; pin explicit failures with
  `@example(...)` or re-run with `@seed(...)`.
- **fast-check:** replay with `{ seed, path, endOnFailure: true }` options; copy from the failure output.
- **proptest:** regressions persist in `proptest-regressions/` files; commit them.
- **rapid:** reproduce with `-rapid.seed=N -rapid.failfile=...`.
- **jqwik:** `@Property(seed = "…")` or the printed `tries` / `seed` pair.
- **Hedgehog / QuickCheck:** re-run with the printed seed; Hedgehog also supports `recheck`.

If a reported failure does not reproduce under its printed seed, the property or generator has
non-determinism. Treat that as a higher-priority bug than the original failure.

## Diagnosing Flaky Properties

A flaky property — fails sometimes, passes sometimes, under the same seed — is always a property-author
bug, not a library bug. Usual causes:

1. **Nondeterministic generators.** A generator that calls `time.time()`, `random.random()` directly, or
   reads from a global RNG not controlled by the library. Fix: route all randomness through the library's
   seeded RNG.
2. **Global state in the SUT.** Singleton caches, process-wide counters, thread-local state that leaks
   between runs. Fix: reset state in a setup/teardown hook, or make the function pure for the test.
3. **Time and locale.** `datetime.now()`, `time.sleep`, locale-sensitive formatting, time-zone defaults.
   Fix: inject a clock, pin a locale, or freeze time in the test harness.
4. **Concurrency.** Properties that spawn threads or tasks and assert on observable order. Fix: assert on
   causal order or use a state-machine harness that sequences operations.
5. **External resources.** Files, network, databases. Fix: fakes or in-memory substitutes; property tests
   should not hit the network.

A property that cannot be made deterministic should be deleted and replaced with a targeted integration
test. Flaky properties poison the signal of the whole suite.

## Genuine Bug vs. Property-Statement Bug

Not every failing property points at SUT code. Three failure modes need separate responses.

- **SUT bug.** The property is correctly stated, the generator is healthy, and the shrunk input exposes a
  real defect. Fix the SUT; lock in the failure with an explicit `@example` or regression file.
- **Property-statement bug.** The property asserts something that is not actually true of the SUT —
  sometimes because the spec is subtler than the author thought (e.g. IEEE-754 addition is not
  associative; unicode normalization is not commutative with case-folding). Fix the property; add a
  comment explaining why the naive statement fails.
- **Generator bug.** The generator produces values the SUT was never meant to handle — for example,
  negative sizes, null bytes in filenames on Windows, or pre-conditions the spec explicitly excludes. Fix
  the generator; push the constraint into construction, not filtering.

When unsure which category you are in, paste the shrunk counterexample into a standalone unit test and
run only that. If it still fails, the SUT bug is real. If it passes, the property or generator is lying.

## Regression Locking

Once a shrunk counterexample is diagnosed, lock it in so it cannot silently regress.

- Add an explicit example: `@example(...)` (Hypothesis), `@Example` (jqwik), handwritten unit test
  otherwise. Explicit examples run on every invocation, not just under matching seeds.
- Commit the regression file if the library produces one (`proptest-regressions/`, Hypothesis database if
  your policy commits it).
- Write a one-line comment explaining what bug this case caught. Future refactors will want to delete
  "inexplicable" examples; the comment is what prevents that.

## A Short Debug Protocol

1. Read the shrunk counterexample. Write it down verbatim.
2. Confirm it reproduces under the printed seed. If not, treat flakiness as the primary bug.
3. Copy the shrunk input into a standalone test and run in isolation.
4. If it passes in isolation, suspect global state or an impure property.
5. If it fails in isolation, fix the SUT or the property statement.
6. Lock the case in with an explicit example and a one-line comment.
7. Re-run the property suite at an elevated example count (10x default) to confirm no sibling bugs remain.
