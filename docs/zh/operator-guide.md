# 运维指南

本指南面向维护 Slipway 工作区的人员和 agent。

## 状态权威源

| 路径 | 作用 | Git 策略 |
| --- | --- | --- |
| `.slipway.yaml` | 仓库本地的 Slipway 配置。 | 项目默认值变更时提交。 |
| `artifacts/changes/<slug>/change.yaml` | 活动变更的当前生命周期与路由权威源。归档快照保留在其所属工作区中，省略机器本地的 `worktree_path`，并使用归档本地的 artifact 路径。 | 可纳入版本管理的项目记录。 |
| `artifacts/changes/<slug>/*.md` | 意图、调研、需求、决策、任务和保障。 | 可纳入版本管理的项目记录。 |
| `artifacts/changes/<slug>/events/lifecycle.jsonl` | 仅追加的生命周期变更事件轨迹。 | 默认仅本地保留的原始证据。 |
| `artifacts/changes/<slug>/verification/*.yaml` | 技能与验证证据。 | 默认仅本地保留的原始证据。 |
| `.git/slipway/runtime/changes/<slug>/evidence/**` | 供 wave 执行和时效性诊断使用的运行时任务证据。 | Git 内部的本地运行时状态。 |
| `.git/slipway/runtime/changes/<slug>/handoff.md` | 面向活动变更的全新 AI 会话的可选辅助续接说明。它不是生命周期权威源、受治理的证据、时效性输入，也不是关卡。 | Git 内部的本地运行时状态。 |
| `.git/slipway/locks/change-create.lock`、`.git/slipway/locks/repair.lock` | 用于变更创建和 repair 的工作区/作用域级协调锁。它们保持全局，因为这些临界区在稳定的每变更锁建立之前或之外就已开始。 | Git 内部的本地运行时状态。 |
| `artifacts/codebase/**` | 由 `slipway codebase-map` 生成的辅助性代码库地图。 | 可纳入版本管理的项目记录；默认纳入 git 跟踪（已有仓库会在下一次受管块重写时自动迁移）。 |
| `.worktrees/<slug>` | 专用的受治理 worktree 检出目录。 | 默认仅本地保留。 |

不要把 `events/lifecycle.jsonl` 当作 `change.yaml` 的替代品。它只是审计证据。
不要把运行时任务证据写入活动 bundle；`slipway evidence
task` 会把它记录到 `.git/slipway/runtime/changes/<slug>/evidence/tasks/` 下。
如果续接说明有用，先用最新的 `status` 或 `next` 输出解析出 `<slug>`，再把说明写入
`.git/slipway/runtime/changes/<slug>/handoff.md`。不要从 handoff 文本推导生命周期状态、技能选择或时效性。
`slipway init`、`slipway new` 和 `slipway codebase-map` 会幂等地维护 Slipway 本地状态的 `.gitignore` 块。

## Worktree

受治理的工作可以绑定到 `.worktrees/<slug>` 下的专用 worktree。使用持有当前受治理差异的那个 worktree：

```bash
git status --short --branch
go run . status --json
```

不要仅凭 `main...HEAD` 判断就绪状态。把分支比较与直接的 worktree 状态和差异检查搭配使用。
`artifacts/codebase/**` 下的持久代码库地图不计入 scope-contract 的变更文件统计：当只有这些上下文文件处于 dirty 状态时，它们不会进入
`scope_contract.changed_files` 和
`scope_contract.out_of_scope_files`，且 `scope_contract.status` 保持 `pass`。
为了让这种过滤可见、而不是要从常规差异输出
的不一致中推断，被豁免的文件会在 `slipway validate`、`slipway status --json` 和 `slipway review --json` 暴露的
`scope_contract.exempt_context_files` 字段中披露出来。
执行 `slipway done` 后，Git 可安全跟踪的归档记录会留在其所属 worktree 中；在移除该 worktree 之前先提交或合并它们。
当一个绑定到 worktree 的变更仍有未提交的源码或非活动的治理改动时，`done` 仍会归档，并返回一个非阻塞的
`worktree_dirty_warning`（带 `worktree_dirty_files`），以便运维人员把这些文件与归档 bundle 一起提交。`done` 不会移除该 worktree，而 `git worktree remove` 也会拒绝删除 dirty 的 worktree，所以这条提示已经足够。活动的 `artifacts/changes/<slug>/` bundle 不在该提示范围内，因为 `done` 会把它重写到
`artifacts/changes/archived/<slug>/`；dirty 的同级 bundle 或归档 bundle 则会列入提示。

## 健康检查与修复

变更前先检查：

```bash
slipway health --doctor --json
slipway validate
slipway status --json
```

只有当 doctor 输出与观察到的问题相符时，才运行 repair：

```bash
slipway repair --json
```

repair 用于处理有界的本地完整性问题，例如残留的锁、未持有的锁锚点、被中断的归档、损坏的配置或可修复的布局漂移。它会报告诸如
`.git/slipway/runtime/handoff.md` 之类的旧式仓库级运行时 handoff 文件，让运维人员在删除前把有用的上下文迁移到当前的每变更 handoff 路径。
在 JSON 输出中，`applied_repairs` 列出已执行的修复，而
`unrepaired_drift` 列出仍需运维人员处理的漂移，并附带目标、原因和下一步动作。不要手工编辑时效性字段或时间戳；应改为重新生成所指名的证据，或对源产物做一次同一意图的变更修订。
对于仅因运行时任务证据更新而变得陈旧的就绪执行摘要，repair 可以基于当前的 wave 支撑任务证据重建该摘要。
规划源漂移仍保持未修复状态，并指回规划或评审证据的刷新。

