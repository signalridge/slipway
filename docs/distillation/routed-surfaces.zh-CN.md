# 路由面（非目录技能）

以下为冻结的非目录面清单。它们不是目录技能，分为只读视图（`status` / `health` 诊断）、
仅显式路由（通过命令路由或覆盖可达）、吸收（归并至其他目标）、或延后（未来工作）。

## view-only

挂在 `status` / `health` / `validate` 下的只读诊断落点。不具备推进权威，
且只有其中一部分目前真正暴露为显式 CLI selector。

| 面 | 落点 | 原因 |
|---------|--------------|--------|
| `review-queue` | `status` 视图 | 队列聚合 |
| `observability-query` | `status` / `health` 视图 | 只读检查 |
| `claude-settings-audit` | `health` / `validate` 诊断 | 权限/配置审计 |
| `skill-scanner` | `health` / `validate` 诊断 | 技能安全审计 |
| `skill-security-auditor` | `health` / `validate` 诊断 | 与 skill-scanner 重合 |
| `skill-tester` | `validate` 诊断 | 质量门与报告 |
| `gh-review-requests` | `status` 评审队列视图 | 查询助手 |
| `sentry` | `status` / `health` 观察视图 | 厂商只读查询 |

### 最小视图 schema（B5 条目 20）

保证诊断视图与 T3 `incident-response` 的风格和字段密度一致。

| 面 | 最小 schema 形状 |
|---------|----------------------|
| `sentry` | `service`, `incident_hint`, `signal`, `severity`, `observed_at`, `evidence` |
| `skill-scanner` | `skill_id`, `risk_level`, `finding`, `severity`, `evidence`, `remediation` |
| `observability-query` | `service`, `metric_or_trace`, `window`, `anomaly`, `evidence`, `next_step` |
| `review-queue` | `queue_id`, `item_ref`, `age`, `priority`, `owner`, `action` |

## route-only

仅通过显式命令路由/覆盖可达。

| 面 | 落点 | 原因 |
|---------|--------------|--------|
| `second-opinion` | `review` 路由覆盖 (`--mode=second-opinion`) | 有价值的评审面，非可复用方法 |

## absorbed

归并入某个目录技能的流程/姿态，不单独成节点。

| 面 | 落点 | 原因 |
|---------|---------|--------|
| `agent-workflow-designer` | plan-authoring 指南 | 编写元技能 |
| `designing-workflow-skills` | 蒸馏 SOP | 工作流技能设计规则 |
| `writing-skills` | 蒸馏 SOP | 技能 TDD 流程 |
| `antigravity-workflows` | 蒸馏 SOP + 工作流路由 | 编排元技能 |
| `acceptance-orchestrator` | incident-response 门禁姿态 | 保留门禁姿态 |
| `block-no-verify-hook` | git-recovery / 策略 | 针对特定 hook 的策略 |
| `spec-kitty-charter-doctrine` | plan-authoring 宪章注记 | 已被吸收 |
| `simplification-pass` | independent-review + code-quality-review 分部 | 评审内技术 |
| `review-request-response` | independent-review + review-comment-triage | 跨生命周期 |
| `hypothesis-arbitration` | root-cause-tracing | 作为高级分支更干净 |
| `code-simplifier` | code-quality-review 分部 | 简化姿态 |

## posture-only

仅保留姿态；不单独推升。

| 源 | 落点 |
|--------|-------------|
| `superpowers/using-superpowers` | project- 与 agent-level 的 skill-first 姿态 |
| `superpowers/executing-plans` | plan-authoring 执行契约段落 |
| `spec-kitty/mission-system` | plan-authoring 分类/流程注记 |
| `spec-kitty/runtime-next` | 解析器约束 + 条件式 hydration |
| `sickn33/agent-orchestrator` | 能力解析器启发 |
| `wshobson/error-handling-patterns` | independent-review + code-quality-review 分部 |

## deferred

| 面 | 原因 |
|---------|--------|
| `skill-factory` | 未来 repo-local `skill` 命令族 |
| `prompt-governance` | 未来 prompt 治理面 |

## 命令落点汇总

| 命令 | 绑定目录技能 | diagnostics landing zones / 已落地 override |
|---------|---------------------|--------------------------|
| `review` | independent-review, multi-reviewer-calibration, security-review, threat-modeling, gha-security-review, supply-chain-audit, sast-orchestration, differential-review, variant-analysis, spec-trace；`second-opinion` 覆盖 | — |
| `validate` | spec-trace, coverage-analysis, property-testing, mutation-testing, performance-profiling | skill-tester, skill-scanner, skill-security-auditor, claude-settings-audit |
| `repair` | root-cause-tracing, ci-triage, review-comment-triage, git-recovery, supply-chain-audit, gha-security-review, variant-analysis | — |
| `status` | incident-response, supply-chain-audit, ci-triage, performance-profiling | review-queue, observability-query, gh-review-requests, sentry |
| `health` | incident-response（T3 视图） | observability-query, sentry, claude-settings-audit, skill-scanner, skill-security-auditor |

`--mode` / `--view` 标志已落地：
`review` / `validate` / `repair` 支持 `--mode`，
`status` / `health` 支持 `--view`。
自动 `--view` 路由仅在存在具体 active/selected change 上下文时生效。
当没有 active change 且进入 diagnostics 回退时，`status` / `health` 会保持
`view` 为空，除非操作者显式传入 `--view`。
当前非 catalog 的显式 `--view` 覆盖值是
`review-queue` 与 `observability-query`。
对具体 active/selected change，当前 auto-route 会选择已落地的 catalog T3
视图 `incident-response`。
`validate` 目前没有独立的 `--view` flag；表中挂在 `validate` 下的条目表示
诊断/报告落点，而不是可逐个选择的 CLI surface。
表内其余 `view-only` 条目是诊断落点说明，
不保证都能作为独立的 `--view <id>` 选择器直接传参。
`status` / `health` 当前共用一套 payload renderer：选中的 `--view`
会被校验并保留在输出里，但只有当某个 surface 后续补上专门渲染逻辑时，
才保证拥有独立的 per-view payload。
