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
provenance_ref: provenance.yaml
---

# Threat Modeling

```
IRON LAW: ENUMERATE THREATS AGAINST ASSETS, NOT AGAINST CODE
```

## Purpose
Enumerate threats against the assets and trust boundaries the change touches.
Do not hand-wave "what could go wrong"; attach each threat to an asset, an
actor, and a pre-existing or proposed mitigation.

## Procedure
1. Name the assets the change affects (data, credentials, privileged
   operations). Asset naming comes first; skipping it produces vague threats.
2. Name the actors that can reach each asset (anonymous, authenticated user,
   internal service, admin). Cite the surface that exposes each actor.
3. For each (asset, actor) pair, enumerate threats using STRIDE (spoofing,
   tampering, repudiation, information disclosure, denial of service,
   elevation of privilege). Drop pairs with no plausible threat.
4. For each threat, cite the mitigation: existing code path, proposed change,
   or accepted residual risk. Residuals require explicit acceptance.
5. Produce an ownership map: which skill or review surface owns catching
   regressions of each mitigation.

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
