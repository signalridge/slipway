# Windows rendering 与 durability

规范语义见[中文产品契约](product-contract.md)和版本化 [machine schema](../../reference/machine-protocol.schema.json)。

## 结构化 argv 是权威

Recovery/pause 返回 `next.operation`、workspace identity 与 typed variants。每个 variant 包含完整 `base_argv` 和有序 inputs；展开时按 schema 顺序追加 input flag 与**一个原样、未引用的 argv value**。不得从 display prose 重建命令，也不得产生 `<answer>`/`<file>` 占位伪命令。

Slipway 分别渲染 POSIX、`cmd.exe` 与 PowerShell display command；rendering 只用于展示/复制，不写 journal、不改变 machine operation。Root、Issue URL、source/outcome file、answer、candidate recovery 中的空格、引号、Unicode、CR/LF、`%`、`!`、`&`、`^` 必须保持原 argv。由于 cmd 会展开 `%`/`!`，renderer 可使用 PowerShell UTF-16LE `EncodedCommand` 或等价安全 argv 路径。必须在 `cmd.exe /v:on` 与 PowerShell native 进程实际捕获 argv；Linux cross-build 只证明能编译，不是 W evidence。

CI matrix 构建 `slipway.exe` 后会执行以下两个 native 入口，本地 Windows 也可运行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tests\acceptance\windows\native-powershell.ps1 -SlipwayExe C:\path\slipway.exe
cmd.exe /d /v:on /c tests\acceptance\windows\native-cmd.cmd C:\path\slipway.exe
```

资产覆盖 doctor、initial Orient、issue source、Outcome file/stdin（平台支持处）、decision answer、ad-hoc resume、current-candidate keep/adopt 和特殊 argv。Workflow wiring 只是 W collector；必须有同一次已完成的 `windows-latest` 运行记录两个脚本都成功后才能记录 W。本次本地修改不宣称已取得该运行；静态语法或 cross-build 不能冒充 W。

## Crash durability 与 ACL

Windows 会 flush journal/lock/projection files，但目前没有 Unix 同等级的 run-directory fsync。Doctor 明示 `level:"file_fsync_only"`、`directory_sync:false`、`limitation:"directory_fsync_unsupported"`；file flush 不等于新建/rename directory entry 的相同 crash guarantee。

Run dir 应使用 current-user ACL，但 inherited ACL、管理员、backup agent、恶意软件和同账户进程不在绝对保护承诺内。敏感仓库应检查 ACL、retention 和 backups。删除 run dir 只移除恢复能力，不是 secure erase、backup purge 或 key destruction。
