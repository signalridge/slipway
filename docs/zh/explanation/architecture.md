# 架构

> 本页为非规范性说明；规范语义见[中文产品契约](../reference/product-contract.md)。

Slipway 是一个面向有边界、可恢复的 AI 辅助工作的精简控制平面。它不取代 AI coding 工具、项目跟踪器或 Git。工作由宿主执行；Slipway 每次调度一个版本化 Action，独立观察 Git，按 digest 固定 source，并保存恢复历史。

## 依赖方向

```text
cmd → autopilot → runstore
cmd → adapter → tmpl
cmd → recoverycmd
runstore / adapter / autopilot → fsutil (only required low-level primitives)
```

依赖方向固定，并由架构守卫测试强制执行。`autopilot` 只产生结构化 `next` 值，绝不依赖 `recoverycmd`；`recoverycmd` 既不读取 journal，也不决定路由。

## 代码包

| Package | 职责 |
| --- | --- |
| `cmd` | 七个公开命令、隐藏的版本化 machine commands，以及 text/JSON rendering。 |
| `internal/autopilot` | 严格的 Action/Outcome union、source envelope/revision/candidate、budget、routing、destructive authorization 和结构化 `next` 值。 |
| `internal/runstore` | 发现规范 Git identity，并维护 anchored append-only journal 与可替换 projection。 |
| `internal/adapter` | 为十个宿主规划 ownership-safe capability generation。 |
| `internal/tmpl` | 嵌入精确六项显式能力，以及带 attribution 的 `grill-me` reference。 |
| `internal/fsutil` | Rooted atomic transaction、Git discovery、symlink/reparse 防御和 rollback post-state validation。 |
| `internal/recoverycmd` | 只消费完整 argv，并渲染 POSIX/cmd/PowerShell display command。绝不读取 journal 或决定路由。 |
| `internal/jsonstrict` | 共享 structural scanner，用于拒绝重复 key、无效 JSON 和 trailing data。 |
| `internal/testlint` | 仓库测试策略分析器。 |

## Run 启动与 Git 观察

Run 启动时，CLI 会保存 immutable workspace identity 和 Git fingerprint：精确的 index 与 porcelain-v2 bytes，以及每个预先存在的 dirty/untracked path 排序后的 metadata/digest。恢复过程会在加载和 mutation 前重新验证 identity。若 root 被复用、换成另一个 linked worktree，或 Git metadata 被移动或重定向，系统会在任何 journal mutation 发生前以 `workspace_identity_mismatch` 失败。

从 Run 启动至今观察到的差异会驱动安全侧 Review 路由，但绝不证明该差异由 Run 造成。并发的用户编辑、另一个 Run 或其他工具都可能产生贡献。Slipway 记录事实性的 `observed_since_start` observation 与 `attribution_uncertainty`，绝不会把差异归因于某个宿主或 Run。

## 宿主与 GitHub

Go binary 不持有 provider token。它严格验证经宿主 attestation 的 raw Change envelope，并把规范化后的固定 snapshot 写入 journal。GitHub 的读取和写入发生在宿主侧，使用用户自己的已认证工具；发布过程使用经批准的 operation/item UUID Marker 与 reconciliation，不会因此恢复 repository Change runtime。

架构中不存在 model provider、old-state reader、compatibility alias、dual runtime、ambient activation、required-command registry、Spec/artifact lifecycle、worktree binding 或 automatic review-repair loop。历史数据与 legacy namespace 保持原样，并被运行时忽略。

## 未引入的内容

```text
internal/change   internal/spec   internal/plan
internal/lifecycle   internal/gate   internal/tracker runtime
```

继续阅读[产品权威](../reference/product-overview.md)、[机器协议](../reference/machine-protocol.md)，以及[采用 manifest-addressed source bundle 的决策](../../decisions/0001-source-bundle-v2.md)。
