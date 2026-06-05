# Test-Double Strategy

A test double replaces a collaborator for a specific reason. Choose the real
collaborator unless the boundary is slow, nondeterministic,
external, destructive, or too costly to set up. Decide per dependency, not per
test suite.

Vocabulary matters:
- A dummy is passed only to satisfy a parameter.
- A stub returns fixed answers.
- A fake has a working simplified implementation.
- A spy records observed interactions for later inspection.
- A mock defines required interactions and fails when they are not met.

Prefer in-memory fakes over deep mock chains when the dependency has meaningful
state or multiple operations. Do not mock what the system does not own unless
the test is explicitly describing an adapter boundary. Inject time, randomness,
IO, and network edges so tests can drive them deterministically.

A double must represent the complete contract needed by the behavior, not just
the fields this test happens to read. A test that passes when the implementation
body is removed is not constraining behavior.
