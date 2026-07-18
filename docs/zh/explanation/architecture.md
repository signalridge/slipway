# 架构

Slipway 将控制循环放在本地 CLI，将模型相关工作放在生成的宿主 adapter 中。这样 CLI 可以验证状态，而无需持有模型或 GitHub 凭据。

![Slipway 进程架构：用户主动调用 AI 编程工具中的生成能力；宿主负责模型、仓库和经授权的 GitHub 工作，只有 Run-backed 路径会通过版本化 JSON 连接本地 CLI 与持久 Run store。](../../assets/diagrams/architecture.svg)

## 进程边界

```text
用户
  └─ 主动调用生成的能力
       └─ AI 编程工具
            ├─ 读取并修改仓库
            ├─ 调用模型与开发工具
            ├─ 经授权后获取或发布 GitHub 数据
            └─ 与 Slipway 交换版本化 JSON
                 └─ 本地 CLI 与 Run store
```

Run 状态引擎不调用模型或 GitHub API。它验证宿主提供的 source envelope、每次调度一个 Action、独立观察 Git 并保存恢复状态。公开 `doctor` 命令是 command layer 的诊断例外：它可能调用用户本机的 `gh` 检查认证与仓库权限。生成的宿主指令 定义宿主如何调查、发布、实现和报告，但不是另一套状态引擎。

## 代码包方向

Architecture test 会约束 production dependency：

```text
cmd ───────────────→ adapter
 │                   ├─→ tmpl
 │                   ├─→ fsutil
 │                   └─→ jsonstrict
 ├─────────────────→ autopilot
 │                   ├─→ runstore
 │                   │    ├─→ fsutil
 │                   │    └─→ jsonstrict
 │                   └─→ jsonstrict
 └─────────────────→ recoverycmd
```

| Package | 职责 |
| --- | --- |
| `cmd` | Cobra 命令、human/JSON output、root discovery 与 exit behavior。 |
| `internal/autopilot` | Action/Outcome validation、routing、source candidate、budget 与 structured recovery。 |
| `internal/runstore` | Journal replay、projection、locking、material storage 与 Git observation。 |
| `internal/adapter` | Host registry、generated file、ownership manifest 与 transactional install/remove。 |
| `internal/tmpl` | 跨宿主共享的 embedded capability instruction。 |
| `internal/fsutil` | Anchored path、no-follow operation、transaction、sync 与平台安全。 |
| `internal/jsonstrict` | Protocol、source、store 与 adapter 边界共享的 strict JSON decoding。 |
| `internal/recoverycmd` | 将已结构化 argv 渲染为人类命令。 |

底层 package 不反向 import command 或 host-policy layer。GitHub publication 保留在 生成的宿主指令 中，不成为 core 内的 network provider。

## Run 启动与仓库观察

新 Run 会发现三个 canonical path：worktree root、per-worktree Git directory 与 Git common directory。它们组成的 ID 将 Run 绑定到该 worktree。Slipway 不创建、切换或删除 worktree，但会拒绝从其他 worktree identity 修改 Run。

Initial Git observation 保存 exact index/porcelain-v2 output 的 fingerprint，以及 dirty path 的范围明确 metadata 与 fingerprint；它不保存 raw Git stream 或文件内容。后续 observation 用于 diff-first routing 和中性的“since start changed”报告，不声称是谁造成变化。

## Run 存储

```text
<git-common-dir>/slipway/runs/<run-id>/
├── journal.jsonl   追加式状态转换记录
├── run.json        可替换 projection
├── run.lock        经过验证的协调文件
└── materials/      按内容 digest 保存的 accepted source section
```

Unix 使用 opened Run directory 上的 OS lock 串行化 writer；Windows 使用 named mutex。可见的 `run.lock` 用于验证和诊断，但不是唯一 writer guard。

Mutation 会先写入被引用 material，再允许 journal event 引用它。Journal sync 后才替换 projection。如果 journal 已 commit 但 projection refresh 失败，error 会报告 committed mutation 与 stale projection，不会声称 rollback。

## Source 边界

Issue-backed 工作中，trusted host 获取 Issue 和 manifest 引用 comments，再传入临时 strict envelope。CLI 能验证内部一致性和稳定 ID，但无法以密码学方式证明宿主确实从 GitHub 获取了数据。

Accepted section 按内容寻址，并通过本地 material reader 提供。Action 只带 revision 和范围明确 catalog，使大型 requirements 不进入 Action context，也支持离线恢复。

Source Bundle 的设计理由与被拒方案记录在 [ADR-0001](../../../adr/0001-source-bundle-v2.md)。issue #434 的基础契约、后续 Accepted ADR 与版本化 schema 共同定义预期契约；[ADR-0002](../../../adr/0002-seventh-capability-workflow.md) 加入第七项宿主能力并重申不引入 router 的边界，[ADR-0003](../../../adr/0003-scope-workflow-to-slipway-functions.md) 将其范围限定为 Slipway 自有功能之间的生命周期路由。Runtime test 是当前实现行为的可执行证据，不能替代这些依据。

## 安全边界

![Slipway 信任边界：Issue 内容与工作区是不可信数据，永远不能授予 shell 权限、泄露凭据、绕过确认或扩大破坏性范围；AI coding host 被信任去执行动作并持有全部凭据；本地 CLI 校验严格 JSON、大小、身份与 digest，自身不持有任何凭据，但无法证明宿主诚实地抓取了 GitHub。](../../assets/diagrams/trust-boundary.svg)

Slipway 假设同账号进程、root、malware 或 compromised host 可以越过其保护。在该边界内，它会：

- anchor filesystem operation 并拒绝 unsafe symlink traversal；
- 验证 strict JSON、size、identity 与 digest；
- 不在 Slipway storage 中保存凭据，并让 GitHub 获取/发布留在 Run core 之外；
- 将一次性 destructive grant 与自然语言 answer 分离；
- 保留用户修改过的 generated file；
- 报告平台 durability limitation。

Issue content 是数据，不是宿主指令。Generated capability 不能把 Issue 中的命令、链接或 credential request 当作授权。

## 明确不负责的内容

Slipway 不会：

- 运行 hosted service 或 project tracker；
- 管理 model-provider 或 GitHub credential；
- 创建或管理 worktree；
- 认证 merge、deployment 或 release readiness；
- 把 test、finding、label 或 Issue state 变成通用 repository policy；
- 自动修复 Review finding；
- 覆盖用户修改过的 adapter file。

外部 branch protection、CI、组织策略和人工 Review 保持独立。

继续阅读[核心概念](concepts.md)、[机器协议](../reference/machine-protocol.md)与 [Run、恢复与隐私](../guides/runs-and-recovery.md)。
