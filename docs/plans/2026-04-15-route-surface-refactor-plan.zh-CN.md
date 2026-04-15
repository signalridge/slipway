# Route Surface 重构计划

**状态。** 提议中。若采纳，本计划将取代当前强化 wave 计划中关于原始
`--mode=<skill-id>` / `--view=<skill-id>` 的假设：

- `2026-04-15-skills-wave2-plan*.md`
- `2026-04-15-skills-wave3-plan*.md`

本计划**不**改变 Slipway 的 progression kernel。`ResolveNextSkill`
仍然是唯一的 progression authority。此次重构只处理 user-facing route
surface，以及驱动它的内部分类。

除非另有说明，本计划中所有 `*.md` 文档 / 计划 glob 都同时覆盖 EN 与 zh-CN
变体（若两者都存在）。

## 1. 问题

当前 route 语义把四类本应分开的东西压在同一套机制里：

- primary command routing
- 可选高级分析
- 只读诊断视图
- support attachments

这种复用在当前代码里带来了三个具体问题：

- catalog 在公共 CLI surface 上暴露了过多原始 skill ID。
  当前总共有 25 个 catalog skills；其中 19 个至少出现在一个 route/view
  surface 上，而其中 13 个是 `command-manual`。
- `command-manual` 并不是真的"纯手动"。只要 trigger 命中，带
  `BindingCommandManual` 的技能仍可能参与 `Supports` 竞争，因为当前
  support attachment 选择逻辑会同时看 `BindingCommandAuto` 与
  `BindingCommandManual`。
- `status` / `health` 通过 `ValidViewsForCommand` 接纳了非 view 技能，
  结果 checklist / procedure 型技能也会伪装成 view selector，尽管它们并不是
  read-only projection。

所以当前 route layer 的问题不是"不够激进"，而是：

- 在错误的地方太保守（`13` 个 manual selectors）
- 在错误的地方太泄漏（把内部 skill ID 直接公开）

## 2. 目标

- 在正常 CLI 使用中隐藏原始 catalog skill ID。
- 保留 Go-owned catalog registry 作为内部 skill authority。
- 让每个 command surface 都清晰可解释：
  一个默认 primary route、有限 suggestions、少量 explicit focuses，以及
  只保留真正的 read-only views。
- 停止让 `command-manual` 继续扮演"半自动 support channel"。
- 把公共高级 selector 从 10+ 个内部 ID 缩成少数几个用户可理解的名字。

## 3. 非目标

- 不引入第二个 progression kernel。
- 不新增 catalog skills。
- 不大改 trigger DSL。
- 除非本次 route split 需要，否则不改 host-embedded / technique-hint 行为。
- 一旦公共 surface 切换完成，就不再为 raw `skill-id` selector 保留任何
  兼容层；本计划是纠错式 hard cut，不是先弃用再等待的迁移。

## 4. 目标模型

### 4.1 两层 authority

保留两个明确分层的 authority：

- **Skill registry**（`internal/engine/capability/registry*.go`）
  - 负责内部 skill identity、triggers、evidence contract、
    host/support attachment 行为
- **Surface policy registry**（新增）
  - 负责 user-facing command exposure：
    `primary`、`suggested`、`explicit focus`、`view`
  - 在本计划族里，每条 policy record 都直接落到 catalog skill
    （`backing_id=<skill-id>`）。如果后续另有获批计划真正引入
    command-owned diagnostics implementation，应当在那份后续计划里再扩 schema，
    而不是在这里预留第二种 backing kind。

skill registry 继续保持内部、描述性。
surface policy registry 成为 operator 显式可选面的唯一权威。
这次 hard cut 实际只需要 1 个公共 `view` alias（`incident` ->
`incident-response`）。本次重构不落任何 command-owned diagnostic view 抽象。

route 解析遵守这条分层：

- 默认 primary route/view 的解析先查 surface policy registry，而不是
  `BindingCommandManual`
- explicit focus/view alias 必须先解析到 surface-policy backing record，再做
  hydrate lookup、diagnostics rendering 与 help generation
- `BindingCommandAuto` 继续作为内部 command-scoped automatic candidates 的
  元数据存在，但它本身不再自动获得公共 selector 暴露权
- PR-1 起，`BindingCommandManual` 退化为过渡期元数据：它不再喂 `Supports`，
  也不再参与 explicit focus 解析；PR-3 在完成重分类后删除剩余条目，并将该类型
  从 command-surface validation 中移除
