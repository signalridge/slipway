# 机器协议

当前 `contract_version` 为 **1**。规范 JSON Schema 见 [`machine-protocol.schema.json`](../../reference/machine-protocol.schema.json)。未知版本、未知字段、重复 key、无效 UTF-8、BOM、尾随数据和超过 1 MiB 的 Outcome 都会被拒绝。

启动与恢复命令固定为：

```text
slipway run "<goal>" [--root ROOT] [--source-file FILE] [--budget N] [--no-review] --json
slipway run resume RUN [--budget N]
slipway run resume RUN (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
```

新 Run 的 `--source-file` 可选；提供时 CLI 对不超过 256 KiB 的普通 no-follow 文件只打开一次，先严格验证 JSON/identity，再分类 body。新 Run 遇到非法 marker/section 会拒绝且不创建 journal。持久化内容只有 `pinned_source` 的 identity/projection、两个 revision 和五段 accepted requirements；raw body、labels、timestamps、comments 与文件路径永不进入 journal。

第一种 resume 只用于 ad-hoc Run，且拒绝所有 source flags。Issue-bound Run 没有 candidate 时必须且只能使用 fresh `--source-file` 或显式 `--use-pinned-source`；有 candidate 时只能提交完全匹配的 `--source-choice pinned|adopt --candidate ID`，invalid candidate 只能 pinned。跨 Issue 在任何 mutation 前拒绝；repository/number/URL transfer 会记录旧 URL alias，但仍继续 amendment 比较。

相同、projection-only 或 non-material refresh 会原子 void 旧 Action/queue/authorization 并 fresh Orient。Requirements 变化或 invalid body 会持久化 path-free current candidate、暂停为 `decision_required`，且不应用该调用的 budget，响应明确返回 `budget_applied:false`。`adopt` 会把旧 Requirements 派生 answer 从 active context 移除但保留历史记录；相同 `(candidate_id,choice)` 重试不产生事件或新 Action。显式 budget 必须为 1..1000；省略时保留正余额，耗尽时恢复为 `max(initial,3)`，随后 Orient 消耗一条。状态输出安全暴露 `pinned_source`、`source_candidate`、`resume_operation` 与 `budget_applied`，不会暴露 source-file 路径。

错误、pause、stop、ended 与完整 status 使用结构化 `next={operation,workspace_identity,variants}`，不再使用 shell string。`next.workspace_identity` 是稳定的小写 `sha256:<64 hex>` ID，不是路径；每个 variant 含 `id`、完整固定 `base_argv` 和非 null `inputs`，并在唯一的 `--root` 后保留 Run 原始 canonical absolute worktree root。输入类型仅为 `string|path|enum|digest`，按 schema 顺序把 `flag` 与未经 shell 解释的原值作为两个 argv element 追加。必填输入未解析时只显示类型说明，不生成伪命令；无输入或已解析 argv 才在 POSIX/cmd.exe/PowerShell 边缘渲染，且渲染文本不入 journal。

Run 初始化会持久化 version 1 workspace identity：canonical absolute worktree root、该 worktree 独有的 Git directory、Git common directory，以及对这三个路径做 length-framed SHA-256 得到的 ID。Linked worktree 因 Git directory 不同而身份不同。每次 Load/status recovery，以及 submit/answer/skip/stop/resume mutation 前，CLI 都不用 shell 重新发现并逐字段比较；root 被复用、linked worktree 错误或 Git metadata 被移动/重定向时，在写 journal 前返回 `workspace_identity_mismatch`、`next.operation:none` 和空 variants。

Version 1 Git observation 包含 HEAD、对 `git ls-files --stage -z` 原始 bytes 的 `index_fingerprint`、对 `git status --porcelain=v2 -z --untracked-files=all` 原始 bytes 的 `status_fingerprint`、sorted non-null dirty paths/path observations，以及覆盖所有结构字段的 snapshot hash。ordinary、rename/copy（含 origin）、unmerged 与 untracked path 都保留空格和 Unicode。每个 path 记录 category/state、已知 size 与安全的 content SHA-256；regular file 最多读取并 hash 16 MiB，symlink 不跟随且只 hash link target，missing/non-regular/unreadable/oversize 明确记录且不使整体失败。Raw content 永不进入 journal；oversize 文件内部的同尺寸变化超出该 bounded observer 的能力范围。`initial_git` 在 replay 中不可变。

