# 命令参考

Slipway 只有七个公开命令：

规范语义见[中文产品契约](product-contract.md)与 [machine schema](../../reference/machine-protocol.schema.json)。工作流 issue-first 但不 issue-gated，详见 [Issue 工作流](issue-workflow.md)。

- `install`：安装六个宿主能力。
  首次安装无需 `--refresh`；已有 current ownership manifest 时，普通 `install` 不会改写托管文件，修复或更新需显式使用 `--refresh`。只有 marker 而缺少 current manifest 时返回 no-op 安全提示；非当前版本 manifest 会在 mutation 前失败。
- `uninstall`：仅删除仍保持原样的托管文件。
- `list`：列出宿主检测、安装、`needs_refresh` 与 capability 状态；非当前版本 manifest 只会把对应宿主降级为 `installed:false` 并附可选 `warning`，只读列表仍继续报告其他所有宿主且不修改文件系统。`needs_refresh` 表示 drift，不授权覆盖用户修改：缺失 pristine 内容可由 refresh 重建，修改后的文件或 sentinel 会一直保留，直到用户显式处理。
- `doctor`：诊断适配器和运行环境，不检查代码改动；非当前版本 manifest 报告 `adapter_manifest_unreadable`，不完整 current surface 报告 `needs_refresh`/warning。
- `run "<goal>" [--root ROOT] [--source-file FILE] [--budget N] [--no-review] --json`：启动软自动驾驶；Action budget 默认是 `8`，显式值必须在 `1..1000`。宿主生成和 machine `start` 固定采用安全规范形式 `slipway run --budget N --json --root ABSOLUTE_ROOT [--no-review] [--source-file FILE] -- GOAL`，所有 flags 都在唯一 `--` 前；公共 Cobra 命令仍接受用户输入的等价合法排列。带 source 时只读取一次严格 Source Bundle v2 envelope，把 manifest 引用章节固定为本地内容寻址 material，并仅持久化 accepted normalized materials 与 catalog/provenance。
- `status [run-id] [--root ROOT] [--json]`：列出或查看 Run。无 ID 时列出当前仓库的所有 Run：当前 canonical workspace 的 Run 完整 replay；其他 linked worktree 的 Run 仅以 `FirstEvent` header stub 出现，JSON 标记 `workspace_foreign:true`，人类输出标记 `foreign=true` 并显示所属 workspace。Foreign stub 只用于发现；Load、resume 与 mutation 仍必须从原 workspace 执行。
- `stop [run-id] [--root ROOT] [--json]`：停止但保留恢复日志。

```bash
slipway run --budget 8 --json --root /绝对路径/仓库 -- "小型私密修复"
slipway run --budget 8 --json --root /绝对路径/仓库 --source-file /安全临时目录/change-envelope.json -- "实施有界 Change"
```

导入 source 前警告 accepted Requirements、goal、answers 与 command summaries 可能敏感。宿主只临时获取精确 Issue body 与 manifest 引用 raw comment fields，raw envelope 仅交给 CLI consume，随后删除临时文件；journal 只保留 accepted normalized materials 与 bounded catalog/provenance，不保存 raw body/comments/token/env。详见[运行日志与隐私](../explanation/runs-and-privacy.md)。

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

`install/uninstall --json` 固定返回 `{contract_version,hosts,transaction_outcome,written,removed,preserved,recovery_artifacts,warnings}`，所有数组即使为空也存在。`transaction_outcome` 为 `committed|rolled_back|not_committed|ambiguous`；`preserved` 只表示普通用户修改，事务/quarantine 恢复路径单列于 `recovery_artifacts`，非 committed 结果不宣称计划的 `written`/`removed`。`list --json` 返回 `{contract_version,hosts:[...]}`；无 ID 的 `status --json` 返回 `{contract_version,runs:[...]}`，空列表也是 versioned object。有 ID 的 status 保持扁平 Run projection，并在顶层包含必填 `contract_version` 与 fresh `next`。所有错误固定包含 `contract_version`。

`doctor --json` 返回 `{contract_version,checks:[{code,status,host_id,name,detail}]}`，status 仅为 `ok|warning|error`。Repository/adapter 稳定 code 为 `repository_ok`、`adapter_manifest_unreadable`、`adapter_not_detected`、`adapter_not_installed`、`adapter_refresh_required`、`adapter_modified`、`adapter_healthy`。GitHub 稳定 code 为 `github_cli_unavailable|github_cli_version_unknown|github_cli_rest_fallback_required|github_cli_compatible`、`github_auth_unavailable|github_auth_available`、`github_issue_permissions_ok|github_issue_permissions_limited|github_issue_permissions_unknown`；`gh <2.94.0` 需要官方 REST fallback。Legacy code 为 `legacy_runtime_residue`、`legacy_cache_residue`、`legacy_scope_root_residue`、`legacy_scopes_residue`、`legacy_locks_residue`、`legacy_processes_residue`、`legacy_repair_backups_residue` 与 `legacy_unknown_residue`。Doctor 只检查顶层 metadata，不读取、迁移或删除内容；GitHub/legacy warning 不阻塞 ad-hoc Run，且单独出现时 doctor 仍成功退出。
