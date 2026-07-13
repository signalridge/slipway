# 命令参考

Slipway 只有七个公开命令：

规范语义见[中文产品契约](product-contract.md)与 [machine schema](../../reference/machine-protocol.schema.json)。工作流 issue-first 但不 issue-gated，详见 [Issue 工作流](issue-workflow.md)。

- `install`：安装六个宿主能力。
  首次安装无需 `--refresh`；已有 current ownership manifest 时，普通 `install` 不会改写托管文件，修复或更新需显式使用 `--refresh`。只有 marker 而缺少 current manifest 时返回 no-op 安全提示；非当前版本 manifest 会在 mutation 前失败。
- `uninstall`：仅删除仍保持原样的托管文件。
- `list`：列出宿主检测、安装、`needs_refresh` 与 capability 状态；非当前版本 manifest 会 fail closed。
- `doctor`：诊断适配器和运行环境，不检查代码改动；非当前版本 manifest 报告 `adapter_manifest_unreadable`，不完整 current surface 报告 `needs_refresh`/warning。
- `run "<goal>" [--root ROOT] [--source-file FILE] [--budget N] [--no-review] --json`：启动软自动驾驶；带 source 时只读取一次严格 Source Bundle v2 envelope，把 manifest 引用章节固定为本地内容寻址 material，并仅在 journal 中持久化 catalog/provenance。
- `status [run-id] [--root ROOT] [--json]`：列出或查看 run。
- `stop [run-id] [--root ROOT] [--json]`：停止但保留恢复日志。

```bash
slipway run "小型私密修复" --json
slipway run "实施有界 Change" --source-file /安全临时目录/change-envelope.json --json
```

导入 source 前警告 accepted Requirements、goal、answers 与 command summaries 可能敏感；raw body/comments/token/env 不进入 journal。详见[运行日志与隐私](../explanation/runs-and-privacy.md)。

宿主使用隐藏的协议命令：

```text
slipway _machine submit --run ID --action ID (--outcome-file FILE | --outcome-stdin)
slipway _machine answer --run ID --action ID --text TEXT
slipway _machine answer --run ID --action ID --confirm-destructive --scope-sha256 DIGEST [--text TEXT]
slipway _machine skip --run ID --action ID
slipway _machine resume ID [--budget N]
slipway _machine resume ID (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
slipway _machine material --run ID --action ID --section KEY
```

第一种 resume 仅适用于 ad-hoc Run。Issue-bound resume 必须且只能选择 fresh `--source-file`、显式 `--use-pinned-source` 或按当前 candidate ID 执行 `pinned|adopt` 之一。Skip 不需要理由。暂停、错误与 status JSON 包含带原始绝对 `--root` 的结构化 `next`；只有无未解析必填输入的 argv 才渲染成人类命令。

`install/uninstall --json` 固定返回 `{contract_version,hosts,written,removed,preserved,warnings}`，所有数组即使为空也存在；`list --json` 返回 `{contract_version,hosts:[...]}`；无 ID 的 `status --json` 返回 `{contract_version,runs:[...]}`，空列表也是 versioned object。有 ID 的 status 保持扁平 Run projection，并在顶层包含必填 `contract_version` 与 fresh `next`。所有错误固定包含 `contract_version`。

`doctor --json` 返回 `{contract_version,checks:[{code,status,host_id,name,detail}]}`，status 仅为 `ok|warning|error`。Repository/adapter 稳定 code 为 `repository_ok`、`adapter_manifest_unreadable`、`adapter_not_detected`、`adapter_not_installed`、`adapter_refresh_required`、`adapter_modified`、`adapter_healthy`。GitHub 稳定 code 为 `github_cli_unavailable|github_cli_version_unknown|github_cli_rest_fallback_required|github_cli_compatible`、`github_auth_unavailable|github_auth_available`、`github_issue_permissions_ok|github_issue_permissions_limited|github_issue_permissions_unknown`；`gh <2.94.0` 需要官方 REST fallback。Legacy code 为 `legacy_runtime_residue`、`legacy_cache_residue`、`legacy_scope_root_residue`、`legacy_scopes_residue`、`legacy_locks_residue`、`legacy_processes_residue`、`legacy_repair_backups_residue` 与 `legacy_unknown_residue`。Doctor 只检查顶层 metadata，不读取、迁移或删除内容；GitHub/legacy warning 不阻塞 ad-hoc Run，且单独出现时 doctor 仍成功退出。
