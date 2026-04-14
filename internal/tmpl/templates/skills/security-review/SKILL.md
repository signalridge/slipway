---
skill_id: security-review
domain: review-security
function: secure-default and framework-aware security review
tier: T1
primary_attachment: checklist
summary: "Use when reviewing security-relevant code. Triggers on review command, security-classified guardrail, or changes to auth/crypto/input paths."
size_rationale: "Warn-band accepted: checklist + report schema are intentionally co-located so reviewers can apply and output in one pass."
trigger_signals:
  - command: review
    reason: "review command invoked; attach security checklist"
  - host: ["spec-compliance-review", "code-quality-review"]
    reason: "Review host active; include security checklist"
  - changed_files_include: ["**/auth/*", "**/crypto/*", "**/session*"]
    reason: "Security-sensitive paths changed"
evidence_contract: verdict
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
provenance_ref: provenance.yaml
---

# Security Review

```
IRON LAW: SECURE DEFAULT, EXPLICIT DEVIATION
```

## Purpose
Review security-relevant code against secure-default expectations and
framework-specific known-bad patterns. Every deviation from a secure default
must be called out with a reproducible observation, not a taste argument.

## Checklist
- [ ] Input boundaries validated: untrusted input is rejected or typed before
      reaching business logic.
- [ ] Authentication paths use framework-standard primitives; no hand-rolled
      crypto or session handling.
- [ ] Authorization is re-checked at the resource boundary, not inferred from
      the caller.
- [ ] Secrets are not logged, serialized, or returned in error payloads.
- [ ] Error paths fail closed; unexpected states do not expose privileged
      operations.
- [ ] Third-party calls cite the library's documented safe-usage pattern.
- [ ] Insecure defaults (e.g., permissive CORS, unverified TLS, weak hashing)
      are either absent or called out with a justification the reviewer
      accepted.

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
- Framework misuse flagged without citing the documented safe pattern.
