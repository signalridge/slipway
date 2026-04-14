# Domain: code-review-change-shape

| Skill | Tier | Primary bindings |
|-------|------|------------------|
| `differential-review` | T1 | command `review` |
| `variant-analysis` | T1 | commands `review`, `repair` |
| `spec-trace` | T1 | host `spec-compliance-review`; commands `validate`, `review` |

Role:

1. Risk-prioritized diff review with blast-radius awareness.
2. Hunt for variants of known bug/vuln patterns.
3. Bidirectional spec-to-code trace.

Notes:

- `spec-trace` is the spec-compliance-review host-embedded skill.
- `variant-analysis` extends naturally into `repair` for pattern-driven fixes.
