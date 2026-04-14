# Domain：code-review-security

| Skill | Tier | 主要绑定 |
|-------|------|----------|
| `security-review` | T1 | command `review`；hosts `spec-compliance-review`、`code-quality-review` |
| `threat-modeling` | T1 | commands `review`、`validate`；`export-only` |
| `gha-security-review` | T2 | commands `review`、`repair` |
| `supply-chain-audit` | T2 | commands `review`、`repair`、`status` |
| `sast-orchestration` | T2 | commands `review`、`validate`、`repair` |

作用：

1. 执行 secure-default + framework-aware 的代码安全审查。
2. 对 trust boundary 做 threat modeling，并可作为 verdict/export 输入。
3. 把 GHA、供应链、SAST 等 specialist 能力放在命令面后方（T2 路由）。

说明：

- T2 路由携带 `tool-recipe` 附着（`semgrep`、`codeql`、`sarif`、GHA 审计套路），不进入 governed kernel。
- `threat-modeling` 虽绑定窄，仍是 T1，因为它表达可复用分析方法。
