# TDD Evidence Patterns

## Rationalization Red Flags
| Rationalization | Counter-rule |
| --- | --- |
| "Tests can be added after" | Test-first is the point of TDD governance. After is not TDD. |
| "The code is too simple to test" | Simple code gets simple tests. Complexity is not the gate. |
| "Tests pass so TDD was followed" | Passing tests do not prove test-first discipline. Check git history. |
| "Refactoring doesn't need tests" | Refactoring needs existing test coverage to be verified green. |
| "Time pressure justifies skipping" | TDD governance exists specifically to resist time pressure. |
| "One commit with test+impl is fine" | Same-commit is not test-first evidence. Separate commits required. |
| "The test is trivial but it exists" | Trivial tests with no meaningful assertions are stubs, not tests. |
| "Coverage tools say 80%, that's enough" | Coverage percentage doesn't prove test-first. Check commit order. |
| "Integration tests cover this unit" | Unit behavior needs unit tests. Integration is not a substitute. |
| "I'll fix the test after the PR" | Tests are not post-merge tasks. Fix before verification is frozen. |

## Failure Handling
- Tasks failing TDD check must be remediated before wave verification is frozen.
- If TDD compliance cannot be verified from git history, require explicit attestation with interim commit evidence from the implementer.
- If attestation is disputed, mark the task as `blocked` and surface it to the user.
- Multiple TDD failures in a wave suggest the executor is not following the `tdd` technique skill. Surface that pattern explicitly.
