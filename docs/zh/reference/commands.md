# 命令参考

Slipway 有七个用户命令，外加 generated adapter 调用的 `protocol` 操作。请对正在使用的二进制执行 `slipway <command> --help`；包管理渠道可能仍提供旧命令版本。

| 命令 | 用途 |
| --- | --- |
| `install` | 为选定 AI 编程宿主生成能力。 |
| `uninstall` | 删除未修改的 Slipway managed host file。 |
| `list` | 显示 adapter detection 与安装状态。 |
| `doctor` | 诊断仓库、adapter、GitHub tooling 与 Run storage。 |
| `run` | 启动 ad-hoc 或 issue-backed Run 并返回首个 Action。 |
| `status` | 列出 Run 或检查一个 Run。 |
| `stop` | 停止 Run，但不删除恢复数据。 |

所有命令都接受 `--help`。JSON output 包含 `contract_version`；机器消费者必须校验版本，不得解析 human prose。

## `slipway install`

```text
slipway install [--root PATH] [--tool ID]... [--surface ide|cli] [--refresh] [--json]
```

省略 `--tool` 时选择检测到的宿主。可重复 `--tool` 选择多个宿主。Kiro 首次安装必须指定一个 `--surface`。在混合选择中，`--surface` 只作用于 Kiro；仅当未选择 Kiro 时该 flag 才无效。`--tool all --surface ide` 与 `--tool all --surface cli` 都合法。

首次 install 只认领自己创建的文件。`--refresh` 更新仍与记录一致的 Slipway-owned file，并重建缺失的 pristine file；被修改或未知内容不会被覆盖。

JSON 会报告 host、transaction outcome、written/removed path、preserved content、recovery artifact 与 warning。未 commit 的 transaction 不会把计划写入或删除报告为已完成。

## `slipway uninstall`

```text
slipway uninstall [--root PATH] [--tool ID]... [--json]
```

只删除 hash 仍匹配的 managed file。被修改文件和 host settings 保留，Run journal 不受影响。
省略 `--tool` 时会选择所有带 ownership manifest 的 host；若一个也没有则失败。重复 `--tool` 可将卸载范围限制为指定 host。

## `slipway list`

```text
slipway list [--root PATH] [--json]
```

列出十个 adapter target 的 detection、installation、refresh 和 capability 信息。格式错误或不支持的 ownership manifest 只会让对应 host 的只读结果降级，不修改文件，也不隐藏其他 host。

## `slipway doctor`

```text
slipway doctor [--root PATH] [--json]
```

检查 repository discovery、host adapter、generated file、Run storage durability、GitHub CLI/auth/repository permission 与 retired-state residue。GitHub 或 residue warning 不会修改 Run；认证响应和 token 不会写入报告。

`doctor` 只描述观察结果，不运行项目测试，也不判断代码是否 ready。

## `slipway run`

```text
slipway run [--root ROOT] [--source-file FILE] [--budget N] [--no-review] [--json] -- <goal>
```

创建 Run 并返回初始 `orient` Action。Action budget 默认 8，合法范围为 1–1000。`--no-review` 禁用 advisory Review；否则只有 Slipway 在某项 Action 后观察到代码变化时才签发 Review。

省略 `--source-file` 时是 ad-hoc Run；提供时，CLI 打开并验证一个有界 GitHub Change source envelope，固定已接受 section 后关闭文件。CLI 不负责获取 GitHub，也不显示宿主 publication warning；这些由 generated host instructions 执行。

Canonical machine invocation 将所有 flags 放在 `--` 前，goal 放在其后：

```bash
slipway run --budget 8 --json --root /absolute/repository -- "small private fix"
slipway run --budget 8 --json --root /absolute/repository \
  --source-file /private/temp/change-envelope.json -- "implement the Change"
```

该命令返回 Action，不会自行执行代码修改。

## `slipway status`

```text
slipway status [run-id] [--root ROOT] [--json]
```

省略 ID 时列出 Git common directory 中的 Run。当前 worktree 的 Run 会 replay；其他 linked worktree 的 Run 只显示标记为 `workspace_foreign` 的只读 header。完整检查和 mutation 必须在 owning worktree 中执行。

`status` 对文件系统是只读的：不会创建 Run namespace 或 lock file，不会修改权限，也不会修复中断的 journal tail。无法 replay 的本地 recovery directory 仍会在 JSON 的 `unavailable_runs` 中可见；指定该 ID 会返回 `run_journal_invalid`，真正不存在的 ID 则返回 `run_not_found`。如果 writer 持有 commit boundary 超过有界检查时限，指定读取和列表输出都会报告 `run_busy`，不会把 journal 误报为损坏。

指定 ID 时返回当前 Run projection 和实时派生的结构化 `next`。空列表是合法输出。

## `slipway stop`

```text
slipway stop [run-id] [--root ROOT] [--json]
```

停止 Run 并保留 journal。省略 ID 时会扫描列出的 active/paused entry，且只有计数为一时才继续；只要存在无法读取的本地 recovery directory，也必须显式指定 ID，不能忽略。Active/paused `workspace_foreign` stub 不会被隐式选中。Stopped Run 可以 resume；ended Run 不可以。

## 机器协议操作

Generated adapter 使用 `protocol` 操作提交 Outcome、回答或 skip Action、resume Run，并读取 pinned material。它们出现在 top-level help 中，因为它们是已发布的契约而非实现细节；隐藏一份契约会让人误解它。

它们仍然不是另一套用户工作流。每个操作都需要一个 Run 和一个 Action，而这些只能来自递给你的那个 Action，因此没有任何一个可以独立调用。

应使用 CLI 返回的结构化 `next` variant，不要根据文档 prose 拼接命令。详见[机器协议](machine-protocol.md)。
