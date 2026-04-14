# Skills 集成方案

## 1. 目标

把当前以 capability pack 为中心的 skills 集成设计，重构成以 catalog
为中心的设计。

Slipway 会先把 `skills_ref/` 当前工作集蒸馏成 25 个按 `domain x function`
组织的独立 Slipway skill，再通过 Go 侧持有的 binding registry 和自动
capability resolver，把这些 skill 重新绑定回现有 Slipway 框架。

Source corpus 说明：`skills_ref/` 是权威 source corpus。定稿时它包含 80 份权威
`SKILL.md`，另含 1 份随 `skill-tester` 一起下发的 fixture：
`alirezarezvani/skill-tester/assets/sample-skill/SKILL.md`。该 fixture 是
`skill-tester` 技能内部的测试数据，**不属于** source corpus，不计入 disposition
和 provenance coverage。如果交付阶段为了分批蒸馏采用更窄的 working set，那也
只能作为 rollout batching 视图，不能替代 disposition / provenance 的覆盖口径。

这个方案坚持四条不可动摇的原则：

1. 保持单内核。`ResolveNextSkill` 继续作为唯一 progression authority。
2. 不要求手动调用 skill。在 Slipway 内部，被吸收的 skill 由系统自动选择
   并附着。
3. 蒸馏方法，不搬运包。source skill 是新 Slipway catalog 的输入，不是
   vendored runtime unit。
4. 路由权在代码里，不在 prose 里。生成出来的 `SKILL.md` 是描述面和导出
   面；真正的 runtime binding 继续由 Go registry 持有。

## 2. 硬约束

### 2.1 Runtime authority

Slipway 保持唯一的控制闭环：

1. `ResolveNextSkill` 是唯一 progression authority。
2. 当前按状态选择的 governed host 继续保持为：
   `intake-clarification`、`research-orchestration`、`plan-audit`、
   `worktree-preflight`、`wave-orchestration`、`tdd-governance`、
   `spec-compliance-review`、`code-quality-review`、`goal-verification`、
   `final-closeout`。
3. `review`、`validate`、`repair`、`status`、`health` 继续是 command
   surface。它们可以增长 router 或 view，但不会变成第二个 workflow
   engine。

### 2.2 产品边界

这个方案不会改变 Slipway 的产品身份：

1. Slipway 继续保持 multi-tool 能力。
2. Slipway 不导入 mission/work-package/dashboard runtime。
3. Slipway 不变成 home-directory skill installer。
4. 重分析器和 provider-specific 工具继续放在显式 routed command path
   之后。

### 2.3 Binding authority

必须尊重当前实现边界：

1. Toolgen 目前是从 `SKILL.md` 和 `SKILL.md.tmpl` 渲染 adapter skill。
2. Governance skill loader 目前只解析 `name` 和 `description`，真正的
   runtime 行为来自 Go 侧默认 registry。
3. 当前代码真相里，registry-backed governance definition 是 9 个；
   `worktree-preflight` 是 kernel-owned standalone surface，不在默认
   registry map 里。
4. 因此，新增到 `SKILL.md` frontmatter 里的 catalog metadata 是描述面、
   审计面、导出面，不是 runtime binding authority。
5. 任何 catalog 到 host / command 的 binding，都必须落在 Go 侧的 registry
   和可测试 resolver 里。

## 3. 目标架构

### 3.1 三层模型

| Layer | 作用 | Authority |
|------|------|-----------|
| Kernel layer | 通过现有 Slipway host 执行 governed progression | `ResolveNextSkill` 与当前治理逻辑 |
| Catalog layer | 25 个按 `domain x function` 切分的独立 Slipway skill | 不具备 progression authority |
| Binding layer | 把 catalog skill 绑定到 host、routed command、hint、view、export | Go 持有的 binding registry 与 auto resolver |

规则：

1. Catalog 不替代 kernel。
2. Kernel 不需要为每个独立 skill 新增一个 runtime state。
3. Capability pack 降级成文档标签，不再是主架构单元。

### 3.2 独立 skill 契约

每个新的 Slipway skill 都按 function 定义，而不是按上游 source skill 名字
平移。

每个目标 skill 带有如下概念契约：

| 字段 | 含义 |
|------|------|
| `skill_id` | 稳定的 Slipway skill ID |
| `domain` | 关注域，例如 review、verification、repair |
| `function` | 这个 skill 只做的那一件事 |
| `tier` | `T1` core capability、`T2` specialist route、或 `T3` diagnostic view |
| `primary_attachment` | `posture`、`procedure`、`checklist`、`tool-recipe`、`report-schema` 五选一 |
| `summary` | 面向触发的描述，并镜像到 frontmatter 与导出面 |
| `trigger_signals[]` | capability resolver 使用的受限 trigger DSL 子句 |
| `evidence_contract` | `verdict`、`artifact`、或 `checklist` 契约 |
| `bindings[]` | Go 持有的 host、command、hint、view、export binding 的镜像 |
| `provenance_ref` | 指向结构化 `provenance.yaml` 的路径 |

规则：

