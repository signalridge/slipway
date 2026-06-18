---
skill_id: security-review
domain: review-security
function: workflow-owned S3 secure-default, boundary- and framework-aware security review
tier: T1
primary_attachment: checklist
summary: "Use when running the S3 security review for auth/authz, injection, secrets, SSRF, and insecure defaults. Triggers on the workflow-owned S3 review host, the `slipway review` command, a security-classified guardrail, or security-review control selected by blast-radius policy."
trigger_signals:
  - command: review
    reason: "review command invoked; attach security checklist"
  - selected_control: security-review
    reason: "Workflow or policy selected the S3 security reviewer"
  - guardrail: security-classified
    reason: "Security-classified guardrail requires secure-default review"
scope_hints:
  - changed_files_include: ["**/auth/*", "**/crypto/*", "**/session*"]
    reason: "Security-sensitive paths expand review scope after selection; they do not select this skill by themselves"
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
---

# Security Review

```
IRON LAW: SECURE DEFAULT, EXPLICIT DEVIATION
```

## Purpose
Review security-relevant code against secure-default expectations and known
risky escape hatches. In S3 this runs as a workflow-owned review host with its
own native subagent context. Every deviation from a secure default must be
called out with a reproducible observation, not a taste argument.

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
