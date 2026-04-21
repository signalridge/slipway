---
skill_id: gha-security-review
domain: review-security
function: review GitHub Actions workflows for privilege, pinning, and agentic-action risk
tier: T2
primary_attachment: checklist
summary: "Use when reviewing GitHub Actions workflows. Triggers on review or repair commands or on changes to .github/workflows paths."
trigger_signals:
  - changed_files_include: [".github/workflows/*", ".github/workflows/**/*"]
    reason: "GitHub Actions workflow changed"
  - command: ["review", "repair"]
    reason: "Review or repair command invoked; workflow surface may be in scope"
evidence_contract: verdict
hydrate_references:
  - name: pwn-request.md
    reason: "pull_request_target fork-code execution vector"
  - name: comment-triggered-commands.md
    reason: "comment-triggered workflow command injection"
  - name: expression-injection.md
    reason: "github.event.* interpolation injection into run blocks"
  - name: permissions-and-secrets.md
    reason: "least-privilege permissions and secret scoping rules"
bindings:
  - type: command-auto
    target: review
    attachment: checklist
  - type: command-auto
    target: repair
    attachment: tool-recipe
---

# GitHub Actions Security Review

```
IRON LAW: PINNED, LEAST-PRIVILEGED, UNTRUSTED-INPUT-AWARE
```

## Purpose
Review `.github/workflows/` for the three recurring GHA failure modes:
privilege overreach, mutable action references, and agentic/untrusted-input
exposure. The checklist is strict because remediation after compromise is
painful.

## Checklist
- [ ] `permissions:` is declared at workflow or job scope; defaults are not
      relied on. Read-only by default; write scopes justified per job.
- [ ] Third-party actions are pinned to a full commit SHA, not a tag or
      branch. First-party actions may use a tag if explicitly trusted.
- [ ] `pull_request_target` is used only when required, and never executes
      untrusted checked-out code.
- [ ] Agentic actions (actions that run model-driven steps) are gated: they
      cannot see secrets unavailable to the PR author, and their output is
      sanitized before re-entering the workflow.
- [ ] `${{ github.event.* }}` user-controlled values are not interpolated
      into shell commands without explicit escaping.
- [ ] Secrets are scoped per job; `secrets: inherit` is called out and
      accepted only when every callee is trusted.
- [ ] Reusable workflows are referenced by pinned SHA if third-party.

## Report schema
```yaml
verdict: pass | changes-requested | blocked
findings:
  - severity: blocker | major | minor
    category: permissions | pinning | untrusted-input | agentic | secrets
    location: "<workflow:line>"
    evidence: "<quote>"
    remediation: "<concrete action>"
```

## Anti-patterns
- Pinning to `@main` or a tag for third-party actions.
- Broad `permissions: write-all` to "keep the workflow simple".
- Unquoted `${{ github.event.issue.title }}` inside `run:` blocks.
