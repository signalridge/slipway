# 按源索引（反向索引）

每行对应 `skills_ref/` 下一个权威源技能。语料含 80 个权威 `SKILL.md`；
`alirezarezvani/skill-tester/assets/sample-skill/SKILL.md` 是 `skill-tester`
技能内嵌的测试 fixture，**不**计入本文档。本文档由人工维护，引用
`provenance.yaml`，不替代各技能自身的 provenance 文件。

处置词汇：

- `standalone` — 实质贡献于指定目录技能
- `posture-only` — 仅保留姿态/措辞，非独立推升
- `partial-only` — 仅消费子章节或模板分部
- `view-only` — 保留为诊断/视图面，不进入 governed 运行时方法层
- `route-only` — 通过显式命令路由或覆盖可达
- `absorbed` — 并入其他目标的流程/姿态，不作独立节点
- `deferred` — 超出当前推进范围，仅作完整性列出

| 源 | 处置 | 目录技能 / 落点 | 状态 |
|--------|-------------|----------------------------|--------|
| alirezarezvani/adversarial-reviewer | standalone | multi-reviewer-calibration | B4 |
| alirezarezvani/agent-workflow-designer | absorbed | plan-authoring 编写指南 | B6 |
| alirezarezvani/code-reviewer | standalone | independent-review | B1 |
| alirezarezvani/dependency-auditor | standalone | supply-chain-audit | B3 |
| alirezarezvani/incident-commander | standalone | incident-response | B5 |
| alirezarezvani/incident-response | standalone | incident-response | B5 |
| alirezarezvani/performance-profiler | standalone | performance-profiling | B4 |
| alirezarezvani/pr-review-expert | standalone | differential-review | B4 |
| alirezarezvani/prompt-governance | deferred | 未来 prompt 体系治理面 | n/a |
| alirezarezvani/skill-security-auditor | view-only | health / validate 诊断 | B6 |
| alirezarezvani/skill-tester | view-only | validate 诊断 | B6 |
| getsentry/claude-settings-audit | view-only | health / validate 诊断 | B6 |
| getsentry/code-review | standalone | independent-review | B1 |
| getsentry/code-simplifier | absorbed | code-quality-review 简化分部 | B4 |
| getsentry/find-bugs | standalone | differential-review | B4 |
| getsentry/gh-review-requests | view-only | status 评审队列视图 | B6 |
| getsentry/gha-security-review | standalone | gha-security-review | B3 |
| getsentry/iterate-pr | standalone | ci-triage + review-comment-triage | B5 |
| getsentry/security-review | standalone | security-review | B2 |
| getsentry/skill-scanner | view-only | health / validate 诊断 | B6 |
| openai/gh-address-comments | standalone | review-comment-triage | B5 |
| openai/gh-fix-ci | standalone | ci-triage | B5 |
| openai/security-best-practices | standalone | security-review | B2 |
| openai/security-ownership-map | standalone | threat-modeling | B3 |
| openai/security-threat-model | standalone | threat-modeling | B3 |
| openai/sentry | view-only | status / health 观察视图 | B6 |
| sickn33/acceptance-orchestrator | absorbed | incident-response 门禁姿态 | B5 |
| sickn33/agent-orchestrator | posture-only | 能力解析器匹配启发 | B6 |
| sickn33/antigravity-workflows | absorbed | 蒸馏 SOP / 工作流路由 | B6 |
| sickn33/audit-context-building | standalone | context-assembly | B2 |
| sickn33/code-review-ai-ai-review | standalone | multi-reviewer-calibration | B4 |
| spec-kitty/spec-kitty-charter-doctrine | absorbed | plan-authoring 宪章注记 | B6 |
| spec-kitty/spec-kitty-git-workflow | standalone | git-recovery | B5 |
| spec-kitty/spec-kitty-implement-review | standalone | parallel-executor-contract | B2 |
| spec-kitty/spec-kitty-mission-review | standalone | spec-trace | B2 |
| spec-kitty/spec-kitty-mission-system | posture-only | plan-authoring 分类注记 | B6 |
| spec-kitty/spec-kitty-runtime-next | posture-only | 解析器约束与条件式 hydration | B6 |
| spec-kitty/spec-kitty-runtime-review | standalone | independent-review | B1 |
| superpowers/brainstorming | standalone | scope-clarification | B1 |
| superpowers/dispatching-parallel-agents | standalone | parallel-executor-contract | B2 |
| superpowers/executing-plans | posture-only | plan-authoring 执行契约 | B6 |
| superpowers/receiving-code-review | standalone | independent-review | B1 |
| superpowers/requesting-code-review | standalone | independent-review | B1 |
| superpowers/subagent-driven-development | standalone | parallel-executor-contract | B2 |
| superpowers/systematic-debugging | standalone | root-cause-tracing | B2 |
| superpowers/test-driven-development | standalone | tdd-proof | B1 |
| superpowers/using-superpowers | posture-only | skill-first 姿态文本 | B6 |
| superpowers/verification-before-completion | standalone | fresh-verification-evidence | B1 |
| superpowers/writing-plans | standalone | plan-authoring | B1 |
| superpowers/writing-skills | absorbed | 蒸馏 SOP / 导出指南 | B6 |
| trailofbits/agentic-actions-auditor | standalone | gha-security-review | B3 |
| trailofbits/ask-questions-if-underspecified | standalone | scope-clarification | B1 |
| trailofbits/audit-augmentation | standalone | sast-orchestration | B3 |
| trailofbits/audit-context-building | standalone | context-assembly | B2 |
| trailofbits/codeql | standalone | sast-orchestration (tool-recipe) | B3 |
| trailofbits/coverage-analysis | standalone | coverage-analysis | B4 |
| trailofbits/debug-buttercup | partial-only | root-cause-tracing 筛查姿态 | B2 |
| trailofbits/designing-workflow-skills | absorbed | 蒸馏 SOP | B6 |
| trailofbits/differential-review | standalone | differential-review | B4 |
| trailofbits/insecure-defaults | standalone | security-review | B2 |
| trailofbits/mutation-testing | standalone | mutation-testing | B4 |
| trailofbits/property-based-testing | standalone | property-testing | B4 |
| trailofbits/sarif-parsing | standalone | sast-orchestration | B3 |
| trailofbits/second-opinion | route-only | 显式 `review` 路由/覆盖 | B6 |
| trailofbits/semgrep | standalone | sast-orchestration (tool-recipe) | B3 |
| trailofbits/sharp-edges | standalone | security-review | B2 |
| trailofbits/spec-to-code-compliance | standalone | spec-trace | B2 |
| trailofbits/supply-chain-risk-auditor | standalone | supply-chain-audit | B3 |
| trailofbits/variant-analysis | standalone | variant-analysis | B4 |
| wshobson/block-no-verify-hook | absorbed | git-recovery / 策略指南 | B5 |
| wshobson/code-review-excellence | standalone | independent-review | B1 |
| wshobson/context-driven-development | standalone | context-assembly | B2 |
| wshobson/debugging-strategies | standalone | root-cause-tracing | B2 |
| wshobson/distributed-tracing | partial-only | performance-profiling 清单 | B4 |
| wshobson/e2e-testing-patterns | standalone | coverage-analysis | B4 |
| wshobson/error-handling-patterns | posture-only | independent-review + code-quality-review 分部 | B6 |
| wshobson/git-advanced-workflows | standalone | git-recovery | B5 |
| wshobson/multi-reviewer-patterns | standalone | multi-reviewer-calibration | B4 |
| wshobson/parallel-debugging | standalone | root-cause-tracing（竞争假设分支） | B2 |
| wshobson/workflow-patterns | standalone | plan-authoring + tdd-proof | B1 |

## 覆盖概览

| 处置 | 数量 |
|-------------|-------|
| standalone | 56 |
| posture-only | 6 |
| partial-only | 2 |
| absorbed | 8 |
| view-only | 6 |
| route-only | 1 |
| deferred | 1 |
| **合计** | **80** |

`状态` 列记录该源落点落地的 rollout 批次（`B1`-`B6`）；延期项维持 `n/a`。
