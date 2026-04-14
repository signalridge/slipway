# Domain: execution-discipline

Catalog skills that govern how S2 execution stays honest.

| Skill | Tier | Primary bindings |
|-------|------|------------------|
| `tdd-proof` | T1 | hosts `tdd-governance`, `wave-orchestration`; technique-hint |
| `parallel-executor-contract` | T1 | host `wave-orchestration` |
| `fresh-verification-evidence` | T1 | hosts `goal-verification`, `final-closeout`, `tdd-governance` |

Role:

1. Keep the guardrail-domain path test-first.
2. Constrain parallel subagent dispatch.
3. Forbid completion claims without fresh commands + fresh evidence.

Notes:

- `tdd-proof` + `fresh-verification-evidence` together cover the tdd-governance
  host; they do not replace it, they enrich its prompt.
- `parallel-executor-contract` absorbs `dispatching-parallel-agents`,
  `subagent-driven-development`, and the spec-kitty implement-review posture.
