# SLA Management During Incidents

Distilled from `alirezarezvani/incident-commander/references/sla-management-guide.md`.
Operational rules for SLA clock management, breach handling, credit
calculation, and customer communication.

## SLA clock basics

Three clocks matter during an incident:

| Clock | Starts | Stops | Used for |
|-------|--------|-------|----------|
| Time-to-detect (TTD) | Failure occurs | Monitoring alert fires | Monitoring quality |
| Time-to-ack (TTA) | Page sent | Responder acknowledges | Paging policy |
| Time-to-resolve (TTR) | Incident declared | All customer impact ends | SLA credit, PIR |

TTR stops **only** when impact ends, not when a fix is deployed. If a
deploy fixes 95% of users but 5% still see errors, TTR keeps running.

## Breach thresholds

Tier your customer contracts and know the thresholds before the incident
starts. Typical enterprise SaaS contract:

| Tier | Monthly uptime SLA | Credit threshold | Notification window |
|------|---------------------|------------------|---------------------|
| Enterprise | 99.95% | < 99.95% in a month → 10%; < 99.5% → 25%; < 99% → 50% | 24 hr proactive notice for any breach |
| Pro | 99.9% | < 99.9% → 5%; < 99.5% → 15% | 48 hr notice |
| Starter | 99.5% | < 99.5% → credit on request | no proactive notice |

The IC should know the current burn rate by the end of mobilize phase. The
customer liaison uses the burn rate to decide whether to trigger
proactive-notice comms.

## Error budget calculation

```
error_budget_month = (1 − sla_target) × month_total_minutes
monthly_budget_99_95 = 0.0005 × 43200 ≈ 21.6 min
monthly_budget_99_9  = 0.001  × 43200 ≈ 43.2 min
```

At 15 min into a SEV1, you have burned most of the 99.95% monthly budget.
Communicate this explicitly to leadership; it drives the severity
reassessment and the mitigation aggressiveness trade-off.

## Proactive-notice rule

If breach is confirmed or highly probable, customer liaison sends the
proactive notice within the contractual window (typically 24 hr) — even if
the incident is still ongoing. Waiting for resolution to notify is a
contractual breach on top of the service breach.

Template: see `communication-templates.md::Customer email — SEV1 direct
outreach`. Add a dedicated line: `SLA impact: your account crossed the
99.95% monthly threshold at [time] UTC.`

## Partial-impact crediting

When impact is scoped (one region, one tenant, one feature), credit is
calculated against the affected scope, not the full contract:

```
credit_percent = impact_scope_fraction × outage_minutes / month_minutes
```

Document the scope carefully during the incident (scribe owns this). After
the fact it is hard to reconstruct whether EU-West was affected or only
EU-Central, and that reconstruction work falls on customer success.

## Handoff to contract / customer success

Within 48 hr of SEV1 / SEV2 resolution:

1. Scribe compiles: start time, end time, affected scope, affected tenants
   (list), observed symptoms, mitigation applied.
2. Customer success computes credits per contract.
3. Legal reviews wording for regulatory-adjacent incidents before external
   send (see `regulatory-deadlines.md`).
4. Credits and PIR go out together for consistency.

## IC decision points involving SLA

- **Rollback vs forward-fix** with budget pressure. If error budget is
  already blown, prefer the safe rollback even if it takes longer; a second
  incident in the same window doubles the credit exposure.
- **Proactive notice timing.** Notifying before a breach is confirmed can
  be over-cautious; notifying after the window closes is a contractual
  breach. IC and customer liaison jointly decide; document the decision.
- **Extending the monitoring window before declaring resolved.** For SLA-
  material incidents, extend the monitoring window to at least two full
  cycles of the affected user behavior (e.g., two checkout waves).

## Anti-patterns

- Stopping TTR when the deploy completes instead of when impact ends. This
  under-reports and shows up during the next contract review.
- Skipping proactive notice because resolution felt "close". A fast
  mitigation does not waive contractual notification.
- Calculating credits informally post-hoc. Credits must come from the
  contracted formula with documented scope, not from a customer-success
  apology discretion.
- Sharing raw SLA math with customers before legal review. Keep percent-
  level statements; legal wording varies by jurisdiction.

## Cross-references

- Response roles + cadence: `incident-response-framework.md`.
- Notification wording: `communication-templates.md`.
- Compliance-specific windows: `regulatory-deadlines.md`.
