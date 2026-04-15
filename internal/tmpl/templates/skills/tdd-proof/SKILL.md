---
skill_id: tdd-proof
domain: execution
function: enforce RED-GREEN-REFACTOR and test-first proof for guardrail work
tier: T1
primary_attachment: procedure
summary: "Use when executing guardrail-domain work. Triggers on tdd-governance host or on execution covered by a guardrail domain."
trigger_signals:
  - all_of:
      - host: ["tdd-governance", "wave-orchestration"]
    reason: "Execution host active; inject TDD procedure"
  - blocker_reason: ["guardrail_domain_requires_tdd", "missing_red_proof"]
    reason: "Blocker cites missing TDD proof"
evidence_contract: verdict
hydrate_references:
  - name: testing-anti-patterns.md
    reason: "Anti-patterns that defeat test-first proof"
bindings:
  - type: host-embedded
    target: tdd-governance
    attachment: procedure
  - type: host-embedded
    target: wave-orchestration
    attachment: procedure
  - type: technique-hint
    target: tdd-governance
    attachment: procedure
---

# TDD Proof

```
IRON LAW: NO GUARDRAIL CHANGE WITHOUT A FAILING TEST FIRST
```

## Purpose
Force a documented RED → GREEN → REFACTOR progression for any guardrail-domain
work. The procedure is prompt-level; it does not replace the existing
`tdd-governance` host.

## Procedure
1. **RED** — Write the failing test and record its exact failure output.
   No production change is authored in this step.
2. **GREEN** — Make the minimum production change that turns RED into GREEN.
   Capture the command + run version used to observe the transition.
3. **REFACTOR** — Improve structure without changing behaviour. Re-run the
   full test target; a fresh GREEN run is required.
4. **EVIDENCE** — Write the verdict record with RED + GREEN + REFACTOR
   timestamps, commands, and run versions.

## Guardrails
- Do not skip RED, even for "obvious" fixes.
- Do not amend a test after making it pass to widen its scope without a new
  RED cycle.
- Do not claim GREEN without a full command transcript.

## Anti-patterns
- Adding a test and the fix in one commit without a RED capture.
- Mock-heavy tests in a domain where the incident risk is mock/prod drift.
- Refactoring before evidence capture is complete.

## Failure handling
- Missing RED capture → block as `missing_red_proof`.
- Stale GREEN (different run version) → block as `stale_verification_evidence`.