1. 一个 source skill 可以喂给多个 target skill。
2. 一个 target skill 可以吸收多个 source skill。
3. `tier` 描述语义角色，不规定绑定数量或密度。像 `threat-modeling`、
   `differential-review` 这类 T1 绑定较窄，也仍然是 T1，因为它们承载的是
   可复用的方法，而不是工具路由或只读视图。
4. `primary_attachment` 是 authoring 侧元数据；运行时注入位置由 resolver
   根据 attachment mode 决定。
5. `bindings[]` 会镜像到 authoring metadata，但 runtime authority 继续由
   Go 侧 registry 持有。
6. `trigger_signals[]` 使用受限算子集合，而不是任意 prose。
7. `provenance_ref` 采用结构化格式，方便保持 by-source 索引可审计，并支持
   coverage 校验自动化。

### 3.3 Binding 类型

每个 catalog skill 可以绑定到一个或多个目标：

| Binding type | 含义 |
|------|------|
| `host-embedded` | 作为 directive、checklist 或 partial 注入 governed host |
| `command-auto` | 由 routed command 自动选择 |
| `command-manual` | 支持显式 command flag override |
| `technique-hint` | 复用 `cmd/next_skill_view.go` 中现有的 `TechniqueHints` surface，不影响 progression |
| `command-view` | 以只读 command surface 或 diagnostics view 的形式暴露 |
| `export-only` | 只给 adapter export，不进入核心 runtime |

规则：

1. `technique-hint` 复用现有 `TechniqueHints` 渲染路径；Go 侧只返回
   skill id 和 hint 种类，文案由 host LLM 组织。
2. Binding type 决定运行时附着 surface；attachment mode（见 §3.3.1）决定
   内容被附着时的形状。

### 3.3.1 Attachment mode

每个 catalog skill 声明一个 `primary_attachment`；具体 binding 可再追加。
五种模式在下面固定：

| Mode | 含义 | 典型载体 |
|------|------|----------|
| `posture` | 持续性立场，注入 host prompt 顶部 | "enforce TDD"、"fresh verification required" |
| `procedure` | 有序步骤 | `RED -> GREEN -> REFACTOR`、`Extract -> Dedup -> Reframe -> Anchor` |
| `checklist` | 离散检查项 | security review 条目、spec-trace 对齐 |
| `tool-recipe` | 工具 / 命令调用模式 | semgrep 配置、codeql query scaffold |
| `report-schema` | 结构化输出约束 | verdict 形状、incident timeline schema |

规则：

1. 五种 mode 在 `docs/distillation/schema.md` 中冻结。
2. Template 映射：`PROSE.tmpl` -> `posture` / `procedure`；
   `CHECKLIST.tmpl` -> `checklist`；`VERDICT.tmpl` -> `report-schema`；
   `tool-recipe` 走 `scripts/` 或 inline。
3. Resolver 根据 attachment mode 决定注入位置（prompt 顶部 / checklist
   段 / tool-invocation hint / output-constraint 段）。
4. `primary_attachment` 必填；当单个 skill 承载多种形状时，具体 binding
   可以再额外声明 attachment mode。

### 3.4 Auto capability resolver

Slipway 继续保持 AI-driven，不要求操作员手动点选被吸收的 skill。

当前已落地的 auto capability resolver 直接消费
`internal/engine/capability.Signals` 这组输入：

1. 当前 command 上下文，例如 `review`、`validate`、`repair`、`status`、
   `health`
2. 当前 governed host（用于 support skill 附着）
3. blocker reason
4. changed-file signal
5. referenced path signal
6. 当调用方显式提供时的 user-text match

workflow state/sub-step、guardrail 分类、evidence freshness、artifact
上下文可以由调用方先折算成这些 signal，但它们并不是当前 shipped resolver
struct 的一等字段。

它输出：

1. 当 command 需要自动选 mode/view 时，返回一个 bound route
2. 当 governed host 需要附加方法论时，返回 0-3 个 ranked support skill
3. `hydrate_references[]` 标记按需注入的 `references/*.md`（仿 spec-kitty
   的条件 hydration）
4. 可选 `llm_tiebreak` 字段：当 DSL 评分产生平局时，列出候选 skill id
   与裁决 criterion，交由 host LLM 在 prompt 内对照 user text 裁决
5. 每个自动附着项都带一个简短的 `reason`，其内容来自命中的
   trigger clause

规则：

1. Resolver 绝不能改变 `ResolveNextSkill` 选出来的下一个 governed host。
2. 显式 operator flag 优先于自动路由。
3. 在正常 Slipway 流程里，不要求手动调用被吸收 skill。
4. 导出的 skill 仍然可以被其他工具直接调用；这不改变 Slipway 内部 runtime
   模型。
5. `hydrate_references[]` 与 `llm_tiebreak` 都属于可选扩展输出，而不是 B1
   必交字段。B1 只要求 route / support attachment / technique-hint 跑通。
6. `technique-hint` binding 通过 `cmd/next_skill_view.go` 现有的
   `TechniqueHints` surface 发出 skill id 与 hint 种类；hint 文案不归 Go 管。
7. `hydrate_references[]` 最早在 B2 引入，由 `context-assembly` 这类
   reference-heavy 能力证明；在此之前 resolver 不需要产出该字段。
