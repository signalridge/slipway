# Root-Cause Analysis Frameworks

Distilled from `alirezarezvani/incident-commander/references/rca_frameworks_guide.md`.
Methodology reference for postmortem authoring. Blameless by default; the
framework chosen depends on the failure shape.

## Framework selection

| Failure shape | Framework | Why |
|---------------|-----------|-----|
| Single-system, deterministic root cause | 5 Whys | Fast, linear; ships the action items the team actually owns. |
| Multi-system interaction, no single cause | Fishbone (Ishikawa) | Exposes cross-team contributing factors. |
| Systemic, cultural, or process failure | Cynefin / Causal Loop Diagram | Names feedback loops that 5 Whys flattens. |
| Near-miss or novel failure mode | Pre-mortem (reverse) | What would we have had to believe in advance to prevent this? |

Do not pick the framework after the data is in. Pick during scoping so the
investigation collects the right evidence.

## 5 Whys — operational recipe

1. State the failure in one sentence. This is the "what", not the "why".
2. Ask why. Answer with a fact grounded in the timeline, not a narrative.
3. Ask why again of the answer. Repeat until the answer names a system,
   policy, or process — never a person.
4. Stop when the next "why" would point at human nature or market
   realities. Those are out of scope; the actionable cause is the layer
   just above.
5. For each layer, record: the fact, the evidence (link / timestamp), and
   the generalization (does this fail only here or everywhere?).

**Person-as-cause is a dead end.** If the last "why" is "engineer forgot to
set the flag", the next why is "why was it easy to forget?" or "why did the
flag not default safe?". Process is always the layer above human.

## Fishbone — when to reach for it

Use when the incident touches three or more systems and no single cause is
dominant. Spine categories (adapt per org):

- **People** — authority confusion, missing runbook, handoff gap.
- **Process** — review bypass, severity mislabel, rollout policy.
- **Tooling** — monitoring blind spot, deploy tool bug, rollback path
  broken.
- **Dependencies** — upstream provider, third-party SDK, internal shared
  service.
- **Data** — schema drift, bad migration, dirty input.
- **Environment** — config drift, capacity, network.

For each category, list contributing factors with evidence. The "cause" is
the conjunction of factors, not any single one.

## Causal loop — systemic failures

Draw arrows between forces that strengthen each other. Example pattern:
on-call fatigue → slower ack → longer incidents → faster on-call rotation
→ less runbook investment → on-call fatigue. Action items target the loop,
not any single edge.

## Timeline discipline

Every postmortem opens with a timestamped timeline. Minimum fields per
entry: UTC time, actor (system or person), action (observed), evidence
link. Scribe output from the incident is the raw material; authoring
narrows it to the load-bearing events.

Mark the three critical timestamps:

- **T0** — when the failure started (not when it was detected).
- **T_detect** — when monitoring actually alerted.
- **T_mitigate** — when impact began decreasing.

`T_detect − T0` is the detection gap; it drives monitoring action items.
`T_mitigate − T_detect` is the response gap; it drives runbook / tooling
action items.

## Action item authoring

Every action item has: owner, due date, verification method, and severity
of the failure mode it addresses. Action items without verification are
tasks, not prevention. Example:

```
AI-3: Add synthetic check for checkout-path end-to-end latency.
Owner: @jordan (platform)
Due: 2026-04-30
Verification: alert fires on staged latency injection in load test.
Prevents: undetected latency regression (SEV2 failure mode from this PIR).
```

## Blameless framing

- Name systems, teams, and gaps. Never individuals as root cause.
- Assume everyone made the best decision with the information available.
- Ask "what information or tooling was missing?" not "why did X do this?".
- Circulate the draft to participants before publishing. Not for veto, for
  factual correction.

## Publication

SEV1 → public postmortem within 5 business days. SEV2 → internal within 10
business days; public if customer-visible. SEV3/SEV4 → internal ticket
with RCA summary within the sprint.

## Anti-patterns

- Stopping at the first plausible cause. Go one layer deeper than
  comfortable.
- Listing action items without owners or verification. Those items do not
  ship.
- Naming individuals. Violates blameless contract and destroys the signal
  for next time.
- Letting the timeline be a narrative. Timeline is evidence; narrative
  goes in the summary section.

## Cross-references

- Incident lifecycle and roles: `incident-response-framework.md`.
- Severity drives PIR publication window: `incident-severity-matrix.md`.
- Regulatory-filing requirements interact with PIR timing:
  `regulatory-deadlines.md`.
