# 宿主适配器

每个宿主只生成六个显式能力：`slipway-run`、`slipway-clarify`、`slipway-propose`、`slipway-decompose`、`slipway-implement`、`slipway-review`。Clarify 保持无状态。

能力目录分别位于 `.claude/skills`、`.codex/skills`、`.github/skills`、`.cursor/skills`、`.kilocode/skills`、`.kiro/skills`、`.opencode/skills`、`.pi/skills`、`.qwen/skills`、`.windsurf/skills`。

不会生成 ambient session hook、prompt-submit hook、launcher、总 router 或独立技术检查能力。宿主 settings 不属于 adapter ownership，install、refresh 与 uninstall 永不修改它们。每个生成 skill 都携带相同的 untrusted Issue、trusted attester、确认 publication 与精确 destructive authorization 边界；只有 Clarify 含一份 decision interview reference，它派生自 Matt Pocock 的 MIT `grill-me` 并保留 attribution。

Clarify 保留 Matt Pocock `grill-me`/`grilling` 的事实先查、依赖顺序、一次一问+推荐、shared-understanding confirmation、stateless 与 wrap-up 立即停止；不提供隐式澄清文档化能力。
Codex 的每个能力还包含受管的 `agents/openai.yaml`，并设置 `allow_implicit_invocation: false`；Codex 不读取通用 frontmatter 中的同类开关，因此该策略确保只有用户显式调用时 Slipway 能力才可用。

只接受 version 2 ownership manifest；其他任何版本都视为不可读，不能授权 install、refresh、uninstall 或 list。Version 2 记录路径与 SHA-256。Refresh/uninstall 只处理哈希匹配的文件；用户修改文件会被保留并停止认领。
首次安装只认领新创建的文件；current manifest 已存在后，必须显式运行 `slipway install --refresh` 才会更新。只有 marker 而缺少 current manifest 时不建立任何 ownership：install、refresh 与 uninstall 保持 adapter surface 原样，只中性提示 current ownership 缺失，不迁移也不推断。

## Publication 与隐私边界

Propose/decompose 探测 `gh`，2.94.0+ 用一等关系操作，否则用官方 REST API 或 `environment_unavailable`。它们要求精确 Level/Kind labels、同 `github.com` transfer identity refetch、100/50 限制、approved operation/item UUID markers、body files、expected revisions、readback，以及零/一/多匹配的 `created|matched|failed|ambiguous` 对账；不盲重试。

所有能力警告 accepted Requirements、goal、answers 与 command summaries 可能敏感；公开 Issue 无 private switch。识别到 credential 时保留命令身份并脱敏 value；不收 token、raw comments、env dump、transcript 或 hidden reasoning。见 [Issue 工作流](issue-workflow.md)和[隐私](../explanation/runs-and-privacy.md)。