8. `llm_tiebreak` 是唯一被允许的 AI 让渡点，但只在后续批次出现真实 DSL
   平局时引入；B1 不要求实现或测试它。

### 3.5 Source authoring 布局

Catalog skill 的 source 布局调整为“固定核心契约 + 受约束 support 目录”：

```text
internal/tmpl/templates/skills/<skill-id>/
  SKILL.md
  provenance.yaml
  PROSE.tmpl           # optional
  CHECKLIST.tmpl       # optional
  VERDICT.tmpl         # optional
  references/          # optional
  scripts/             # optional
```

说明：

1. `SKILL.md` 与 `provenance.yaml` 是每个 catalog skill 必备的核心文件。
2. `PROSE.tmpl`、`CHECKLIST.tmpl`、`VERDICT.tmpl` 是按 binding 或 evidence
   contract 条件启用的 typed optional template。
3. `references/` 是一跳可达的 support shelf，用来承载长说明、反例、
   framework 变体、source note；它不是 routing authority。
4. `scripts/` 只放确定性的 helper、validator、aggregator、report
   generator，并要求显式输入/输出契约；它不是 progression authority。
5. Assembler 装配顺序固定为：frontmatter -> `SKILL.md` body -> 按 binding
   类型条件注入 `PROSE` / `CHECKLIST` / `VERDICT`。
6. Support 目录默认不自动消费；只有 skill body 或 assembler config
   显式点名，才会被纳入流程。
7. Catalog skill 现在已经使用 assembler。非 catalog 的 governed host
   仍可能直接使用单文件或 `.tmpl` source。

### 3.6 蒸馏工作流

每次 source-skill 吸收都遵循同一套四步法：

1. `Extract`：只保留触发条件、决策规则、反例，以及能承载证据的检查项。
2. `Deduplicate`：重复规则保留更具体的版本；多个 source 冲突时，保留更
   保守的规则，并把冲突写入 `provenance.yaml`。
3. `Reframe`：围绕目标 Slipway skill 的唯一职责重写，不保留 source skill
   的命名、语气和叙事结构。
4. `Anchor`：一条规则只有在能映射到 `trigger_signals[]`、
   `evidence_contract` 或 typed template 消费点时才保留。

规则：

1. 长故事、背景叙述、冗长 example 默认移入 `references/` 或直接删除。
2. 不能锚定到 runtime 选择、证据输出或 typed prompt 装配的规则，不带入
   catalog 层。
3. Catalog 层默认保持精炼：CI 会鼓励小型 `SKILL.md`，把溢出内容推到 typed
   template 或 `references/`。
4. 蒸馏工作本身由 Claude Code session 扮演蒸馏器执行，不另造
   `slipway distill` 子命令。批次之间的上下文交接仅依赖已合并的
   `provenance.yaml` 与 `docs/distillation/by-source.md`，不写专门的交接
   笔记。

### 3.7 Trigger DSL

`trigger_signals[]` 使用 Go 持有的受限 DSL，而不是自由文本匹配规则。

示例：

```yaml
trigger_signals:
  - all_of:
      - command: review
      - path_includes: ".github/workflows"
      - changed_files_include: "**/*.{yml,yaml}"
    reason: "review 阶段修改了 GitHub Actions workflow"
```

首批支持的算子：

- `all_of`
- `any_of`
- `not`
- `command`
- `host`
- `blocker_reason`
- `changed_files_include`
- `path_includes`
- `user_text_matches`

规则：

1. 算子集合固定在 Go 代码 `internal/engine/capability/trigger.go`。
2. 评分仍由 Go 持有；resolver 负责返回一个 routed command mode/view，或
   至多三个 support attachment。
3. Trigger 子句只是路由证据，不是第二个 workflow engine。

### 3.8 结构化 provenance

每个 catalog skill 都带一个结构化 `provenance.yaml`，用于保持 source
追踪、冲突说明和 by-source 索引可审计，而不是只写手工叙述。

最小形态：

```yaml
sources:
  - source: superpowers/systematic-debugging
    absorbed_as: standalone
    extracted:
      - 先追 root cause 再修复
    dropped:
      - 冗长的调试叙事故事
    conflicts_with: []
```

规则：

1. 只有在 `by-source.md` 中被标为 `standalone` 或 `partial-only` 的
   source，才必须在 `extracted`、`dropped` 或 `conflicts_with` 之一里落位。
   `posture-only`、`absorbed`、`view-only`、`route-only` 与 `deferred`
   继续在 `by-source.md` 中跟踪，不作为 provenance gate。
2. `absorbed_as` 记录这个 source 是 standalone target、posture-only
   输入，还是 partial-only 输入。
3. `docs/distillation/by-source.md` 是人工维护的反向索引，引用
   `provenance.yaml` 与 rollout 状态；它不是自动生成的 source of truth。

### 3.9 蒸馏质量门

只有以下 gate 全部通过，catalog skill 才算蒸馏完成：

1. `schema-lint`：只读解析 frontmatter、typed template reference 与
   trigger operator，断言结构与白名单合法。