- surface policy record 同时拥有显式 public selector 的 hydrate 语义。
  在本计划族里，所有公共 selector 都是 skill-backed，hydrate 一律来自对应
  backing skill；禁止再走“把 alias 直接丢给
  `HydrateReferenceKeysForSkill(view)` 碰运气”的隐式回退

### 4.2 暴露分类

定义四类 exposure：

- **Primary**
  - 每个 command surface 一个默认 routed behavior
  - 无需 operator flag，自动选择
- **Suggested**
  - 基于客观信号或用户文本给出建议
  - 自身不会直接抢成 command 的 primary route
  - 不再通过原始 skill ID 直接选择
- **Explicit focus**
  - 少量重型 / 专门分析，不能静默运行
  - 通过 user-facing alias 选择，而不是 raw skill ID
- **View**
  - 只保留真正的 read-only diagnostic landing zones
  - 仅用于 `status` / `health`
  - 自动 view 只在 `status` / `health` 正在评估某个具体 active/selected
    change 时生效；没有 active change 的 diagnostics 模式可以合法地返回空
    view

这里的 `change-scoped` 约束由 command layer 保证，而不是通过新增 resolver
`Signals` 字段来表达。`status` / `health` 只有在已经解析出具体
active/explicitly selected change target 之后，才会向 surface policy 请求
auto-selected view；纯 diagnostics 路径会直接跳过 auto-view 解析。

### 4.3 公共 CLI 语法

公共 CLI 改为：

- `review --focus <name>`
- `validate --focus <name>`
- `repair --focus <name>`
- `status --view <name>`
- `health --view <name>`

新增：

- `review --list-focuses`
- `validate --list-focuses`
- `repair --list-focuses`
- `status --list-views`
- `health --list-views`

`--mode` 从公共文档与 help text 中退役。任何 user-facing help 都不再把 raw
skill ID 当作 canonical selector syntax 展示。

`--focus` 只注册在 `review` / `validate` / `repair` 上，`--view` 只注册在
`status` / `health` 上；`--list-focuses` 也只注册在支持 focus 的命令上，
`--list-views` 只注册在支持 view 的命令上。跨命令误用统一保持为普通的
parse-time unknown-flag 错误；本计划不再保留 discovery flags 专属的第二条
usage-error 路径，也不返回 unsupported surface 的空列表。

### 4.4 输出契约

为 routed command 输出增加稳定的 suggestion channel：

- JSON：
  - `suggested_capabilities[]`，字段：
    `name`、`summary`、`reason`、`kind`（`suggested` 或 `explicit_focus`）
- text：
  - `Suggested:` 区块，列出 user-facing 名称与一行理由

约束（在 PR-1 之前锁定）：

- **Cap：** 最多 3 条，与当前 `Resolve()` 中 `Supports` 的上限保持一致。
- **Order：** 按 `(clause score desc, skill id asc)` 稳定排序，与当前
  `Resolve()` 对 match 的排序规则一致。
- **与 `Supports` 严格互斥（确定性规则）：** `Supports` 只保留
  host/technique attachment。只要当前调用命中了
  `BindingHostEmbedded` 或 `BindingTechniqueHint`，该 skill 就必须进入
  `Supports`，且不得同时进入 `suggested_capabilities[]`。未被 host-scoped
  binding 选中的 command-scoped 候选，才允许进入
  `suggested_capabilities[]`。PR-1 的 census / migration 必须以这条规则为
  判定标准，不允许再写成 "when in doubt" 的启发式取舍。PR-1 之后，
  `pickSupportAttachment()` 只能看 `BindingHostEmbedded` 与
  `BindingTechniqueHint`；`BindingCommandAuto`、`BindingCommandManual`
  都不得继续进入 `Supports`。
- **JSON schema：** 挂到现有 routed-command output contract 文档下；
  PR-2 落地后视为稳定。
- **JSON 稳定性的含义：** 冻结的是字段名、枚举值、以及字段出现/省略语义；
  `reason`、`summary` 这类解释性文案不属于逐字节冻结的 schema contract，
  可以继续演进。若文案不可用，`reason` / `summary` 在 JSON 中应省略，而不是
  输出空字符串；text renderer 也应省略对应行，而不是打印空标签。
- **Schema 演进方式：** PR-2 不引入 `schema_version` 字段，而是把契约定义为
  additive-only：现有字段、枚举值、以及出现/省略语义保持稳定；后续计划若要
  扩展，只能增加新的 optional 字段，不能静默改写现有字段含义。
