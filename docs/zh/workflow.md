# 受治理的工作流

Slipway 把工作引导到一条受治理的生命周期中：

1. `S0_INTAKE`：记录意图、范围、待解决的问题，以及初始证据。
2. `S1_PLAN`：产出研究、需求、决策、任务和 plan-audit 等产物。plan-audit 是允许 S2 启动的评审，它评审的是计划包本身，而不是某个冻结的 wave 缓存。
3. `S2_IMPLEMENT`：执行计算出的 wave。Slipway 根据当前 `tasks.md` 中每个任务声明的依赖和目标文件，实时计算 wave 调度；作者无需声明 wave 编号。无依赖、文件互不重叠的任务会归入同一个 wave，默认并发派发——`slipway next --json` 会把这类 wave 标记为 `parallel: true`。在 `.slipway.yaml` 中设置 `execution.parallelization: off`，可改为顺序执行各个 wave。
4. `S3_REVIEW`：对照产物核验实现，运行选定的评审检查，通过独立的子代理修复反馈，然后运行唯一的终态 `ship-verification` 关卡（一次权威的完整测试套件、验收证明、时效性复核、`assurance.md` 证明，以及评审独立性证明），得出可交付（done-ready）的结果。

当前的生命周期状态存放在 `artifacts/changes/<slug>/change.yaml` 中。
包内的生命周期事件保存在
`artifacts/changes/<slug>/events/` 下，技能验证记录保存在
`artifacts/changes/<slug>/verification/` 下。wave 执行期间记录的运行时任务证据
位于
`.git/slipway/runtime/changes/<slug>/evidence/...`。

<div align="center" markdown>

![Slipway 受治理的生命周期：new、S0 Intake、S1 Plan、S2 Implement、S3 Review、done-ready、done，配有显式的生命周期命令，以及作为快捷方式的 run](../assets/diagrams/lifecycle.svg)

</div>

## 创建一项变更

```bash
slipway new "refresh governance docs" --preset standard
```

通过 JSON 标准输入，AI 调用方可以直接提供分类信息：

```bash
echo '{"guardrail_domain":"","needs_discovery":true,"complexity":"complex","test_cmd":"go test ./...","build_cmd":"go build ./...","languages":["Go","Markdown"]}' \
  | slipway new --json "refresh governance docs"
```

省略分类时，Slipway 采用保守的默认值：

- `guardrail_domain=""`
- `needs_discovery=true`
- `complexity="complex"`

## 推进方式

需要显式控制交接时，使用 `next`：

```bash
slipway next --json
# complete the surfaced skill or resolve blockers
slipway run --json
slipway next --json
```

想让 Slipway 一直推进到需要操作者介入的停靠点时，使用 `run`：

```bash
slipway run --json --diagnostics
```

`run` 会在遇到待执行的技能、阻塞项或 done-ready 结果时停下。

## 独立性证明令牌

review、ship-verification 和 wave-orchestration 这几个阶段，会在验证记录的
`references` 上记录少量供引擎使用的令牌（通过
`slipway evidence skill --reference ...`）。在 `standard`/`strict` 下，每个令牌都是
error 级别的阻塞项，在 `light` 下仅为提示性（以 Pattern-A 省略的方式实现——
关卡在 `light` 下直接不返回阻塞项，这个边界里没有单独的提示性通道）。这些令牌
都不是对时效性或最终裁定的自我盖章；引擎始终是唯一的时间戳与运行版本盖章者。

