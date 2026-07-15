# Pre-release prompt scenarios

These twelve prompt-level evaluations cover host behavior that deterministic tests cannot establish alone. Run them against Claude, Codex, and Pi and sample the other adapters. They complement—not replace—the 35-scenario [evidence matrix](../README.md).

For every scenario record the binary revision, host/version, generated capability digest, fixture revision, sanitized transcript location, observed Actions, exact submitted Outcome JSON, and evaluator notes. Never infer an activity from prose; compare the Outcome and journal. Do not fabricate uncollected transcripts.

Baseline rules apply to all twelve: explicit invocation only; repository facts are investigated; Clarify follows Matt Pocock `grill-me` dependency order with at most one question plus recommendation/trade-offs, zero questions for a complete request, shared-understanding confirmation only when grilling changed execution understanding, stateless immediate wrap-up; no implicit documentation materialization; Review is read-only and findings flow directly to Summary without repair/re-review; destructive authority is exact structured scope and never prose.

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
