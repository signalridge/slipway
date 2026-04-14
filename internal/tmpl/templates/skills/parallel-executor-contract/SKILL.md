---
skill_id: parallel-executor-contract
domain: execution
function: bounded parallel subagent dispatch with reviewable handoff
tier: T1
primary_attachment: procedure
summary: "Use when dispatching subagents in parallel. Triggers on wave-orchestration host or when the plan calls for multi-agent work."
trigger_signals:
  - host: wave-orchestration
    reason: "Wave orchestration host active; enforce parallel executor contract"
  - user_text_matches: ["parallel", "subagent", "in parallel"]
    reason: "Request involves parallel execution"
evidence_contract: artifact
bindings:
  - type: host-embedded
    target: wave-orchestration
    attachment: procedure
  - type: host-embedded
    target: wave-orchestration
    attachment: checklist
provenance_ref: provenance.yaml
---

# Parallel Executor Contract

```
IRON LAW: EVERY PARALLEL DISPATCH CARRIES A REVIEWABLE CONTRACT
```

## Purpose
Dispatch subagents in parallel only under a bounded contract. Each executor
receives an isolated scope, explicit inputs, a deterministic output schema,
and a rejoin point. The orchestrator retains verdict authority.

## Procedure
1. Partition work along disjoint seams; reject partitions that share mutable
   state without an explicit merge strategy.
2. For each executor, write a one-screen brief: scope, inputs, output schema,
   rejection criteria.
3. Dispatch in one batch; do not interleave orchestration and execution.
4. On rejoin, validate each output against its schema before integrating.
5. Record the dispatch manifest (executor id, scope, status) alongside the
   execution artifact so a reviewer can reconstruct the wave.

## Checklist
- [ ] Partitions are disjoint or carry an explicit merge strategy.
- [ ] Each executor brief names scope, inputs, output schema, reject criteria.
- [ ] Dispatch happens in a single batch.
- [ ] Rejoin validates schema before integrating.
- [ ] Manifest recorded with executor ids and status.

## Anti-patterns
| Rationalization | Counter-rule |
|---|---|
| "The executors will coordinate" | They cannot; coordination is the orchestrator's job. |
| "Schema is overhead for small work" | Without schema, rejoin drifts to prose merging. |
| "Dispatch one, wait, dispatch next" | That is serial; call it serial, not parallel. |
