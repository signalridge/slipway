---
skill_id: root-cause-tracing
domain: debugging
function: trace root cause before attempting fixes; branch competing hypotheses when traces disagree
tier: T1
primary_attachment: procedure
summary: "Use when a fix is being considered before the root cause is documented. Triggers on repair, wave-orchestration host, or debugging-centric user text."
trigger_signals:
  - command: repair
    reason: "repair command invoked; block fixes until root cause is documented"
  - host: wave-orchestration
    reason: "Execution host may be masking a missing root-cause step"
  - user_text_matches: ["debug", "crash", "flaky", "regression"]
    reason: "User text signals debugging work"
evidence_contract: artifact
hydrate_references:
  - name: root-cause-tracing.md
    reason: "Trace error origins, named failure patterns, and debugging anti-patterns before proposing fixes"
  - name: condition-based-waiting.md
    reason: "Replace sleep/retry guards with condition-based waits in flaky tests"
  - name: hypothesis-testing.md
    reason: "Structure competing hypotheses, defense layers, and their falsification experiments"
bindings:
  - type: host-embedded
    target: wave-orchestration
    attachment: procedure
  - type: command-auto
    target: repair
    attachment: procedure
  - type: technique-hint
    target: wave-orchestration
    attachment: procedure
---

# Root-Cause Tracing

```
IRON LAW: NO FIX WITHOUT A DOCUMENTED ROOT CAUSE
```

## Purpose
Trace the root cause before proposing or applying a fix. When two traces give
incompatible stories, branch competing hypotheses and disprove them in
parallel rather than guessing.

## Procedure
1. Capture the exact symptom and reproduce the failure deterministically;
   record the minimal trigger.
2. Trace backwards from the observed symptom to the first invariant that was
   violated or the first divergence from known-good behavior. Cite file:line
   and transcript. Compare with a working case when that narrows the boundary.
3. If two hypotheses explain the same symptom, list both with a predicted
   observation that distinguishes them.
4. Run the distinguishing observation; eliminate the losing hypothesis in
   writing before continuing.
5. Only after the root cause is documented, write the failing regression test
   and propose the smallest fix that addresses that cause.
6. Re-run the original reproduction and nearby regression tests before
   declaring the fix complete.

## Checklist
- [ ] Exact symptom and minimal trigger recorded.
- [ ] Minimal reproduction recorded.
- [ ] First violated invariant cited with file:line.
- [ ] If hypotheses branched, the distinguishing observation is recorded.
- [ ] Losing hypothesis is explicitly eliminated in writing.
- [ ] Fix proposal references the documented root cause.
- [ ] Original reproduction and regression tests were rerun after the fix.

## Anti-patterns
| Rationalization | Counter-rule |
|---|---|
| "I can see the fix from the stack trace" | Stack traces show symptoms, not causes. |
| "It's flaky; just retry" | Flaky means under-specified; document the race. |
| "Two fixes both make it green" | One of them is masking; pick by the trace, not the outcome. |
| "Let me refactor first, then debug" | Debug the code as-is, then refactor after the cause is proven. |

## Reference Shelf
For deeper failure-mode guidance, read:
- `references/root-cause-tracing.md`
- `references/hypothesis-testing.md`
- `references/condition-based-waiting.md`
