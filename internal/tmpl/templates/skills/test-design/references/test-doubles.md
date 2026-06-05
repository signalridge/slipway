# Test-Double Strategy

A test double replaces a collaborator for a specific reason. Choose the real
collaborator unless the boundary is slow, nondeterministic,
external, destructive, or too costly to set up. Decide per dependency, not per
test suite.

Pick the test level before picking the double. Unit tests isolate one small
behavior, integration tests exercise a real boundary, and end-to-end tests
reserve cost for critical user-visible flows. When an integration test uses one
real dependency, double unrelated expensive boundaries so a failure still points
to the broken contract.

Vocabulary matters:
- A dummy is passed only to satisfy a parameter.
- A stub returns fixed answers.
- A fake has a working simplified implementation.
- A spy records observed interactions for later inspection.
- A mock defines required interactions and fails when they are not met.

Prefer in-memory fakes over deep mock chains when the dependency has meaningful
state or multiple operations. Do not mock what the system does not own unless
the test is explicitly describing an adapter boundary. Inject time, randomness,
IO, and network edges so tests can drive them deterministically. When the
scaffolding to set up a double dwarfs the behavior under test, treat that as a
design signal and reconsider the boundary or interface instead of adding more
doubles.

A double must represent the complete contract needed by the behavior, not just
the fields this test happens to read. A test that passes when the implementation
body is removed is not constraining behavior.
