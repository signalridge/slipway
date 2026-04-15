# Reviewing Property-Based Tests

Use when triaging an existing property-test suite — your own, a teammate's,
or one you inherited. Sort findings by severity so you flip the
test-is-broken cases before nit-picking settings.

## Severity triage

| Issue | Severity | Quick detector | Fix |
|-------|----------|----------------|-----|
| Tautological assertion | CRITICAL | LHS and RHS are the same expression | Rewrite around the actual property |
| Vacuous test | CRITICAL | Contradictory or pinhole `assume()` | Redesign the strategy, drop filters |
| Missing / weak assertion | HIGH | Body has no `assert`, or only `isinstance` | Replace with a contract-level assertion |
| Function reimplemented in test | HIGH | Assertion duplicates the implementation | Swap for an algebraic property |
| Narrow strategy | MEDIUM | Tight `min_value` / `max_value`, no `@example` | Widen strategy, pin edge cases |
| Missing stronger property | MEDIUM | Only length/type checked | Add ordering, round-trip, or idempotence |
| Poor settings | LOW | `max_examples` tiny, no `deadline` | Set budgets appropriate to the context |

## The four critical shapes

**Tautology.** `assert sorted(xs) == sorted(xs)` passes even on a broken
`sorted`. Detect by reading the assertion alone — if the symbol under test
appears on both sides, or not at all, the property is not about the SUT.

**Vacuous.** `assume(x > 100); assume(x < 50)` rejects every input; the
framework silently reports zero effective examples. Look for multiple
`assume()` calls or equality filters (`assume(x == 42)`).

**Weak.** A body that only calls the function ("no crash") is a smoke
test, not a property. `assert isinstance(compute(x), int)` barely rises
above that. If deleting the body still lets the test pass, strengthen it.

**Reimplementation.** `assert add(a, b) == a + b` is vacuous when `add` is
defined as `return a + b`. Swap to an algebraic law (commutativity,
associativity) or a metamorphic relation against a reference.

## Locating tests across libraries

| Stack | Search pattern |
|-------|----------------|
| Python / Hypothesis | `rg "@given\(" --type py` |
| JS / fast-check | `rg "fc\.(assert\|property)"` over `.ts,.js` |
| Rust / proptest | `rg "proptest!"` |
| Go / gopter, rapid | `rg "rapid\.Check\|gopter\."` |

## Review sequence

1. **List candidates** with the patterns above. Skim names for shape
   (`test_*_commutative`, `test_*_roundtrip` are good signals).
2. **Classify each test** against the severity table. Stop and fix any
   CRITICAL entry before reading further — those tests actively produce
   false confidence.
3. **Evaluate shrinking.** Trigger a deliberate failure locally (inject a
   `raise` or negate the property) and check the shrunk counterexample is
   readable. Opaque shrinks usually mean the strategy is over-composed.
4. **Check determinism.** Walltime, RNG, network, floating-point comparison
   without tolerance, and global state all cause flakes under shrink.
5. **Suggest stronger properties** where only minimal guarantees are
   asserted. Prefer round-trip > idempotence > type-preservation > no-crash.

## Mutation probe

Before approving a test, propose two or three mutations the SUT *should*
fail under and confirm the suite catches each:

```
sort returns input unchanged       → expect failure in ordering property
sort drops last element            → expect failure in length-preservation
sort uses reverse=True internally  → expect failure in ordering property
```

If the suite is silent on any of these, it does not actually pin the
invariant it claims to. Use mutation-testing tools (`mutmut`, Stryker,
cargo-mutants) to automate this sweep.

## Test-health scorecard

| Category | 1–5 | What raises the score |
|----------|-----|------------------------|
| Property strength | — | Contract-level invariants, metamorphic relations |
| Input coverage | — | Wide strategy, explicit `@example` edges |
| Assertion quality | — | Non-tautological, distinct from impl |
| Settings | — | `max_examples` and `deadline` match CI budget |

Record the scorecard in the review output when approving or requesting
changes — it gives the author a concrete list, not just "LGTM".

## Red flags that block approval

- Any tautology. `assert x == x` is never a valid property.
- "No crash" treated as sufficient without an explicit rationale.
- Vacuous tests shipped as "coverage".
- Reimplementation-style assertions hiding under property-test decorators.
- Silent acceptance of narrow `@given` ranges without an accompanying
  `@example` pinning the boundary.
