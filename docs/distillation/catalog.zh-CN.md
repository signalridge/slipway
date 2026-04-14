# 目录（目标索引）

每行对应一个 Slipway 目录技能。共 25 条技能，分布于 9 个 domain。

| # | Skill | Domain | Tier | Primary attachment | 主绑定 | 状态 |
|---|-------|--------|------|--------------------|------------------|--------|
| 1 | `scope-clarification` | intake | T1 | posture + checklist | host `intake-clarification`; technique-hint | B1 |
| 2 | `context-assembly` | intake | T1 | procedure + posture | hosts `research-orchestration`, `plan-audit`; technique-hint | B2 |
| 3 | `plan-authoring` | intake | T1 | procedure + checklist | host `plan-audit`; host-embedded; export-only | B1 |
| 4 | `tdd-proof` | execution | T1 | procedure | hosts `tdd-governance`, `wave-orchestration`; technique-hint | B1 |
| 5 | `parallel-executor-contract` | execution | T1 | procedure + checklist | host `wave-orchestration` | B2 |
| 6 | `fresh-verification-evidence` | execution | T1 | checklist + report-schema | hosts `goal-verification`, `final-closeout`, `tdd-governance` | B1 |
| 7 | `root-cause-tracing` | debugging | T1 | procedure | host `wave-orchestration`; command `repair`; technique-hint | B2 |
| 8 | `independent-review` | review-quality | T1 | procedure + checklist + report-schema | hosts `spec-compliance-review`, `code-quality-review`; command `review` | B1 |
| 9 | `multi-reviewer-calibration` | review-quality | T1 | procedure + checklist | host `code-quality-review`; command `review` | B4 |
| 10 | `security-review` | review-security | T1 | checklist | command `review`; hosts `spec-compliance-review`, `code-quality-review` | B2 |
| 11 | `threat-modeling` | review-security | T1 | procedure + report-schema | commands `review`, `validate`; export-only | B3 |
| 12 | `gha-security-review` | review-security | T2 | checklist + tool-recipe | commands `review`, `repair` | B3 |
| 13 | `supply-chain-audit` | review-security | T2 | checklist + tool-recipe | commands `review`, `repair`, `status` | B3 |
| 14 | `sast-orchestration` | review-security | T2 | tool-recipe | commands `review`, `validate`, `repair` | B3 |
| 15 | `differential-review` | review-change-shape | T1 | procedure + checklist | command `review` | B4 |
| 16 | `variant-analysis` | review-change-shape | T1 | procedure | commands `review`, `repair` | B4 |
| 17 | `spec-trace` | review-change-shape | T1 | checklist + report-schema | host `spec-compliance-review`; commands `validate`, `review` | B2 |
| 18 | `coverage-analysis` | verification | T1 | checklist + report-schema | command `validate`; host `goal-verification` | B4 |
| 19 | `property-testing` | verification | T1 | procedure + checklist | command `validate`; host `goal-verification` | B4 |
| 20 | `mutation-testing` | verification | T1 | tool-recipe + report-schema | command `validate`; host `goal-verification` | B4 |
| 21 | `performance-profiling` | verification | T1 | procedure + report-schema | command `validate`; host `goal-verification`; command `status` | B4 |
| 22 | `ci-triage` | repair-ci | T2 | procedure + checklist | commands `repair`, `status` | B5 |
| 23 | `review-comment-triage` | repair-ci | T2 | procedure | command `repair` | B5 |
| 24 | `git-recovery` | repair-ci | T2 | procedure | commands `repair`, `status`；`worktree-preflight` 失败支持 | B5 |
| 25 | `incident-response` | ops-diagnostics | T3 | report-schema | commands `status`, `health`; export-only | B5 |

状态标注：`Bn` 指示该技能落地的批次。对应批次合入后，状态切换为 `shipped`。

Attachment 注记：

- 表中的 `Primary attachment` 列是压缩后的 attachment 画像。
- 第一项始终是 schema 层面的 `primary_attachment`；后续 `+ ...` 是为了
  便于阅读而补充的 binding 层 attachment。

## 层级分布

| Tier | 数量 |
|------|-------|
| T1 | 18 |
| T2 | 6 |
| T3 | 1 |
