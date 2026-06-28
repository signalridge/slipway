# 工作流

Slipway 把受治理的工作依次串过 intake、规划、实现、评审和收尾几个阶段。当前的
生命周期状态保存在 `artifacts/changes/<slug>/change.yaml` 中。

完整的工作流文档见 [受治理的工作流](../workflow.md)。本页讲的是其中的心智模型。

## 生命周期

| 阶段 | 用途 |
| --- | --- |
| `S0_INTAKE` | 记录意图、范围、待解决的问题以及初始证据。 |
| `S1_PLAN` | 产出研究结论、需求、决策、任务清单以及计划审计证据。 |
| `S2_IMPLEMENT` | 按依赖顺序执行任务波次，并记录任务证据。 |
| `S3_REVIEW` | 运行选定的评审者、修复发现的问题、撰写保证材料，最终达到 done-ready。 |
| `done` | 在 done-ready 收尾后归档终态。 |

`slipway run` 是一个快捷驱动器。它会一直推进，直到遇到需要操作者介入的停顿点：
技能交接、阻塞，或 done-ready 状态。

## 先只读，再变更

用只读命令了解当前状态：

```bash
slipway status --json
slipway validate
slipway next --json --diagnostics
```

然后再运行对应阶段的命令，或者完成被点出的技能。这样 agent 就不会凭旧上下文乱猜。

## 失败即停的恢复

失败即停的阻塞，意味着当前 worktree 中某项证明缺失、过期、损坏，或超出范围。
好的恢复做法是下列之一：

- 按 `slipway instructions <artifact>` 撰写缺失的产物。
- 在全新上下文中重新运行对应阶段或选定的评审者。
- 通过波次执行路径记录任务证据。
- 仅当 `health --doctor` 明确点名时，才执行有限的本地修复。
- 如果目标本身变了，就开启一个新的受治理变更。

不好的恢复做法是手工编辑状态文件、改时间戳、在没有新证明的情况下移除阻塞，
或是教 agent 绕过评审。

## done-ready 不等于 done

done-ready 表示各项门禁已通过、变更可以定稿。`slipway done` 会归档终态。
如果 `done` 报出 worktree 有未提交改动的警告，请把预期的实现差异连同归档的
Slipway 记录一起提交，之后再移除 worktree。

## 相关阅读

- [第一个受治理的变更](../tutorials/first-governed-change.md)
- [恢复与排障](../how-to/recover-and-troubleshoot.md)
- [设计](design.md)
