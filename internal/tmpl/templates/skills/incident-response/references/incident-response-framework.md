# Incident Response Framework

Operational synthesis of PagerDuty, Google SRE (*Site Reliability Engineering*
ch. 14), and Atlassian incident-management patterns. Distilled to the
action-changing material: roles, phase entry/exit gates, handoffs, and
decision authority. Upstream:
`alirezarezvani/incident-commander/references/incident-response-framework.md`.

## Core roles

| Role | Owns | Does NOT do |
|------|------|-------------|
| Incident Commander (IC) | End-to-end authority, delegation, escalation | Technical investigation |
| Scribe | Timestamped decisions + actions + findings in channel | Technical work |
| Subject Matter Expert (SME) | Targeted investigation, reports to IC | Coordinate with other SMEs directly |
| Customer Liaison | Outbound customer comms, status page drafts | Inbound triage, technical debug |
| Ops Lead (optional, SEV1) | Operational track execution | Stakeholder comms |

**Rule:** the person debugging is never the person communicating. Google
measured ~40% MTTR regression when one person held both; it is a discipline
failure, not a staffing problem.

## Phase gates

1. **Detect** — alert fires, page acknowledged. Exit: confirmed real incident
   (not monitor glitch), owner paged, severity assigned.
2. **Triage** — severity assessed, IC assigned, channel opened. Exit: named
   IC, SEV level, one-line incident summary.
3. **Mobilize** — SMEs paged, bridge opened if SEV1/SEV2, status page
   acknowledged. Exit: all critical SMEs joined, command post active.
4. **Mitigate** — stop the bleeding (rollback, failover, rate-limit,
   circuit-break). Mitigation precedes root cause. Exit: impact contained.
5. **Resolve** — service restored to baseline. Exit: healthy metrics ≥ one
   full monitoring window, customer-facing status updated.
6. **Postmortem** — blameless review, action items, regulatory filings.
   Exit: PIR published, actions assigned with owners + dates.

## Severity ladder (drives paging + SLA)

| Sev | Blast radius | Examples | Response time | Status cadence |
|-----|--------------|----------|---------------|----------------|
| SEV1 | Full outage, revenue loss, data loss, security breach | Site down, auth broken, PII leak | 5 min | every 15 min |
| SEV2 | Major degradation, one region or one tenant | Checkout slow, one data center down | 15 min | every 30 min |
| SEV3 | Partial degradation, workaround exists | Non-critical feature broken | 1 hr | 2×/day |
| SEV4 | Minor, no user impact | Internal tool glitch | next business day | on resolve |

Reassess severity every 30 min (Atlassian rule). Upgrading is cheap;
downgrading prematurely costs trust.

## Decision authority

- IC has final say during the incident. Conflicting opinions resolve via IC,
  not committee.
- When a runbook/handbook does not cover the situation, decision hierarchy:
  (1) protect customer data, (2) restore service, (3) preserve evidence for
  root cause analysis.
- Escalation: if IC is blocked (unknown system, missing credentials, cross-
  org dependency), escalate to named VP/Director within 15 minutes. Do not
  sit on a block waiting for someone to notice.

## Operational vs communication track

Split the work. Operational track runs mitigation + debug. Communication
track runs status page + customer emails + executive briefings. IC bridges
the two. Status updates post on a fixed cadence regardless of whether
anything new has happened; silence reads as abandonment.

## Handoff protocol (IC rotation)

Rotate IC every 60–90 min on SEV1. Handoff happens on the bridge call, not
asynchronously. Script:

1. Current status in one sentence.
2. Active hypotheses (ranked).
3. Pending actions with owners.
4. Escalation state (who is paged, who has not responded).
5. Next planned status update time.

## Runbook-first, improvise-second

Follow runbooks before improvising. Atlassian measures 50–60% MTTR
reduction for known failure modes when responders follow the playbook. If
no runbook exists, write the first line of one at the top of the channel
transcript before debugging — the postmortem will need it.

## Anti-patterns

- IC also debugging. Split the role within 10 min or get off the debug.
- Status updates in DMs or side threads. Everything goes to the channel.
- Silent handoffs. Announce rotation in-channel with the 5-field script.
- "We'll figure out severity later." Severity drives paging. Assign in
  triage, reassess every 30 min.
- Root cause before mitigation. Stop the bleeding first; autopsy later.
- Declaring resolved on first green metric. Wait one full monitoring window.

## Cross-references

- Severity taxonomy: `incident-severity-matrix.md`.
- Status templates and customer scripts: `communication-templates.md`.
- SLA clocks and breach handling: `sla-management-guide.md`.
- Postmortem methodology: `rca-frameworks-guide.md`.
- Regulatory reporting deadlines: `regulatory-deadlines.md`.