健康检查结果包含对活动变更的影响。代码库地图警告默认是辅助性的，应针对当前关卡标记为非阻塞，并在地图需要重建时附上刷新路径或命令。

## 诊断 JSON

执行时效性诊断是结构性的，而非基于哈希。当前执行摘要会记录任务时效性输入，例如 `change_id`、
`run_summary_version`、`task_id` 和 `guardrail_domain`；仅含哈希的旧式摘要会被视为陈旧，必须重新生成。

`next --json --diagnostics`、`run --json --diagnostics`、`validate` 和
`status --json` 会暴露时效性失败，包含陈旧的源/证据配对、首个陈旧成因、下游证据链、期望/当前的任务输入值、权威的 bundle 与运行时路径，以及一个安全的下一步动作。
缺失任务证据的阻塞项会包含运行时任务证据目录、
`record_command=slipway evidence task --task-id <task_id> --verdict <verdict> --evidence-ref <ref> [--changed-file <path> ...] --json`，以及 host-owned fields：
`task_id,verdict,evidence_ref,changed_files,no_op_justification,blockers,session_id`。
wave host 决定 verdict 并记录任务 evidence；executor 或 subagent 输出只是事实输入，不是自证的治理 payload。
活动变更的目录是 `.git/slipway/runtime/changes/<slug>/evidence/tasks/`；bundle 本地的 `events/` 和 `verification/` 仍位于 `artifacts/changes/<slug>/` 下。

Reason-code 的 `code` 值是面向阻塞项、恢复路由、JSON 消费方和生成技能的稳定机器契约。规范枚举是
`internal/model/reason_code.go` 中
`canonicalReasonDefinitions` 的键集合，`internal/model/reason_code_contract_test.go`
通过快照测试冻结该集合以及每个 code 的严重程度。把 `message` 当作展示用文案：reason/error 载荷测试和技能逻辑必须断言诸如 `code`、`detail`、`error_code`、`category`、`exit_code`
之类的稳定字段或结构化细节，而不是匹配 message 文本。仓库本地的 AST lint 会对语法上可识别的 reason/error 载荷面（`ReasonCode`、
`CLIError`、`HealthFinding`、已知的构造函数/辅助函数，以及 blocker/reason 集合）强制执行该规则；其他名为 `Message` 的字段仍归评审管辖，除非它们成为该 reason/error 载荷面的一部分。如果生产方发出未识别的 token，归一化会向 `unknown_reason_code` 失败关闭，并在 `detail` 中保留原始 token，以便修复生产方并将其加入规范枚举。从 CLI 错误到 reason 载荷的桥接只能直接保留规范的 reason code；非 reason 域的 `error_code` 值必须放在规范包装 reason 的 detail 中携带，而不是作为独立的 reason code 归一化。

评审交接使用精确的层级 token。spec-compliance 证据记录
`layer:R0=pass`，并在 guardrail 域要求时记录 `layer:R3=pass`。
code-quality 证据记录 `layer:IR1=pass`，并在要求时记录
`layer:IR3=pass`。诸如 `layer:CORRECTNESS=pass`、
`layer:SAFETY=pass` 或 `layer:QUALITY=pass` 之类的 token 不能作为满足关卡的替代项。

状态 artifact 的 DAG 条目包含 `blocking` 和 `blocking_reason`。当生命周期已越过规划关卡后，一个草稿状态的规划 artifact 可以只是信息性的；应把该标志当作当前关卡信号。

## 验证栈

实现过程中使用有针对性的检查：

```bash
go test ./internal/stringutil ./internal/engine/progression ./internal/engine/governance -run 'TestHasBlockingOpenQuestions|TestFirstBlockingOpenQuestion|TestAdvanceIntake_OpenQuestionsUseChecklistStructure|TestOpenQuestionsRoutingNoteNamesEntryAndEscapeHatch|TestTraceability.*OpenQuestions|TestGovernanceReadinessUsesTraceabilitySnapshot' -count=1
```

收尾前使用完整证明：

```bash
go test -timeout=20m ./... -count=1
go build ./...
go vet ./...
(cd website && npm run build)
```

只有在本地具备 Node 依赖时才运行文档构建（Astro Starlight）；先执行 `cd website && npm install`。CI 会运行同样的文档构建做验证。

## 适配器刷新

更改模板或命令契约后，刷新生成的 AI 工具面：

```bash
slipway init --tools all --refresh
```

提交前检查生成的路径变化。Codex 命令面位于
`.codex/skills/slipway-<command>/SKILL.md` 下；Codex 刷新不再触碰宿主全局的 `$CODEX_HOME/prompts` 文件。

## 收尾

执行 `done` 之前：

1. 确认 `go run . validate` 报告相关的活动变更关卡均已批准。这是归档前的时效性/就绪关卡，并不保证同一归档 bundle 在 `done` 之后还能重新验证。
2. 确认任务证据对当前运行版本仍然有效。
3. 确认 `git diff --check`。
4. 只暂存预期内的文件。
5. 确认 `git diff --cached --check`。
6. 当变更达到 done-ready 时运行 `slipway done`。活动变更 bundle 不需要在 `done` 之前提交。
7. 如果返回了 `worktree_dirty_warning`，说明变更已经归档；在移除 worktree 之前，把列出的 `worktree_dirty_files` 与归档 bundle 一起提交。

执行 `done` 之后，把归档的 `change.yaml` 和 bundle 内容当作冻结的项目记录使用。`validate --change <slug>` 会刻意以 `archived_change_not_validatable` 拒绝归档的 slug；只读的归档审计将是另一个独立的命令面。
