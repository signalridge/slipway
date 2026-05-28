# Designing Properties

Property tests fail well only when the property is worth asserting. Most bad property tests are not bad
generators — they are restatements of the implementation in assertion form. This note is about picking
properties that catch bugs the implementation's author did not anticipate.

## Invariants vs. Examples

An example test names one input and one expected output. A property test names a relationship that must
hold for an entire class of inputs. The relationship is useful only if:

1. It is cheaper to state than to enumerate examples.
2. It is unlikely to be accidentally true.
3. Its failure localizes a defect.

If any of these fail, write examples. "For all x, sort(x) == sort(x)" satisfies none — it is true for any
pure function and catches no bug. "For all x, sort(x) is non-decreasing AND is a permutation of x"
satisfies all three.

## Property Classes

### Oracle Properties

You have a second implementation — slow-but-obviously-correct, a reference spec, or a mathematical
identity — and assert the system under test agrees with it on all inputs.

- **Reference implementation.** Replace `fast_sort(x) == slow_sort(x)`. The slow version is the oracle.
- **Model checker.** A higher-level model produces the same observable trace as the real system.
- **Mathematical identity.** `pow(x, n) == pow(x, n/2) * pow(x, n/2) * (x if n odd)`.

Oracle properties are the strongest class when an oracle exists. They fail cleanly — any divergence is a
bug in one side or the other.

### Round-Trip Properties

Two functions compose to the identity (modulo a known equivalence).

- `decode(encode(x)) == x`
- `parse(print(x)) == x`
- `deserialize(serialize(x)) == x`

Round-trips catch asymmetry bugs that unit tests routinely miss: an encoder that emits a form the decoder
cannot parse, a serializer that drops a field only on some code paths, a printer that emits ambiguous
syntax.

Beware the trivial round-trip: `normalize(normalize(x)) == normalize(x)` is idempotence, not a round-trip.

### Idempotence

`f(f(x)) == f(x)`. True for normalization, deduplication, canonicalization, most pure caches. A violation
almost always indicates hidden state.

### Commutativity and Associativity

`f(a, b) == f(b, a)` and `f(f(a, b), c) == f(a, f(b, c))`. True for set union, addition, merging of
commutative CRDTs. False for subtraction, division, string concatenation — do not assert these blindly.

### Monotonicity

`a <= b ==> f(a) <= f(b)`. Common in ranking, scoring, rate limiters, cache eviction policies. A failure
here often points at tie-breaking bugs or hidden randomness.

### State-Machine Properties

The system is stateful. You generate a sequence of commands, run them against both the real system and a
model, and assert observable equivalence after each command. This is the heaviest property class; reach
for it when unit testing a state machine has produced too many brittle setup-heavy tests.

## A Decision Tree for Picking a Property

Given a function `f: A -> B`, work through these in order and stop at the first "yes":

1. **Is there an obvious inverse `g: B -> A`?** → Round-trip: `g(f(x)) == x`.
2. **Is there a slow/reference version `f'`?** → Oracle: `f(x) == f'(x)`.
3. **Does `f` claim to normalize, dedupe, or canonicalize?** → Idempotence: `f(f(x)) == f(x)`.
4. **Does `f` claim an ordering or ranking?** → Monotonicity: `a <= b ==> f(a) <= f(b)`.
5. **Does `f` combine two values?** → Check commutativity and associativity individually; assert only the
   ones the spec requires.
6. **Is there a mathematical identity `f` satisfies?** → Identity: e.g. `f(x + 0) == f(x)`.
7. **Is `f` a stateful command?** → State machine with a model.
8. **None of the above?** → Invariant: pick a post-condition that must always hold (sorted-ness, size
   bound, type-correctness, no-negative-balance) and assert it on `f(x)` for arbitrary `x`.

If step 8 is all you can find and the invariant is weak, you do not yet have a property worth testing.
Write examples and revisit when the spec is clearer.

## Combining Properties

A single function often deserves more than one property. `sort` deserves at least two — sortedness and
permutation-preservation — because either alone is trivially satisfiable (return `[]`; return `x`). Treat
the set of properties as an adversarial specification: an implementation that satisfies all of them should
be indistinguishable from correct.

## Anti-Patterns

- **Restating the implementation.** `assert f(x) == (x * 2 if x > 0 else -x * 2)` is not a property; it is
  `f` written twice.
- **Over-constrained preconditions.** `assume(len(x) > 5 and x[0] > 0 and ...)` rejects so many inputs
  that the property rarely runs. Push constraints into generators instead.
- **Properties that only hold in the happy path.** If your property needs a try/except to pass, split it.
- **Asserting on the generator's output shape.** If your property relies on the generator producing
  specific values, your generator is doing the test's work.

## When to Stop

A property is "done" when you can describe, in one sentence, a class of bugs it catches that examples
would miss. If you cannot, either sharpen the property or delete it.

## Review Heuristics

- Reject tautologies and properties that merely restate the implementation.
- Prefer properties that would fail if the implementation were replaced with a
  stub or a buggy approximation.
- If a property needs a paragraph of caveats to be true, split it into
  narrower properties or fall back to example tests.
