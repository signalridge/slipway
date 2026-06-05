# Case Enumeration

Use more than happy-path examples. Pick cases from several lenses:
- Equivalence partitioning: one representative from each meaningful class.
- Boundary values: just below, at, just above, typical, max-adjacent, and outside
  the valid range. Include non-numeric boundaries such as empty, missing,
  duplicate, reordered, and malformed.
- Decision table: rows for independent conditions and the outcome each row
  requires.
- State transition: valid moves, invalid moves, repeated moves, and terminal
  states.
- Pairwise or combinatorial: interactions among option sets without exhaustive
  explosion.
- Negative and fault-based: invalid input, dependency failure, timeout,
  cancellation, partial data, and permission rejection.
- Critical branches: when the path is high-risk, reason about branch coverage
  and MC/DC so each condition is shown to affect the decision.

When several distinct causes share one outcome surface — absent, malformed,
expired, and unauthorized inputs can all end in a rejection — enumerate each
cause separately and give each its own oracle, so a test cannot pass by treating
causes that must differ as interchangeable.

Mark impossible, prohibited, or unreachable combinations explicitly instead of
pretending coverage is exhaustive. Do not invent artificial tests for dead code
or invalid states only to satisfy a count; name the uncovered condition and the
reason it has no executable oracle.

Attach an oracle to every case: exact, tolerance, invariant, monotonic, or
rejection. The oracle explains why the expected result is correct, not merely
what value was copied into the test.
