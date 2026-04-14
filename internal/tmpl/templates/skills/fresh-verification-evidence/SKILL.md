---
skill_id: fresh-verification-evidence
domain: execution
function: block completion claims without fresh commands and fresh proof
tier: T1
primary_attachment: checklist
summary: "Use when a change is approaching a verify/closeout gate. Triggers on goal-verification, final-closeout, or completion-adjacent step."
trigger_signals:
  - host: ["goal-verification", "final-closeout", "tdd-governance"]
    reason: "Verification or closeout host active; enforce fresh-evidence checklist"
  - blocker_reason: ["stale_verification_evidence", "missing_fresh_run"]
    reason: "Blocker cites stale or missing fresh verification"
evidence_contract: verdict
bindings:
  - type: host-embedded
    target: goal-verification
    attachment: checklist
  - type: host-embedded
    target: goal-verification
    attachment: report-schema
  - type: host-embedded
    target: final-closeout
    attachment: checklist
  - type: host-embedded
    target: tdd-governance
    attachment: checklist
provenance_ref: provenance.yaml
---

# Fresh Verification Evidence

```
IRON LAW: FRESH EVIDENCE OR NO COMPLETION CLAIM
```

## Purpose
Stop completion from advancing on stale, cached, or inferred evidence. Every
verdict must cite a command that ran after the last code change.

## Checklist
- [ ] Run version in the verdict matches the latest execution run.
- [ ] Command transcript in the verdict is the most recent run, not an earlier
      successful run retained for convenience.
- [ ] Every acceptance signal names a concrete, reproducible observation.
- [ ] Coverage or verification reports cite the same timestamp window as the
      command transcript.
- [ ] If a previous verdict passed, the newer verdict references it by run
      version to make the freshness delta auditable.

## Report schema (minimum)
```yaml
verdict: pass | blocked
run_version: <int>
timestamp: "<ISO-8601-UTC>"
command: "<reproducible command>"
transcript_ref: "<path to captured transcript>"
fresh: true
blockers: []
```

## Anti-patterns
- Copying the previous run's command transcript.
- Claiming pass based on "I tested locally last week."
- Declaring pass before the run completes.

## Failure handling
- Run version mismatch → block as `stale_verification_evidence`.
- Missing command transcript → block as `missing_fresh_run`.