2. `size-lint`：只读测量 `SKILL.md` body，并校验 prompt-density discipline；
   预算按 tier 分层：
   - T1 core capability：目标 <= 2 KB；2-6 KB 告警；超过 6 KB 需显式理由。
   - T2 specialist route：目标 <= 3 KB（承载 tool-recipe 开销）；3-8 KB
     告警；超过 8 KB 需显式理由。
   - T3 diagnostic view：目标 <= 1.5 KB；以 report-schema 为主，但允许在同一
     预算内保留极简 posture / anti-pattern 上下文。
   警戒带仅记录为提示日志，不会单独导致 gate fail。
   只有出现无边界 prose、长 example 本应下沉到 typed template /
   `references/` 却仍堆在主文件，或超大 body 且没有批准豁免时，才直接失败。
3. `binding-compare`：只读比对 authoring `bindings[]` 与 Go 持有 registry
   是否 1:1 一致。
4. `provenance-coverage-scan`：只读扫描 `by-source.md` 中所有
   `standalone` / `partial-only` source 是否都在 `provenance.yaml` 的并集中
   被覆盖，并反向检查每个 provenance source 都已出现在 `by-source.md`。

Gate 自动化现在已经通过 Go 测试落地，并应由 CI 执行。更早的 B0-B7 阶段
曾依赖 PR review checklist，直到 Go 侧 registry schema、frontmatter
契约与 by-source 索引稳定下来。

### 3.10 吸收自 superpowers 与 spec-kitty 的范式

以下两个源框架塑造了 Slipway 的蒸馏姿态，但它们不作为 runtime 单元被导入：

| 吸收的范式 | 来源 | Slipway 落点 |
|---|---|---|
| `description`-as-dispatcher | `superpowers` | catalog `SKILL.md` frontmatter `summary` 统一使用 `Use when ... / Triggers on ...` 句式，让导出面的外部 adapter 能做 description 级分诊。 |
| 目录入口 manifest | `superpowers/using-superpowers` | Toolgen 导出时生成一份 `using-slipway-catalog.md`，只面向外部 agent，不进入 Slipway kernel。 |
| `references/` 按需 hydration | `spec-kitty` / `runtime-next` | schema 预留 `hydrate_references[]`；等后续 resolver batch 真正产出该字段后，再由 host 决定是否内联对应 `references/*.md`。 |
| 自动发现 manifest 姿态 | `sickn33/agent-orchestrator` | 作用于 B8 的 toolgen 多文件 assembler，而不是升格为独立 catalog skill。 |

规则：

1. 这些吸收的范式是声明式契约，不新增 runtime progression authority。
2. 不按源仓库一比一镜像，只吸收范式本身。

## 4. Domain x Function Catalog

`skills_ref/` 继续是本方案的权威 source corpus。任何 rollout batching 用的
working set，都必须在 `by-source.md` 或 disposition matrix 里显式列出，不得
隐式缩小 provenance 覆盖范围。

### Tier 分布

25 个 catalog skill 按三个 tier 切分（tier 语义见 §3.2）：

| Tier | 数量 | 成员 |
|---|---|---|
| **T1** core capability | 18 | `scope-clarification`、`context-assembly`、`plan-authoring`、`tdd-proof`、`parallel-executor-contract`、`fresh-verification-evidence`、`root-cause-tracing`、`independent-review`、`multi-reviewer-calibration`、`security-review`、`threat-modeling`、`spec-trace`、`differential-review`、`variant-analysis`、`coverage-analysis`、`property-testing`、`mutation-testing`、`performance-profiling` |
| **T2** specialist route | 6 | `sast-orchestration`、`gha-security-review`、`supply-chain-audit`、`ci-triage`、`review-comment-triage`、`git-recovery` |
| **T3** diagnostic view | 1 | `incident-response` |

Tier 是语义角色标签。像 `threat-modeling`、`differential-review` 这类 T1
skill 绑定较窄，但仍然是 T1，因为它们承载的是可复用方法，而不是工具路由
或视图。

### A. Intake and Framing

| # | Skill | 功能 | 主要 binding | 来源启发 |
|---|---|---|---|---|
| 1 | `scope-clarification` | 在规划前收敛意图与范围 | `intake-clarification`、`technique-hint` | `brainstorming`、`ask-questions-if-underspecified` |
| 2 | `context-assembly` | 组织产品、代码库、风险上下文 | `research-orchestration`、`plan-audit`、`technique-hint` | `context-driven-development`、`audit-context-building`、`spec-kitty` 的 action-scoped context posture |
| 3 | `plan-authoring` | 把需求拆成 bounded、可审计的实现任务 | `plan-audit`、`host-embedded`、`export-only` | `writing-plans`、`workflow-patterns`、`agent-workflow-designer` |

### B. Execution Discipline

