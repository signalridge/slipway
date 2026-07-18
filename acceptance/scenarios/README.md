# Host prompt scenarios

These thirteen prompt-level evaluations cover host behavior that deterministic core tests cannot establish alone. Run them against the primary supported hosts and sample the remaining adapters. They complement, rather than replace, the [acceptance matrix](../README.md).

For every collected scenario, record the binary revision, host and version, generated capability digest, fixture revision, sanitized transcript location, observed Actions, exact submitted Outcome JSON, and evaluator notes. Never infer an activity from prose; compare the Outcome and journal. Do not fabricate an uncollected transcript.

All scenarios use the same baseline: explicit invocation only; repository facts before questions; at most one genuine human decision per turn with recommendation and trade-offs; zero questions for a complete request; confirmation only when clarification changed execution understanding; immediate stateless wrap-up; no implicit documentation materialization; read-only Review flowing directly to Summary without repair/re-review; and exact structured destructive authority rather than prose.

| Scenario | Focus |
| --- | --- |
| [01](01-complete-request.md) | Complete request asks zero questions |
| [02](02-repository-facts.md) | Repository facts are investigated |
| [03](03-dependent-decisions.md) | One dependent decision per turn |
| [04](04-wrap-up.md) | Interview stops and remains stateless on wrap-up |
| [05](05-skip-review.md) | Review can be skipped without a reason |
| [06](06-failed-activity.md) | Failed activity does not gate Summary |
| [07](07-review-findings.md) | Findings are reported without repair/re-review |
| [08](08-resume.md) | Interrupted Run resumes via fresh Orient |
| [09](09-stateless-clarify.md) | Standalone Clarify writes no files |
| [10](10-no-implicit-materialization.md) | Clarification never materializes docs implicitly |
| [11](11-destructive-confirmation.md) | Destructive work requires exact structured authorization |
| [12](12-activity-truth.md) | An unstarted activity is never reported as run |
| [13](13-workflow-orchestration.md) | Workflow routes every #434 function and lifecycle provenance class without becoming a skill router or crossing explicit capability boundaries |
