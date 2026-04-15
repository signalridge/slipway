---
skill_id: mutation-testing
domain: verification
function: run mutation testing to score the test suite, not the implementation
tier: T1
primary_attachment: tool-recipe
summary: "Use when test strength is in doubt. Triggers on validate command, goal-verification host, or user text naming mutation testing."
size_rationale: "Warn-band accepted: tool-selection and survivor-triage rules are kept inline to avoid fragmented mutation verdicts."
trigger_signals:
  - command: validate
    reason: "validate command invoked; mutation testing may apply"
  - host: goal-verification
    reason: "Verification host active; mutation testing is a verification booster"
  - user_text_matches: ["mutation testing", "mutmut", "stryker", "pitest"]
    reason: "User text names a mutation testing tool"
evidence_contract: artifact
hydrate_references:
  - name: optimization-strategies.md
    reason: "Make mutation runs finish in bounded time"
  - name: configuration.md
    reason: "Pick the right mutators and exclusions for the target"
bindings:
  - type: host-embedded
    target: goal-verification
    attachment: checklist
---

# Mutation Testing

```
IRON LAW: MUTATION SCORE RATES THE TESTS, NOT THE CODE
```

## Purpose
Mutation testing mutates the implementation and asks whether the test suite
catches the mutation. A surviving mutant means the test suite is weaker than
its pass/fail output suggests. Read the output as a test-suite rating.

## Tool recipe
- **Select** the mutation tool for the language in scope (stryker for JS/TS,
  mutmut / cosmic-ray for Python, pitest for JVM, go-mutesting for Go).
- **Pin** the tool version and mutation operator set; record both.
- **Scope** mutations to the change surface. Full-codebase runs are valid
  inputs to a separate quality budget, not to per-change validation.
- **Run** with a bounded timeout per mutant; record the timeout.
- **Triage** survivors: `test-gap` (add a test that kills it),
  `equivalent-mutant` (mutation is semantically equivalent; justify and
  suppress), `out-of-scope` (mutation is in code the change does not own).
- **Fail** the verdict when any `test-gap` survivor remains unresolved.

## Report schema
```yaml
tool: "<tool + version>"
operator_set: "<name or commit>"
scope: "<paths>"
timeout_per_mutant: "<seconds>"
results:
  total: <n>
  killed: <n>
  survived: <n>
  timeout: <n>
survivors:
  - mutant: "<file:line>"
    operator: "<operator name>"
    disposition: test-gap | equivalent-mutant | out-of-scope
    resolution: "<added test name | justification | ticket>"
```

## Anti-patterns
- Reporting the raw mutation score without triaging survivors.
- Claiming "equivalent mutant" without a written justification.
- Running mutation testing without pinning operator set (scores drift).
