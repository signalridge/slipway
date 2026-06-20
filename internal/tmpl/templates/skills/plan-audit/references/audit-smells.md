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
| "This code task only edits the file that defines the type" | A change to a shared or widely-referenced type/contract (new enum case, struct/record field, changed signature) forces every consumer to update too. Some task's `target_files` must own those call sites, or S2's integration gate and keeping `changed_files` covered by `target_files` become mutually unsatisfiable. |

## Failure Handling Patterns
- If artifacts are missing or stale, set verdict to `fail` with specific blockers.
- Route back to artifact creation or refresh before execution.
- Re-run plan-audit only after artifact refresh verification is available.