- **可见性预算：** 本次重构维持 `Supports` 与
  `suggested_capabilities[]` 各自独立 cap。若 PR-2 的 golden output 仍然过于
  嘈杂，再另起计划讨论联合 cap；本计划不直接引入。

这样能解决 discoverability，而不需要操作者事先知道内部 skill ID。

### 4.5 Hard-cut 策略

本次重构明确不保留任何 selector compatibility layer。这里修正的是一个无效
公共 surface，而不是对一个有效 surface 做平滑迁移。

- PR-2 就是 cutover：`--focus` / `--view` 成为各自 surface 上唯一支持的
  public selector。
- legacy raw `--mode=<skill-id>` 与 raw `--view=<id>` 立即变成硬错误，沿用
  现有 `unknown_route_mode` / `unknown_route_view` 这组 usage error。
- PR-2 与 PR-3 之间不增加 checked-in compatibility table、hidden alias、
  stderr deprecation warning path，也不加 telemetry hold point。
- 外部自动化必须在同一个纠错窗口内更新。若未来还有人想引入兼容 shim，必须用
  新计划重新论证，不能继承本计划的任何口子。

### 4.6 与 Wave-2 / Wave-3 / knowledge-only cleanup 的先后关系

固定落地顺序：

- 先按序落本计划的 PR-1 / PR-2 / PR-3
- 再落 `2026-04-15-skills-wave2-plan*.md` 的 PR-1 / PR-2 / PR-3，随后完成
  Wave-2 结项 / metrics report 评审
- 再落 `2026-04-15-skills-wave3-plan*.md` 的 PR-1 / PR-2 / PR-3，随后完成
  Wave-3 结项报告评审
- 最后落 `2026-04-16-knowledge-only-refactor-plan*.md`

`main` 上不允许存在其它交错顺序。本计划的 PR-1 把 surface-policy
registry 建成唯一 public-surface authority。Wave-2 / Wave-3 随后直接消费
PR-3 后的 surface model。本计划族之间不允许再插入临时 `surfaces[]`
bridge allowlist、handoff table、compatibility alias 或第二份 surface
authority。残余元数据与 dead checked-in source 的唯一清理 PR 是
`2026-04-16-knowledge-only-refactor-plan*.md`，且只能在 Wave-3 closeout
review 之后落地。

## 5. 重分类

### 5.1 Primary routes

| Command surface | Primary route / view | 理由 |
|----------------|----------------------|------|
| `review` | `independent-review` | 最稳定的默认评审契约；当前 auto route 也已实际选它 |
| `validate` | `spec-trace` | 最适合作为默认 code-to-artifact 验证入口 |
| `repair` | `root-cause-tracing` | 修复前先找根因是正确默认姿态 |
| `status` / `health` | `incident-response` | 当前唯一的 command-view 实现；自动默认值是 change-scoped |

说明：`incidentResponse()` 仍是 `status` / `health` 上唯一真实存在的
`BindingCommandView` 实现，但 auto-view 路由只应在已锁定 concrete
active/selected change 时生效。没有 active change 的 diagnostics 模式应保持
空 view。§5.4 中保留 `incident` alias，是为了显式意图与 change-scoped
默认的对称性，而不是因为它代表另一个 fallback surface。

### 5.2 Suggested-only 技能

这些技能在信号充分时继续由系统内部建议 / 附带，但**不**进入公共 explicit
selector 集合。

| 技能 | Surface | 为什么只做 suggestion |
|------|---------|-----------------------|
| `security-review` | `review` | 是默认 review 的高信号补强，适合安全线索出现时自动建议 |
| `threat-modeling` | `review`、`validate` | trust-boundary 变化时很有价值，但不应抢默认主路由 |
| `gha-security-review` | `review`、`repair` | workflow 文件变化是强客观信号；建议优于公开内部 selector |
| `supply-chain-audit` | `review`、`repair`、`status` | manifest / lockfile 信号明确；但它不是 view |
| `coverage-analysis` | `validate` | 是验证增强项，不值得单独做公共 selector |
| `performance-profiling` | `validate`、`status` | perf 信号触发时很有价值，但不是真正的 view |
| `variant-analysis` | `review`、`repair` | 是 follow-on analysis，应从属于主评审 / 修复姿态 |
| `ci-triage` | `repair`、`status` | CI 失败有客观信号，适合自动建议恢复动作 |
| `review-comment-triage` | `repair` | 最适合在 PR comment 上下文存在时建议，不值得做公共 selector |
| `git-recovery` | `repair`、`status` | blocker 驱动的安全姿态；既不是只读 view，也不该暴露成 public mode |

