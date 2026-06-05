# Test Data

Use the smallest data that still represents the behavior. Prefer realistic shape
over production volume. Never use production data, credentials, personal data,
or hardcoded environment-specific identifiers.

Factories centralize defaults. Builders make important variations readable.
Fixtures are useful for stable shared examples, but keep them immutable and
local to the test boundary. Avoid shared mutable state across tests; every test
must be independently runnable.

Keep setup deterministic. Inject clocks, randomness, sequence numbers, and IO.
Clean up created state even when a test fails. Data should make the oracle easy
to inspect: each value exists for a reason, and unused fields are noise.
