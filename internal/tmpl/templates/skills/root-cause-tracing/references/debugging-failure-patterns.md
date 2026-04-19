# Debugging Failure Patterns

## Rationalization Red Flags
| Rationalization | Counter-rule |
| --- | --- |
| "I know the cause without reproducing" | Reproduce first. Confidence is not evidence. |
| "Let's try random fixes quickly" | Random edits increase noise and delay root cause. |
| "It passed once, we're done" | Repeat to confirm stability and avoid false green. |
| "Logs are enough, no instrumentation needed" | Add targeted probes for ambiguous paths. |
| "Regression tests are unnecessary" | Validate neighboring behavior after every fix. |
| "I'll fix it and see if it helps" | That's guessing, not debugging. Prove the cause first. |
| "The stack trace says line 42, so the bug is on line 42" | The symptom is on line 42. The cause may be elsewhere. Trace backward. |
| "It's probably a race condition" | "Probably" is not a hypothesis. State the specific race and design a probe. |
| "This fix is safe enough to try" | Safe fixes still mask root causes. Understand before changing. |
| "The bug is too complex for systematic approach" | Complex bugs need more discipline, not less. Break into smaller investigations. |
| "Let me refactor first, then debug" | Refactoring changes the code. Debug the code as-is, then refactor after the fix. |

## Failure Mode Handling
1. **Non-deterministic symptom**: Collect timing and input variance across 5+ runs. Look for race conditions, uninitialized state, or external dependency timing.
2. **Multiple plausible causes**: Rank by evidence strength. Test the highest-evidence hypothesis first. Do not test all at once.
3. **Invasive fix needed**: Split into smaller validated checkpoints. Each checkpoint should improve the situation measurably.
4. **Cannot reproduce locally**: Check environment differences (versions, config, data). Create a minimal reproduction case that isolates the failing behavior.
5. **Fix works but feels wrong**: If the fix is a workaround rather than a root-cause fix, document it as tech debt and create a follow-up task.
