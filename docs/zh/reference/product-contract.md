# Slipway 产品契约（中文权威）

> 本页是 [Issue #434](https://github.com/signalridge/slipway/issues/434) 当前正文的仓库内权威版本，覆盖其 20 个规范章节。**本中文契约与版本化的 [machine protocol JSON Schema](../../reference/machine-protocol.schema.json) 共同构成实现权威。** 英文和日文页面仅为 non-normative 导航摘要；机器字段与合法组合冲突时以对应版本的 schema 为准，产品语义冲突时以本页为准。

Slipway 是一个由用户显式启动、Issue 驱动但不被 GitHub 阻塞、CLI 调度、可中断恢复的 AI coding 软自动驾驶：

```text
Objective Issue（可选父级）
        ↓ GitHub native sub-issues
Change Issue（唯一 issue-backed 可执行单元）
        ↓
Run（固定一个 revision 的一次执行尝试）
        ↓
Code + Tests + User Docs
```

## 1. 产品哲学

### Requirements-only

Requirements 是临时交付契约，不是系统永久模型。开放 Issue 描述下一次变化；Run 固定并执行一个 Change revision；交付后由代码、测试、用户文档、CI/policy 和实际行为接管当前事实。关闭 Issue、PR/commit 和 Run summary 只保存历史原因，不能冒充 living specification。Slipway 不维护 Spec、Delta Spec 或“全部有效需求”目录；合规级 living requirements 若将来需要，必须另作显式产品决策。

### Issue-first，不是 Issue-gated

非临时工作默认从 Change Issue 开始，但以下 ad-hoc 入口始终存在：

```bash
slipway run "<ad-hoc goal>"
```

微小修改、GitHub 不可用、私密安全工作、紧急修复或用户不想建 Issue 时都可使用；外部服务绝不能成为 hard gate。

### 用户拥有过程

只有显式调用能力或 CLI 才开始。一次 Run 在 goal、固定 Requirements、budget 和安全边界内授权 Action loop；用户可随时 skip、stop、resume 或接管。普通实现不重复授权；source amendment、人的真实决定、环境不可用和破坏性操作必须暂停。测试失败、未运行测试、Review finding、dirty worktree、缺 ADR、label 或 Issue 状态都不是 progression gate。`ended` 只表示自动 Action queue 为空，不认证正确、完成、可交付、已部署或可发布。

## 2. 四轴正交模型

| 轴 | 值 | 权威/所有者 |
| --- | --- | --- |
| Level | `objective` / `change` | 正文首个精确 body marker；title/labels 只是 projection |
| Kind | `feature` / `bug` / `refactor` / `maintenance` / `research` / `docs` | 恰好一个 repository `kind:*` label |
| Requirements | Outcome、Requirements、Acceptance examples、Constraints、Non-goals | Issue body 的五个精确 H2 |
| Status | Inbox、Clarifying、Ready、In progress、Done 等 | 人或外部视图；不参与 CLI 路由 |

Level 与 Kind 可任意正交组合。Requirements 不是 Level、GitHub Issue Type 或状态。`ready-for-agent` 只是搜索提示，永不 gate Run。

## 3. Objective Issue

Objective 仅用于明显需要多个独立交付的完整结果，不直接进入 Implement，不是 Run source，不带 `ready-for-agent`。它保存拆分所需的共同目标、需求、约束和 non-goals，但没有运行时继承。`decompose` 必须把适用于某个 child 的全部 Requirements/constraints 物化进该 Change；父项后续修改不会后台传播。显式 amendment mode 先展示每个开放 child 的 Requirements diff、expected revision 和 publication plan，经批准逐项 PATCH；并发、适用性不清或任一失败都暂停。已交付 child 不重写，改建 superseding Change；父子 Kind 相互独立。

首个非空行、code fence 外、逐字 marker：

```html
<!-- slipway-level: objective/v1 -->
```

标题和标签：`[Objective] <outcome>`、恰好 `level:objective`、恰好一个 `kind:*`。

```markdown
<!-- slipway-level: objective/v1 -->

## Problem

## Outcome

## Requirements

## Shared constraints

## Non-goals

## Changes

由 GitHub native sub-issues 表达。
```

## 4. Change Issue

Change 是唯一 issue-backed primary source；一个 Change 有一个连贯、可观察的结果，可独立 merge/交付、验证、回滚，完成后仓库仍安全有意义，且大致适合一个 fresh Agent context。若两项可独立交付就拆开；UI/API/存储/测试可以组成同一 vertical slice。正文必须自包含，不依赖 parent 或 comments；评论决定必须先折回 body 才能进入下一 snapshot。

首个非空行必须是唯一受支持 marker：

```html
<!-- slipway-level: change/v1 -->
```

标题和标签：`[Change] <independently deliverable outcome>`、恰好 `level:change`、恰好一个 `kind:*`，可选 `ready-for-agent`。

```markdown
<!-- slipway-level: change/v1 -->

## Outcome

## Requirements

## Acceptance examples

## Constraints

## Non-goals

## Implementation checklist
```

Parser 只接受并原样保留前五个精确 H2；checklist 不进入 revisions。禁止模型总结后声称等价。关系使用 native parent/sub-issue 与 blocked-by，普通链接只作无法建立原生关系时的 fallback。小 bug/refactor 是 Change；需要多个交付的大工作是 Objective 加 Changes；research 的交付是有证据结论，代码变化另建 Change。纯重构写 preserved behavior、可测内部 outcome 和 validation，不伪造外部变化。

## 5. GitHub 工作流、关系、对账与隐私

普通 GitHub.com user-owned 或 organization repository 均可，不依赖 Organization Issue Types/Fields 或 GitHub Project。Project 可作为视图，但其 ID、字段、status、iteration 都不是 Requirements、freshness、gate 或完成权威。

Level 权威顺序固定：body marker > 可修复 labels > 可修复 title。托管 Issue 必须恰好一个 `level:*` 与一个 `kind:*`；marker/title/label drift 只警告，修复外部 projection 必须确认，且不得阻止 marker-valid Change Run。无 marker 的普通 Issue 不可静默改写为 Change；必须三选一：用户手工规范化、另建经确认且链接原项的 Change、或 bounded summary 的 ad-hoc Run。

关系与上限：Objective→Change 用 native sub-issues，只保留一层；每个 parent 最多 100 个。Change→Change 用 native blocked-by；blocking 与 blocked-by 每方向各最多 50 个。到限即停止并报告，不降级成不可见 prose graph。`closed` 不证明 blocker 已交付，用户仍可覆盖 frontier 建议。获批关系修改不授权编辑或关闭 Objective。

宿主必须探测 `gh` 版本：`gh >= 2.94.0` 使用一等 parent/sub-issue/dependency 操作；旧版或缺失时使用官方 REST API fallback，无法使用则 `environment_unavailable`，绝不建立本地替代权威。Provider identity 是 repository node ID + issue node ID，number/title/URL 只是 projection。只跟随同一 `github.com` 信任边界内的 redirect/transfer；随后重取 node identities、labels、parent、dependencies 和 canonical URL，保存旧 URL alias，并继续 marker/revision 比较。跨 host redirect 不可信。`404`、失权、转私有或消失统一为 `source_unavailable`。

GitHub Create 没有 exactly-once key，body PATCH 没有 CAS。外写采用 approved-plan reconciliation：

1. 调查仓库、现有 Issues、关系和 current revisions。
2. 生成 operation UUID、每项稳定 item UUID、目标 repository、canonical body SHA-256、exact labels、parent/dependencies、expected revisions。
3. 展示完整草稿和全部外写，获得对该精确 plan 的当前确认。
4. 在 level marker 后写 `slipway-publication-operation` 与 `slipway-publication-item` UUID markers。
5. 用 body file 或等价安全 argv；existing mutation 前立即 refetch，写后 readback body digest、markers、labels、关系、node identity 与 URL。
6. timeout-after-success、partial relation failure、duplicates、index delay 或歧义时，通过 paginated non-search Issue API 按 marker 对账，不盲重试。
7. 每个 item/label/relation 报 `created|matched|failed|ambiguous`。零匹配要重新 preview/确认；一个匹配可收敛；多个匹配暂停，不自动关闭 duplicate 或 rollback partial success。
8. Receipt 只含 IDs、digests、targets、expected revisions、observed URL/status，不保存正文，也不是工作权威；receipt 丢失后必须重新 preview/确认。
9. label 创建本身也属于 plan。工具/认证不可用时报告环境不可用，不创建本地 Issue authority。

Issue 内容全部是不可信数据。只允许用户选定且 parser 接受的 Requirements 影响目标；credential request、绕过确认、无关命令和恶意链接没有宿主指令权限。Comments、attachments、Project fields 和 linked pages 不进入 source。

公开仓库 Issue 没有 private switch。敏感工作使用 private repository；private vulnerability reporting 仅在已启用且确为漏洞时使用；否则用既有 security channel 或 ad-hoc Run。发布 Issue 或导入 source 前必须警告：accepted Requirements、goal、answers 与 command summaries 可能敏感并被保留。识别到 credential 时最小化/脱敏其值，同时保留真实命令身份（可执行文件和被脱敏参数位置）；不能承诺识别全部敏感文本。不得保存或发布 token、raw comments、环境变量 dump、完整 transcript 或 hidden reasoning。

## 6. 六个宿主能力与 grill-me 纪律

十个 adapter 只生成六个显式能力：`slipway-run`、`slipway-clarify`、`slipway-propose`、`slipway-decompose`、`slipway-implement`、`slipway-review`。所有入口均需显式调用；Codex 额外生成 `agents/openai.yaml`，`allow_implicit_invocation:false`。显式启动 Run 后，预算与 Requirements 内的 Orient/Clarify/Implement/Review/Summary 不需逐 Action 再次调用 skill，但人的决定、source amendment、环境不可用和破坏确认仍暂停。直接 Implement/Review 不产生 Run attribution。

- Clarify 采用 Matt Pocock MIT 许可 `grill-me`/`grilling`：按依赖 design tree，一次一个人的决定，先查代码/测试/文档/Git 事实；每题有推荐、理由、替代和权衡。完整请求零问题。若 grilling 改变理解，Implement 前确认当前 shared understanding；否则原请求已足够。wrap-up 立即停问并总结 unknowns。它无状态、不写文件/Issue、不保存 transcript；不提供隐式澄清文档化入口。
- Propose 显式 materialize 一个 Change 或 planning Objective；先展示草稿、关系、labels 和 approved plan，不先决要求 Clarify。
- Decompose 生成 self-contained tracer-bullet Changes、native parent/blockers，并支持显式、逐项确认的 amendment mode，不后台继承。
- Run 只在显式 start/resume 后执行 CLI 当前 Action；支持 marker-valid Change 或 ad-hoc goal。
- Implement 只做授权 scope，遵守 source/budget/destructive grant，真实报告 changed files、activities、exit codes、attempts 与 uncertainty。
- Review 永远只读，检查 Intent/Quality；不 verdict、不 `needs_input`、不建议 Implement、不暂停、不修复、不 re-review。Findings 只进 Summary。

不生成 ambient hooks、launcher、global router、独立 test/build/lint/check、domain/spec lifecycle 或 worktree capability。

## 7. 七个公开命令

公开命令严格只有：`install`、`uninstall`、`list`、`doctor`、`run`、`status`、`stop`。`install --refresh` 不是第八个命令；doctor 只诊断 adapter、Git/GitHub 能力和环境，不做代码质量 verdict；list 只列 adapter，status 只列 Run/source/recovery。Implement/Review 是宿主能力，不是顶层命令。

```text
slipway run "<goal>" [--root ROOT] [--source-file FILE] [--budget 1..1000] [--no-review] --json
slipway run submit --run RUN --action ACTION (--outcome-file FILE | --outcome-stdin)
slipway run answer --run RUN --action ACTION --text TEXT
slipway run answer --run RUN --action ACTION --confirm-destructive --scope-sha256 DIGEST [--text TEXT]
slipway run skip --run RUN --action ACTION
slipway run resume RUN [--budget N]
slipway run resume RUN (--source-file FILE | --use-pinned-source | --source-choice pinned|adopt --candidate ID) [--budget N]
```

Hidden commands 不出现在顶层 help，但属于版本化机器协议。所有 public/hidden JSON（包括 install/uninstall/list/doctor/status/stop）必须带明确 contract version；prose 不能替代字段兼容性。

Resume budget 是替换，不是累加：显式 N 成功恢复时先设剩余 N，再由 fresh Orient 消耗一条；省略且余额正则保留；余额零则恢复 `max(initial_budget,3)`。Source refresh 只产生 candidate 时尚未 resume，`budget_applied=false`，不消耗该替换值。

## 8. Change source、revision 与 amendment

Raw envelope 是最大 256 KiB 的严格 JSON：`source_version:1`、`provider:"github"`、`host:"github.com"`、repository/issue node IDs、number、canonical URL、timestamps、title、body、labels，可选 parent identity。Unknown/duplicate keys、错误类型、invalid UTF-8、BOM、控制字符、trailing data 拒绝。文件以 no-follow 一次打开的普通文件读取并立即关闭；URL 只允许无 userinfo/token/fragment/非默认端口的 `https://github.com/<owner>/<repo>/issues/<number>`。Comments、Project、attachments 和 links 不进入 envelope。Host 是 trusted fetch attester，CLI 只验证内部一致性。

CLI 精确提取 Outcome、Requirements、Acceptance examples、Constraints、Non-goals 五段，LF 规范化后保留其余 bytes，不 Unicode normalize、不模型重写；合计最多 64 KiB。`source_revision` 对 source version/host/node IDs/title/normalized body 做 length-prefixed SHA-256；`requirements_revision` 对 parser version/五段 bytes 做 length-prefixed SHA-256。Timestamps、labels、open state 和 parent 不进 requirements digest。Journal 只存 normalized accepted snapshot、identity、digests 与 URL aliases，不存 raw body。

Issue-bound resume 必须三选一：fresh `--source-file`、显式 `--use-pinned-source`、或 current candidate 的 `--source-choice pinned|adopt --candidate ID`。先验证 provider/host/issue ID；跨 Issue mutation 前拒绝。Transfer 变化只更新 projection 并继续 marker/section/revision 比较。Invalid marker/section 生成不含 raw body 的 invalid candidate；material requirements 生成 valid candidate；两者都原子 void outstanding Action/queue/grant 并 pause。Non-material refresh 记录 drift，void 旧 Action/grant 后 fresh Orient。Candidate ID 必须当前；same `(candidate,choice)` 重试幂等，stale/different 拒绝；invalid 不能 adopt。Pinned 保留旧 decisions；adopt 清除旧 Requirements 派生的 active decisions。Provider unavailable 只有用户显式选择才可 pinned，并记录 refresh skipped，不能说 unchanged。一个 Run 最多一个 primary Change，一个 Change 可有多个 Run。

## 9. Action、Outcome 与状态协议

`contract_version:1`；未知版本 fail closed。上限：source 256 KiB、Outcome 1 MiB、单 journal record 4 MiB、context 128 KiB、suggestion 最多 1、brief 8 KiB、Action 编码 256 KiB。超限在持久化/签发前返回结构化错误。

Action kind 只允许 `orient|clarify|implement|review|summarize`。Issue-bound Action 必须原样带 source identity/revisions 和五段 Requirements；ad-hoc 同时省略两字段，不能发 null。Context 只投影 active decisions 与 Outcome summary/known issues，按 current decision、其他 active decisions、最近 Outcome、其余 Outcome 优先；LF/UTF-8 确定性截断带 original bytes + SHA-256 marker，并记录 per-class omission count。相同 journal replay byte-identical；Requirements 不与 context 抢额度。

Outcome 所有公共字段必须出现；其中 `action_kind` 必填，且必须与当前已签发 Action 的 `kind` 完全相同；缺失、未知或不匹配一律拒绝，不从 Action ID、status 或 result branch 推断。不适用的 `pause|implementation|review` 为 JSON null。Host status 只允许 `completed|needs_input|partial|error`，`skipped` 仅 CLI 产生。

| Action | Host status | 必需组合 | Pause | Suggestions |
| --- | --- | --- | --- | --- |
| Orient | completed/partial/error | implementation=null, review=null | 无 | Clarify/Implement/Summarize，0..1 |
| Orient | needs_input | 两 result null | decision/environment | 无 |
| Clarify | completed/error | 两 result null | 无 | Clarify/Implement/Summarize，0..1 |
| Clarify | needs_input | 两 result null | decision/environment | 无 |
| Implement | completed | `applied\|not_needed` | 无 | 无 |
| Implement | partial | `partial` | 无 | 无 |
| Implement | error | `unable` | 无 | 无 |
| Implement | needs_input | result null | decision/destructive/environment | 无 |
| Review | completed | `no_findings_reported\|findings_reported` | 无 | 无 |
| Review | partial | `inconclusive` | 无 | 无 |
| Review | error | `error` | 无 | 无 |
| Summarize | completed/error | 两 result null | 无 | 无 |

Clarify partial 非法；Review 不得 needs_input，`not_run` 仅 CLI review-skip projection。Pause reason 只允许 `decision_required|destructive_confirmation_required|environment_unavailable`；budget exhausted 仅 CLI。Zero suggestions 确定路由 Summary。Implement activity kind 只允许 test/typecheck/build/lint，并记录真实启动命令和 exit code；零 activity 固定报告 `No test, typecheck, build, or lint activity was reported.` Verdict/approved/gate/freshness/done-ready/ship-ready 不是协议字段。

Run state 只允许 active/paused/stopped/ended。Summary accepted 或 skipped 后 ended；ended 不可 resume。相同 Outcome bytes digest 重试幂等，不同 payload conflict；stale/void/stopped/ended Action 拒绝。普通 answer 必定 void 当前项并 fresh Orient；environment 只能 resume；source candidate 只接受 resume choice。每个成功 mutation 返回 state 与结构化 next。

## 10. 软自动驾驶路由

```text
orient →（人的决定）clarify → implement
                                ├─ start fingerprint 后观察到任何 diff 且 review enabled → review
                                └─ 无 diff 或 --no-review → summarize
review（任意合法结果） → summarize → ended
```

事实先调查；dependent decision 必须是下一 Clarify suggestion/pause，不能藏在 prose。每个 Action 可无理由 skip。Failed/missing activity、finding、dirty worktree 和 Issue status 不控制 progression。Finding 不自动建修复 Change 或 loop。Run start 固定 canonical workspace identity、HEAD、index bytes、porcelain v2 status 和 pre-existing dirty/untracked per-path digest/状态。Observed-since-start diff 是安全侧 Review 信号，不是因果归因；用户、其他 Run 或工具可能贡献，报告必须保留 attribution uncertainty。Host 的 applied/not_needed/unable 与 observed diff 不一致只记 observation/report discrepancy，不指控。

## 11. 破坏性信任边界

`--confirm-destructive` 是 trusted host 对用户当前确认的结构化 attestation，不是密码学人类证明；拥有 shell 权限的恶意 host 可伪造 flag。Target 是非空、去重、按 `(kind,value)` bytewise 排序的 typed list，kind 仅 `path|git_ref|external_resource|data_domain`；impact 非空。CLI 对 RFC 8785 canonical `{scope_version:1,request_id,targets,impact}` 重算 SHA-256。

Implement 先返回 destructive pause；CLI 不签 grant。用户在人机界面确认后，host 必须调用精确 `--confirm-destructive --scope-sha256`。CLI 验证 current request 后只为下一 Implement 签发逐字段匹配、带 originating action 与 timestamp 的 one-shot authorization。空/未知/重复 target、digest mismatch、缺 impact 拒绝。Action complete/partial/error/skip/stop/resume/source amendment 或 scope 改变均使 grant 失效；扩围必须新请求。任意自然语言 yes/no 都不授权，只作为 feedback 后 fresh Orient 非破坏方案。

## 12. Journal、威胁模型、恢复与隐私

每个 Run 位于 Git common dir：

```text
<git-common-dir>/slipway/runs/<run-id>/
├── journal.jsonl   # 唯一 append-only 恢复权威
├── run.json        # 可重建 projection
└── run.lock        # 仅串行 journal mutation
```

Journal 记录 goal、workspace identity、immutable start Git fingerprint、pinned source、Actions/Outcomes、answers/supersession、skip/stop、source choices、destructive request/grant、budget、known issues；不记录 raw Issue body、完整 comments、Project fields、rendered shell command 或 hidden reasoning。单 record 4 MiB，stream replay；仅 interrupted final tail 可在同一 verified handle 修复。

Mutation 只有 bytes 写完且 journal handle fsync 后 committed；首建还同步 file/run dir/parent entry。Projection 用 temp write+fsync+rename+directory fsync。Journal 已 commit 而 projection 失败必须返回 committed/projection-stale、无 retry command；先 replay，不能盲重试。不支持 directory sync 的平台必须在 doctor/docs 收窄承诺。

同 UID 对手可能替换 parent/run/journal/lock/symlink/junction/reparse point。实现必须使用 anchored root/native handles，保存和复验 directory/leaf identity，不能 check-then-reopen；tail truncate 同 handle；lock 不作为 namespace identity。Swap、lock replacement、tail race、projection-after-commit 与 rollback concurrent edit 都需对抗测试并明确 committed/ambiguous 状态。Windows 对任何需要重建 symbolic link、却无法在当前权限下证明可精确恢复的 transaction，必须在首次 mutation 前返回 typed fail-closed error；不能先移动对象再依赖 symlink privilege 回滚，也不能把 privilege-dependent Skip 当作唯一证据。

跨平台删除边界必须诚实收窄：Darwin/Linux/POSIX 的 `unlinkat`/目标平台等价调用按 anchored parent handle + leaf pathname 删除，不提供 portable compare-and-unlink，也不能从已打开 leaf handle 线性化删除某个已验证 directory entry。因此，在没有 exact-object native deletion 的平台上，不承诺抵御持续主动的同 UID watcher 在最后一次 leaf identity validation 返回后、pathname unlink/rmdir 取得对象前替换该 entry。实现仍须使用长期 identity pin、私有随机 quarantine、atomic no-replace relocation、relocation 后 revalidation 与 post-check；任何 validation point 观察到 mismatch 都必须保留并报告。Root、malware、同账户持续竞速该最终 syscall gap 是明确 residual limitation，不能用 C test 或随机名字伪装成已消除。

Per-run lock 不锁 Git/Issue/用户。多个 Run 可共存；歧义 stop/resume 要 ID。同 Change 并发只警告。Recovery 重验 canonical workspace；linked worktree B 不能恢复 A 后改 B。删除 run dir 只移除恢复能力，不改变仓库/Issue/交付状态。

不能承诺 journal 无 secret：goal、accepted Requirements、answers、command summaries 都可能敏感。承诺是：不主动收 token/credential/raw comments/env/unrelated file contents；只存 accepted sections；写前警告；识别到 credential 时最小化脱敏但保留真实 command identity；当前用户最小权限。Unix 目标目录 0700/文件 0600；Windows 尽量使用当前用户 ACL，但继承 ACL、管理员、备份软件和同账户进程限制必须明示。用户应定义 retention、备份策略并手工删除 run dir；删除不是 secure erase、backup purge 或 key destruction。

`runtime/cache/scope-root/scopes/locks/processes/repair-backups` 与其他 unknown sibling 是 unowned legacy residue：run/list/status/replay 忽略；doctor 只读 top-level metadata，用稳定 advisory code 提示 stop old binary/backup/manual cleanup，不读任务内容、不阻塞、不迁移、不删除。Install/refresh/uninstall 也不接管。

## 13. 十个 adapters 与 ownership

支持 Claude Code、Codex、GitHub Copilot、Cursor、Kilo Code、Kiro、OpenCode、Pi、Qwen Code、Windsurf。每宿主精确生成六能力，无 ambient hook。Doctor 的 GitHub version/auth/permission 仅 capability warning；GitHub 不可用不使 ad-hoc Run unhealthy。

Versioned host-specific ownership manifest 只接受当前唯一版本和精确生成路径；任何非当前版本、malformed、duplicate、out-of-host 或 unknown claim 在用于 mutation 前 fail closed。不读取 v1/旧 manifest 作为删除或替换证明，不维护旧路径 inventory，不迁移旧 manifest/marker/settings，也不从旧格式推断 ownership；旧 manifest、marker-only state、retired hooks/settings 与其他未被当前 manifest 认领的内容保持未归属、原样保留，只允许用户显式手工处理。Refresh/uninstall 只依据当前 manifest 删除 hash-matching pristine managed files，用户改动保留并报告。所有 mutation 使用 root-anchored transaction/precondition/rollback validation；在每个 identity validation point 观察到的并发用户改动必须保留并报告，最终 pathname deletion gap 服从上面的 residual limitation，不宣称 linearizable exact-object delete。Windows 不依赖 POSIX shell 或 Unix mode；对无法在当前权限下精确重建的 pre-existing symbolic link，transaction 在 mutation 前安全拒绝，因而不依赖 symlink privilege 完成 rollback。六能力共享 untrusted Issue、trusted fetch、publication reconciliation、privacy 与 destructive grant 边界。

## 14. 结构化恢复与跨平台 rendering

Machine next 是 `{operation,workspace_identity,variants[]}`；每个 variant 有稳定 `id`、完整 `base_argv` 和按 schema 顺序的 typed `inputs{name,type,flag,required,choices?}`。Type 只允许 string/path/enum/digest。展开规则是复制 base argv，再对每个 input 追加 flag 和**一个未引用的精确 argv value**。禁止 `<answer>`/`<file>` 占位伪命令。

Ad-hoc resume 是无输入 variant；issue-bound 提供 fresh source 或 explicit pinned；material candidate 提供绑定 current ID 的 keep-pinned/adopt，invalid 只 keep-pinned。Source temp file 导入后即 ephemeral。每个 variant 保留 absolute root 与 workspace identity。只有 argv 完整后才做 display rendering，display 不写 journal也不改变语义。POSIX、cmd.exe、PowerShell 分别渲染；cmd `%`/`!` expansion 可用 PowerShell UTF-16LE EncodedCommand 或等价安全 argv path。Native Windows 必须实际捕获空格、引号、Unicode、CR/LF、`% ! & ^` 下的原 argv，覆盖 root、Issue URL、source/outcome file、answer 与 recovery。

## 15. 已退休表面

不提供或包装：旧 change/profile/lifecycle commands；`model.Change`/`change.yaml`；`artifacts/changes/**` runtime；S0–S3；artifact/requirements/tasks bundle；verification YAML/evidence freshness；scope/done gates；worktree binding；SessionStart/UserPromptSubmit；global router；profile closure；old readers；compat aliases/state migration/dual mode。历史 artifacts/worktrees 不触碰、运行时忽略。“Change”只指 GitHub Change Issue，不是 CLI bundle/state machine。

## 16. 架构与依赖方向

```text
cmd → autopilot → runstore
cmd → adapter → tmpl
cmd → recoverycmd
runstore / adapter / autopilot → fsutil（仅低层原语）
```

`cmd` 拥有七命令与 rendering；`autopilot` 拥有 protocol/source/routing；`runstore` 拥有 anchored journal；`adapter`/`tmpl` 拥有十宿主六能力；`fsutil` 提供 rooted atomic safety；`recoverycmd` 只把结构化 argv 渲染为 POSIX/cmd/PowerShell。Autopilot 不依赖 renderer，renderer 不读 journal/决定路由。禁止新增 change/spec/plan/lifecycle/gate/tracker runtime 或 wrapper 绕守卫。GitHub publication 留在 host capability；Go binary 不持 token/provider。

## 17. 文档与发布表面

README、start-here、architecture、commands、machine schema/matrix/hidden commands、source/privacy/trust、adapters/ownership/reconciliation、issue workflow、Windows、acceptance matrix、三语 website 需要同步。Release 表面包括 Docker、Nix、GoReleaser、Homebrew/Scoop/AUR/包发布。不得把 Project、PRODUCT.md、总体设计 artifact、Organization-only 功能或 per-Issue privacy 写成前置条件。英文/日文摘要必须标 non-normative，并链接本页和 schema。

## 18. 三十五个验收场景与证据类型

证据标签：C=deterministic Go contract/property/race；S=`tests/acceptance` 调用真实 binary 的 Shell；G=隔离 GitHub.com fixture（host-side fault harness 只能算 G-adjacent，不能替代 live G）；H=Claude/Codex/Pi 脱敏 transcript + evaluator notes；W=native Windows cmd/PowerShell；R=docs/package/release validation。标签不是 CLI pass/fail 或 progression gate；C 不替代 H，fake endpoint 不替代 live G，cross-build 不替代 native W。权威状态与链接见 [acceptance matrix](../../../tests/acceptance/README.md)。

| # | 场景 | 证据 |
| --- | --- | --- |
| 1 | 显式启动；普通聊天不启动 | H |
| 2 | Stateless clarify、grill-me、零/一问题、shared understanding、wrap-up 不写文件 | C+H |
| 3 | Propose self-contained Change、三选项与 approved plan | G+H |
| 4 | Propose Objective，不自动 Implement | G+H |
| 5 | Decompose vertical Changes/native graph/显式 amendment | G+H |
| 6 | User-owned non-Organization repository | G |
| 7 | Timeout/partial/duplicate/index delay publication reconciliation | G+H |
| 8 | Marker 权威、projection drift warning、ready label 不 gate | C+G |
| 9 | Deterministic source、tamper/invalid/oversize/symlink 拒绝 | C+S |
| 10 | Untrusted Issue injection/credential/link 无指令权 | C+H |
| 11 | Issue-bound identity/revisions/sections，raw body 不持久化 | C+S |
| 12 | Amendment candidate void/pinned/adopt/idempotency/transfer compare | C+S+G |
| 13 | Unrefreshed/unavailable/transfer；显式 pinned | C+G |
| 14 | Ad-hoc escape hatch start/stop/resume | S+H |
| 15 | Dependent decisions、answer re-orient、environment resume | C+H |
| 16 | Submit/answer/candidate idempotency 与 stale rejection | C+S |
| 17 | Skip/stop/resume；resume fresh Orient，ended 不恢复 | C+S |
| 18 | Failed activity 如实记录且不 gate | C+S |
| 19 | Review findings 只报告，无 repair/re-review | C+S+H |
| 20 | Exact structured destructive scope；prose 不授权 | C+S+H |
| 21 | Bug/refactor granularity；Level/Kind 正交 | G+H |
| 22 | 未启动 activity 不伪造；零 activity 固定文本 | C+S |
| 23 | Start fingerprint/current-worktree observation/attribution uncertainty | C+S |
| 24 | Namespace checkpoints/quarantine relocation/final deletion limitation；Windows symlink pre-mutation fail-closed | C+W |
| 25 | Structured recovery 可确定 argv，无 placeholder | C+S+W |
| 26 | Native Windows cmd/PowerShell 与特殊 argv | W |
| 27 | GitHub 100/50 limits、`gh<2.94` REST fallback/权限 | G+H |
| 28 | Ten adapters 精确六能力与共享边界 | S+H |
| 29 | Current-only ownership；非当前 manifest/marker/settings 不迁移不删除 | C+S |
| 30 | Privacy minimization/warning/redaction/purge | C+S |
| 31 | Legacy namespace coexistence/advisory only | C+S |
| 32 | Resume budget replacement/candidate not applied | C+S |
| 33 | Public/hidden machine JSON stability across Linux/Windows | C+S+W |
| 34 | 128 KiB bounded context deterministic truncation/omission | C |
| 35 | Action union completeness/schema matrix/Clarify partial rejection | C+S |

## 19. 推荐测试与证据

按影响选用：gofmt/diff-check；全量 Go tests；autopilot/runstore/fsutil/adapter race；source fuzz/property/golden；schema matrix/idempotency/stale tests；namespace swap、quarantine relocation/revalidation、tail/projection fault tests；不可 Skip 的 Windows all-symlink pre-mutation fail-closed policy test与补充 native fixture；vet/testlint/golangci-lint/architecture guards；Linux/Windows cross-build；native Windows argv suite；真实 binary Shell acceptance；十宿主 adapter；host publication fault harness 加隔离 live GitHub fixture；Claude/Codex/Pi 脱敏 prompt matrix；actionlint/yamllint/markdown/link；website sync/build；Nix/Docker/GoReleaser 与包安装 smoke。测试覆盖已定义 validation points，并显式保留最终 pathname deletion residual limitation。Executable fixtures 与 transcripts 只放 `tests/acceptance/`；live credential 使用受保护账号/repo，fork 不暴露 secret。缺证据如实记 uncertainty，不改变 Run 路由。

## 20. 非目标与信任声明

Slipway 不规定总体产品设计或 `PRODUCT.md`；不维护 Spec/Delta/living registry；不要求 Project/Organization 功能；Go binary 不持 GitHub token、不联网验证 provider、不实现 tracker。它不承诺 GitHub create exactly-once，只承诺 approved markers、对账、无盲重试和歧义报告；不声称 CLI flag 密码学证明人类，host 是公开的 trusted attester；不承诺 journal 无敏感文本，只承诺最小化、警告、权限、redaction 与明确删除边界。它不自动 claim/close/merge/archive Issue，不从 close/Project/PR/Run 推断 deployment，不创建/切换/删除 worktree，不恢复 legacy runtime，不自动生成 ADR/CONTEXT/总体设计文档，不存完整 transcript/chain-of-thought/raw body/comments/credential，也不把无 marker Issue 偷当 Change。
