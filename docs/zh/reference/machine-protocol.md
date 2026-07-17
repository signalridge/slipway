# 机器协议

本页面向 adapter 和 integration author。普通用户应调用生成的宿主能力，并参考[命令说明](commands.md)。

当前 JSON contract version 为 **2**：

- [machine-protocol.schema.json](../../reference/v2/machine-protocol.schema.json) 定义公开命令、Action、Outcome、status、error 与 recovery 的形状。
- [source-envelope.schema.json](../../reference/v2/source-envelope.schema.json) 定义 GitHub source transport 形状。

可运行的完整宿主交互见[机器协议 v2 教程](../guides/machine-protocol-v2.md)。

Schema 定义 serialization shape。Runtime 还会验证 JSON Schema 无法完整表达的规则：embedded manifest syntax、ordering、hash、cross-field identity、idempotency、workspace state 与 filesystem safety。集成应验证 schema 并保留 CLI error，不要根据本文重新实现 Go validator。

## 进程边界

宿主调用模型、读取仓库、运行工具，并在用户要求时使用 GitHub 凭据。在这套协议交互中，Run/source 路径是本地确定性程序：验证消息、记录 Run、观察 Git 并返回下一操作，不调用模型或访问 GitHub。独立的公开 `doctor` 命令可能调用用户本机的 `gh` 做只读诊断。

宿主通常在每一步使用 JSON：

```text
slipway run --budget N --json --root ROOT [--no-review] [--source-file FILE] -- GOAL
slipway protocol submit --run RUN --action ACTION --root ROOT (--outcome-file FILE | --outcome-stdin)
slipway protocol answer --run RUN --action ACTION --root ROOT --text TEXT
slipway protocol answer --run RUN --action ACTION --root ROOT --confirm-destructive --scope-sha256 DIGEST [--text TEXT]
slipway protocol skip --run RUN --action ACTION --root ROOT
slipway protocol resume RUN --root ROOT [--budget N]
slipway protocol resume RUN --root ROOT (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate CANDIDATE) [--budget N]
slipway protocol material --run RUN --action ACTION --root ROOT --section KEY
```

协议操作是版本化宿主接口。它们有文档且在 help 中可见，但不是另一套 end-user command sequence：应通过 CLI 返回的结构化 `next` variant 来驱动。

## 启动 Run

Canonical invocation 将所有 flags 放在一个 `--` separator 前，将 literal goal 放在其后：

```bash
slipway run --budget 8 --json --root /absolute/worktree -- "one goal"
```

Ad-hoc Run 省略 source field。Issue-backed Run 提供私密临时 `--source-file`；CLI 只消费一次，之后不依赖该文件或 GitHub。

Start response 包含 Run state、初始 `orient` Action 与结构化 `next` operation。

## Action

Active Run 包含一个非 null Action：

```json
{
  "contract_version": 2,
  "run_id": "...",
  "action_id": "...",
  "kind": "orient",
  "goal": "...",
  "brief": "...",
  "context": "...",
  "remaining_budget": 7
}
```

`kind` 只能是 `orient`、`clarify`、`implement`、`review` 或 `summarize`。

Issue-backed Action 还包含：

- source、manifest 与 requirements revision；
- 有序且有界的 section catalog；
- 结构化 `protocol material` reader；
- 当前 Action 需要的 section key。

Requirements Markdown 不会复制进 `context`。Material reader 只对 current non-void Action 有效，并在返回内容前验证 digest、byte count 和 section revision。

这些 key 的版本化字段是 `requirements.required_for_action`。在 protocol v2 中，它等于 `requirements.sections` 中全部 key 的有序列表；宿主必须保持这种精确相等，不能自行推断更小的子集。

`context` 是 active answer 与之前 Outcome summary 的有界 projection，不是完整 journal、source、conversation 或 hidden model reasoning。

## Outcome

必须从且仅从一种输入提交 Outcome：

```text
slipway protocol submit --run RUN --action ACTION --root ROOT --outcome-file FILE
slipway protocol submit --run RUN --action ACTION --root ROOT --outcome-stdin
```

