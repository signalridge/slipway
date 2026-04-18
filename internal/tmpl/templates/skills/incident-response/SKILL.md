---
skill_id: incident-response
domain: ops-diagnostics
function: incident-response posture - contain, diagnose, communicate, and write up
tier: T3
primary_attachment: report-schema
summary: "Use when a production incident is suspected or active. Triggers on status or health commands or user text naming an incident."
size_rationale: "Warn-band accepted: timeline, containment, diagnosis, and communication fields are intentionally in one schema for on-call handoff."
trigger_signals:
  - command: ["status", "health"]
    reason: "status or health command invoked; incident may be in scope"
  - user_text_matches: ["incident", "outage", "page", "sev1", "sev2"]
    reason: "User text names an incident"
evidence_contract: artifact
hydrate_references:
  - name: incident-response-framework.md
    reason: "Core roles, phase gates, decision authority"
  - name: incident-severity-matrix.md
    reason: "SEV1-4 triage criteria and escalation thresholds"
  - name: communication-templates.md
    reason: "Status-page and stakeholder message templates"
  - name: sla-management-guide.md
    reason: "SLA clock rules, breach thresholds, credit calculation"
  - name: rca-frameworks-guide.md
    reason: "Postmortem frameworks and action-item authoring"
  - name: regulatory-deadlines.md
    reason: "GDPR/HIPAA/PCI notification windows and wording"
bindings:
  - type: export-only
    target: using-slipway-catalog
    attachment: report-schema
---

# Incident Response

```
IRON LAW: CONTAIN FIRST, DIAGNOSE SECOND, COMMUNICATE CONTINUOUSLY
```

## Purpose
During an incident, the order of operations is fixed: stop the bleeding,
then understand the wound. Communication runs in parallel with both. The
post-incident write-up is mandatory, not optional.

## Report schema
```yaml
incident:
  id: "<incident id>"
  severity: sev1 | sev2 | sev3
  opened_at: "<iso8601>"
  commander: "<name>"
  summary: "<one line>"
timeline:
  - at: "<iso8601>"
    actor: "<name or role>"
    action: "<verb phrase>"
    evidence: "<link or quote>"
containment:
  status: active | contained | resolved
  actions:
    - "<what was done, with link>"
diagnosis:
  hypothesis: "<current best explanation>"
  disconfirming_checks: ["<observation that would disprove it>"]
  root_cause: "<only when confirmed>"
communication:
  stakeholders: ["<channel or audience>"]
  last_update_at: "<iso8601>"
postmortem:
  write_up_due: "<iso8601>"
  action_items: ["<owner: item>"]
```

## Posture
- Containment actions precede diagnostic actions when both are available.
- Hypotheses are provisional until a disconfirming check fails.
- Communication cadence is set at open time; missed cadences are logged.
- The commander role owns the verdict; diagnosis can be delegated.

## Anti-patterns
- Deep-diving a root cause before containment stabilizes.
- Silent coordination channels that stakeholders cannot see.
- Skipping the write-up because "we know what happened".
