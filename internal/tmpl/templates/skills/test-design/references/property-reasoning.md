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

Prefer stronger properties. A check that the output merely has the right type or
raises no error constrains far less than an invariant, idempotence, or a round
trip. When only a weak property holds, state why a stronger one does not apply
rather than settling for it.

Name generator input space and edge bias. Include empty, minimal, maximal,
duplicate, malformed, and reordered data when relevant. Preserve shrunk failures
as example regressions. Reject weak properties that always hold regardless of
the implementation.

For stateful properties, model valid command sequences and compare the observed
state with a simple expected model after each step. Include rejected commands,
terminal states, repeated operations, and recovery after failure when those are
part of the contract.