Active Action 的 variants 为 `submit-outcome-file|submit-outcome-stdin|skip-action`；decision 为 `answer-decision`；destructive 为固定当前 digest 的 `confirm-destructive` 与必填 text 的 `decline-or-feedback`；恢复为 `resume-ad-hoc`、`refresh-source|use-pinned-source` 或 candidate 的 `keep-pinned|adopt`（invalid candidate 仅 keep）。ended 使用 `operation:none` 和空数组。Outcome submit 必须显式二选一 `--outcome-file FILE|--outcome-stdin`，幂等依据原始 payload bytes 的 SHA-256，而非重编码后的语义相等；answer 使用 action/text/confirm/scope 的 canonical digest。

## Journal commit 错误

`.git/slipway/runs/<run-id>/journal.jsonl` 是唯一恢复权威，`run.json` 只是可替换 projection。存储 mutation 的机器错误稳定包含 `details.phase`、`details.committed`、`details.projection_stale`、`details.namespace_detached` 和 `details.ambiguous`。`mutation_committed_projection_stale` 表示 journal event 已 fsync、但 projection 后续步骤失败；`mutation_outcome_ambiguous` 表示 inode 已写、但 durability 或 namespace membership 无法证明。二者都返回 `next.operation:"none"`：恢复前必须 inspect/replay journal，绝不能盲目重试。写入前失败使用 `mutation_not_committed`。

支持的类 Unix 系统提供 `file_and_directory_fsync`。Windows 稳定报告 `file_fsync_only`、`directory_sync:false` 与 limitation `directory_fsync_unsupported`；文件内容会 fsync，但不能宣称新建或 rename 的目录项具有 crash durability。

Active 响应直接返回 Action，固定字段为 `contract_version`、`run_id`、`action_id`、`kind`、`goal`、`brief`、`context`、`remaining_budget`。`kind` 只能是 `orient|clarify|implement|review|summarize`。Ad-hoc Action 必须省略 `source` 与 `requirements`；issue-bound Action 必须同时携带两者，不能发送 `null`。`context` 上限 128 KiB，`brief` 上限 8 KiB，完整编码 Action 上限 256 KiB。

Requirements 始终作为独立且不截断的字段，绝不复制进 context。Context 只含 active confirmed decisions 与既往 Outcome projection；选择优先级固定为：最新 active decision、其余 active decisions（新到旧）、最新 Outcome summary 加该 Outcome 的 known issues、其余 Outcomes（新到旧）。Superseded/旧 requirements revision 的 decision 被保留在历史但排除；结构化 destructive confirmation attestation 永远不是产品 decision；只有独立 decline-or-feedback 分支的非空文本可成为 active feedback。每个 candidate 把 CRLF/CR 归一成 LF 并验证 UTF-8，选择后在 `Decisions:`、`Recent outcome:`、`Earlier outcomes:` class 内按时间正序渲染。空间不足时只在 code-point 边界截断并追加精确 marker `...[truncated original_bytes=N sha256=HEX]`；每类遗漏用稳定 `[omitted CLASS: N]` 记录。相同 journal replay 产生 byte-identical context，永不超过 128 KiB；未截断 Requirements 若令 Action 总体超过 256 KiB，则返回 `action_too_large`。

只有结构化确认后的 Implement 可携带 `destructive_authorization`。`--confirm-destructive` 是 trusted host 对当前用户确认的 attestation，不是不可伪造的人类在场证明；拥有 shell 权限的恶意进程可以伪造 flag。破坏目标必须非空、无重复，并按 `(kind,value)` 字节序排列；kind 只能是 `path|git_ref|external_resource|data_domain`。CLI 会对固定的 canonical scope 重新计算 SHA-256。任意自然语言文本（包括 “yes”）都不授予破坏权限：它只记录拒绝/反馈、清除 request/grant 并签发不带授权的 fresh Orient。只有 `--confirm-destructive --scope-sha256 DIGEST` 与当前 request 完全匹配时，才签发一条逐字段复制 scope 的 fresh Implement；target/impact 扩大必须重新请求。

宿主 Outcome 必须显式包含以下全部字段：

```json
{
  "contract_version": 1,
  "action_id": "...",
  "action_kind": "orient",
  "status": "completed",
  "summary": "observed facts",
  "observations": [],
  "known_issues": [],
  "suggested_actions": [],
  "pause": null,
  "implementation": null,
  "review": null
}
```

`action_kind` 必须显式存在并与当前 Action 的 `kind` 完全一致；缺失、未知或不匹配都会被拒绝，不做推断或旧格式回退。