所有 public Outcome field 都必须存在。空集合仍发送 array；不适用 object branch 使用 JSON `null`：

```json
{
  "contract_version": 2,
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

`action_kind` 必须匹配 outstanding Action。Host status 为 `completed`、`needs_input`、`partial` 或 `error`；skip 是 CLI operation，不是 Outcome status。

- Orient 或 Clarify 最多建议一个 `clarify`、`implement` 或 `summarize` Action。
- 非暂停 Implement 使用 `implementation` branch，并报告实际文件、attempt、uncertainty 以及 test/type-check/build/lint activity 与 exit code。
- 非暂停 Review 使用 `review` branch 并报告 finding，不建议 repair work。
- Summary 和所有 `needs_input` Outcome 都没有 suggested Action。

### 合法 Outcome 组合

| Action | Host status | 必需 result branch | 允许的 pause | 允许的 suggestion |
| --- | --- | --- | --- | --- |
| Orient | `completed` / `partial` / `error` | `implementation=null`、`review=null` | 无 | 零或一个 Clarify、Implement 或 Summarize |
| Orient | `needs_input` | `implementation=null`、`review=null` | decision 或 environment | 无 |
| Clarify | `completed` / `error` | `implementation=null`、`review=null` | 无 | 零或一个 Clarify、Implement 或 Summarize |
| Clarify | `needs_input` | `implementation=null`、`review=null` | decision 或 environment | 无 |
| Implement | `completed` | `implementation.result=applied\|not_needed`、`review=null` | 无 | 无 |
| Implement | `partial` | `implementation.result=partial`、`review=null` | 无 | 无 |
| Implement | `error` | `implementation.result=unable`、`review=null` | 无 | 无 |
| Implement | `needs_input` | `implementation=null`、`review=null` | decision、destructive 或 environment | 无 |
| Review | `completed` | `review.result=no_findings_reported\|findings_reported`、`implementation=null` | 无 | 无 |
| Review | `partial` | `review.result=inconclusive`、`implementation=null` | 无 | 无 |
| Review | `error` | `review.result=error`、`implementation=null` | 无 | 无 |
| Summarize | `completed` / `error` | `implementation=null`、`review=null` | 无 | 无 |

Clarify 有意不允许 `partial`：一个 Action 只承载一个决定。Review 不能使用 `needs_input` 或建议 Implement；`not_run` 只属于 CLI 生成的 Review-skip projection。

`needs_input` Outcome 只有三种 pause reason：`decision_required`、`destructive_confirmation_required`、`environment_unavailable`。`budget_exhausted` 只能由 CLI 产生。

Destructive confirmation 只对精确 current Implement request 和 scope digest 有效。`yes` 等自然语言只是 feedback，不是授权。Action 变化、resume、scope 扩大或 mismatch 都会使 grant 失效。

Outcome input 上限 1 MiB，必须是有效 UTF-8，不能含 BOM、duplicate/unknown field 或 trailing data。

## 结构化 `next`

每个可以继续的 success 或 error 都包含 typed `next` object：

- `operation`：`action`、`answer`、`resume`、`start`、`command` 或 `none`；
- 初始 `workspace_identity`；
- 零个或多个带 `id`、`base_argv` 和 typed input 的 variant。

Input type 为 `string`、`path`、`enum` 或 `digest`。Consumer 选择一个 variant，按 schema 顺序将输入值作为独立 argv element 插入；不得解析或拼接 display command。

只有所有 required input 已解决的 variant 才能渲染为人类 shell command。POSIX、`cmd.exe` 和 PowerShell rendering 只用于显示；structured argv 才是 machine value。

当 Windows display command 含有 `cmd.exe` 无法安全保真的 expansion-sensitive `%` 或 `!` 值时，renderer 使用 PowerShell UTF-16LE `EncodedCommand` trampoline。这只改变可复制的显示形式；解码后的 process argv 必须与结构化 variant 逐字节等价。

Ended Run 使用 `operation: "none"` 和空 variant list。

## Source envelope

Source envelope 上限 16 MiB，通过 repository/Issue node ID 标识一个 `github.com` Issue。Valid Change 满足：

- body 第一个非空行为 `<!-- slipway-level: change/v2 -->`；
- 下一个非空 block 是唯一严格 `slipway-manifest` JSON fence；
- ordered manifest 有 5–64 个 section entry，并包含 outcome、requirements、acceptance examples、constraints、non-goals role；
- envelope 只且完整包含被引用 comments；
- 每个 comment 以精确 section marker 开头，并匹配声明 digest。

Normalized section 最大 256 KiB，完整 section payload 最大 4 MiB，manifest 最大 256 KiB。缺失、额外、重复、minimized、edited、oversized 或 hash-mismatched reference 会被拒绝。

Top-level source schema 有意允许 invalid refreshed head 使用空或仅含空白的 Issue/comment body，以及空 comments array。这样，缺失 marker、空 referenced section 与 digest mismatch 都可由 CLI 分类，而不是由宿主先拒绝 envelope 或收集无关讨论。Embedded manifest string 与 semantic digest check 由 runtime 验证，不由 top-level schema 单独完成。

CLI 保存稳定 identity、provenance、byte count、revision 和 content-addressed accepted material；不把 raw envelope、title label、source-file path 或 unreferenced comment 写入 journal。

## Refresh 与 candidate

Issue-backed resume 必须明确执行以下一种操作：

- 导入并比较 fresh envelope；
- 继续 pinned snapshot；
- 对精确 current candidate 选择 keep pinned 或 adopt valid candidate。

省略 source option 既不表示“unchanged”，也不会触发隐式网络访问。Issue identity 不同或 amendment parent requirements revision 不同会在不修改 Run 的情况下被拒绝。Candidate ID 与 choice 支持 stale-safe idempotency。

成功 resume 会按需 void stale outstanding work、重新验证 workspace，并通常返回 fresh Orient。省略 `--budget` 时保留大于零的 remaining budget，若为零则补充到 `max(initial_budget, 3)`；显式 `--budget N` 会替换为 `N`。Replacement 只在真正 resume Run 的 mutation 上生效。

## Workspace 与 Git observation

Workspace identity 包含 canonical worktree root、per-worktree Git directory 与 Git common directory。每次 load 或 mutation 都会重新发现并比较这些路径，再修改 journal。

Repository-wide `status` 是文件系统只读的例外：不创建 namespace 或 lock、不改权限、不修复 journal byte。其他 linked worktree 的 Run 显示为带 `workspace_foreign` 的 FirstEvent header stub，且不会在 owning worktree 外完整 replay。无法读取的本地 Run directory 在列表 JSON 的 `unavailable_runs` 中保留身份；指定读取时会区分损坏与不存在。

Git observation 保存 index、porcelain status 和 dirty path 的 hash 与有界 metadata，不保存文件内容。发现差异只证明 Run start 之后有变化，不能证明当前宿主造成了变化。

## Idempotency 与顺序

- Outcome idempotency 对原始 accepted input bytes 计算 hash；语义相同但 serialization 不同的 JSON 会 conflict。
- Answer、skip、resume 与 candidate operation 都绑定 current ID，并拒绝 stale/conflicting retry。
- 每个 Run 同时只有一个 writer，由平台 lock implementation 强制。
- Journal order 是 recovery record；`run.json` 可以重建。

## Error 与兼容性

Machine error 包含 `contract_version`、稳定 `code`、human `message`、`exit_code`，并在可恢复时带 structured recovery。Consumer 应按 code/version 分支，不能按 message text 分支。

`journal_record_too_large` 带有严格的 `context`、`size`、`limit` detail 字段；已知 Run ID 时还带该 Run 的只读 `status` recovery variant。拒绝过大的 record 不会结束 persistent Run，也不会使其失效。

未知 contract version 和 field 会被拒绝。Version 2 不承诺兼容此前未发布的开发格式；未来不兼容变更必须使用新的显式版本，不能加入静默 alias。
