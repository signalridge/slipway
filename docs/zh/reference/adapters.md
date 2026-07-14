# 宿主适配器

每个宿主只生成六个显式能力：`slipway-run`、`slipway-clarify`、`slipway-propose`、`slipway-decompose`、`slipway-implement`、`slipway-review`。Clarify 保持无状态。

原生表面与调用方式如下：Claude、Cursor、Qwen 使用各自 `skills` 目录并显式调用 `slipway-<name>` skill；Codex 使用 `.codex/skills` 和 `$slipway-<name>`；Pi 使用 `.pi/skills` 和 `/skill:slipway-<name>`；Copilot 在 `.github/copilot/agents` 生成 custom agent，由 agent picker 选择；Kilo、OpenCode、Windsurf 分别在 `.kilo/commands`、`.opencode/commands`、`.windsurf/workflows` 生成 `/slipway-<name>`；Kiro IDE 在 `.kiro/steering` 生成手动 `#slipway-<name>` steering，Kiro CLI 在 `.kiro/agents` 生成可由 `kiro-cli chat --agent slipway-<name>` 调用的 agent。

命令、workflow、steering 与 agent 表面都引用各宿主 `slipway/capabilities` 下的 canonical body。Copilot 自动探测仍接受 `.github/copilot`、`.github/prompts`、`.github/skills` 中任一现有宿主表面。Kiro 首次安装必须用 `--surface ide|cli` 选择一种表面；选择记录在 ownership manifest，refresh 和 uninstall 沿用该选择。

不会生成 ambient session hook、prompt-submit hook、launcher、总 router 或独立技术检查能力。宿主 settings 不属于 adapter ownership，install、refresh 与 uninstall 永不修改它们。每个生成 skill 都携带相同的 untrusted Issue、trusted attester、确认 publication 与精确 destructive authorization 边界；只有 Clarify 含一份 decision interview reference，它派生自 Matt Pocock 的 MIT `grill-me` 并保留 attribution。

Clarify 保留 Matt Pocock `grill-me`/`grilling` 的事实先查、依赖顺序、一次一问+推荐、shared-understanding confirmation、stateless 与 wrap-up 立即停止；不提供隐式澄清文档化能力。
Codex 的每个能力还包含受管的 `agents/openai.yaml`，并设置 `allow_implicit_invocation: false`；Codex 不读取通用 frontmatter 中的同类开关，因此该策略确保只有用户显式调用时 Slipway 能力才可用。

只有 version 2 ownership manifest 可以授权 mutation；其他任何版本都视为不可读，使 install、refresh 与 uninstall 在修改文件前失败。只读 `list` 只把该宿主降级为未安装 advisory，并继续报告其他宿主。Version 2 记录路径与 SHA-256。Refresh/uninstall 只处理哈希匹配的文件；用户修改文件会被保留并停止认领。
首次安装只认领新创建的文件；current manifest 已存在后，必须显式运行 `slipway install --refresh` 才会更新。只有 marker 而缺少 current manifest 时不建立任何 ownership：install、refresh 与 uninstall 保持 adapter surface 原样，只中性提示 current ownership 缺失，不迁移也不推断。

Install/uninstall report 把普通 ownership preservation 与事务恢复分开：`transaction_outcome` 为 `committed|rolled_back|not_committed|ambiguous`，只有 committed 才保留计划中的 `written`/`removed`；并发对象或 quarantine 路径只进入 `recovery_artifacts`，不与 `preserved` 混合。错误返回同一 report 且不给 blind-retry command。

`.adapter-generated` sentinel 是 health evidence，不是 ownership authority。缺失时 `install --refresh` 可重建；修改后视为用户内容，refresh/uninstall 都保留。Doctor 会建议检查并在确实需要重建时先手工删除，不会声称 refresh 能覆盖用户修改。

## Publication 与隐私边界

Propose/decompose 探测 `gh`，2.94.0+ 用一等关系操作，否则用官方 REST API 或 `environment_unavailable`。它们要求精确 Level/Kind labels、同 `github.com` transfer identity refetch、100/50 限制、approved operation/item UUID markers、body files、expected revisions、readback，以及零/一/多匹配的 `created|matched|failed|ambiguous` 对账；不盲重试。

所有能力警告 accepted Requirements、goal、answers 与 command summaries 可能敏感；公开 Issue 无 private switch。识别到 credential 时保留命令身份并脱敏 value；不收 token、raw comments、env dump、transcript 或 hidden reasoning。见 [Issue 工作流](issue-workflow.md)和[隐私](../explanation/runs-and-privacy.md)。
