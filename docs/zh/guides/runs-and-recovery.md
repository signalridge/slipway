# Run、恢复与隐私

Run 可以停止并恢复，但 journal 不是秘密保险箱，也不是完成证明。本指南说明用户可以检查和保留哪些内容。

## 检查 Run

```bash
slipway status
slipway status <run-id>
slipway status <run-id> --json
```

省略 ID 时，`status` 扫描仓库的 Git common directory。属于当前 worktree 的 Run 会完整 replay；由另一个 linked worktree 创建的 Run 只显示标记为 `workspace_foreign` 的只读 header stub，必须回到其 owning worktree 才能检查或恢复完整内容。

检查不会创建 recovery directory 或 lock，也不会修复 journal byte。若本地 Run 无法安全 replay，上面 `status` 的列表形式会在 `unavailable_runs` 中保留其 ID 和诊断，而不是将其隐藏；应先检查或恢复 journal，再尝试 mutation。

指定 ID 后，status 包含当前状态和实时派生的结构化 `next` 操作。生成的宿主遵循该对象，而不是解析显示用 shell command。

## Stop、skip 与 resume

```bash
slipway stop <run-id>
```

`stop` 使 pending work 失效，但保留 journal。省略 ID 时会统计列出的本地 active/paused entry，排除 `workspace_foreign` stub；若任何本地 recovery directory 无法读取，则要求显式 ID。Stopped Run 可以 resume；ended Run 不可以。

Skip 是 Action 级控制，不需要理由。调整顺序或接管会停止自动循环，而不是静默改写队列；只有显式 resume 后才继续。

Resume 会重新验证初始 worktree identity 并生成 fresh work。过期 Action ID、source candidate、answer 和 destructive grant 会按机器协议被拒绝或失效。

Resume 省略 `--budget` 时，会保留大于零的 remaining budget；若为零，则补充为初始 budget 与 3 中的较大值。显式传入 `--budget N` 会把 remaining budget 替换为 `N`。只有操作真正 resume Run 时才应用替换值。

## Issue 来源恢复

对于 issue-backed Run，生成的宿主会提供合法来源选择：

- 获取并比较当前 Change；
- refresh 不可用或用户不想 refresh 时，显式继续 pinned snapshot；
- 检测到变化后，保留 pinned snapshot 或采用精确 current candidate。

省略 refresh 永远不表示 Issue 已证明未变化。Issue identity 不同或 source history fork 都需要新 Run。

## 存储布局

Run 数据位于仓库的 Git common directory，不一定是当前 worktree 的字面 `.git` 路径：

```text
<git-common-dir>/slipway/runs/<run-id>/
├── journal.jsonl
├── run.json
├── run.lock
└── materials/
```

- `journal.jsonl` 是恢复使用的追加式状态转换记录。
- `run.json` 是可由 journal 重建的 projection。
- `materials/` 按内容 digest 保存已接受 Issue section。
- `run.lock` 是经过验证的协调文件；真正的 writer serialization 在 Unix 使用 OS directory lock，在 Windows 使用 named mutex。

每次 load 和 mutation 都重新检查 canonical worktree root、per-worktree Git directory 与 Git common directory。路径被其他 worktree 复用或 Git metadata 被重新指向时，会在修改 journal 前失败。

## 可能保存的内容

Run 可能记录：

- goal 与 source identity；
- 已接受 requirements material 和 digest；
- 用户回答与来源选择；
- Action、Outcome、summary、finding 与不确定性；
- 报告的 test、type-check、build、lint 命令和 exit code；
- 用于比较初始与当前 worktree 的有界 Git observation。

Slipway 不会主动收集 GitHub token、credential store、环境变量 dump、无关文件内容、未引用 Issue comments、完整对话或 hidden reasoning。生成的宿主被要求对已识别凭据值进行脱敏，同时保留真实命令身份。

这些保护并非绝对可靠。Goal、requirements、answer、文件名、summary 和命令参数本身都可能敏感。请将 Run directory 视为本地私密数据，不要粘贴 secret。

## 文件观察

Git observation 保存 fingerprint 和有界 metadata，不保存文件内容。小型 regular dirty/untracked file 使用完整流式 digest；大文件使用大小和固定采样区域，因此可能漏掉采样区域外、长度不变的修改。Symlink 不会被跟随。

观察到 Run 启动后的变化不能证明是谁或哪个进程造成。Review 和 summary 会保留这种归因不确定性。

## Durability 与平台

Unix 会同步 journal/projection 文件，并在支持时同步目录。Windows 会 flush 文件，但不能为新增或 renamed directory entry 提供等价 directory-fsync 保证；`doctor` 会报告实际 durability level。

Slipway 拒绝不安全的 symbolic-link 或 reparse-point mutation path。这些控制可降低意外和并发损坏风险，但不能防御 root、malware 或持续竞争的同账号进程。

## 保留与删除

Run 数据不会自动过期；在用户主动删除前，它会一直保留在 Git common directory 中。在 Unix 上，Slipway 以 `0700` 创建 Run directory、以 `0600` 创建文件；在 Windows 上则应用等价的 owner-only ACL。这些权限可以限制其他本地账户，但不能防御同一账户、管理员、恶意软件、备份或已复制的数据。

删除 `<git-common-dir>/slipway/runs/<run-id>/` 会移除该 Run 的 Slipway 本地恢复数据，但不会删除 GitHub 内容、Git history、backup、filesystem snapshot、log 或已复制到其他位置的数据，也不是安全擦除。

Adapter 卸载彼此独立：

```bash
slipway uninstall --tool claude
```

它只删除未修改的生成文件，不会修改 Run 数据。

## 已退役 namespace 残留

`doctor` 可能报告 Git common directory 的 `slipway/` namespace 下仍有旧的 `runtime`、`cache`、`scope-root`、`scopes`、`locks`、`processes` 或 `repair-backups`。当前 Run engine 会忽略这些条目；adapter install、refresh 与 uninstall 不会迁移、删除这些内容，也不会因此阻塞。

这只是 advisory warning。若希望清理，请先停止所有旧版 Slipway process，备份 Git common directory，检查报告的路径，并仅在确认不再需要后手工删除。Slipway 不会推断这些残留的 ownership，也不会自动删除。

## 故障排查

从以下命令开始：

```bash
slipway doctor
slipway status <run-id> --json
```

不要手工编辑 `journal.jsonl` 或 `run.json`。保留原始 worktree 和结构化 error output，使用 `next` 返回的精确恢复选项。若来源或 workspace identity 无法安全恢复，请启动新 Run，不要强行修改旧状态。

更多信息见[命令参考](../reference/commands.md)与[架构](../explanation/architecture.md)。