### 5.3 Explicit focuses

公共 explicit focus 集合必须刻意保持很小：

| Focus alias | Backing skill | 允许命令 | 理由 |
|------------|---------------|----------|------|
| `sast` | `sast-orchestration` | `review`、`validate`、`repair` | 重型、工具依赖强、必须显式 opt-in |
| `calibration` | `multi-reviewer-calibration` | `review` | 高级评审流程，应要求显式人类意图 |
| `property` | `property-testing` | `validate` | 专门测试策略；高价值但不应默认运行 |
| `mutation` | `mutation-testing` | `validate` | 成本高的 test-strength audit，必须 operator 驱动 |

任何后续 wave plan 如果要新增 public focus alias，必须先修订本计划；不允许
各自的 wave 文档自行追加 alias。
explicit focus alias 通过 surface policy registry 直接解析到 backing skill，
而不是继续依赖 `BindingCommandManual` 查找。这也是
`multi-reviewer-calibration` 能从当前 manual binding 收敛为 `calibration`
focus，同时不保留旧 manual-selection 运行时语义的原因。

### 5.4 Read-only views

公共 `--view` 只保留已经有实现语义的真正 diagnostics：

| View alias | Backing surface | 命令 |
|-----------|------------------|------|
| `incident` | `incident-response` | `status`、`health` |

当前公共 view 的 hydrate 策略如下：

- `incident` 通过 skill backing 复用 `incident-response` 的 hydrate references
- `review-queue` 与 `observability-query` **不**进入 PR-2。当前 `cmd/` 与
  `internal/engine/capability/` 代码只是通过 override / help 路径保留这些
  字符串，还没有在这些 ID 背后提供独立 diagnostics 行为或 view-specific
  tests。因此这次 hard cut 应直接删除这类 override-only 暴露，而不是把
  字符串占位硬写成一等 public view

`sentry` **不**进入 PR-2。当前 `cmd/` 与
`internal/engine/capability/` 代码里并没有一个已落地的 `status` /
`health` diagnostic view 实现使用这个 ID，所以本计划不能把它写成"当前已存在"。
若后续确实新增 `sentry` diagnostic view，必须通过修订本计划进入，而不是沿用旧
view-only 文档的暗示。

### 5.5 Absorbed / host-only 修正

| 当前 / 计划 surface | 新处置 | 理由 |
|----------------------|---------|------|
| `differential-review` | **吸收后删除 registry 条目**；先把它必要的 diff-scoped review 语义并入 `independent-review`，再把 checked-in 的模板 / mirror 目录删除延后到后续 knowledge-only cleanup PR | `differentialReview()` 当前声明的是一个纯 `BindingCommandManual` 的 review 技能，且没有 host-embedded / technique-hint 消费者，因此删除 registry 条目本身是干净的；但它关于 `new` / `pre-existing` / `worsened` 与 diff-scoped blocker policy 的约束不能静默丢失，必须先迁移再删除。把 checked-in 源目录延后删除，并不等于保留 runtime compatibility：toolgen 的生成与清理都由 registry 驱动，所以 registry 条目一旦消失，生成 skill tree 与 manifest 会立刻停止产出 `differential-review`。 |
| `plan-authoring` 未来 `--mode` 假设 | host/support-only | 属于 planning host，不是 review/validate 公共 selector |
| `tdd-proof` 未来 `--mode` 假设 | host/support-only | 属于 execution governance contract，不是公共 selector |
| `status --view=review-queue` / `observability-query` 假设 | 从 PR-2 公共 surface 删除；若未来要恢复，必须等真实实现落地后再用新修订计划引入 | 当前代码只保留 override / help 字符串，并没有对应的独立 command-owned diagnostics 行为 |
| `status --view=supply-chain-audit` / `ci-triage` / `git-recovery` / `performance-profiling` | 删除 | 它们不是真正的 views |

`differential-review` 的首选吸收路径是：保留 `independent-review` 作为基础
review contract，并在存在 diff-scoped 输入时有条件叠加它独有的
`new` / `pre-existing` / `worsened` 与 diff-scoped blocker policy。不要再
用另一个 public diff route 名义把它重新引回 surface。

