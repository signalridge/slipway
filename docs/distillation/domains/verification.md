# Domain: verification

| Skill | Tier | Primary bindings |
|-------|------|------------------|
| `coverage-analysis` | T1 | command `validate`; host `goal-verification` |
| `property-testing` | T1 | command `validate`; host `goal-verification` |
| `mutation-testing` | T1 | command `validate`; host `goal-verification` |
| `performance-profiling` | T1 | commands `validate`, `status`; host `goal-verification` |

Role:

1. Feed goal-verification with fresh, method-appropriate evidence.
2. Surface diagnostics-style summaries via `status` for long-running work
   (`performance-profiling`).

Notes:

- Each verification skill produces a structured verdict consumable by the
  goal-verification host.
- `mutation-testing` carries a tool-recipe attachment (typical driver:
  `pit`, `mutmut`, `stryker`, `gomutation`).
