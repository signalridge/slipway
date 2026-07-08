# 命令参考

大多数路由命令在需要结构化输出时都支持 `--json`。例外是仅用于初始化的
`slipway init`（`--tools`/`--refresh`，没有 `--json`），以及不需要单独
`--json` 标志就输出 JSON 的 `slipway validate` / `slipway done`。
`validate --format` 只决定 `--list-focuses` 的输出格式，不影响主报告。

生成的宿主命令面向那些选择启用宿主提示词的已注册命令。仅供 CLI 使用的辅助命名空间
（例如 `slipway tool` 和 `slipway hook`）会在这里和命令面清单中注册，但不会生成
`$slipway-tool`、`$slipway-hook` 或宿主提示词包装；生成的 skill 和宿主 hook 配置会直接
调用它们的辅助子命令。

## 核心生命周期

| 命令 | 类别 | 用途 |
| --- | --- | --- |
| `slipway new [description]` | mutation | 创建一个从 intake 开始的受治理变更。 |
| `slipway intake` | mutation | 执行 S0 阶段的 intake 澄清与授权。 |
| `slipway plan` | mutation | 执行 S1 阶段的计划产物编写，或对同一意图的变更进行修订。 |
| `slipway implement` | mutation | 执行 S2 阶段的实现 wave 编排。 |
| `slipway review` | mutation | 执行 S3 阶段的评审收敛与评审反馈修复。 |
| `slipway fix` | mutation | 为 S3 评审发现派发按 `contract.subagent` 配置的修复任务。 |
| `slipway done` | mutation | 将一个 done-ready 的变更收尾并归档。 |
| `slipway next` | query | 查看下一个可执行的 skill 或阻塞项，但不推进状态。 |
| `slipway run` | mutation | 以快捷方式驱动当前生命周期阶段，直到出现 skill、阻塞项或 done-ready 结果。 |
| `slipway status` | query | 显示生命周期状态、阻塞项、进度和下一步操作。 |

当一个受治理变更的证据已经过期时，`slipway next` 保持只读，并报告恢复指引。在已知状态的
情况下，优先使用明确的当前阶段命令：`intake`、`plan`、`implement`、`review` 或 `done`。
`run` 是一个自动驱动快捷方式，它会委派给当前阶段，并在 JSON 中报告 `delegated_to`。同一意图
范围内的改动属于当前变更内部的变更修订；意图冲突则会开启一个新的受治理变更。计划时效性以结构化的
任务计划哈希为准。Plan-audit 在 S2 开始前评审计划包；它不会把 `wave-plan.yaml` 认定为计划权威。
`wave-plan.yaml` 是从当前 `tasks.md` 生成的 S2 执行投影/缓存，它的 `generated_at`
是生成时间，用于展示/审计，而非时效性权威。`slipway next --json` 上的
`input_context.wave_plan` 字段是另一个仅用于诊断的投影：它携带的只读字段（`wave_count`、
`advisories`）并不是持久化的 `wave-plan.yaml` 缓存所定义的，因此绝不能复制进缓存。
`wave-plan.yaml` 是由引擎拥有、由工具（`slipway repair`）重新生成的缓存，绝不手工编辑；如果它
读不出来，请用 `slipway repair` 重新生成，而不是去改 `tasks.md`。

`slipway fix` 是 S3 评审发现的修复面。它会发现评审反馈和对齐阻塞项，然后返回一个
`repair_batch_id` 以及一份契约；如果配置了 `fix` slot，契约会包含 `contract.subagent`。宿主应优先使用
这个 directive，否则回退到 native fresh-context 修复子代理。普通的发现过程不会推进生命周期状态；
`slipway fix --start-reexecution` 是显式的评审驱动模式，它会重新打开 S2，并为实现修复生成
一个全新的执行运行边界。对于 S3 阶段的任务计划修订，应使用 `slipway run`：它会在相同的
`run_summary_version` 上就地重新物化 wave projection，并保留既有任务证据。只有在明确要丢弃既有任务证据时，才附加 `--discard-prior-evidence`。宿主会先收集所选评审批次的发现，按根因将它们合并成一份修复简报，并且
在其他所选评审仍在报告期间，绝不能内联或逐条修复发现。等子代理改完代码、产物、测试或同一意图范围
证据之后，重新运行受影响的所选评审，并在 `slipway review` 关闭该批次之前同时记录
`context_origin:stage=review=<handle>` 和 `context_origin:stage=fix=<handle>`。
`slipway repair` 仍然只负责本地完整性。