| # | Skill | 功能 | 主要 binding | 来源启发 |
|---|---|---|---|---|
| 4 | `tdd-proof` | 强制 RED-GREEN-REFACTOR 与 test-first proof | `tdd-governance`、`wave-orchestration`、`technique-hint` | `test-driven-development`、`workflow-patterns` |
| 5 | `parallel-executor-contract` | bounded 的并行子代理分派与可审阅 handoff | `wave-orchestration` | `dispatching-parallel-agents`、`subagent-driven-development`、`spec-kitty-implement-review` |
| 6 | `fresh-verification-evidence` | 没有 fresh command 与 fresh proof 就不能声称完成 | `goal-verification`、`final-closeout`、`tdd-governance` | `verification-before-completion` |

### C. Debugging

| # | Skill | 功能 | 主要 binding | 来源启发 |
|---|---|---|---|---|
| 7 | `root-cause-tracing` | 修复前先回溯真正 root cause，必要时进入竞争假设分支 | `wave-orchestration`、`repair`、`technique-hint` | `systematic-debugging`、`debugging-strategies`、`debug-buttercup` 的 triage posture、`parallel-debugging` |

### D. Code Review - Quality

| # | Skill | 功能 | 主要 binding | 来源启发 |
|---|---|---|---|---|
| 8 | `independent-review` | fresh-context review、明确 verdict contract，以及 review handoff discipline | `spec-compliance-review`、`code-quality-review`、`review` | `code-review`、`code-reviewer`、`code-review-excellence`、`spec-kitty-runtime-review`、`requesting-code-review`、`receiving-code-review` |
| 9 | `multi-reviewer-calibration` | 多 reviewer finding 去重与严重度校准 | `code-quality-review`、`review` | `multi-reviewer-patterns`、`adversarial-reviewer`、`code-review-ai-ai-review` |

### E. Code Review - Security

| # | Skill | 功能 | 主要 binding | 来源启发 |
|---|---|---|---|---|
| 10 | `security-review` | secure-default 与 framework-specific 安全审查 | `review`、`spec-compliance-review`、`code-quality-review` | `insecure-defaults`、`sharp-edges`、`security-review`、`security-best-practices` |
| 11 | `threat-modeling` | trust boundary、abuse path、owner-aware threat model | `review`、`validate`、`export-only` | `security-threat-model`、`security-ownership-map` |
| 12 | `gha-security-review` | 审查 GitHub Actions 与 AI-agent CI 攻击路径 | `review`、`repair` | `gha-security-review`、`agentic-actions-auditor` |
| 13 | `supply-chain-audit` | 依赖、接管、CVE、license 风险审查 | `review`、`repair`、`status` | `supply-chain-risk-auditor`、`dependency-auditor` |
| 14 | `sast-orchestration` | 运行并合并 Semgrep、CodeQL、SARIF findings | `review`、`validate`、`repair` | `semgrep`、`codeql`、`sarif-parsing`、`audit-augmentation` |

### F. Code Review - Change Shape

| # | Skill | 功能 | 主要 binding | 来源启发 |
|---|---|---|---|---|
| 15 | `differential-review` | 以风险优先级和 blast radius 做 diff review | `review` | `differential-review`、`find-bugs`、`pr-review-expert` |
| 16 | `variant-analysis` | 搜索已知 bug 或漏洞模式的变体 | `review`、`repair` | `variant-analysis` |
| 17 | `spec-trace` | 双向 spec-to-code / code-to-spec trace 审查 | `spec-compliance-review`、`validate`、`review` | `spec-to-code-compliance`、`spec-kitty-mission-review` |

### G. Verification

| # | Skill | 功能 | 主要 binding | 来源启发 |
|---|---|---|---|---|
| 18 | `coverage-analysis` | 覆盖率、关键链路、e2e 证明审查 | `validate`、`goal-verification` | `coverage-analysis`、`e2e-testing-patterns` |
| 19 | `property-testing` | invariant、round-trip、decoder 属性测试 | `validate`、`goal-verification` | `property-based-testing` |
| 20 | `mutation-testing` | 跑 mutation campaign 并解读强度信号 | `validate`、`goal-verification` | `mutation-testing` |
| 21 | `performance-profiling` | profiling、前后对比、负载验证 | `validate`、`goal-verification`、`status` | `performance-profiler`、distributed-tracing 的 checklist 内容 |

### H. Repair and CI Loop

| # | Skill | 功能 | 主要 binding | 来源启发 |
|---|---|---|---|---|
| 22 | `ci-triage` | 提炼 CI 失败上下文并给出 bounded remediation plan | `repair`、`status` | `gh-fix-ci`、`iterate-pr` |
| 23 | `review-comment-triage` | 拉取、分类并处理 PR / issue 评论 | `repair` | `gh-address-comments`、`iterate-pr` |
| 24 | `git-recovery` | 处理 rebase、bisect、reflog、worktree、hook-bypass 问题 | `repair`、`status`、`worktree-preflight` 的失败支持 | `git-advanced-workflows`、`spec-kitty-git-workflow`、`block-no-verify-hook` |

### I. Ops and Diagnostics

| # | Skill | 功能 | 主要 binding | 来源启发 |
|---|---|---|---|---|
| 25 | `incident-response` | 严重度分级、时间线重建、PIR 流程 | `status`、`health`、`export-only`（T3 诊断视图；不走 `repair` 路由） | `incident-commander`、`incident-response`、`acceptance-orchestrator` 的 gate posture |

