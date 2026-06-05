---
skill_id: test-design
domain: verification
function: language-agnostic test design, case enumeration, and test-double judgment
tier: T1
primary_attachment: procedure
summary: "Use when designing meaningful test cases, test doubles, properties, or fixtures. Triggers on wave-orchestration host or testing-quality user text."
trigger_signals:
  - host: wave-orchestration
    reason: "Execution host is authoring tests or evaluating test quality before implementation"
  - user_text_matches: ["test design", "test cases", "test doubles", "property test", "fixtures"]
    reason: "User text asks for test design judgment"
evidence_contract: artifact
hydrate_references:
  - name: test-doubles.md
    reason: "Choose real dependencies, fakes, spies, stubs, mocks, and injected time or IO per boundary"
  - name: behavior-vs-implementation.md
    reason: "Assert observable behavior and reject tautologies or internal-call coupling"
  - name: case-enumeration.md
    reason: "Derive equivalence, boundary, decision-table, state, pairwise, negative, and MC/DC cases with oracles"
  - name: property-reasoning.md
    reason: "Frame invariants, generators, shrinking, and stateful properties without weak assertions"
  - name: test-data.md
    reason: "Select fixtures, factories, builders, and deterministic non-sensitive datasets"
bindings:
  - type: technique-hint
    target: wave-orchestration
    attachment: procedure
---

# Test Design

```
IRON LAW: TESTS MUST CONSTRAIN BEHAVIOR, NOT CEREMONY
```

## Purpose
Design meaningful tests before or during implementation. This skill answers
what cases to test, what oracle proves the expected result, and where a real
dependency or test double is justified. It stays language-neutral; use a
host-owned language testing skill for syntax and idiom.

## Workflow
1. Name the behavior and its observable outputs, state changes, or side effects.
2. Enumerate cases from equivalence classes, boundaries, decisions, states,
   combinations, negative paths, and critical-branch reasoning.
3. Attach an oracle to each case: exact value, tolerance, invariant, monotonic
   relationship, or rejection.
4. Choose real dependencies or test doubles per boundary, and reject doubles that
   only mirror the fields a test happens to read.
5. Build deterministic, non-sensitive data with fixtures, factories, or builders
   that keep each test independent.

## Reference Shelf
- `references/test-doubles.md`
- `references/behavior-vs-implementation.md`
- `references/case-enumeration.md`
- `references/property-reasoning.md`
- `references/test-data.md`

## Checklist
- [ ] Every assertion can fail under a plausible wrong implementation.
- [ ] Boundary, negative, and error paths are named, not implied.
- [ ] Each test has an oracle that explains why the expected result is correct.
- [ ] Test doubles are chosen per dependency and represent complete contracts.
- [ ] Data setup is deterministic, minimal, realistic, and non-sensitive.

## Anti-patterns
- Tests that still pass when the implementation is deleted.
- Assertions on internal calls when observable behavior is available.
- Shared mutable fixtures that make tests order-dependent.
- Mock chains that encode today's implementation instead of a stable boundary.
