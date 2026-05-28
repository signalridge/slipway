# Plan Audit Smells

## Rationalization Red Flags
| Rationalization | Counter-rule |
| --- | --- |
| "Plan looks fine manually" | Evidence-backed audit is mandatory. |
| "Proceed despite missing artifacts" | Run is blocked until plan audit passes. |
| "Old audit result is still valid" | Recheck freshness after artifact updates. |
| "Tasks are clear enough" | Each task must have explicit acceptance criteria. |
| "Verification will happen during implementation" | Plan audit still requires testable acceptance criteria and evidence paths before execution. |
| "The task is self-evident" | Self-evident tasks still need observable verification. Implicit means unverifiable. |

## Failure Handling Patterns
- If artifacts are missing or stale, set verdict to `fail` with specific blockers.
- Route back to artifact creation or refresh before execution.
- Re-run plan-audit only after artifact refresh verification is available.