吸收后 diff-scoped 激活契约：

- `independent-review` 是否进入 diff-scoped 子路径，由 command layer 判断，
  不能只靠自由文本 trigger 命中来推断。
- diff-only obligations 只在 review 正在处理明确的 delta-scoped 输入集合时
  生效：也就是当前已存在的 changed/stale-unit review 路径（`review` 默认值
  或显式 `--changed-only`），以及未来若另有批准计划新增的显式 diff selector。
- `review --all` 以及任何其它 full-review 路径都必须保持纯粹的
  `independent-review` contract，不能继承 diff-only blocker rules。
- “diff” / “pull request” 这类 user-text cues 仍可继续参与 routing /
  suggestions，但在 `differential-review` 被删除之后，它们单独不足以触发被吸收
  进来的 diff-only 规则。

binding 收口策略：

- `BindingCommandManual` 不是长期 explicit-focus 机制。
- PR-1 停止让它进入 `Supports`。
- PR-2 让公共 focus/view 全部通过 surface policy 解析。
- PR-3 在受影响技能全部重分类后，删除剩余 `BindingCommandManual` 条目与该类型。

## 6. PR-1 —— Surface Policy Foundation

**目标。** 引入专门的 surface policy 层，把 primary、suggested、explicit、
view 暴露从内部 skill registry 里分离出来。

### 代码范围

- 新增：`internal/engine/capability/surfaces.go`
- 新增：`internal/engine/capability/surfaces_test.go`
- 更新：`internal/engine/capability/resolver.go`
- 更新：`internal/engine/capability/registry.go` 注释，反映 split authority

### 实现

- 增加 surface policy records，字段：
  `command`、`class`、`public_name`、`backing_id`、`summary`
- PR-1 只落 skill-backed records。**不要**预留
  `backing_kind=diagnostic_view`、`hydrate_source_id` 或任何为未实现
  command-owned diagnostics 提前铺的并行 schema。当前代码对
  `review-queue` / `observability-query` 仅剩 overrides/help text，不足以支撑
  在这次重构里先造未来抽象。
- 从 surface policy 直接导出 listing / lookup helpers，供后续调用方复用；
  包括 Wave-2 / Wave-3 的后续工作和最终 knowledge-only cleanup PR 在内，都
  必须直接消费这份 authority，而不是再套 wrapper 或 bridge table。
- 让 surface policy registry 成为公共 route 解析权威：默认 primary route/view
  查找与 explicit focus/view alias 查找都必须走 surface records，而不是再去查
  `BindingCommandManual`。
- 为每个 command surface 保留一个 primary record。对 `status` / `health`
  而言，这个 primary view 是 change-scoped 的；没有 active change 的
  diagnostics 模式仍可保持空 view。
- 在 `Resolution` 中新增 `SuggestedCapabilities`（新字段），用于稳定、
  有界的 suggestions。
- 停止让 command-scoped bindings 自动参与 support 选择。
  split 之后，`pickSupportAttachment()` 只能看 host/technique bindings。
  support attachments 应来自 host/technique policy，而不是 explicit route
  元数据。
- 显式 alias 解析必须先通过 surface policy 定位 backing skill，再进入 hydrate
  lookup。禁止从 public alias 隐式回退成“把 alias 当 skill id 试一遍”。
- 不新增 resolver `Signals` 字段来表达 `change-scoped` views。command layer
  仍负责在请求 auto-selected view 前判定是否已经锁定具体 active/selected
  change target。
- `status` / `health` 对 “是否已锁定具体 active 或显式指定 change” 的判断必须
  抽成一个 shared helper，放在 surface-policy lookup 之前，避免两个诊断命令
  在 change-scoped 语义上继续漂移。
- 在 resolver 变更前做一轮 census：grep 仓内当前依赖
  “command-scoped skill -> Supports” 的消费方，把清单写进 PR 描述，并按 §4.4
  的确定性规则迁移到 host/technique binding 或
  `SuggestedCapabilities`；不允许 silent behavior loss。

### 测试

- `TestPrimarySurfaceForCommand` —— 覆盖 `review`、`validate`、`repair`
- `TestChangeScopedPrimaryViewForCommand` —— 覆盖 `status`、`health`
  在 concrete active/selected change 场景下的默认 view
