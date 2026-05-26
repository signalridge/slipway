# Incident Severity Matrix

Distilled from `alirezarezvani/incident-commander/references/incident_severity_matrix.md`.
This is the triage table; it sits next to the framework and drives
paging/SLA/communication cadence.

## Four-level matrix

| Sev | User impact | Revenue / compliance | Examples | Time-to-ack | Status cadence | Stakeholder tier |
|-----|-------------|----------------------|----------|-------------|----------------|------------------|
| SEV1 | Full outage or data loss for all / majority of users | Direct revenue loss, regulatory breach, security incident | Site down, auth broken, PII leak, payment path broken, ransomware | 5 min | every 15 min | Exec, all eng leadership, legal on data |
| SEV2 | Major degradation for a segment | Material revenue impact, SLA breach imminent | One region down, checkout slow, API error rate ≥ 5%, single tenant broken | 15 min | every 30 min | Affected product lead, on-call eng mgr |
| SEV3 | Partial degradation, workaround exists | Minor revenue impact, non-critical SLO breach | Non-critical feature broken, one job failing, latency 2× but within SLA | 1 hr | 2×/day | Owning team, PM |
| SEV4 | No user impact | No revenue impact | Internal tool glitch, doc error, cosmetic bug | next business day | on resolve | Owning team |

Reassess severity every 30 min during the incident. Do not downgrade to
close the incident faster; downgrade only when impact has demonstrably
reduced for one full monitoring window.

## Axis definitions

- **User impact** = percent of user-visible actions degraded × baseline
  activity. A 5% action-error rate across 100% of users is SEV2, not SEV3.
- **Revenue / compliance** = direct monetary, contractual, or regulatory
  impact. Regulatory exposure (GDPR, HIPAA, PCI) shortcuts to SEV1 or SEV2
  regardless of user-visible impact.
- **Blast radius** = scope of the impact (global / regional / tenant /
  internal). Use this axis when the user-impact axis is ambiguous.

## Tie-breaker rules

1. **When axes disagree, take the higher severity.** A minor UX bug with a
   compliance breach is SEV1, not SEV3.
2. **Unknown-scope at T0** → assume one level higher than the smallest
   plausible scope. Downgrade when evidence arrives.
3. **Security incidents** default to SEV2 or higher until scope is known.
   Data-loss or data-exfiltration confirmed → SEV1.
4. **Repeated incidents** (same failure mode 3× in a week) → upgrade by one
   level for the next occurrence. Chronic pain is a reliability failure.

## Assignment discipline

- Severity is assigned by the IC within 5 minutes of paging, not by the
  first responder debating it.
- Severity is **announced in-channel** with one line: `SEV2 — checkout path
  5×× error rate, one region, blast radius US-East, compliance none`.
- Severity is **reassessed** at every status cadence. Changes are
  announced in-channel with reason: `Escalating SEV2→SEV1; error rate now
  global`.

## Anti-patterns

- Starting with SEV3 "to be safe" and escalating only when exec ping.
  Under-severity wastes the first 30 min on under-resourced triage.
- Rolling your own severity definitions per team. The org-wide matrix is
  the only one that paging + status pages + PIRs agree on.
- Using customer count as the only axis. Regulatory exposure and data
  safety are independent axes and can override user-count.

## Cross-references

- Response cadence + role contracts: `incident-response-framework.md`.
- This matrix intentionally stops at severity assignment, escalation, and
  cadence triggers; downstream communication/compliance playbooks are out of
  the default hydrate set.