配置化的 subagent 委派目标位于 `.slipway.yaml` 的 `subagents.*` 下。可用 slot 是
`default`、`plan_audit`、`executor`、`review`、`fix` 和 `verify`；每个 slot 可选择
`type: native|mcp|skills`、`name`、`session_instructions` 和 `timeout`。Schema 和
JSON 输出面见 [Subagent 配置](reference/subagents.md)。

## 创建选项

```bash
slipway new "add install docs" --preset standard
slipway new "docs-only change" --profile docs
slipway new --from-doc docs/installation.md "refresh install docs"
slipway new "small fix" --trivial
slipway new "auth refactor" --discuss   # carry open questions forward into context
slipway new "schema migration" --full   # force fresh ship-verification evidence before ship
```

预设（preset）控制门禁的严格程度：`light`、`standard` 或 `strict`。

工作流配置（profile）决定要做哪些检查：`code`、`docs`、`research`、`config` 或 `meta`。

`--discuss` 会在执行前把尚未解决的灰色地带持久化进上下文；`--full` 要求在 ship 门禁之前有一份
刷新过的 `ship-verification` 通过记录。

## 探查

| 命令 | 类别 | 用途 |
| --- | --- | --- |
| `slipway codebase-map` | mutation | 在 `artifacts/codebase/` 下创建或刷新仓库范围的咨询性上下文。 |

当文档只包含 CLI 检测到的仓库事实时，`codebase-map --json` 会报告 `status: "baseline"`。
baseline 文档是有用的起步上下文，但不是经过编写的存量（brownfield）分析；调用方在依赖它们做计划
或评审之前，应先用有源头支撑的发现来完善它们。

`artifacts/codebase/` 下的 codebase map 默认纳入 git 跟踪——持久的存量上下文本就该被评审和共享，
而不是藏成本地状态。已有仓库会在下一次 `slipway new`、`slipway codebase-map` 或 `slipway init`
重写受管 `.gitignore` 块时自动迁移（`next`/`run`/`status`/`repair` 不会调和它）；包内的
`events/`、`verification/`、遗留的逐变更 `evidence/` 以及 `.worktrees/` 路径仍然被忽略。运行时的
任务证据存放在 `.git/slipway/runtime/changes/<slug>/evidence/` 下，并通过 `slipway evidence task` 写入。

## 情境命令

| 命令 | 类别 | 用途 |
| --- | --- | --- |
| `slipway preset <level>` | mutation | 确认或更改当前变更的预设。 |
| `slipway validate` | query | 重新计算证据和门禁就绪状态，但不推进。 |
| `slipway abort` | mutation | 中止当前执行会话，但不归档变更。 |
| `slipway cancel` | mutation | 取消一个活动变更并归档其终态。 |
| `slipway delete` | mutation | 丢弃一个被放弃的受治理变更：它的包、运行时绑定、可选的 worktree，或一条已归档记录（默认 dry-run）。 |
| `slipway repair` | mutation | 执行有界的本地完整性修复。 |
| `slipway evidence task` | mutation | 为 wave 执行记录受支持的运行时任务证据。 |

`slipway cancel` 和 `slipway delete` 不是同一个操作。`cancel` 把一个**活动**变更带入终态
`cancelled`，并**归档**到 `artifacts/changes/archived/<slug>` 下，让这个决定留在审计轨迹里。
`delete` 则是为一个被放弃、误建或部分删除的变更**丢弃本地受治理状态**——它的包、它的运行时绑定，
以及（加上 `--worktree` 时）绑定的 git worktree——而加上 `--archived` 还能清除一条已归档记录。
`delete` 默认是 dry-run：单纯的 `slipway delete --change <slug>` 只打印删除计划而不删除任何东西；
加上 `--yes` 才会执行。它失败即停：除非加 `--force`，否则它拒绝删除一个含有未提交跟踪改动、
或在生成的 Slipway 路径之外含有未跟踪文件的 worktree，并且永远不删除实现分支或已推送的 PR 分支。
当一个变更被放弃、损坏或绑定到了另一个 worktree 时，`slipway status`/`slipway next` 和恢复输出会
路由到那条确切的 `slipway delete --change <slug>` 命令。

## 辅助命令

