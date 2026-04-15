---
skill_id: property-testing
domain: verification
function: write property-based tests that specify invariants, not examples
tier: T1
primary_attachment: procedure
summary: "Use when invariants are clearer than example cases. Triggers on validate command, goal-verification host, or property-oriented user text."
trigger_signals:
  - command: validate
    reason: "validate command invoked; property tests may apply"
  - host: goal-verification
    reason: "Verification host active; consider property tests"
  - user_text_matches: ["property test", "invariant", "quickcheck", "hypothesis"]
    reason: "User text signals property-based testing"
evidence_contract: artifact
hydrate_references:
  - name: design.md
    reason: "How to pick properties that are worth testing"
  - name: generating.md
    reason: "Write generators that exercise the property space"
  - name: strategies.md
    reason: "Core property strategies (idempotence, roundtrip, oracle, invariants)"
  - name: libraries.md
    reason: "Choose an appropriate property-testing library"
  - name: interpreting-failures.md
    reason: "Read shrunk counterexamples and extract real bugs"
bindings:
  - type: host-embedded
    target: goal-verification
    attachment: checklist
---

# Property Testing

```
IRON LAW: TESTS SPECIFY INVARIANTS, NOT MEMORIZED EXAMPLES
```

## Purpose
Property tests state *what must always be true* about a function or system,
not a handful of remembered inputs. They catch classes of bugs that
example-based tests miss, but only when the property is tight and the
generator covers the relevant input space.

## Procedure
1. Write the property as a single invariant predicate: for all `x`
   satisfying `precondition(x)`, the post-condition holds.
2. Name the generator and its input space. If the generator is biased away
   from edge cases (zero, max, empty, unicode, negative), call it out and
   add targeted cases.
3. Size the run: property tests need enough iterations to shrink
   meaningfully. Pin the iteration count and the seed strategy.
4. When a property fails, preserve the shrunk counter-example and add it as
   a regression example test. Do not "fix the generator to avoid it".
5. Review properties like code: tautologies (`f(x) == f(x)`) are worthless;
   reject them.

## Checklist
- [ ] Property expressed as a precondition + post-condition predicate.
- [ ] Generator input space is named and edge-biased.
- [ ] Iteration count and seed strategy pinned.
- [ ] Shrunk counter-examples kept as regression tests.
- [ ] Properties reviewed for tautology before landing.

## Anti-patterns
- Tautological properties that pass by construction.
- Narrow generators that never reach edge cases.
- Deleting the shrunk counter-example after fixing the bug.
