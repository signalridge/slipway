# Test Data

Use the smallest data that still represents the behavior. Prefer realistic shape
over production volume. Never use production data, credentials, personal data,
or hardcoded environment-specific identifiers. Also assert that secrets and
personal data never appear in outputs, logs, or error messages; their absence is
itself an oracle for redaction and privacy boundaries.

When sensitive examples must be masked, preserve the format the behavior relies
on. A masked identifier should still satisfy required length, category, checksum,
ordering, or parser constraints; otherwise state that format is irrelevant to
the oracle.

Factories centralize defaults. Builders make important variations readable.
Fixtures are useful for stable shared examples, but keep them immutable and
local to the test boundary. Avoid shared mutable state across tests; every test
must be independently runnable.

Keep setup deterministic. Inject clocks, randomness, sequence numbers, and IO.
Clean up created state even when a test fails. Data should make the oracle easy
to inspect: each value exists for a reason, and unused fields are noise.