| 令牌 | 证明内容 | 强制级别 | 关卡失败即停时的恢复方式 |
| --- | --- | --- | --- |
| 贯穿链路参与方的 `context_origin:stage=<stage>=<handle>`，其中选定的 S3 评审者一律使用 `stage=review`，评审发现的修复在存在时使用 `stage=fix` | 每个归属的参与方都在共享 worktree 上以各自独立的上下文运行；选定评审者以技能名为键，且必须两两互不相同；记录下的修复 handle 不得与实现或评审的 handle 重合 | standard/strict error，light advisory | 在一个全新的原生子代理中重跑归属的评审者或修复，使其重新发出一个独立的 `context_origin` handle |
| ship-verification 上的 `closeout:reviewer_independence=pass` | 终态 ship 记录上存在评审独立性证明（Pattern-A）；缺失则以 `ship_verification_reviewer_independence_missing` 失败即停 | standard/strict error，light advisory | 重跑 **ship-verification** 并记录该令牌 |
| ship-verification 上的 `closeout:assurance_complete=pass` | 宿主证明终态 ship 记录上的 `assurance.md` 已完成；缺失则以 `ship_verification_assurance_attestation_missing` 失败即停 | standard/strict error，light advisory | 重跑 **ship-verification** 并记录该令牌 |
| 终态排序 `ship-verification >= 每个选定的 S3 peer`（始终开启，无令牌） | 终态 ship 记录是在每个选定 S3 评审 peer 之后盖章的，而非早于其中任何一个，因此关卡观察到的是最终的评审证据 | 所有 preset（始终开启；light 不豁免） | 给那个过期的选定评审者重新盖章，再重跑 **ship-verification**，使其裁定时间戳不早于每个 peer |
| wave-orchestration 上的 `degraded_dispatch_justification:wave=<n>:tool_unavailable=<detail>` | `degraded_sequential` 派发确实搭配了真实的工具不可用理由 | standard/strict error，light advisory | 带上理由引用重新记录 wave-orchestration 证据，或用真正的并发派发重跑该 wave |

未搭配任何理由的裸 `degraded_sequential`，在所有同步受治理 wave 执行的路径上
都会被拒绝，包括 `slipway evidence skill` 路径——不只是 advance/next。

`context_origin:stage=<stage>=<handle>` 是一套贯穿整条受治理链路的统一语法。
S3 选定评审集对所有 workflow profile 都包含 spec 与 independent 评审者；当 profile
要求时 code-quality 评审会加入，当引擎推导出的安全控制选中它时 security 评审会加入。
终态 `ship-verification` 关卡不是选定 peer——它在 peer 收敛之后最后运行。所有选定
评审宿主都发出 `context_origin:stage=review=<handle>`；R2 lattice 以技能名而非共享的
`review` 阶段作为每个评审参与方的键。其余参与方是 S2 wave 的 `executor`、S1 plan-audit
的 `audit_origin`（与计划的 `plan_origin` 作者配对核对），以及记录在评审者证据上的
可选 S3 评审发现 `fix` 句柄。冲突 lattice 按边界归属，因此每条边只检查一次：

| 边界 | 拥有 | 边数 |
| --- | --- | --- |
| 计划关卡（S1） | 仅本地的 `audit_origin != plan_origin` 这条边（plan-audit 作者 vs 自审审计者） | 1 |
| 评审权威 | `{executor, fix}` 之间的每条边，加上选定评审技能的键；S1 `audit_origin` 不是活跃的 S3 参与方 | 随 workflow profile、选定的安全控制以及可选 fix handle 而变 |
| 交付权威 | 不增加任何 context-origin 边；终态 `ship-verification` 关卡拥有终态排序不变量，以及评审独立性与 assurance 完成两项存在性证明 | 0 |

当某个边界失败即停时，在全新的原生子代理中重跑它所属的阶段或选定评审者，
使该阶段重新发出一个独立的 `context_origin` handle；引擎始终是唯一的裁定盖章者，
绝不会给已重合的 handle 重新盖章。

`context_origin` lattice 属于**审计/结构层**：这些 handle 是宿主发出的字符串——
与执行派发 handle 同属一个结构层——因此它提高了把多个链路阶段塌缩进同一个
撰写上下文的成本和可审计性，但绝非独立性的密码学证明。真正不可伪造的独立上下文
判别（由引擎签发的每阶段 nonce 或生命周期事件边界，即“Option B”）在本次变更的
约束下不可行，所以这里没有任何关卡被夸大为密码学级别的独立上下文证明。