- `TestAutoViewRequiresConcreteChangeTarget` —— 回归保护：command layer
  不会在 diagnostics-without-change 路径上请求 auto-selected view
- `TestDiagnosticsModeDoesNotAutoSelectViewWithoutChange` —— 防止破坏当前
  “无 active change 时空 view” 语义
- `TestSuggestedCapabilitiesAreStableAndBounded`
- `TestSuggestedCapabilitiesDisjointFromSupports`
- `TestExplicitFocusRegistryPerCommand`
- `TestViewRegistryPerCommand`
- `TestSurfacePolicyBackingsResolveToRegisteredSkills`
- `TestCalibrationHostAttachmentSurvivesFocusMigration` —— 即便移除了 manual
  review binding，`code-quality-review` host 仍会附带
  `multi-reviewer-calibration`；host-path 的 support 语义（含当前 no-hydrate
  行为）不变
- `TestCommandScopedBindingsDoNotAutoPopulateSupports`

### 验收

- `review`、`validate`、`repair` 的 primary route 在新 contract 下保持确定性。
- `status`、`health` 的 change-scoped primary view 在新 contract 下保持确定性；
  无 active change 的 diagnostics 模式仍可返回空 view。
- suggested capabilities 与 primary route、supports 分开表达。
- 任何 command-scoped binding 都不会仅因为 trigger 命中就静默进入
  `Supports`。
- `multi-reviewer-calibration` 在 focus 迁移后仍保留
  `code-quality-review` 的 host-embedded 附着行为；变化只发生在公共显式选择面。

## 7. PR-2 —— CLI Cutover 与 Discoverability

**目标。** 用 user-facing focus/view surface 取代 raw `skill-id`
route selection。

### 代码范围

- 更新：`cmd/route_flags.go`
- 更新：`cmd/review.go`
- 更新：`cmd/validate.go`
- 更新：`cmd/repair.go`
- 更新：`cmd/status.go`
- 更新：`cmd/health.go`
- 更新：`internal/engine/capability/routes.go`
- 更新：`internal/toolgen/toolgen.go`
- 更新：`internal/toolgen/testdata/*` 中所有会被 command-registry
  selector/help 输出触发变化的 goldens
- 更新：`docs/command-contract-matrix.md`
- 更新：command help / usage text

### 实现

- 公共 selector flag 重命名：
  `review` / `validate` / `repair` 的 `--mode` -> `--focus`
- `--view` 只保留给真正的 read-only views。
- 增加 `--list-focuses` 与 `--list-views`，并提供 `--format=json` 变体。
- help、remediation text、routed-command 输出字段（`mode` / `view`）以及
  text renderer 全部从 surface alias + summary 渲染，不再直接泄漏 raw
  skill IDs。
- JSON 输出加 `suggested_capabilities[]`；text 输出加 `Suggested:` 区块。
- 在同一 PR 里删除 ad hoc `routeOnlyViewOverrides`。本次 hard cut 只保留
  `incident` 这一个公共 view alias；`review-queue` /
  `observability-query` 在未来有真实 command-owned diagnostics 之前，不提供
  policy-backed 替代项。
- discovery flags 只注册在真正拥有它们的 command surface 上。错 surface 的
  `--list-focuses` / `--list-views` 与错 surface 的 `--focus` / `--view`
  一样，统一在 parse-time 作为 unknown flag 失败；不再额外保留 usage-error
  或空列表回退路径。
- `cmd/route_flags.go` 切到 surface-policy enumeration。现有
  `ValidModesForCommand` / `ValidViewsForCommand` 必须直接删除，所有剩余调用方
  在同一 PR 内统一改为直接查询 surface policy；不保留 thin wrapper。
- PR-2 直接 hard-cut selector surface：legacy `--mode` 与 raw `--view`
  立即按 `unknown_route_mode` / `unknown_route_view` 拒绝；不引入 hidden
  alias、compatibility fixture、deprecation warning，亦不设置 telemetry
  pause gate。
- 显式 alias 选择必须先解析到 backing skill，再做 hydrate lookup 与
  diagnostics 渲染；不能因为 alias cutover 打断当前 explicit-view hydrate
  short-circuit。
- 重写仍在宣传 raw skill IDs 或 dead selectors 的 command help / usage text。
  特别是 `review` help 不能再提 `second-opinion`。
- `status` / `health` help 也不能再提 `review-queue` /
  `observability-query`。
- 删除 `cmd/route_flags.go` 里陈旧的
  `routeOnlyModeOverrides["review"] = {"second-opinion"}`。