数组不得省略或发送 `null`。宿主 status 只能是 `completed|needs_input|partial|error`；`skipped` 仅是 CLI `run skip` 事件。`needs_input` 必须有 `pause`，其他 status 必须 `pause:null`；宿主 pause reason 只能是 `decision_required|destructive_confirmation_required|environment_unavailable`，`budget_exhausted` 仅由 CLI 产生。破坏请求只允许出现在 Implement 的破坏性 pause 中。

Orient 支持 `completed|partial|error|needs_input`；Clarify 支持 `completed|error|needs_input`，不支持 `partial`。二者的非暂停 Outcome 最多建议一个 `clarify|implement|summarize`；无建议时确定性进入 Summary。所有 `needs_input`、Implement、Review 与 Summary 的建议数组必须为空。

Implement 的 `implementation` 固定包含 `result`、`files_changed`、`activities`、`uncertainties`、正整数 `attempts`：

- `completed` 对应 `applied|not_needed`；
- `partial` 对应 `partial`；
- `error` 对应 `unable`；
- `needs_input` 对应 `implementation:null`。

activity kind 只能是 `test|typecheck|build|lint`。零 activity 合法，最终报告固定写出：

```text
No test, typecheck, build, or lint activity was reported.
```

Review 的 `review` 固定包含 `result`、`findings`、`uncertainties`：`completed` 对应 `no_findings_reported|findings_reported`，`partial` 对应 `inconclusive`，`error` 对应 `error`。`findings_reported` 至少一个 finding，`no_findings_reported` 必须为零。Review 不得 `needs_input`、不得建议或自动安排修复；`not_run` 仅是 CLI review-skip projection，宿主提交会被拒绝。

路由固定为：有效建议优先；Orient/Clarify 无建议进入 Summary；只要 CLI 观察到代码差异且 Review 已启用，Implement 无论报告 `applied|not_needed|partial|unable` 都进入 Review，否则进入 Summary；每个合法 Review 都进入 Summary；Summary 结束 Run。Skip 同样 diff-first：Orient/Clarify/Implement skip 后有 start-to-current diff 且启用 review 则进入 Review，否则 Summary；Review skip 进入 Summary；Summary skip 写最小事实报告并结束。所有 skip 清除 destructive state。activity 退出码与 Review findings 只进入报告，不控制路由。

任何 start-to-current 差异都会记录事实 `observed_since_start` 和 `attribution_uncertainty`：并发用户编辑、另一个 Run 或工具都可能有贡献，CLI 不把差异归因给宿主或某个 Run。两种方向都只记为中性 report discrepancy：报告 `applied|partial` 但未观察到 diff，以及报告 `not_needed|unable` 但观察到 diff；路由仍然 diff-first。Review brief 与最终 Summary 会保留归因不确定性和 Run 开始时已有 dirty path 的结构化观察。

## Public JSON envelope 与 Doctor advisory

每个 JSON success/error 都是顶层 `contract_version:1` object。Install/uninstall 固定为 `{contract_version,hosts,written,removed,preserved,warnings}` 且数组不省略；list 为 `{contract_version,hosts:[...]}`；无 ID status 为 `{contract_version,runs:[...]}`，空值也是 `{"contract_version":1,"runs":[]}`。单 Run status 保持 flat Run projection，顶层必须有 `contract_version` 和 fresh `next`。Doctor 为 `{contract_version,checks:[{code,status,host_id,name,detail}]}`，所有 object 在规范 schema 中均 `additionalProperties:false`。Repository/adapter code 为 `repository_ok`、`adapter_manifest_unreadable`、`adapter_not_detected`、`adapter_not_installed`、`adapter_legacy_manifest`、`adapter_refresh_required`、`adapter_modified`、`adapter_healthy`。

GitHub code 为 `github_cli_unavailable|github_cli_version_unknown|github_cli_rest_fallback_required|github_cli_compatible`、`github_auth_unavailable|github_auth_available` 和 `github_issue_permissions_ok|github_issue_permissions_limited|github_issue_permissions_unknown`。命令有 timeout，不经 shell；`gh <2.94.0` 需要官方 REST fallback；报告不包含 raw auth/API output 或 token。

Legacy code 为 `legacy_runtime_residue`、`legacy_cache_residue`、`legacy_scope_root_residue`、`legacy_scopes_residue`、`legacy_locks_residue`、`legacy_processes_residue`、`legacy_repair_backups_residue`、`legacy_unknown_residue`。Doctor 不打开 runstore，只 Lstat Git common dir 下的顶层名字，排除当前 `runs`，不读、迁移或删除 residue。建议停止旧 binary、备份后按需手工清理；这些 warning 不阻塞 doctor，也不影响 ad-hoc Run health。