## S3 评审派发

在 `S3_REVIEW`，引擎解析出一个选定评审集，并通过命令界面对外暴露这个集合。
spec 与 independent 评审对所有 workflow profile 都会被选中；code-quality 评审仅在
profile 要求时加入，security 评审者仅在引擎推导出的安全控制被选中时加入。
`slipway next` 暴露这个选定集，宿主适配器会把这些评审者作为并发的原生子代理扇出。
任何沿用约定的单一主技能，只是给那些确实需要主技能的界面留的兼容投影；它并不
意味着评审有先后顺序。终态 `ship-verification` 关卡在这组 peer 收敛之后派发，
绝不作为其中的一员。

选定评审者是**无序的 peer**：彼此都不阻塞对方，且必需性、评审权威、交付权威
和过期证据恢复都使用同一个选定集。每个选定评审者都记录各自独立句柄的
`context_origin:stage=review=<handle>`。R2 lattice 在技能名参与方键下比较这些 handle，
因此即便线协议令牌的阶段标签是共享的，重复的评审者句柄仍会失败即停。
当某条评审发现通过 `slipway fix` 修复时，受影响的评审者还会在重审时记录
`context_origin:stage=fix=<repair-handle>`；任何记录下的 fix handle 都参与同一套
独立上下文 lattice。
缺失选定评审者证据由必需技能阻塞项负责；一条通过的选定评审记录若没有格式良好的
`stage=review` 句柄，会以 `context_origin_handle_invalid` 失败即停；冲突则以
`cross_stage_context_not_distinct` 失败。磁盘上未被选中的安全证据保持静默，
绝不会成为隐藏的参与方。

选定评审者的时效性以当前差异、规划产物和 `run_summary_version` 为锚；不存在供
peer 集使用的共享 suite-result 基准点。那一次权威的完整测试套件——以及任何
guardrail SAST 基线——由终态 `ship-verification` 关卡在 peer 收敛之后运行一次，
并记录在它自己的证据记录上，而非与评审者共享的记录上。

## 只读界面

下列命令只检视状态，不改动生命周期权威：

- `slipway next`
- `slipway status`
- `slipway validate`

`validate` 直接输出机器可读的 JSON 报告。对于 `next`、`status` 这类默认输出
文本的只读命令，使用 `--json` 获取机器可读输出。当你需要关卡细节、产物就绪
状态、转换轨迹时，在 `next` 或 `run` 上加 `--diagnostics`。

## Open Questions 语义

`intent.md` 中可以有一个规范的 `## Open Questions` 小节。引擎只对**结构而非散文**
设关卡：只有未勾选的清单项才会阻塞 intake。

下面这些都视为已解决（intake 推进到 `S0_INTAKE/confirm`）：

```markdown
## Open Questions
(none)
```

```markdown
## Open Questions
- None requiring research — the page model is already specified.
```

```markdown
## Open Questions
- [x] Installer path resolved by research.
```

只有未勾选的 `- [ ]` 条目才会阻塞（路由到 `S0_INTAKE/research`）：

```markdown
## Open Questions
- [ ] Which installer path should be documented?
```

自由散文和裸列表项都是**文档，绝不是阻塞项**。判断某项是否为真正的待解决问题，
是一项语义判断，归 `intake-clarification` 技能所有，由它把真正的未知项记录为
`- [ ]` 条目；引擎从不解析 intent 散文。这样既能让一项没有未知项的变更
（`None`、一句兜底说明，或一个空小节）不会悄悄绕进研究阶段，又能让产物用 `- [x]`
保留历史问题记录。当某个条目确实造成阻塞时，`slipway run` 会点名具体的那一行
`- [ ]`，使路由不至于无声无息。

## 完成

当受治理状态为 done-ready 时：

```bash
slipway done
```

`done` 会终结当前活动变更并归档终态。如果中断后本地状态看起来不一致，先用
`slipway health --doctor` 检视，再在建议的修复与问题相符时运行 `slipway repair`。
