# Domain: code-review-security

| Skill | Tier | Primary bindings |
|-------|------|------------------|
| `security-review` | T1 | command `review`; hosts `spec-compliance-review`, `code-quality-review` |
| `threat-modeling` | T1 | commands `review`, `validate`; export-only |
| `gha-security-review` | T2 | commands `review`, `repair` |
| `supply-chain-audit` | T2 | commands `review`, `repair`, `status` |
| `sast-orchestration` | T2 | commands `review`, `validate`, `repair` |

Role:

1. Secure-default + framework-aware code security review.
2. Trust-boundary threat modeling, usable as verdict export.
3. Specialist T2 routes behind command surfaces for GHA, supply chain, SAST.

Notes:

- T2 routes carry tool-recipe attachments (`semgrep`, `codeql`, `sarif`,
  GHA audit patterns). They do not enter the governed kernel.
- `threat-modeling` remains T1 despite narrow binding because it captures a
  reusable analytical method.
