# Property Reasoning

Use properties when invariants describe the behavior better than a list of
examples. Good properties constrain the result for a wide input space and fail
under realistic wrong implementations.

Useful families:
- Invariant: a condition always holds after the operation.
- Round trip: encode then decode returns an equivalent value.
- Idempotence: repeating the same operation does not change the result again.
- Commutativity or associativity: order or grouping does not matter where the
  domain promises that property.
- Identity or inverse: a neutral element or undo operation preserves meaning.
- Monotonicity: increasing an input cannot decrease the measured result, or the
  reverse when that is the contract.

Name generator input space and edge bias. Include empty, minimal, maximal,
duplicate, malformed, and reordered data when relevant. Preserve shrunk failures
as example regressions. Reject weak properties that always hold regardless of
the implementation.
