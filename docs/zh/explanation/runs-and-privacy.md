# 运行日志与隐私

恢复数据位于 Git common directory：`.git/slipway/runs/<run-id>/{journal.jsonl,run.json,run.lock}`，但 workspace identity 仍按 worktree 区分。无 ID 的 `status` 只列当前 canonical workspace 所属 Run；兄弟 linked worktree 的 Run 即使物理上共享 common directory，也不会出现在列表中，且只能从原 workspace 查看恢复路由或执行 mutation。`journal.jsonl` 是唯一 append-only 恢复权威；`run.json` 是可重建 projection；`run.lock` 只串行 journal mutation。Unix 目标权限为目录 `0700`、leaf `0600`。旧 `events.jsonl` 以及 `runtime/cache/scope-root/scopes/locks/processes/repair-backups` 是 unowned residue：Run 忽略，doctor 只看 top-level name 给 advisory，任何命令都不读、迁移、别名或删除其任务内容。

## 保存内容与真实隐私承诺

Journal 包含原始 goal、canonical workspace identity、immutable initial Git observation、structured Git delta、bounded issue-source chapter catalog/provenance、Actions/Outcomes、answers 与 supersession metadata、skip/stop、source choice、destructive request/grant、budget、如实的 activity command summaries、known issues 和 uncertainties。Accepted chapter Markdown 只在私有 content-addressed `materials/` 中存一份，不复制到每个 Action 或 journal event。**Goal、accepted Requirements、用户回答与命令摘要可能包含敏感文本。** Source import 或 journal creation 前必须警告，并把 `.git/slipway/runs/` 当作本地私密数据。

Slipway 不绝对承诺 journal 没有 secret。它承诺最小化：不主动收 GitHub token、credential store、raw Issue body、raw/full comments、环境变量 dump、无关文件内容、完整会话 transcript 或 hidden reasoning。Source 只保存 accepted sections、identity 和 revisions。Git path observation 只含 category/state、size、bounded SHA-256；regular hash 超过 16 MiB 或 unreadable 时只记状态。

生成宿主在发布或 journaling 前，对识别到的 credential value 做最小脱敏，同时保留真实 command identity，例如 executable 与被脱敏参数的位置/名称。识别不可能完美，因此用户仍不得粘贴 secret。公开仓库 Issue 没有 private switch；敏感工作用 private repository、仅真实漏洞且已启用时用 private vulnerability reporting、既有 security channel 或 ad-hoc Run。

Action context 限 128 KiB，不是完整 replay。Requirements 独立保留；active decisions 与 Outcome summaries/known issues 按确定优先级选取，newline/UTF-8 boundary truncation 带 byte-count/SHA-256 marker，并记录 omission counts。

## 权限、保留与删除

Unix mode 不防 root、backup、malware 或同 UID 进程。Windows 采用 current-user ACL 意图，但 inherited ACL、管理员、backup agent 与同账户进程仍可能访问，不能承诺绝对 Windows ACL 隔离。仓库 owner 应定义 retention，检查 ACL，保护 backups，并避免发布 `.git/slipway/runs/`。

Namespace mutation 的保证更窄：anchored handles 与长期 identity pin 防 parent traversal、identity reuse，并在 validation checkpoint 检出 replacement；portable POSIX `unlinkat` 只按 parent descriptor + leaf name 删除，不能把最终 directory entry 与已打开 leaf handle 做 compare-and-unlink。因此，在没有 exact native primitive 的平台上，Slipway 不承诺抵御持续同 UID watcher 在最后一次 validation 与 unlink 之间的替换。私有随机 quarantine、atomic no-replace relocation、revalidation 与 post-check 会缩小窗口，并保留每个 checkpoint 观察到的 replacement；root、malware 与同账户进程持续竞速最终 syscall gap 是明确 residual limitation。

删除 run directory 只移除 Slipway 恢复能力和 projection，不改变 Git/source/Issue/deployment，也不会清理 replicas、snapshots、cloud backups、filesystem remnants 或 encryption keys。它不是 secure erase、backup purge 或 key destruction。

## Commit 与恢复语义

只有 journal bytes 与 file fsync 成功后 mutation 才 committed。Projection 经过 temp encode/write/fsync、rename 与平台支持时的 directory sync。Journal 已 commit 而 projection 失败时返回 `mutation_committed_projection_stale` 与 no retry；先 replay 权威 journal，不盲重试。Load/status/mutation 前重验 workspace identity，mismatch 不改 journal。Interrupted final record 在同一 verified handle 修复，更早 corruption 拒绝。Windows flush file 但无等价 directory fsync，doctor 报 `file_fsync_only`/`directory_fsync_unsupported`，不声称 Unix 同级 crash durability。
