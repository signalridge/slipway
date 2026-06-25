# 如何恢复与排查

当 Slipway 报告阻塞项、证据过期、产物缺失、适配器漂移，或本地状态令人困惑时，参考本指南。

原则很简单：先检查，再按指定的恢复路径处理。不要手工编辑生命周期权威状态、证据裁决、时间戳或运行时任务凭证。

## 只检查，不修改

下面这些命令是恢复工作主要依赖的诊断 JSON 接口。

在受治理的 worktree 中运行：

```bash
git status --short --branch
slipway status --json
slipway validate --json
slipway next --json --diagnostics
```

用 `status` 查看生命周期快照，用 `validate` 查看 gate 是否就绪，用 `next --diagnostics` 获取可操作的阻塞项或技能交接。

## 运行 Doctor 输出

当本地状态看起来不一致时：

```bash
slipway health --doctor --json
```

阅读 `applied_repairs`、`unrepaired_drift` 以及具名的 `next_action` 字段。只有当 doctor 输出与你看到的问题相符时，才执行 repair：

```bash
slipway repair --json
```

`repair` 针对的是有限范围的本地完整性问题，不能用来强行让一个变更通过生命周期 gate。

## 任务证据缺失或过期

症状通常出现在 `validate --json` 或 `next --json --diagnostics` 中，表现为运行时任务证据缺失、执行摘要过期，或新鲜度输入不匹配。

安全的恢复方式：

1. 在 JSON 输出中找出对应的任务和所需的证据路径。
2. 重新运行负责该任务的 implementation 或 wave-orchestration 交接。
3. 通过负责该任务的 Slipway 命令或生成的技能来记录任务证据。
4. 重新运行 `slipway validate --json`。

不要手工在 `.git/slipway/runtime/changes/<slug>/evidence/` 下写文件。

## 产物内容缺失

如果某个受治理产物缺失、只有占位内容，或结构不合法，使用恢复输出指定的撰写接口：

```bash
slipway instructions requirements --json
slipway instructions decision --json
slipway instructions research --json
slipway instructions tasks --json
slipway instructions assurance --json
```

命令会给出模板和质量标准。真正的产物内容必须由撰写技能或人来完成，依据当前目标和源头事实撰写。照抄模板会被拒绝。

## 评审发现的问题

如果评审发现了需要处理的问题，不要在同一上下文里把评审和修复混在一起。使用修复接口：

```bash
slipway fix --json
```

把返回的修复契约交给一个全新上下文的修复 agent。修复完成后，重新运行受影响的选定评审者，然后再运行：

```bash
slipway review --json
slipway validate --json
```

选定评审者的证据必须对当前 diff、规划产物和执行摘要输入都是新鲜的。唯一权威的完整测试套件由终态 `ship-verification` gate 在各评审者收敛之后运行，而不是来自某个评审者共享的关键凭证。

## 范围漂移

如果 `scope_contract` 报告有文件改动落在某个任务的 `target_files` 之外，从以下安全路径中选一条：

- 如果是误改，自己撤销或移动这些越界改动。
- 通过指定的 Slipway 规划或评审路径，修订同一意图的任务或产物。
- 如果目标本身变了，开启一个新的受治理变更。

不要靠编辑证据来掩盖被改动的文件。

## Done 之后 worktree 不干净

`slipway done --json` 可能在归档一个已就绪变更的同时，返回一个 `worktree_dirty_warning`，提示还有非活跃文件需要提交。

安全的恢复方式：

```bash
git status --short
git diff --check
```

把预期的实现 diff 和已归档的 Slipway 记录一起提交。活跃 bundle 会被重写到 `artifacts/changes/archived/<slug>/`。

## 适配器漂移

如果生成的 AI 工具命令或技能看起来已经过期：

```bash
slipway init --refresh
```

然后检查 diff：

```bash
git status --short .claude .codex .cursor .opencode
```

生成的适配器只是交接辅助。如果适配器行为和 CLI 行为不一致，以当前 worktree 的 CLI 输出为准，并刷新生成的文件。

## 恢复速查表

| 症状 | 检查 | 安全操作 |
| --- | --- | --- |
| 不确定下一步该做什么 | `slipway next --json --diagnostics` | 按返回的技能、阻塞项或命令处理。 |
| Gate 提示证据过期 | `slipway validate --json` | 重新运行对应的 stage、评审者或任务证据路径。 |
| 本地状态看起来损坏 | `slipway health --doctor --json` | 仅针对具名的有限修复运行 `slipway repair --json`。 |
| 产物只有占位内容 | `slipway instructions <artifact> --json` | 撰写真实内容并重新运行校验。 |
| 评审发现问题 | `slipway fix --json` | 在全新上下文中修复，重新运行受影响的评审者。 |
| 适配器文件已过期 | `slipway init --refresh` | 检查生成的 diff，保留用户自有的文件。 |

## 相关内容

- [命令](../reference/commands.md)
- [工作流](../explanation/workflow.md)
- [操作员指南](../operator-guide.md)