| 命令 | 类别 | 用途 |
| --- | --- | --- |
| `slipway tool <helper>` | mutation | 运行生成的 skill 所使用的辅助工具；缺少显式后端或领域工具时，辅助工具失败即停。 |
| `slipway hook <event>` | mutation | 运行生成的宿主 hook 辅助命令，例如 `session-start`；hook 会静默失败，避免阻塞宿主自动化。 |

`slipway tool` 和 `slipway hook` 有意只供 CLI 使用。它们没有 `$slipway-tool`、
`$slipway-hook`，也没有生成的宿主提示词包装；生成的 skill 和宿主配置会直接调用具体的
辅助子命令。

## 诊断

| 命令 | 类别 | 用途 |
| --- | --- | --- |
| `slipway health` | query | 显示仓库本地的完整性和可修复性发现。 |
| `slipway instructions <artifact>` | query | 显示模板、质量标准，以及（在某个变更内部时）某个受治理产物或 codebase-map 文档解析后的输出路径 + 依赖图。 |

`slipway instructions <artifact>` 提供产物模板及其质量标准，让编写 skill 直接写出真正的文件——
引擎拥有结构，skill 拥有内容，没有需要替换的预置正文。在受治理变更内部，它还会返回解析后的输出
路径、依赖/解锁图，以及带标签的背景信息（`context`/`rules`），skill 必须遵守但绝不能把它们抄进产物。
它覆盖六个受治理包产物（`intent`、`requirements`、`decision`、`research`、`tasks`、`assurance`）
以及仓库范围的 codebase-map 文档（`stack`、`architecture`、`structure`、`conventions`、
`integrations`、`testing`、`concerns`）。
在 `--json` 中，`context_is_baseline: true` 标记应被保留并扩展进所编写文档的 codebase-map
baseline 上下文；当该字段缺失或为 false 时，`context` 是要遵守但不要照抄的背景。

`next --json` 和 `run --json` 会在默认的、非 `--diagnostics` 的交接中包含
`input_context.codebase_map_status`（以及逐文档的 `input_context.codebase_map_doc_states`），
让调用方能判断引用的 map 是否持久。其取值与 `slipway codebase-map` 的评估一致（`missing`、
`scaffold_only`、`baseline`、`partial`、`populated`）；缺失的 map 会报告 `"missing"` 且每个文档
为 `missing`，而不是省略该字段。当一个使用 map 的计划 skill（research-orchestration 或
plan-audit）即将执行而状态为 `scaffold_only` 或 `baseline` 时，`warnings` 会带上一条非阻塞的
codebase-map 提示。

## 初始化

| 命令 | 类别 | 用途 |
| --- | --- | --- |
| `slipway init` | mutation | 初始化 `.slipway.yaml`、仓库本地的运行时布局，以及可选的 AI 工具适配器。 |
| `slipway config [list\|get\|set]` | mutation | 查看并更新仓库级 `.slipway.yaml` 配置键；`config list --env` 会列出运行时/密钥环境变量及其归属。它是 CLI-only，不生成适配器 prompt surface。 |

`docs/SURFACE-MANIFEST.json` 是已提交的生成面清单，记录适配器、命令、skill、JSON 和文档各类行。
该清单由 Slipway 拥有的 Go 权威源重建，并由面向 CI 的 Go 测试核对：

```bash
go run ./internal/toolgen/cmd/gen-surface-manifest --check
go run ./internal/toolgen/cmd/gen-surface-manifest --write
```

新增命令、skill、JSON 输出契约或面向文档的面时，运行 `--write`，并确保清单行的文档标记仍存在于
所指定的文档文件中。清单过期或缺少文档标记会让 `go test ./internal/toolgen` 失败。

## 输出与水合标志

查询类和评审类命令共享一套一致的输出与水合（hydration）面，由一项反向的标志契约测试保持与 CLI 对齐：

- `--format <text|yaml|json>` —— `status` 支持全集；`review`、`validate`、`repair` 和
  `health` 只用 `--format` 来决定 `--list-focuses` 的输出（`text|json`）。在支持的地方，
  `--json` 是 `--format json` 的简写。
- `--hydrate` / `--hydrate-ref <skill-id>/<name>` —— `status`、`review` 和 `health` 会把所选
  的水合引用正文追加到文本输出；`--hydrate-ref` 把水合限定到指定的引用（可重复）。
