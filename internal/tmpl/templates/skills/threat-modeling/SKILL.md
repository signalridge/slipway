---
skill_id: threat-modeling
domain: review-security
function: structured threat enumeration with ownership map and mitigation trace
tier: T1
primary_attachment: procedure
summary: "Use when a change alters the trust boundary or asset surface. Triggers on review or validate commands, security-classified guardrails, or user text naming threats."
size_rationale: "Warn-band accepted: STRIDE walk + ownership mapping need compact examples to keep threat reviews reproducible."
trigger_signals:
  - command: ["review", "validate"]
    reason: "review or validate command invoked; enumerate threats against the change"
  - user_text_matches: ["threat model", "attack surface", "adversary", "trust boundary"]
    reason: "User text asks for threat modeling"
evidence_contract: artifact
bindings:
  - type: command-auto
    target: review
    attachment: procedure
  - type: command-auto
    target: validate
    attachment: procedure
  - type: export-only
    target: using-slipway-catalog
    attachment: report-schema
---

# Threat Modeling

```
IRON LAW: ENUMERATE THREATS AGAINST ASSETS, NOT AGAINST CODE
```

## Purpose
Enumerate threats against the assets and trust boundaries the change touches.
Do not hand-wave "what could go wrong"; attach each threat to an asset, an
actor, and a pre-existing or proposed mitigation.

## Report schema
```yaml
assets:
  - name: "<asset>"
    actors: ["<actor>", "<actor>"]
threats:
  - asset: "<asset>"
    actor: "<actor>"
    stride: spoofing | tampering | repudiation | disclosure | dos | eop
    mitigation: "<file:line | proposed change | accepted residual>"
    owner: "<skill or review surface>"
```

## Anti-patterns
- Threats written against the diff rather than the asset.
- STRIDE category left blank; every threat must fit a category or be dropped.
- Residual risk without named acceptance.