### J. 非 catalog disposition matrix

| Source / surface | Disposition | 落点 | 原因 |
|---|---|---|---|
| `review-queue` | `view-only` | `status` view | 更像薄队列聚合视图，不是可复用的方法型 skill |
| `observability-query` | `view-only` | `status` / `health` view | 本质上是只读诊断视图，做成 view 比独立 skill 更贴框架 |
| `claude-settings-audit` | `view-only` | `health` / `validate` diagnostics | repo permission / config 审计，更像诊断面而不是运行时方法 |
| `skill-scanner` | `view-only` | `health` / `validate` diagnostics | skill 安全检查更适合作为审计报告 surface |
| `skill-security-auditor` | `view-only` | `health` / `validate` diagnostics | 与 `skill-scanner` 高重叠，保留为安全审计输入而不是 catalog 节点 |
| `skill-tester` | `view-only` | `validate` diagnostics | 质量门 / 报告面，不是 governed workflow 方法 |
| `gh-review-requests` | `view-only` | `status` review queue view | queue / query helper，不是可复用方法节点 |
| `sentry` | `view-only` | `status` / `health` observability view | provider-specific 的只读查询包装 |
| `second-opinion` | `route-only` | 显式 `review` route 或 override | 有价值，但更像 review surface，而不是核心方法型 skill |
| `skill-factory` | `deferred` | future repo-local command family | 当前 CLI 还没有 `skill` command family |
| `prompt-governance` | `deferred` | future prompt-system governance surface | 能力真实存在，但超出当前 code-change governance rollout |
| `agent-workflow-designer` | `absorbed` | `plan-authoring` authoring guidance | authoring meta-skill 更适合蒸馏进 SOP / checklist |
| `designing-workflow-skills` | `absorbed` | distiller SOP / `plan-authoring` guidance | workflow-skill 设计规则属于 authoring process，不是 runtime catalog |
| `writing-skills` | `absorbed` | distiller SOP / adapter export guidance | TDD-for-skills 过程面向作者，不属于运行时 |
| `antigravity-workflows` | `absorbed` | distiller SOP / workflow routing heuristic | orchestration meta-skill，不应提升成 Slipway runtime unit |
| `acceptance-orchestrator` | `absorbed` | `incident-response` / gate posture | gate posture 被吸收即可，不需要独立 surface |
| `block-no-verify-hook` | `absorbed` | `git-recovery` / policy guidance | hook-specific policy，不是可复用 catalog method |
| `spec-kitty-charter-doctrine` | `absorbed` | `plan-authoring` / runtime constraints commentary | doctrine framing 已被吸收到计划与约束层 |
| `simplification-pass` | `absorbed` | `independent-review` 与 `code-quality-review` partial | 更适合作为内部 review technique，而不是独立节点 |
| `review-request-response` | `absorbed` | `independent-review` 与 `review-comment-triage` | 横跨两个生命周期点，边界噪声较大 |
| `hypothesis-arbitration` | `absorbed` | `root-cause-tracing` | 与调试核心重叠太高，做成高级分支更干净 |

### K. 只作为姿态吸收，不提升成独立 skill 的 source

| Source | 融入到 |
|---|---|
| `superpowers/using-superpowers` | project / agent 层 skill-first posture 文案 |
| `superpowers/executing-plans` | `plan-authoring` 的 execution-contract 段落 |
| `spec-kitty/mission-system` | `plan-authoring` 的 taxonomy 与 procedure 注释 |
| `spec-kitty/runtime-next` | Slipway runtime 文档与 resolver 约束 |
| `sickn33/agent-orchestrator` | auto capability resolver 的 matching heuristic |
| `wshobson/error-handling-patterns` | `independent-review` 与 `code-quality-review` partial |

## 5. Binding 与蒸馏模型

### 5.1 Binding registry

这个重构方案引入一个专门的 binding registry，概念落点为：

```text
internal/engine/capability/
  registry.go
  trigger.go
  resolver.go
  provenance.go
```

它负责：

1. 持有 25-skill catalog metadata
2. 持有受限 trigger DSL 的算子集合与求值规则
3. 持有 host、command route、hint、view 的 binding target
4. 让 runtime routing 可测试，并与生成出来的 prose 文件解耦
5. 暴露 adapter export 所需的最小 metadata

### 5.2 Host binding

Governed host 仍然少而稳定，但会有意吸收 catalog skill：

| Governed host | 绑定的 catalog skill |
|---|---|
| `intake-clarification` | `scope-clarification` |
| `research-orchestration` | `context-assembly` |
| `plan-audit` | `plan-authoring`、`context-assembly` |
| `worktree-preflight` | 继续 kernel-owned；当前实现也在该 host 上 host-embed `git-recovery` procedure 作为 worktree failure support |
| `wave-orchestration` | `tdd-proof`、`parallel-executor-contract`、`root-cause-tracing` |
| `tdd-governance` | `tdd-proof`、`fresh-verification-evidence` |
| `spec-compliance-review` | `independent-review`、`spec-trace`、`security-review` |
| `code-quality-review` | `independent-review`、`multi-reviewer-calibration`、`security-review`，以及内嵌的 simplification guidance |
| `goal-verification` | `fresh-verification-evidence`、`coverage-analysis`、`property-testing`、`mutation-testing`、`performance-profiling` |
| `final-closeout` | `fresh-verification-evidence` 加 residual-risk closeout 文案 |