- 同一个 PR 内同步改写 `docs/command-contract-matrix.md`，确保本计划族之外仍然
  存活的 live doc 在 CLI cutover 同一时点切到 `--focus` / `--view` 与
  `suggested_capabilities[]`。PR-3 可以继续收尾后续重分类影响，但 PR-2 不允许
  留下“runtime/help 已 hard-cut，而 live contract doc 还在教旧 selector”
  的窗口。

### 测试

- `cmd/route_flags_test.go`
  - 接受新的 focus aliases
  - 接受新的 view aliases
  - legacy raw IDs 与 legacy raw view IDs 立即拒绝；不存在 alias window，
    也不存在 warning path
  - `status --focus ...`、`review --view ...`、`status --list-focuses`、
    `review --list-views` 一律在 parse-time 作为 unknown flag 失败；
    不允许静默接受、重映射，或回退为空列表
  - `--list-focuses` / `--list-views` text 与 JSON 输出稳定
  - help / remediation text 不再宣传 `second-opinion` 或 raw
    `skill-id` selectors
- command golden tests：
  - `review --focus sast`
  - `review --focus calibration`
  - `validate --focus property`
  - `validate --focus mutation`
  - `status --view incident`
- text / JSON 输出覆盖 `suggested_capabilities[]`
- text / JSON 输出覆盖 routed `mode` / `view` 字段，确保暴露的是 public
  alias 而不是 backing ID
- 否定路由测试证明 `status --view review-queue`、
  `status --view observability-query`、`health --view observability-query`
  现在都会走 `unknown_route_view`
- hydrate tests 证明 `status --view incident` 在 cutover 之后仍保持
  explicit-view hydrate path

### 验收

- 公共 help 不再指导用户传 raw skill IDs。
- routed command 输出与生成命令目录文本不再把 raw skill IDs 当成 canonical
  selector contract 暴露给用户。
- `status` / `health` 不再接受非 view selector。
- cutover 之后，唯一 shipped 的公共 `--view` alias 是已有实现语义的
  `incident`；`review-queue` / `observability-query` 不再以字符串占位形式保留。
- 所有 discoverability 路径都不要求用户预先知道内部 IDs。
- `docs/command-contract-matrix.md` 必须在同一个 PR 内切到 post-cutover
  selector contract，并写明 `suggested_capabilities[]`；CLI surface 已
  hard-cut 后，不允许仍有 surviving live doc 继续教学
  `--mode=<skill-id>`。
- `rg -n "routeOnly(Mode|View)Overrides|ValidModesForCommand|ValidViewsForCommand" cmd internal/engine/capability`
  必须归零。

## 8. PR-3 —— Reclassification 与 Plan Family 对齐

**目标。** 把 §5 的新分类应用到当前 catalog surfaces，同时清理现有 plan 文档
里冲突的未来假设。

### 代码 / 文档范围

- 更新：`internal/engine/capability/registry_b2.go`
- 更新：`internal/engine/capability/registry_b3.go`
- 更新：`internal/engine/capability/registry_b4.go`
- 更新：`internal/engine/capability/registry_b5.go`
- 更新：`internal/engine/capability/registry.go`
- 更新：`internal/tmpl/templates/skills/independent-review/`
- 更新：如果仓内仍保留同步 mirror，则更新
  `.codex/skills/slipway/independent-review/`
- 更新：`internal/toolgen/testdata/skill_tree_inventory.codex.golden`
- 更新：当前会列 routed bindings 的生成文档 / export 文档
- 更新：
  - `docs/plans/2026-04-15-skills-wave2-plan*.md`
  - `docs/plans/2026-04-15-skills-wave3-plan*.md`

### 实现

- 依 §5 对当前 route participants 重新分类。
- 删除 `status` / `health` 上的 pseudo-view 假设。
- 先把 `differential-review` 的必要 diff-scoped review 语义吸收到
  `independent-review`，再移除 registry entry。执行这一步前，下面列出的
  preservation test 必须先通过。
- 吸收 `differential-review` 时必须保留其 verdict-shaped evidence contract；
  不能因为并入 `independent-review`，就把 diff-scoped review 的输出契约静默降级
  成 artifact。
- **不要**在吸收后保留 `differential-review` 的 registry stub、隐藏 selector
  或 compatibility alias。PR-3 仍然是 runtime hard cut：registry 条目消失，
  toolgen 产物不再导出该技能，refresh 后的生成工作区也会因为 catalog
  generation/cleanup 依赖 `DefaultRegistry().IDs()` 而清掉旧目录。