- `--focus <alias>` / `--list-focuses` —— `status`、`health`、`review`、`validate` 和
  `repair` 接受一个公开的 focus 覆盖；运行 `<command> --list-focuses` 即可枚举。已知别名：
  `status`/`health` → `incident`；`review` → `sast`、`calibration`；`validate` → `sast`、
  `property`、`mutation`、`spec-trace`；`repair` 目前没有暴露任何别名。
- `status --root` 打印规范的 Slipway scope 根；`status --stats` 显示工作区诊断（活动变更数、过期
  摘要、完整性问题）。
- `next --no-auto-pass` 报告 skill 的可执行性，而不是自动放行。
- `done --all-ready` 归档当前所有处于 done-ready 的活动变更。
- 同一意图范围内的改动由当前阶段命令作为变更修订处理：更新所属产物和证据，然后继续向前推进。执行代理
  绝不能悄悄写到已声明的任务范围之外；它们要么提出修订，要么返回一个阻塞项。如果目标变了，就开启一个
  新的受治理变更。

## 实用的 JSON 调用

```bash
slipway new --json "refresh docs"
slipway intake --json
slipway plan --json
slipway implement --json
slipway fix --json
slipway next --json --diagnostics
slipway run --json --diagnostics
slipway status --json
slipway validate
slipway handoff show --json
slipway config list --json
slipway evidence task --task-id t-01 --verdict pass --evidence-ref host:proof --changed-file cmd/example.go --json
slipway health --doctor --json
```

用于 JSON 契约覆盖的稳定清单标记：

| 契约 | 标记 |
| --- | --- |
| abort JSON | `slipway abort --json` |
| cancel JSON | `slipway cancel --json` |
| codebase-map JSON | `slipway codebase-map --json` |
| config JSON | `slipway config list --json` |
| delete JSON | `slipway delete --change <slug> --json` |
| done JSON | `slipway done` |
| evidence skill JSON | `slipway evidence skill --skill <name> --verdict pass --json` |
| evidence skill refresh-current JSON | `slipway evidence skill --skill <selected-review-skill> --verdict pass --refresh-current --reference "context_origin:stage=review=<handle>" --notes-file artifacts/changes/<slug>/verification/<selected-review-skill>-notes.md --json` |
| evidence task JSON | `slipway evidence task --task-id t-01 --verdict pass --evidence-ref host:proof --changed-file cmd/example.go --json` |
| fix JSON | `slipway fix --json` |
| handoff JSON | `slipway handoff show --json` |
| health JSON | `slipway health --json` |
| implement JSON | `slipway implement --json` |
| instructions JSON | `slipway instructions <artifact> --json` |
| intake JSON | `slipway intake --json` |
| new JSON | `slipway new --json` |
| next JSON | `slipway next --json` |
| plan JSON | `slipway plan --json` |
| preset JSON | `slipway preset <level> --json` |
| repair JSON | `slipway repair --json` |
| review JSON | `slipway review --json` |
| run JSON | `slipway run --json` |
| status JSON | `slipway status --json` |
| validate JSON | `slipway validate` |

`next --json` 会包含 `next_skill.name`，用于 AI 工具交接。宿主工具会按自己的适配器约定推导出本地
`SKILL.md` 路径。

启用诊断时，评审状态交接 JSON 还可以包含：

- `next_skill.display_name`、`next_skill.blocking_name` 和 `next_skill.resolution_reason`，当
  概念阶段与可执行的缺失 skill 不一致时给出。
- `next_skill.review_context.required_artifact_layers` 和
  `next_skill.review_context.required_implementation_layers`，它们映射到确切的门禁标记，例如
  `layer:R0=pass`、`layer:R3=pass`、`layer:IR1=pass` 和 `layer:IR3=pass`。
- 顶层的 `confirmation_requirement`，它报告硬停是否需要新的用户确认、先前的授权是否足够、下一步操作员
  动作的人类可读说明（`next_action`）、一个机器可读的 `next_action_kind`（`skill_handoff` |
  `review_batch` | `preset_confirmation` | `command` | `blocker_resolution` | `confirmation` |
  `none`），以及当某个命令可原样运行时要执行的确切 `next_command`。请基于
  `next_action_kind`/`next_command` 分支处理；把 `next_action` 仅当作展示文案。
- `freshness_diagnostics`，它报告过期的源/证据对、字段级的执行输入不匹配、路径权威，以及下一步重新
  生成的动作。

