# Windows rendering 与 durability

规范语义见[中文产品契约](product-contract.md)和版本化 [machine schema](../../reference/machine-protocol.schema.json)。

## 结构化 argv 是权威

Recovery/pause 返回 `next.operation`、workspace identity 与 typed variants。每个 variant 包含完整 `base_argv` 和有序 inputs；展开时按 schema 顺序把 input flag 与**一个原样、未引用的 argv value**插到唯一 `--` separator 前，没有 separator 时才追加。不得从 display prose 重建命令，也不得产生 `<answer>`/`<file>` 占位伪命令。

Slipway 分别渲染 POSIX、`cmd.exe` 与 PowerShell display command；rendering 只用于展示/复制，不写 journal、不改变 machine operation。Root、Issue URL、source/outcome file、answer、candidate recovery 中的空格、引号、Unicode、CR/LF、`%`、`!`、`&`、`^` 必须保持原 argv。由于 cmd 会展开 `%`/`!`，renderer 可使用 PowerShell UTF-16LE `EncodedCommand` 或等价安全 argv 路径。必须在 `cmd.exe /v:on` 与 PowerShell native 进程实际捕获 argv；Linux cross-build 只证明能编译，不是 W evidence。

CI matrix 构建 `slipway.exe` 后会执行以下两个 native 入口，本地 Windows 也可运行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tests\acceptance\windows\native-powershell.ps1 -SlipwayExe C:\path\slipway.exe
cmd.exe /d /v:on /c tests\acceptance\windows\native-cmd.cmd C:\path\slipway.exe
```

资产覆盖 doctor、initial Orient、issue source、Outcome file/stdin（平台支持处）、decision answer、ad-hoc resume、current-candidate keep/adopt 和特殊 argv。Workflow wiring 本身仍只是 W collector；[run 29197908671 / Windows job 86664073429](https://github.com/signalridge/slipway/actions/runs/29197908671/job/86664073429) 已针对 source `4c1741ae35b42d903fa1ccc4ec5ae32469aaca47` 完成两个 native assets，因此为该 source、binary 与 assets 记录 W。后续相关修改必须重新完成收集；静态语法或 cross-build 仍不能冒充 W。

## Symbolic-link transaction 边界

Windows file transaction 通过 anchored reparse-point handle 检查 symbolic link；但重建 file 或 directory link 可能需要检查/移动既有对象时并不需要的创建权限。因此 Slipway 对所有 pre-existing Windows symbolic link 在第一次 transaction mutation 前返回 typed error，不会先移动 link、再依赖 symlink privilege 回滚。不可 Skip 的 policy test 无需创建 link 即证明该 fail-closed 决策；runner 有权限时的 native fixture 只作补充。

## Crash durability 与 ACL

Windows 会 flush journal/lock/projection files，但目前没有 Unix 同等级的 run-directory fsync。Doctor 明示 `level:"file_fsync_only"`、`directory_sync:false`、`limitation:"directory_fsync_unsupported"`；file flush 不等于新建/rename directory entry 的相同 crash guarantee。

Run dir 应使用 current-user ACL，但 inherited ACL、管理员、backup agent、恶意软件和同账户进程不在绝对保护承诺内。敏感仓库应检查 ACL、retention 和 backups。删除 run dir 只移除恢复能力，不是 secure erase、backup purge 或 key destruction。