### 5.3 Command binding

Command surface 按“自动优先、显式 override 次之”的方式绑定 catalog skill：

| Command | Catalog skill |
|---|---|
| `review` | `independent-review`、`multi-reviewer-calibration`、`security-review`、`threat-modeling`、`gha-security-review`、`supply-chain-audit`、`sast-orchestration`、`differential-review`、`variant-analysis`、`spec-trace`，以及作为显式 route / override 的 `second-opinion` |
| `validate` | `spec-trace`、`coverage-analysis`、`property-testing`、`mutation-testing`、`performance-profiling` |
| `repair` | `root-cause-tracing`、`ci-triage`、`review-comment-triage`、`git-recovery`、`supply-chain-audit`、`gha-security-review`、`variant-analysis` |
| `status` | `incident-response`、`supply-chain-audit`、`ci-triage`、`performance-profiling` 的 summary，以及作为 view 的 `review-queue`、`observability-query` |
| `health` | diagnostics-first 的完整性与 observability 视图；当前在 change-scoped 自动路由下默认落到 `incident-response`，`observability-query` 保持显式 `--view` 覆盖入口 |

规则：

1. Routed command binding 已在当前代码落地：
   `review` / `validate` / `repair` 提供 `--mode`，
   `status` / `health` 提供 `--view`。
2. 自动选择是默认姿态。
3. 显式 `--mode` / `--view` 覆盖优先于 resolver 自动路由回退。
   同时支持 route-only 非 catalog 覆盖：
   `review --mode second-opinion`、
   `status --view review-queue|observability-query`、
   `health --view observability-query`。
4. `status` / `health` 当前共用一套 payload renderer。对具体
   active/selected change，当前 auto-route 会选择已落地的 T3
   `incident-response` 视图；当没有 active change 且进入 diagnostics
   回退时，除非操作者显式传入 `--view`，否则 `view` 保持为空。
5. 当前非 catalog 的显式 `--view` 覆盖值只有
   `review-queue` 与 `observability-query`。其余 `view-only` 条目仍属于
   文档化的 diagnostics landing zone；当前代码中的 `validate`
   也还没有独立的 `--view` 选择器。
6. `incident-response` 是 T3，仅绑定到 `status` / `health` / export，不再
   进入 `repair` 路由；`repair` 主要由 `root-cause-tracing`、`ci-triage`、
   `git-recovery` 承载。
7. `fresh-verification-evidence` 保持 host 绑定
   （`goal-verification`、`final-closeout`、`tdd-governance`），不作为
   直接 `validate` 路由。
8. `command-auto` 只用于低延迟、高信号的默认路由；扫描器重、外部提供方耦合强
   的路由保持 `command-manual`，通过显式 `--mode` 选择。

### 5.4 蒸馏文档面

文档模型改成 catalog-first：

```text
docs/distillation/
  schema.md
  catalog.md
  by-source.md
  domains/
    intake-and-framing.md
    execution-discipline.md
    debugging.md
    code-review-quality.md
    code-review-security.md
    code-review-change-shape.md
    verification.md
    repair-and-ci.md
    ops-and-diagnostics.md
  routed-surfaces.md
```

`schema.md` 用来冻结：

1. frontmatter、typed template、`provenance.yaml` 的 authoring 契约
2. resolver 可消费的受限 trigger DSL 算子集合
3. 判断一个 catalog skill 是否可合并的 CI gate

`catalog.md` 以 target 为索引：

1. 每个 Slipway catalog skill 一行
2. 记录 domain、function、primary attachment、bindings、provenance 摘要
3. 记录实现状态和测试覆盖状态

`by-source.md` 以 source 为索引，但它是人工维护的反向索引：

1. 每个权威 source corpus 条目一行
2. 记录它的 disposition、被哪些 target catalog skill 吸收，以及 rollout 状态
3. 通过引用 `provenance.yaml` 维持可审计性，但不作为自动生成产物

`routed-surfaces.md` 记录：

1. `view-only` / `route-only` / `deferred` surface 的固定清单
2. 每个 surface 的命令落点与边界
3. 哪些 source 被明确判定为不进入 catalog

### 5.5 为什么 capability pack 不再是主架构

Capability pack 仍然存在，但仅作为 tag 与文档视图，不再定义系统形状。

原因：

1. pack 适合做 survey，不适合做 runtime binding
2. `domain x function` 能得到更干净的 skill 边界
3. binding layer 可以把一个 skill 绑定到多个 surface，而不需要把 pack 变成
   pseudo-runtime object

## 6. Rollout Record

本节记录已经完成并落地的 B0-B8 rollout。`skill-factory`、
`prompt-governance` 等 deferred surface 仍保持 deferred，但下文描述的
catalog registry、routed flags、assembler、export manifest 与 Go test gate
都已经 shipped。

### 6.1 批次总表

