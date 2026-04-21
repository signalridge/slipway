---
skill_id: tdd-governance
name: slipway-tdd-governance
description: "Use when verifying TDD discipline for guardrail-domain wave execution. Triggers on guardrail-domain execution or before wave verification is frozen."
---

# TDD Governance

```
IRON LAW: NO PRODUCTION CODE WITHOUT A FAILING TEST FIRST
```

Violating the letter of this rule is violating the spirit of this rule.

## Purpose
Enforce test-driven development discipline during wave execution for
guardrail-domain changes. This host owns the RED/GREEN/REFACTOR/EVIDENCE
contract directly; it no longer relies on a separate overlay skill for the
procedure definition.
Mitigates: guardrail-domain tasks executed without test-driven proof.

## Workflow Outline
1. Read the current wave state and collect task-level TDD evidence.
2. Verify RED, GREEN, REFACTOR, coverage, and regression proof for each task.
3. Write a verdict, surface blockers, and wait for approval before advancing.

## When This Runs
During wave execution phase for guardrail-domain governed changes. Validates that tasks follow TDD protocol before wave orchestration verification is frozen.

## Process

### 1. Read Context
Run `slipway next --json` and read the task plan and wave execution state.
Treat the following as the authoritative proof contract for each task:
- **RED**: failing test recorded before any production change
- **GREEN**: smallest change that makes the failing test pass
- **REFACTOR**: structure cleanup only while the target remains green
- **EVIDENCE**: RED/GREEN/REFACTOR timestamps, commands, and run versions recorded task-by-task

### 2. TDD Compliance Checklist (MANDATORY)
For EACH task in the current wave, verify ALL of the following:

- [ ] **Test-First Verification**: Git history shows test file commit BEFORE implementation commit for this task
- [ ] **RED Evidence**: The failing test was observed before implementation, the exact failure output was captured, and no production change was authored in the RED step
- [ ] **GREEN Evidence**: The MINIMUM production change turned RED into GREEN, and the GREEN command plus run version were recorded
- [ ] **REFACTOR Evidence**: Structure was improved without changing behaviour, and the full target was re-run green after refactor
- [ ] **EVIDENCE Record**: Verification notes capture RED/GREEN/REFACTOR timestamps, commands, and run versions task-by-task
- [ ] **No Stub Tests**: Every test has meaningful assertions (not `assert.True(true)` or empty bodies)
- [ ] **Coverage Gate**: New/changed code has corresponding test coverage
- [ ] **Critical Path Coverage**: Auth, data, external contracts have explicit test proof
- [ ] **Regression Scope**: Nearby tests were re-run after implementation

Each unchecked item is a BLOCKER. No partial credit.

### 3. Git History Verification Protocol
To verify test-first discipline, use this concrete method:

```bash
# For each task, check commit order
git log --oneline --name-only -- <target_files>

# Verify test file appears in an earlier commit than implementation file
# If commits are squashed, check file modification timestamps
git log --format="%H %ai" --diff-filter=AM -- <test_file>
git log --format="%H %ai" --diff-filter=AM -- <impl_file>
```

If test and implementation are in the SAME commit:
- This is NOT test-first evidence
- The implementer must demonstrate the test was written first (e.g., interim commits, branch history)
- If no evidence exists, mark as FAIL

### 4. Test Quality Assessment
Tests must be MEANINGFUL, not ceremonial:

**Pass criteria**:
- Tests assert specific behavior, not just "no error"
- Tests cover the happy path AND at least one edge case
- Tests use concrete values, not just type checks
- Tests would FAIL if the implementation were removed or stubbed

**Fail criteria** (any one triggers FAIL):
- Empty test bodies
- `assert.True(true)`, `assert.NotNil(result)` without behavior checks
- Tests that only check return type, not content
- Tests that mock the thing they're testing
- Tests with commented-out assertions

### 5. Enforcement Rules
- Tasks without test-first evidence: `fail` verdict
- Tasks with stub-only tests: `fail` verdict
- Tasks modifying critical paths without explicit test coverage: `fail` verdict
- Refactor-only tasks: may skip test-first IF existing tests cover refactored code AND tests were re-run green
- Investigation/doc tasks: TDD not applicable, skip with note

### 6. Write Verification
Verification requires `run_version` matching `verification/execution-summary.yaml`.

```yaml
# Write to: artifacts/changes/{slug}/verification/tdd-governance.yaml
verdict: pass
blockers: []
timestamp: "<ISO-8601-UTC>"
run_version: 1
references:
  - "tdd:task-001=pass:test-first-verified"
  - "tdd:task-002=pass:test-first-verified"
  - "tdd:task-003=fail:no-test-commit-before-impl"
notes: |
  <verification notes>
```

### 7. Present and Advance
Show TDD compliance summary per task. <HARD-GATE>Wait for explicit user confirmation before advancing. Do not call `slipway next` until the user approves.</HARD-GATE>

After confirmation: `slipway next`

## DO NOT SKIP
1. Test-first verification for EACH task (not a sample).
2. Git history verification (not implementer claims).
3. RED/GREEN/REFACTOR evidence language must match the host-owned proof contract above.
4. Coverage gate for critical paths.
5. Test quality assessment (not just "tests exist").
6. Verification record written after compliance check.

See `references/tdd-evidence-patterns.md` for recurring rationalizations,
attestation edge cases, and reminder examples for RED/GREEN/REFACTOR evidence.

## Hard Gate Enforcement
DO NOT advance past TDD governance until the user has explicitly confirmed the compliance summary. Even if all tasks pass, the user must approve because they may have domain knowledge about test adequacy that automated checks cannot detect.

**Anti-pattern**: "All tasks have test-first evidence, advancing automatically." — TDD governance is a guardrail-domain gate. Automatic advancement defeats its purpose.

## Step Declaration
Declare current step and expected output before executing each workflow step.