- `differential-review` 的 checked-in 模板 / mirror 目录在 PR-3 之后只剩
  dead source 身份，物理删除延后到
  `2026-04-16-knowledge-only-refactor-plan*.md` cleanup PR。这是仓库卫生
  整理，不是 live compatibility 阶段。
- 将 `multi-reviewer-calibration`、`property-testing`、
  `mutation-testing`、`sast-orchestration` 重分类为 explicit-focus-backed
  surfaces；运行时选择通过 surface policy 解析，而不是继续依赖
  `BindingCommandManual`。
- 本 PR 直接删除全部剩余 `BindingCommandManual` 条目与该类型。PR-3 结束时，
  不允许再有任何 validator、route enumerator 或 public-surface helper
  引用它。
- 将未来 plan 中现有的
  `Manual explicit via --mode=<skill-id>` 行全部改写为：
  - `suggested`
  - `explicit focus <alias>`
  - `host/support-only`
  - `absorbed`
- 除非后续另有批准的计划重开本集合，否则公共 explicit focus 固定为 §5.3
  的四个 aliases。

### 测试

- 扩展 resolver 与 command golden tests 以覆盖最终分类
- 增加 `TestIndependentReviewPreservesDiffOnlyRules`，覆盖：
  - `new`
  - `pre-existing`
  - `worsened`
  - diff-scoped blocker policy
- 增加 `TestIndependentReviewPreservesDifferentialReviewEvidenceVerdictContract`，
  防止吸收后把原本 `EvidenceVerdict` 的 diff-only review 契约静默弱化为
  `EvidenceArtifact`
- 增加 `TestIndependentReviewWithoutDiffContextKeepsBaseReviewContract`，
  防止被吸收的 diff-only obligations 泄漏进 `review --all` 或其它 full-review
  执行路径
- 增加否定测试：
  - `differential-review` 不再出现在 registry 中，也不再被任何 surface 接纳
  - `supply-chain-audit` / `ci-triage` / `git-recovery` /
    `performance-profiling` 不再是合法 `--view`

### 验收

- 当前代码、template inventory、剩余 live docs、以及未来 wave plans 采用同
  一套 surface model。
- 任一 active plan file 都不再把 raw `--mode=<skill-id>` 语法写成 preferred
  surface。
- `differential-review` 在 PR-3 后从 runtime registry、generated catalog
  manifest 与 refresh 后的 generated skill tree 中消失；即使 checked-in dead
  source 目录稍后才删，也不再属于 live surface。

## 9. 门禁

本计划内每个 PR 都要跑：

- `go test ./internal/engine/capability/... ./cmd/... -count=1`
- 针对 touched selectors / outputs 的命令烟测
- `git diff --check`

PR-2 与 PR-3 额外跑：

- `go test ./internal/toolgen/... -count=1`
- `go test ./... -count=1`
- `go vet ./...`

PR-3 额外跑 live paths 上的 docs residue checks：

- `rg -- "--mode=[a-z][a-z0-9-]*" internal/toolgen/`
  必须归零。任何残留都说明 generated export surface 仍在继续教授退役语法。
- `rg -n -- "--mode=[a-z][a-z0-9-]*" docs/plans/2026-04-15-skills-wave*.md`
  只允许命中明确要求 `unknown_route_mode` 的 negative smoke /
  hard-error 断言。只要是肯定式、教程式或 preferred-surface 语境下继续出现
  raw `--mode=<skill-id>`，都算失败。
- `rg -n "suggested_capabilities" docs/command-contract-matrix.md`
  必须至少命中一次。该文件是本计划 family 之外承接新输出契约的存活 live
  doc。
- `rg -n -- "--mode <skill-id>|--view <skill-id>" internal/toolgen/ docs/plans/2026-04-15-skills-wave*.md`
  必须归零。
- `rg -n "sentry" docs/plans/2026-04-15-skills-wave*.md` 在后续有新批准计划
  明确加入真实 `sentry` diagnostic view 之前必须归零。

## 10. 不在范围

- 给每个 specialist skill 都新增独立 command。
- 重做 host names 或 trigger DSL。
- 修改 progression state machine。
- 在 explicit focus 集已经收缩之后，又重新扩回一大张公共 selector 矩阵。
