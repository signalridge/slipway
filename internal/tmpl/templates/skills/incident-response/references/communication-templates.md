# Communication Templates

Distilled from `alirezarezvani/incident-commander/references/communication_templates.md`.
Use these to eliminate wordsmithing during active incidents. Severity-keyed;
IC or customer liaison approves before sending.

## Status page — initial acknowledgement (SEV1 / SEV2)

```
INVESTIGATING — [time] UTC

We are investigating reports of [symptom: checkout errors / login failures /
slow responses] affecting [scope: all users / US region / enterprise tier].

Impact: [user-visible behavior]
Scope: [geographic or tenant scope]
Next update: [time, within cadence for sev]
```

Rule: do not promise a root cause or ETA in the first post. Acknowledge
symptom + scope + next update time only.

## Status page — update (SEV1 every 15 min; SEV2 every 30 min)

```
IDENTIFIED / MONITORING — [time] UTC

[One-sentence state change: root cause identified / mitigation applied /
metrics recovering.]

Current impact: [updated scope]
Action in progress: [current mitigation]
Next update: [time]
```

Silence reads as abandonment. Post the cadence update even when nothing has
changed: `Still investigating; no new findings. Next update 15:30 UTC.`

## Status page — resolved

```
RESOLVED — [time] UTC

[Symptom] has been resolved. Service has been stable for [duration] across
affected [regions / tenants].

Root cause summary will follow in a public postmortem within [5 business
days for SEV1 | 10 for SEV2].
```

Wait for one full monitoring window of green before declaring resolved.

## Customer email — SEV1 direct outreach (enterprise)

```
Subject: [Service] Incident Update — [date]

[Customer name] team,

At [start time UTC] we detected [symptom]. [Restored / still mitigating] as
of [time]. Your account [was / was not] affected; observed impact on your
tenant: [details].

We will send a postmortem within [5 business days]. If you observed
additional impact, reply here and our team will investigate.

— [IC name or customer liaison], [title]
```

Rule: send only when customer impact is confirmed. Vague "you may have been
affected" emails erode trust.

## Executive summary (SEV1, every 30 min to leadership)

```
[Time UTC] — SEV1 — [one-line symptom]

Status: [Investigating / Identified / Mitigating / Monitoring / Resolved]
Impact: [users affected, revenue estimate if available, compliance exposure]
Current action: [what IC is doing right now]
Blockers: [none / waiting on X / need authority for Y]
Next milestone: [e.g. rollback complete in 10 min]
```

This is IC-drafted, customer-liaison-delivered. Leadership should read it
in 30 seconds.

## Internal channel — mobilize message

```
🚨 SEV[N] declared: [one-line symptom]
IC: @[name]  Scribe: @[name]  Ops lead: @[name]
Bridge: [zoom / meet link]
Thread here. Status updates every [cadence]. Customer liaison: @[name].
```

Always in the incident channel; never DMs. Pin the message.

## Handoff — IC rotation announcement

```
IC handoff at [time]:
Incoming IC: @[name] (taking over)
Outgoing IC: @[name]
Status: [one-line summary]
Active hypotheses: [ranked list]
Pending actions: [with owners]
Escalation state: [pages outstanding]
Next status update: [time]
```

Read this on the bridge. Incoming IC confirms acceptance in-channel.

## Postmortem — stakeholder announce

```
[Service] SEV[N] Postmortem — [date]

Summary: [two-sentence narrative].
Timeline: [key events with timestamps; see attached full timeline].
Root cause: [one sentence; reference contributing factors].
Mitigation: [what stopped the bleeding].
Action items: [N committed, owners + dates; see doc].

Full PIR: [link]
```

Blameless. No individual names as causes; name teams, systems, and gaps.

## Status-page language anti-patterns

- "We are experiencing issues" — vague; users cannot decide if they need to
  retry or route around.
- "Should be resolved shortly" — unbounded. Commit to a next-update time
  instead.
- "Our engineers are working hard on this" — self-congratulation; users do
  not care about effort, only impact.
- Root cause speculation before it is confirmed. Wait for RCA stage.

## Cross-references

- Severity-to-cadence mapping: `incident-severity-matrix.md`.
- SLA clock rules and breach notifications: `sla-management-guide.md`.
- Regulatory wording requirements (GDPR, HIPAA, PCI): `regulatory-deadlines.md`.