`run --auto` / `run --no-auto` 为单次调用覆盖 `execution.auto`。配置级的 `execution.auto` 也适用
于 `intake`、`plan` 和 `implement`；这些阶段命令没有覆盖标志。auto 只在一次成功推进之后
跨越例行的 `run_slipway_run_to_advance` 命令边界。技能交接和评审批次仍会让 run/stage
循环停下等待 host 工作；非敏感/非 guardrail 的交接可能报告为 `evidence_continuation`
而不是 `hard_stop`。`security-review` 边界、敏感/guardrail 确认、intake 的
Approved Summary、done finalize 以及证据门禁仍然是硬停点。

`validate` 是活动就绪状态的权威：它回答当前受治理状态现在能否推进，并通过
`actionable_next_skill` 映射可执行的评审交接，其中包括可执行 skill 必须提供的确切层引用
`required_tokens`。`run --json` 是发生变更的转换面：`advanced` 报告本次调用改了什么，而 `blockers`
报告任何转换之后的当前停止条件。因此一次成功的推进之后，可能紧跟着针对下一个必需 skill 的
error 级阻塞项。`health --governance --json` 是诊断性的健康反馈；用它来检查控制项和可追踪性细节，
而不是把它当作判断 `run` 是否刚刚推进的生命周期权威。

`status --json` 在已知执行证据过期时会包含 `freshness_diagnostics`，并给每个 `artifact_dag` 节点
标上 `blocking` 加 `blocking_reason`，这样草稿计划产物就不会被误认作当前的评审阻塞项。

`validate --change <slug>` 选择一个明确的活动变更。如果该 slug 指向一个已归档的终态变更，命令会以
`archived_change_not_validatable` 失败，并返回终态状态以及已归档的 `change.yaml` 路径，而不是返回
通用的“无活动变更”诊断。这是一项活动就绪契约：`validate` 在 `done` 之前证明当前活动的受治理状态；
它不是面向已冻结包的归档后审计面。
如果明确给出的 slug 不对应活动或归档变更，`validate --change <slug>` 会 fail closed，
以退出码 3 和 `error_code=change_not_found` 返回。相对地，未指定 `--change` 的
`validate` 在没有活动变更时是诊断视图：退出码为 0，并报告
`invocation_route.kind=no_active` 与 `next_command=slipway new`。

`artifacts/codebase/**` 下的持久 codebase map 不计入 scope-contract 的改动文件核算。当只有这些
上下文文件是脏的时候，它们不会进入 `scope_contract.changed_files` 和
`scope_contract.out_of_scope_files`，而 `scope_contract.status` 保持 `pass`——单单刷新一次
codebase map 不会触发 scope-contract 漂移。为了让这种过滤是可见的、而不是从 Git 差异输出的不一致
里去推断，被豁免的文件会在 `scope_contract.exempt_context_files` 字段里显式披露，由
`slipway validate`、`slipway status --json` 和 `slipway review --json` 呈现。诚实地零改动的
pass code 任务会带上 `no_op_justification`；范围契约将其豁免于变更文件要求，并在同样这三个面上以
`scope_contract.no_op_justified_tasks` 字段披露（任务 id 与其理由），这样审查者无需读取原始证据即可
看到一个零改动任务为何通过。

`slipway evidence task` 把扁平的运行时任务 JSON 写到
`.git/slipway/runtime/changes/<slug>/evidence/tasks/` 下，供 wave-orchestration 同步。S2 wave host 拥有每个任务的 verdict，并用 `--task-id`、`--verdict`、`--evidence-ref`、完整的 `--changed-file`、可选的 `--blocker`、可选的 `--session-id`，以及零改动 pass code 任务使用的 `--no-op-justification` 来记录。executor 或 subagent 输出只是宿主判断的事实输入，不是自证的治理 payload。该命令会计算 `freshness_inputs`，从活动 wave plan 和当前任务证据运行中推导 ledger-owned 字段，校验任务种类/裁决/阻塞项，并拒绝未知或路径不安全的任务 ID，而不是依赖手写 JSON。`freshness_inputs` 包含当前由任务派生的 `tasks_plan_hash`，这样在 `tasks.md` 发生语义变化之后，任务证据就不能被复用。

`slipway evidence skill --skill wave-orchestration` 是执行摘要证据的 S2 引导。在
`execution-summary.yaml` 存在之前，它从当前扁平任务证据 ledger 推导 wave 运行版本，要求所有任务证据
使用单一且有效的 `run_summary_version`，并据此 ledger 给 wave-orchestration 摘要盖戳。之后那些绑定
运行摘要的 skill，例如 `spec-compliance-review`、`code-quality-review` 和终态的
`ship-verification` 门禁，仍然要求执行摘要已存在，缺失时会以
`evidence_skill_run_summary_missing` 失败即停。

