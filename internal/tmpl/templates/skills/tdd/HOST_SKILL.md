---
skill_id: tdd
name: slipway-tdd
description: "Use when executing RED/GREEN/REFACTOR with strict test-first discipline. Triggers on code changes or whenever implementation would otherwise run ahead of proof."
---

# Test-Driven Development

```
IRON LAW: NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST
```

## Purpose
Apply a strict RED → GREEN → REFACTOR loop so implementation never gets ahead
of verification. This is a technique skill used by execution hosts; it does
not replace governance gates such as `slipway-tdd-governance`. It is not a suggestion
for code changes.

## Workflow Outline
1. RED: write one failing test and make the failure clean.
2. GREEN: make the smallest implementation change that turns RED into GREEN.
3. REFACTOR: improve structure only while the full relevant test set stays green.
4. Repeat one behavior slice at a time.

## Detailed Process

### Phase 1: RED — Write One Failing Test
1. Identify the smallest behavior slice to implement next.
2. Write ONE test that asserts the expected behavior.
3. Run the test. It MUST fail.
4. If it passes: either the behavior already exists (verify) or the test is wrong (fix the test).
5. If it errors (compile/syntax): fix the error until you get a clean FAIL (assertion failure, not crash).

**The test defines the contract.** Once written, the test is the specification. Do not modify it during GREEN phase.

### Phase 2: GREEN — Smallest Possible Change
1. Write the MINIMUM code to make the failing test pass.
2. Do not write "clean" code. Do not write "complete" code. Write the smallest change.
3. Run the specific test. It must pass.
4. Run nearby regression tests. They must still pass.
5. If regressions appear, fix them while keeping the new test green.

**"Minimum" means minimum.** If you can make the test pass with a hardcoded return, that's a signal your test may need refinement — but the hardcoded return IS the correct GREEN step. The next RED test will force generalization.

### Phase 3: REFACTOR — Clean Up While Green
1. Improve readability, remove duplication, rename for clarity.
2. After EVERY change: run all tests. They MUST stay green.
3. If a refactor breaks a test: REVERT immediately. Do not debug the refactor. Revert to GREEN state, then try a different refactoring approach.
4. Commit after refactoring is complete and all tests are green.

### Repeat
Move to the next behavior slice. One slice = one RED-GREEN-REFACTOR cycle.
Do NOT batch multiple behavior slices into one cycle.

## Mandatory Checklist
For each behavior slice, verify:
- [ ] Test written BEFORE production code
- [ ] Test observed to FAIL (not error, not skip — FAIL)
- [ ] Implementation is the MINIMUM change to pass
- [ ] All existing tests still pass after GREEN
- [ ] Refactoring done only while green
- [ ] Commit after each complete cycle

## Test Quality Standards
A test is NOT acceptable if it:
- Has no assertions (empty body)
- Only asserts "no error" without checking behavior
- Uses `assert.True(true)` or equivalent no-ops
- Mocks the unit under test (only mock dependencies)
- Is commented out or skipped
- Tests implementation details instead of behavior

A test IS acceptable when:
- It would FAIL if the production code were deleted
- It asserts observable behavior (output, state change, side effect)
- It is readable: someone can understand WHAT is being tested without reading the implementation
- It covers at least one edge case for critical paths

## DO NOT SKIP
1. Never write production code first. If you already did: DELETE IT. Write the test. Then rewrite the production code.
2. Never mark GREEN without running the test.
3. Never refactor with red tests.
4. Never batch multiple behavior slices into one cycle.
5. Never modify the test during GREEN phase (that's moving the goalpost).

## Failure Mode Handling
1. **Flaky tests**: Isolate deterministic preconditions before proceeding. Do not ignore flakiness.
2. **Unclear minimum fix**: Reduce scope to a smaller failing assertion. The smallest test reveals the smallest fix.
3. **Refactor regressions**: Revert to last GREEN state immediately. Try a different approach. Do not debug forward.
4. **Test infrastructure missing**: Create a minimal test helper or fixture as a separate RED-GREEN cycle before the main task.
5. **Cannot write test first**: This means you don't understand the behavior yet. Research first, then write the test. If you still can't: the task needs to be broken down further.
