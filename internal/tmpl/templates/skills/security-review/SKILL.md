---
skill_id: security-review
domain: review-security
function: secure-default, boundary- and framework-aware security review
tier: T1
primary_attachment: checklist
summary: "Use when reviewing security-relevant code for auth/authz, injection, secrets, SSRF, and insecure defaults. Triggers on the `slipway review` command, a security-classified guardrail, or changes to auth/crypto/session paths."
trigger_signals:
  - command: review
    reason: "review command invoked; attach security checklist"
  - host: ["spec-compliance-review", "code-quality-review"]
    reason: "Review host active; include security checklist"
  - changed_files_include: ["**/auth/*", "**/crypto/*", "**/session*"]
    reason: "Security-sensitive paths changed"
evidence_contract: verdict
hydrate_references:
  - name: authentication.md
    reason: "Password storage / session / MFA / recovery secure-default rules"
  - name: authorization.md
    reason: "Resource-boundary re-check, IDOR, multi-tenant isolation"
  - name: injection.md
    reason: "Per-sink parameterization, deserialization-as-injection"
  - name: xss.md
    reason: "Context-aware encoding and framework escape-hatch review cues"
  - name: ssrf.md
    reason: "Fetcher allow/deny-list, metadata endpoints, DNS rebinding"
  - name: infrastructure-docker.md
    reason: "Container hardening, K8s securityContext, image supply chain"
bindings:
  - type: command-auto
    target: review
    attachment: checklist
  - type: host-embedded
    target: spec-compliance-review
    attachment: checklist
  - type: host-embedded
    target: code-quality-review
    attachment: checklist
---

# Security Review

```
IRON LAW: SECURE DEFAULT, EXPLICIT DEVIATION
```

## Purpose
Review security-relevant code against secure-default expectations and known
risky escape hatches. Every deviation from a secure default
must be called out with a reproducible observation, not a taste argument.

## Report schema
```yaml
verdict: pass | changes-requested | blocked
bottom_line: "<one-sentence summary>"
findings:
  - severity: blocker | major | minor | nit
    category: input | authn | authz | secrets | errors | dependency | default
    location: "<path:line>"
    evidence: "<quote or command output>"
    remediation: "<concrete action>"
```

## Anti-patterns
- "Looks fine" with no traversal of the listed categories.
- Blocker without a reproducible observation.
- Escape-hatch misuse flagged without citing the documented safe pattern.
