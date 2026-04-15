# Library Selection Guide

Pick the property-testing library that matches the ecosystem's idioms. A library fighting the language's
conventions costs more in review friction than it saves in generator expressiveness. Notes below focus on
the three things that matter in practice: shrinking quality, stateful-test support, and generator API
ergonomics.

## Python — Hypothesis

The strongest general-purpose property-testing library across languages. Use unless there is a specific
reason not to.

- **Shrinking:** excellent. Integrated, deterministic, and preserves structure across composed strategies.
- **Stateful:** first-class `RuleBasedStateMachine`. Commands, invariants, and preconditions are decorator-driven.
- **Generators:** `strategies` module is composable and exhaustive — `lists`, `dictionaries`, `recursive`,
  `builds`, `from_regex`, `from_type`. `@given` attaches strategies to pytest tests.
- **Notable extras:** `@example` for regression-locked cases, `@seed` for determinism, the example
  database for automatic regression persistence, `deadline` for perf contracts.
- **Decision line:** reach for it whenever you have pytest and a property to state.

## JavaScript / TypeScript — fast-check

The de-facto choice for JS/TS.

- **Shrinking:** very good. Integrated shrinkers per arbitrary, and `fc.asyncProperty` handles async.
- **Stateful:** supports command-based model testing via `fc.commands` / `fc.modelRun`.
- **Generators:** `fc.integer`, `fc.string`, `fc.record`, `fc.letrec` for recursion, `fc.tuple`,
  `fc.oneof`. TypeScript types flow through composition cleanly.
- **Notable extras:** works inside Jest, Vitest, Mocha, and node:test; a `ava`-style runner is also supported.
- **Decision line:** pick it for any JS/TS codebase; no serious competitor.

## Rust — proptest, quickcheck

Two choices; they differ in philosophy.

- **proptest**
  - Shrinking: strategy-driven, integrated, high quality.
  - Stateful: supported via `proptest-state-machine` crate.
  - Generators: `Strategy` trait composes with combinators; macros for structs/enums.
  - Picks: prefer for new code; shrinking is the decisive advantage.
- **quickcheck**
  - Shrinking: trait-based, requires implementing `Arbitrary::shrink` manually for custom types.
  - Stateful: not first-class.
  - Generators: lightweight, minimal ceremony.
  - Picks: prefer for small utilities or ports of existing Haskell properties.

## Go — gopter, rapid

The standard library's `testing/quick` is weak (no shrinking, fixed generator set) — avoid for anything
non-trivial.

- **rapid**
  - Shrinking: integrated, binary-search style, works out of the box.
  - Stateful: `rapid.Check` supports state-machine tests via `*rapid.T.Repeat`.
  - Generators: `rapid.Int`, `rapid.SliceOf`, `rapid.Custom`; generics-friendly in modern Go.
  - Picks: default for new Go code.
- **gopter**
  - Shrinking: available, less polished than rapid.
  - Stateful: state-machine harness available but verbose.
  - Generators: broad set; API is older and heavier.
  - Picks: consider when gopter is already in use or rapid's API feels too minimal.

## Java / JVM — jqwik

The modern JVM choice.

- **Shrinking:** integrated and deterministic.
- **Stateful:** action-based; `ActionChain` models sequences against a tracked state.
- **Generators:** `Arbitraries` provides primitives; `@ForAll`, `@Provide`, and combinators compose well.
- **Notable extras:** JUnit 5 native integration, lifecycle hooks, statistics collection via `Statistics.collect`.
- **Decision line:** preferred over junit-quickcheck for new code; both coexist in long-lived codebases.

## Haskell — QuickCheck, SmallCheck, Hedgehog

Three choices, three philosophies.

- **QuickCheck**
  - Shrinking: manual, type-class driven.
  - Stateful: via `quickcheck-state-machine`.
  - Generators: `Arbitrary` typeclass; widely understood.
- **SmallCheck**
  - Shrinking: unnecessary — enumerates smallest inputs exhaustively up to a depth.
  - Stateful: limited.
  - Generators: depth-bounded enumeration.
  - Picks: use when exhaustive small inputs are more valuable than randomized coverage.
- **Hedgehog**
  - Shrinking: integrated with generators (like Hypothesis and proptest).
  - Stateful: first-class, strong state-machine harness.
  - Generators: monadic, composable, shrinker-preserving.
  - Picks: preferred for new Haskell projects over QuickCheck.

## Summary Table

| Ecosystem  | Library        | Shrinking  | Stateful     | Generator ergonomics |
|------------|----------------|------------|--------------|----------------------|
| Python     | Hypothesis     | excellent  | first-class  | excellent            |
| JS / TS    | fast-check     | very good  | supported    | very good            |
| Rust       | proptest       | integrated | via crate    | good                 |
| Rust       | quickcheck     | manual     | no           | minimal              |
| Go         | rapid          | integrated | supported    | modern, generics     |
| Go         | gopter         | available  | verbose      | older                |
| JVM        | jqwik          | integrated | first-class  | good                 |
| Haskell    | Hedgehog       | integrated | first-class  | monadic              |
| Haskell    | QuickCheck     | manual     | via package  | typeclass            |
| Haskell    | SmallCheck     | n/a (enum) | limited      | depth-bounded        |

## When to Switch from Examples to Properties

Move from examples to properties when any of these hold:

1. You have written more than about five example tests for the same function, and they feel
   copy-paste-modified rather than distinct.
2. A bug escaped your example suite and, after fixing, you can name the class of inputs your examples
   missed ("non-ASCII", "empty", "large").
3. You are about to refactor and want a safety net that does not encode the old implementation's shape.
4. The function has an obvious invariant, oracle, or round-trip partner.
5. You are reviewing a PR that changes a pure function's hot path — ask for one property before approving.

Conversely, stay with examples when: the function's behavior is defined case-by-case (business rules with
no underlying algebra), the input space is tiny (enums with four values), or you cannot state a property
in one sentence. Property tests built on weak properties are worse than strong example suites.