| 批次 | 目的 | 主要交付 | 推进门 |
|---|---|---|---|
| **B0** | 契约冻结 | `docs/distillation/schema.md` 冻结（tier、attachment mode、trigger DSL 算子）；`catalog.md`、`by-source.md`、`routed-surfaces.md` 骨架；`provenance.yaml` schema 冻结 | schema review 通过 |
| **B1** | 端到端验证 | `internal/engine/capability/{registry,trigger,resolver,provenance}.go`；对接 `TechniqueHints`（`cmd/next_skill_view.go`）；5 个 foundation T1 完整蒸馏：`scope-clarification`、`plan-authoring`、`tdd-proof`、`fresh-verification-evidence`、`independent-review`；含 registry load + resolver selection + hint 发出的测试 | 端到端闭环在测试里真的跑通 |
| **B2** | 扩 foundation | 剩余 5 个 foundation T1：`context-assembly`、`parallel-executor-contract`、`root-cause-tracing`、`security-review`、`spec-trace` | 多 skill 并存下 resolver 行为稳定 |
| **B3** | 安全集群 | T1 `threat-modeling` + T2 `sast-orchestration`、`gha-security-review`、`supply-chain-audit` | T2 command-route 绑定验证通过 |
| **B4** | 变更形态 + verification | T1 `multi-reviewer-calibration`、`differential-review`、`variant-analysis`、`coverage-analysis`、`property-testing`、`mutation-testing`、`performance-profiling` | |
| **B5** | Repair/CI + ops | T2 `ci-triage`、`review-comment-triage`、`git-recovery` + T3 `incident-response` | T3 view-only 绑定验证通过 |
| **B6** | 非 catalog 清账 | `routed-surfaces.md` 完整；6 条 posture-only 吸收注记完成；disposition matrix 闭环 | 所有 `standalone` / `partial-only` by-source 行在 provenance coverage 扫描中 clean |
| **B7** | Routed command rollout | `review` / `validate` / `repair` auto routing 与 `--mode` flag；`status` / `health` 的 `--view` flag；route selection 与 fallback 的 resolver 测试 | Routed flag 已 shipped 并完成验证 |
| **B8** | Export + gate 自动化 | Toolgen 多文件 assembler；`using-slipway-catalog.md` export；自动化 `schema-lint`、`size-lint`（按 tier）、`binding-compare`、`provenance-coverage-scan` | CI 强制四项 gate，不再依赖 PR review |

### 6.2 B1 foundation 固定集

B1 蒸馏以下 5 个 T1 catalog skill，用以端到端证明 host absorption、
hint 发出、command binding 都能跑通：

1. `scope-clarification` — intake host + technique-hint；attachment：`posture` + `checklist`
2. `plan-authoring` — plan-audit host + host-embedded；attachment：`procedure` + `checklist`
3. `tdd-proof` — tdd-governance 与 wave-orchestration host；attachment：`procedure`
4. `fresh-verification-evidence` — goal-verification 与 final-closeout host；attachment：`checklist` + `report-schema`
5. `independent-review` — spec-compliance-review 与 code-quality-review host，以及 `review` command；attachment：`procedure` + `checklist` + `report-schema`

这 5 个覆盖了 4 个不同的 governed host、1 个 routed command、5 种
attachment mode 中的 4 种。B2 补齐另外 5 个 foundation skill，合并后即为
原 plan 中的 foundation ten。

### 6.3 批次执行规则

1. 每个批次落在一个 PR。批次之间的上下文交接只依赖已合并的
   `provenance.yaml` 与持续维护的 `docs/distillation/by-source.md`；不写
   专门的交接笔记。
2. 冲突裁决默认：source 之间规则冲突时，保守合并 + 在 `provenance.yaml`
   的 `conflicts_with` 里标注 + 在 PR 描述列出冲突清单，不停批。
3. 确实需要升级裁决的冲突，只阻塞对应的 skill，不阻塞整批。
4. EN 与 zh-CN 文档在同一个 PR 内同步。

### 6.4 历史 rollout 护栏

1. B1 先证明了 registry 与 resolver，再扩展到完整 25 个 catalog skill。
2. CI gate 曾被有意推迟到 B8；现在已经通过 Go 测试落地，并预期由 CI 执行。
3. `--mode` / `--view` 已在 B7 shipped，并且现在可用。
4. Routed command rollout 是在 foundation 批次之后独立落地的，而不是混进
   初始蒸馏证明阶段。

## 7. 非目标

1. 不在 `ResolveNextSkill` 旁边再加第二个 progression kernel。
2. 不要求 operator 在正常 Slipway 流程里手动调用被吸收 skill。
3. 不按 source repository 一比一镜像 Slipway 内部目录。
4. 不把生成出来的 `SKILL.md` frontmatter 当成 runtime binding authority。
5. 不为每个 catalog skill 新增一个顶层命令。
6. 不导入 mission/work-package/dashboard/doctrine runtime 行为。
7. 不把 capability pack 再次抬回主架构。
8. 不把 Slipway 收缩成某个工具专属的 skill installer。
9. 不把薄队列、observability、review wrapper 继续维持成独立 catalog
   skill；更适合的形态是 routed surface。
