# 命令参考

Slipway 有七个用户命令，外加 generated adapter 调用的 `protocol` 操作。请对正在使用的二进制执行 `slipway <command> --help`；包管理渠道可能仍提供旧命令版本。

| 命令 | 用途 |
| --- | --- |
| `install` | 为选定 AI 编程工具生成能力。 |
| `uninstall` | 删除未修改的 Slipway managed host file。 |
| `list` | 显示 适配器检测 与安装状态。 |
| `doctor` | 诊断仓库、adapter、GitHub tooling 与 Run storage。 |
| `run` | 启动 ad-hoc 或 issue-backed Run 并返回首个 Action。 |
| `status` | 列出 Run 或检查一个 Run。 |
| `stop` | 停止 Run，但不删除恢复数据。 |

所有命令都接受 `--help`。JSON output 包含 `contract_version`；机器消费者必须校验版本，不得解析 human prose。

## `slipway install`

```text
slipway install [--root PATH] [--tool ID]... [--surface ide|cli] [--refresh] [--json]
```

省略 `--tool` 时选择检测到的宿主。可重复 `--tool` 选择多个宿主，也可以传入一个逗号分隔的值，例如 `--tool claude,codex`；两种写法等价。Kiro 首次安装必须指定一个 `--surface`。在混合选择中，`--surface` 只作用于 Kiro；仅当未选择 Kiro 时该 flag 才无效。`--tool all --surface ide` 与 `--tool all --surface cli` 都合法。

首次 install 只认领自己创建的文件。`--refresh` 更新仍与记录一致的 Slipway-owned file，并重建缺失的 pristine file；被修改或未知内容不会被覆盖。

JSON 会报告 host、transaction outcome、written/removed path、preserved content、recovery artifact 与 warning。未 commit 的 transaction 不会把计划写入或删除报告为已完成。

## `slipway uninstall`

```text
slipway uninstall [--root PATH] [--tool ID]... [--json]
```

只删除 hash 仍匹配的 managed file。被修改的文件和宿主设置会保留，Run journal 不受影响。
省略 `--tool` 时会选择所有带 ownership manifest 的 host；若一个也没有则失败。重复 `--tool` 可将卸载范围限制为指定 host。

## `slipway list`

```text
slipway list [--root PATH] [--json]
```

列出十个 适配器目标 的 detection、installation、refresh 和 capability 信息。格式错误或不支持的 ownership manifest 只会让对应 host 的只读结果降级，不修改文件，也不隐藏其他 host。

## `slipway doctor`

```text
slipway doctor [--root PATH] [--json]
```

检查 repository discovery、host adapter、generated file、Run storage durability、GitHub CLI/auth/repository permission 与 retired-state residue。GitHub 或 residue warning 不会修改 Run；认证响应和 token 不会写入报告。

`doctor` 只描述观察结果，不运行项目测试，也不判断代码是否 ready。

## `slipway run`

```text
slipway run [--root ROOT] [--source-file FILE] [--budget N] [--no-review] [--json]
  (--goal-file FILE | --goal-stdin | -- <goal>)
```

创建 Run 并返回初始 `orient` Action。Action budget 默认 8，合法范围为 1–1000。`--no-review` 禁用 advisory Review；否则只有 Slipway 在某项 Action 后观察到代码变化时才签发 Review。

省略 `--source-file` 时是 ad-hoc Run；提供时，CLI 打开并验证一个范围明确 GitHub Change source envelope，固定已接受 section 后关闭文件。CLI 不负责获取 GitHub，也不显示宿主 publication warning；这些由 生成的宿主指令 执行。

Goal input 必须且只能选择一种：human caller 可使用一个 positional goal、`--goal-file` 或 `--goal-stdin`，三者互斥。Generated adapter 会使用私密临时 regular file，避免 exact goal 暴露在 process list 中，也避免平台 command-line length 限制。Canonical machine invocation 为：

```bash
slipway run --budget 8 --json --root /absolute/repository \
  --goal-file /private/temp/goal.txt
slipway run --budget 8 --json --root /absolute/repository \
  --goal-file /private/temp/goal.txt \
  --source-file /private/temp/change-envelope.json
```

CLI 消费后，宿主会删除临时 goal/source file。直接使用 `-- <goal>` 仍是方便的 human form。

该命令返回 Action，不会自行执行代码修改。

## `slipway status`

```text
slipway status [run-id] [--section KEY] [--root ROOT] [--json]
```

省略 ID 时列出 Git common directory 中的 Run。当前 worktree 的 Run 会 replay；其他 linked worktree 的 Run 只显示标记为 `workspace_foreign` 的只读 header。完整检查和 mutation 必须在 owning worktree 中执行。

`status` 对文件系统是只读的：不会创建 Run namespace 或 lock file，不会修改权限，也不会修复中断的 journal tail。指定 Run ID 时，不存在返回 `run_not_found`，本地 Run 损坏返回 `run_journal_invalid`，writer 在范围明确检查时限内持续持有 commit boundary 则返回 `run_busy`。Repository-wide JSON 会把无法读取的本地 identity 保留在 `unavailable_runs`；其中每个 entry 的 `code` 只能是 `run_journal_invalid`、`run_unavailable` 或 `run_busy`。`run_not_found` 只属于 targeted error，不会出现在 `unavailable_runs[].code`。

指定 ID 时返回当前 Run projection 和实时派生的结构化 `next`。空列表是合法输出。

`--section KEY` 以 `pinned_material` 消息返回当前钉住的一个 source chapter：与 Action 读到的字节完全相同，并附带其 section 与 requirements revision。它需要 Run ID，在包括 `stopped` 与 `ended` 在内的所有状态下都可用；ad-hoc Run 没有 pinned source，返回 `material_unavailable`。`status --json` 的 projection 已列出被钉住的 chapter，此处返回它们的正文。

这是检查而非执行路径。Action 读取 material 仍然只能走 `protocol material`，因为它额外把读取绑定到当前非 void 的 Action。在此读取 chapter 不授予任何实现、发布或 Run 权限，不追加 journal event，返回的是当前钉住的 revision 而非历史 revision。

## `slipway stop`

```text
slipway stop [run-id] [--root ROOT] [--json]
```

停止 Run 并保留 journal。Stop 会撤回当前 Action，因此 stopped Run 不再报告 `current_action`，也不再报告 destructive authorization；journal 仍记录它签发过的每个 Action。Resume 总是签发新的 Orient。省略 ID 时会扫描列出的 active/paused entry，且只有计数为一时才继续；只要存在无法读取的本地 recovery directory，也必须明确指定 ID，不能忽略。Active/paused `workspace_foreign` stub 不会被隐式选中。Stopped Run 可以 resume；ended Run 不可以。

## 机器协议操作

Generated adapter 使用 `protocol` 操作提交 Outcome、回答或 skip Action、resume Run，并读取 pinned material。它们出现在 top-level help 中，因为它们是已发布的契约而非实现细节；隐藏一份契约会让人误解它。

它们仍然不是另一套用户工作流。每个操作都只作用于既有 Run；在适用时，Action、candidate 或其他 typed identity 必须来自 CLI 返回的结构化 `next`。应使用这些 variant，不要根据文档 prose 拼接命令。`run` 和 `status` 是生成这些 variant 的入口。详见[机器协议](machine-protocol.md)。