被接受的治理 skill 证据还额外受 `verification/evidence-digests.yaml` 约束，这是一个引擎拥有的本地
文件，记录每个通过的 skill 所认证输入的内容摘要。该条目还存储被接受的验证裁决时间戳，使得较新的
宿主重跑裁决能在发生变更的推进过程中替换过期的摘要。只读命令只是比较已存摘要与当前输入；发生变更的
推进路径会在接受通过证据时给文件盖戳。差异类评审摘要认证的是当前工作差异（`git diff HEAD` 加上
未被忽略的、可评审的未跟踪文件，排除 `artifacts/changes/**` 下的 Slipway 受治理/运行时产物），所以
在评审和收尾之间的一次 commit 会让只读投影报告评审过期，直到所属评审阶段针对新的差异边界通过
`slipway run` 重新运行。如果必需的摘要证据缺失或过期，所属治理 skill 会被报告为过期，必须重新运行。

所选的 S3 评审同伴（spec-compliance-review、independent-review、在工作流配置要求时的
code-quality-review，以及按策略选中时的 security-review）针对当前差异、计划产物和运行摘要版本断言
各自的裁决；它们不使用某个共享的 suite-result 基石。唯一一次权威的全套运行——加上任何 guardrail SAST
基线——由终态的 `ship-verification` 门禁拥有，它在评审同伴收敛之后运行一次，绝不依赖某条同伴共享的
记录。没有 `slipway evidence suite-result` 子命令：ship-verification 会作为它单次终态证据过程的一
部分，自己运行并记录整套测试。

`repair --json` 把 `applied_repairs` 和 `unrepaired_drift` 分开。已应用的修复是真正执行过的有界
本地修复；未修复的漂移则包含一个目标、一个原因，以及针对 Slipway 没有自动改动的证据或产物工作的
`next_action`。那些仅仅因为运行时任务证据更新而过期的就绪执行摘要，可以从当前有 wave 支撑的任务证据
重建；过期的计划源漂移则保持未修复。归档清理后留下的空孤立活动包目录会作为 `empty_orphan_bundle`
已应用修复被移除；非空的孤立包仍是需要操作员审阅的完整性发现。诸如
`.git/slipway/runtime/handoff.md` 的遗留仓库级交接文件会被报告为需手动迁移到
`.git/slipway/runtime/changes/<slug>/handoff.md`。空的、未持有的锁锚点会被报告为
`cleaned_lock_anchor`；`change-create.lock` 和 `repair.lock` 仍是工作区/scope 级的协调锁，而不是
逐变更锁。缺失任务证据的阻塞项包含运行时任务证据路径、
`record_command=slipway evidence task --task-id <task_id> --verdict <verdict> --evidence-ref <ref> [--changed-file <path> ...] --json`，以及 host fields：
`task_id,verdict,evidence_ref,changed_files,no_op_justification,blockers,session_id`。`health --json` 的发现包含 `active_change_blocking` 和 `active_change_impact`；咨询性
的 codebase-map 警告对活动变更被标记为非阻塞。

`done` 会归档 done-ready 且绑定 worktree 的变更，即使源文件或非活动治理产物仍未提交，并返回
一个非阻塞的 `worktree_dirty_warning`，带上 `worktree_dirty_files`，让操作员把这些文件和已归档的
包一起提交。`done` 永远不会移除 worktree，而 `git worktree remove` 本来就会拒绝删除一个脏 worktree，
所以这条提示取代了硬阻塞。当前的 `artifacts/changes/<slug>/` 包不在提示范围内，因为 `done` 会把它
重写进 `artifacts/changes/archived/<slug>/`；同级或已归档的包则会被列出。

## 恢复执行

如果某个执行会话可恢复：

```bash
slipway run --resume --json
```

当状态看起来被中断或不一致时，在 repair 或 resume 之前先用 `health --doctor`。

`run --resume` 只适用于可恢复的执行状态，例如 `S2_IMPLEMENT`。如果活动变更已经处于 S3 评审或
done-ready，JSON 错误会包含 `current_state`、`resumable_states`，以及一个把操作员引回 S3 评审/
done-ready 流程的 `next_action`。
