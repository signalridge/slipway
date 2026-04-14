# Domain: code-review-quality

| Skill | Tier | Primary bindings |
|-------|------|------------------|
| `independent-review` | T1 | hosts `spec-compliance-review`, `code-quality-review`; command `review` |
| `multi-reviewer-calibration` | T1 | host `code-quality-review`; command `review` |

Role:

1. Fresh-context, verdict-contracted review at host level.
2. Dedupe + severity calibration across multiple reviewer passes.

Notes:

- `independent-review` is the B1 demonstration skill for host + command binding.
- `multi-reviewer-calibration` absorbs `adversarial-reviewer`,
  `multi-reviewer-patterns`, and `code-review-ai-ai-review`.
