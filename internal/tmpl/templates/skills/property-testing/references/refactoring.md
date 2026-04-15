# Refactoring Under Property Tests

Property tests are a refactor's best safety net: they pin **behavior** rather
than **implementation**, so a rewrite that keeps the contract passes without
edit. Use them to drive the change, not to chase it.

## Sequence

1. **Pin the contract.** Before touching the implementation, list the
   invariants the code must preserve and the inputs it must accept. If a
   property is missing, write it first, watch it pass on the current
   implementation, and only then refactor.
2. **Shrink the working set.** Scope the refactor to a single module whose
   properties are well-defined. Cross-module refactors should compose from
   module-level refactors, each anchored by their own property suite.
3. **Refactor in one direction at a time.** For example, *extract* before you
   *reshape*. Interleaving transformations hides which step broke which
   property.
4. **Re-run the suite on every step.** If a property fails, stop: the diff
   that broke it is the smallest reproducer you'll ever get.

## Contract vs. implementation properties

A property is contract-shaped when it talks about inputs and outputs the
caller can observe. A property is implementation-shaped when it references
internal helpers, private state, or temporary fields. Refactoring
implementation-shaped properties alongside the implementation defeats the
purpose — the suite no longer pins the external contract.

Bad: `assert parser._token_stream.peek() == expected`

Good: `assert parse(source) == expected_ast`

If a property must inspect internal structure (for diagnostics or
invariants), label it as an *internal property* and expect to rewrite it
when the corresponding internals change.

## Metamorphic properties

When you refactor a pure function and an equivalent, slower reference
implementation is available (even the old version pinned in a tag),
metamorphic checks catch subtle drift:

```python
@given(strategies.lists(strategies.integers()))
def test_new_sort_matches_reference(xs):
    assert new_sort(xs) == reference_sort(xs)
```

Keep the reference in-tree while the refactor is in flight. Delete it in a
follow-up commit once the new implementation has proved itself.

## Signal shrinking

When a property fails, the shrinker narrows the failure to a minimal input.
Read the shrunk case before looking at the implementation: it tells you
which invariant is broken. Resist the urge to weaken the property to silence
the failure — if the shrunk input is legal under the contract, the
implementation is wrong.

## Property-preserving rename

Renaming a function or type rarely changes behavior, but it often renames a
property as a side effect. Keep property names aligned with the contract
they describe, not the function they exercise. `test_add_is_commutative` is
contract-aligned; `test_compute_sum_works` leaks implementation into the
test name.

## When to delete a property

Delete a property when the contract it pinned is intentionally narrowed, and
record the reason in the commit message. Deleting for convenience — because
the shrunk input is awkward, or because the property now fails on a
legitimate change — is a silent contract weakening.

## Pairing with example tests

Property tests catch whole classes of bugs; example tests catch the specific
regression you already hit. Keep both. When a property flakily fails on an
input that should pass, promote that shrunk case into an explicit example
test so the suite names it going forward.
