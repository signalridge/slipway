# Property-Testing Strategies

A catalog of the strategies worth reaching for, each with a one-line recognition heuristic and a minimal
language-agnostic pattern. Pick by matching the heuristic to the function under test; do not stack
strategies reflexively.

## Identity / Idempotence

**Recognize:** the function claims to normalize, canonicalize, dedupe, or reach a fixed point.

```
forall x: f(f(x)) == f(x)
```

Applies to: path normalization, Unicode NFC/NFD, dedup, cache warm-up, sort, set construction.

## Round-Trip

**Recognize:** there is a natural inverse — encode/decode, parse/print, serialize/deserialize.

```
forall x: decode(encode(x)) == x
```

Extension: assert both directions when both are total — `encode(decode(y)) == y` for any `y` in the
encoding's image. Inequality after round-trip is an information-loss bug.

## Oracle / Model-Based

**Recognize:** a slow-but-obvious, a reference spec, or a small in-memory model exists.

```
forall x: fast(x) == reference(x)
```

Model-based variant: maintain a tiny data structure that mirrors the SUT's invariants and compare
observable outputs, not internal state.

## Algebraic Laws

**Recognize:** the function implements an operation with known algebra — monoid, group, ring, lattice.

```
forall a, b, c:
  f(a, f(b, c)) == f(f(a, b), c)     -- associativity
  f(a, b)      == f(b, a)            -- commutativity (only if spec allows)
  f(a, id)     == a                  -- identity
```

Assert only the laws the specification promises. Not every merge is commutative.

## Invariants

**Recognize:** the function has a post-condition expressible in one sentence ("the result is sorted", "no
negative balance", "tree remains balanced").

```
forall x: invariant(f(x))
```

Layer invariants: for a red-black tree insert, assert (a) BST property, (b) black-height equality, (c) no
consecutive red nodes, (d) the inserted key is now a member.

## Metamorphic

**Recognize:** you have no oracle, but you know how the output should change when the input changes.

```
forall x, t: relation(f(x), f(transform(x, t)))
```

Examples: `sort(x ++ [v]) == insert_sorted(sort(x), v)`; `search(index, q).len >= search(index, q ++ "filter").len`;
`render(theme_light, dom) and render(theme_dark, dom)` produce the same DOM tree modulo color attributes.
Indispensable for ML, search, and rendering, where a true oracle rarely exists.

## State Machines

**Recognize:** the system has operations whose effect depends on prior operations.

```
forall cmds: run_sut(cmds).observables == run_model(cmds).observables
```

The generator produces sequences of valid commands given the current model state; after each command the
SUT and model are compared on externally visible outputs. Use when a set of independent properties leaves
obvious coverage holes on stateful code.

## Invariance Under Refactor

**Recognize:** two implementations must remain equivalent across a refactor, rewrite, or optimization.

```
forall x: new_impl(x) == old_impl(x)
```

A time-boxed variant: keep the old implementation in the test binary only, run both for one release,
delete when confidence is earned. This is the cheapest migration safety net available.

## Quick Reference

| Strategy              | One-line heuristic                                               |
|-----------------------|------------------------------------------------------------------|
| Identity/Idempotence  | "calling it twice gives the same answer"                         |
| Round-trip            | "there is an inverse function"                                   |
| Oracle/Model-based    | "a slower, simpler version already exists"                       |
| Algebraic laws        | "this operation has known algebra"                               |
| Invariants            | "the output always satisfies a one-sentence predicate"           |
| Metamorphic           | "I know how the output must change when the input changes"       |
| State machines        | "the effect of a call depends on prior calls"                    |
| Invariance under refactor | "the new code must match the old code"                       |

## Choosing Between Strategies

Prefer oracle over invariant when an oracle exists — oracle failures are unambiguous. Prefer metamorphic
over invariant when the output is high-dimensional (images, rankings, embeddings). Prefer state-machine
over invariant when the bug you are chasing requires a specific history. Never stack a weak invariant on
top of a strong oracle; the invariant adds noise without coverage.
