# Writing Generators

A property is only as strong as the generator that feeds it. A generator that produces one shape of input
99% of the time turns every property into an example test. This note is about building generators that
actually explore.

## Constrained vs. Unconstrained

An unconstrained generator produces any value of its type. A constrained generator produces only values
satisfying some predicate. Prefer unconstrained generators composed from primitives; reach for constraints
only when the type system cannot express the shape you need.

- **Unconstrained.** `integers()`, `text()`, `lists(integers())`. Cover the whole type.
- **Constrained by composition.** `lists(integers()).map(sorted)` produces sorted lists directly.
- **Constrained by filter.** `integers().filter(lambda n: n % 2 == 0)`. Cheap for low rejection rates,
  disastrous for high ones (see below).
- **Constrained by construction.** Build a valid AST node-by-node rather than filtering random bytes for
  parseability.

Rule of thumb: if a filter rejects more than ~5% of draws, refactor into a constructor.

## Weighted Choice

Uniform sampling over a sum type is rarely what you want. A JSON generator that picks `null`, bool,
number, string, array, object with equal weight will spend 5/6 of its budget on scalars. Weight toward the
shapes that stress the property.

- Weight recursive cases lower than base cases, or recursion will diverge or time out.
- Weight "interesting" sizes (0, 1, boundary) higher than the bulk.
- When in doubt, log the distribution of draws for one run and eyeball it.

## Shrinking-Friendly Structure

Shrinking is the single most important property of a generator. A generator that produces correct
counterexamples but cannot shrink them is nearly useless — the human sees a 400-element list and gives up.

To stay shrinking-friendly:

- **Build values from the generator's primitives.** Libraries shrink their own primitives; values built
  by raw `random.randint` do not shrink.
- **Avoid `map` through lossy transforms.** `integers().map(lambda n: hash(n))` loses the structure the
  shrinker needs.
- **Prefer `flat_map` to nested sampling.** `sizes.flat_map(lambda n: lists(..., min_size=n, max_size=n))`
  lets the shrinker coordinate size and contents.
- **Do not post-process.** If your property needs a sorted list, generate sorted lists; do not generate
  arbitrary lists and sort them inside the test.

## Avoiding Over-Filtering

`filter` is a trap. Every filtered-out draw is wasted budget, and the shrinker may hit the filter so often
during shrinking that it reports "could not shrink" on a reducible counterexample.

Signs of over-filtering:

- `HealthCheck.filter_too_much` warnings (Hypothesis) or equivalent in other libraries.
- Runs that finish far faster than expected (most draws rejected before reaching the property body).
- Counterexamples that clearly are not minimal.

Fix by constructing valid inputs directly. A generator for "non-empty list with no duplicates" is not
`lists(ints()).filter(lambda xs: xs and len(set(xs)) == len(xs))` — it is `sets(ints(), min_size=1).map(list)`.

## Assume / Precondition Discipline

Inside a property body, `assume(cond)` discards the current draw and asks for another. Treat it as a last
resort.

- `assume` on cheap predicates (e.g. divisibility) is fine.
- `assume` on expensive predicates (regex match, parse success) burns time.
- `assume` that rejects >10% of draws should be refactored into the generator.
- Never `assume` on something the test itself computed; that is a logic bug wearing a filter's clothes.

## Generating Edge Cases

The bugs that hide from property testing are almost always in values the generator never produces. Make
the rare-by-default explicit:

- **Empty collections.** `lists(..., min_size=0)`; `text(min_size=0)`. Always include zero.
- **Single-element collections.** Often the site of off-by-one errors.
- **Unicode.** Text generators default to ASCII-leaning; explicitly include surrogates, combining marks,
  bidi overrides, zero-width joiners if the system touches text at all.
- **Boundary integers.** 0, 1, -1, INT_MAX, INT_MIN, and one on either side of each. Libraries typically
  do this automatically — verify rather than assume.
- **NaN, +0.0 vs -0.0, subnormals.** Any floating-point code path must generate these or explicitly
  exclude them with documented rationale.
- **Duplicates.** For structures where duplicates matter (sets, maps, CRDTs), weight toward draws with
  collisions.

## Generator Smells

Bad generators usually smell the same way. If you see one of these, refactor before trusting the property.

- **High rejection rate.** `filter` hit more than generation count. Fix by construction.
- **Never shrinks past 3 elements.** The generator's shape is opaque to the shrinker.
- **Timeouts only on recursive types.** Recursion weighting or depth limit missing.
- **Passes for 10_000 runs but fails the first manual example.** The generator's distribution is starved
  of an entire region of the input space.
- **Property passes; coverage shows the body's `else` branch was never entered.** Generator weights bias
  away from a branch. Check with a coverage-aware run.
- **Counterexamples that vary wildly between runs.** Likely non-determinism in the generator itself —
  generators should be pure functions of the seed.
- **`map` that hides the raw draw.** If the shrunk counterexample says `<object at 0x...>`, the map is
  throwing away shrinker state.

## When to Stop Tuning a Generator

A generator is good enough when:

1. Rejection rate is near zero.
2. Counterexamples, when produced, shrink to two or three elements for collection types.
3. Coverage of the code under test, measured on a property-testing run, matches what you would expect
   from a thorough example suite.

If any of these fail, the generator is still masking bugs.

## Library Selection

- Prefer the project-native property library when one already exists in the
  stack; ergonomics and replay support matter more than theoretical breadth.
- Prefer libraries with replayable seeds and shrink traces that can be checked
  into version control.
- If a library cannot express the generator you need without heavy filtering,
  it is usually the wrong library for that property.
