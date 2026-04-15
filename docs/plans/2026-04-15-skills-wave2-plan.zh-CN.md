# Skills 强化计划 —— Wave 2（草案）

**状态。** 草案；依赖 `2026-04-15-route-surface-refactor-plan*.md` 的 PR-1 /
PR-2 / PR-3 按该计划的 cross-plan 顺序全部落完。在该门禁通过之前，不得开启
本波实施 PR。下面的每技能字节目标、reference 清单、hydrate 绑定行均为暂定，
必须在首个 Wave-2 PR 合并之前，根据最新的 Wave-1 metrics baseline 与重构后
的 surface model 重新校核。
Wave-1 已把 `gha-security-review` 推进接近 64 KiB 上限的高预警区。后续任何
wave 若要再次改动该技能，必须先预算 collapse/defer，再谈新增内容；并且要
重新计算当时的 live 字节总量，不能继续沿用旧快照数字。

## 1. 背景

Wave-1 完成了共享基础设施：PR-0 为非 catalog 技能打通支撑文件导出、PR-4a/4b
在三条 selection path 上打通 hydrate wiring、对 T1/T2 target budget 做了一次
上调。因此 Wave-2 的定义以前置的 route-surface 重构终态为准：届时 raw
`--mode=<skill-id>` / `--view=<skill-id>` 已从公共 surface 移除，
`differential-review` 已吸收到 `independent-review`，`BindingCommandManual`
也不再作为公共机制存在。Wave-2 是在这套重构后 surface 上的第一波"模式应用"，
把同一套处理（references + 选择性脚本 lift + hydrate wiring）落到 4 个上游
`references/` 或 `resources/` 结构高度平行的 catalog 技能上：

- 3 个 Trail of Bits 分析族（`variant-analysis`、`property-testing`、
  `mutation-testing`）。每个都有边界清晰的上游 reference 结构，
  reference 模板与 fixture 契约可跨技能复用。
- `performance-profiling`，上游（`alirezarezvani/performance-profiler` +
  `wshobson/distributed-tracing`）既提供值得迁移的 reference 材料，也带 1 个
  可 lift 的 helper，但前提是必须把它诚实改名并按真实契约描述为
  repo-level performance scan，而不是 process / binary profiling launcher。

`differential-review` **不**在本波范围内：route-surface 重构 PR-3 把它
diff-only 的契约义务吸收到 `independent-review`（由
`TestIndependentReviewPreservesDiffOnlyRules` +
`TestIndependentReviewPreservesDifferentialReviewEvidenceVerdictContract`
做保留性契约回归），并删除 runtime registry 条目。
checked-in 的 `differential-review` 模板 / mirror 目录会在最终的
knowledge-only cleanup PR 里删除，但在 Wave-2 开始前它已经退出 live
routed surface。继续为这个技能做强化，仍然只是纯浪费。

重构后 4 个 Wave-2 技能的暴露面：

- `property-testing` → `validate` 的 `--focus property`
- `mutation-testing` → `validate` 的 `--focus mutation`
- `variant-analysis` → `review` / `repair` 的 suggested-only；无公共 selector
- `performance-profiling` → `validate` / `status` 的 suggested-only；无公共
  selector，也不再有 `--view` 落点（`--view=performance-profiling`
  已被重构删除）

`coverage-analysis` **明确不**进入 Wave-2 或 Wave-3。它仍然是已 shipped 的
suggested-only verification booster，但当前模板已经落在 T1 body target 内，
也没有值得单独开 strengthening PR 的上游 `references/` / `scripts` 架子，
并且在重构后的 surface 上已经通过 `goal-verification` host 路径附着。对当前
plan family，应把它视为显式 no-op freeze，而不是“漏掉的第 5 个 verification
技能”。

本波不引入任何新基础设施，只是 Wave-1 契约在一个族内的受控应用。

## 2. 非目标

- 不改 `ResolveNextSkill` 或 `capability.Resolve()` 的决策语义。本波加入的
  hydrate wiring 复用 Wave-1 PR-4a 字段。
- 本波不新增 catalog 技能，只强化既有技能。
- 本波不引入新的 typed partials（`PROSE.tmpl` / `CHECKLIST.tmpl` /
  `VERDICT.tmpl`）。4 个技能都是 reference-heavy，body 已在 Wave-1 预算
  上调后落在 T2 目标内。typed partials 不在当前 Wave-2 scope 内。
- 不再做 tier-budget 上调。T1 保持 2.5 KB、T2 保持 3.5 KB、T3 保持
  1.5 KB。若有技能在本波之后仍落在 warning-band，走 `references/` 再平衡，
  不再抬预算。
- 不改写 `skills_ref/`，本波只向其追加指针。
- 本波不做 process / binary profiling recipe runner。唯一允许的 helper lift
  是 §4 定义的 repo-level scan contract。
- 本波不做 Slipway-ruleset adapter 型 helper。`find-variant.sh` 可以基于上游
  CodeQL / Semgrep 模板做 scaffold，但不能硬绑本地
  `sast-orchestration` 命名，也不能假装自己直接产出最终查询。

## 3. PR-1 —— 4 个技能的 references

**目标。** 按 Wave-1 PR-1 同一套蒸馏规则，把上游 reference 结构迁为条件
触发的 reference 内容：保留条件触发的操作性内容，叙事和长示例收拢或丢弃，
文件名尽可能对齐源语料。

### 计划中的 references

| 技能 | 计划 `references/` | 映射的源段落 | 策展合成（如有） | 备注 |
|------|--------------------|--------------|------------------|------|
| `variant-analysis` | `methodology.md`、`codeql-variant-queries.md`、`semgrep-variant-rules.md`、`variant-report-template.md` | `trailofbits/variant-analysis/METHODOLOGY.md`；`resources/codeql/*`；`resources/semgrep/*`；`resources/variant-report-template.md` | `codeql-variant-queries.md` 与 `semgrep-variant-rules.md` 是对 `resources/codeql` / `resources/semgrep` 子树的多文件合成摘要；在 PR mapping log 标注"多文件合成"。 | 与当前仓库里已经 checked-in 的 `sast-orchestration/{codeql-*,semgrep-*}.md` refs 有交叠。两边交叉引用，不在本波重复 CodeQL / Semgrep 基础知识 —— 公共部分指向 sast-orchestration 的 refs。 |
| `property-testing` | `design.md`、`generating.md`、`strategies.md`、`libraries.md`、`interpreting-failures.md`、`refactoring.md`、`reviewing.md` | `trailofbits/property-based-testing/references/*`（7 份，1:1） | 无 | 源已对齐，文件名逐字保留。 |
| `mutation-testing` | `optimization-strategies.md`、`configuration.md` | `trailofbits/mutation-testing/references/optimization-strategies.md`；`trailofbits/mutation-testing/workflows/configuration.md` | 无 | workflow 内容并入同级 reference；在 PR mapping log 记录这次扁平化映射。 |
| `performance-profiling` | `profiling-recipes.md`、`distributed-tracing-playbook.md` | `alirezarezvani/performance-profiler/references/profiling-recipes.md`；对 `wshobson/distributed-tracing/SKILL.md` 的策展合成 | `distributed-tracing-playbook.md` 为策展撰写（上游无对应 reference）；理由记录在 PR mapping log：上游只发 SKILL.md，且 tracing 内容不应进 body。 | body 聚焦 profiling 工作流，tracing 内容变成 on-demand。 |

### 跨波 overlap 处理

- `variant-analysis` vs `sast-orchestration`：两者都会引用 CodeQL 与
  Semgrep 相关内容。Wave-2 的 variant-analysis references 需要链接到当前仓库
  已经 checked-in 的 `sast-orchestration/codeql-*.md` 与
  `sast-orchestration/semgrep-*.md` reference files，作为基础知识来源，只保留
  variant 发现 / 规则演化这一特有维度。不允许逐行复制。PR notes 必须列出
  被链接的 sast-orchestration refs。
- 若跨技能链接需要 runtime 支持（例如跨 skill 的 hydrate references），
  **停下来升级讨论** —— 本波不引入此类功能。reference 正文里的人写
  跨引用（prose 层面的指针）是唯一允许的机制。
- Wave-2 **不**承接任何 provenance bookkeeping。本波 committed scope 只是强化
  这 4 个技能；任何元数据或 source-coverage 清理都只属于
  `2026-04-16-knowledge-only-refactor-plan*.md`，那是唯一获授权的删除步骤。

### 代码改动

- `internal/tmpl/templates/skills/<id>/references/*.md` —— 按上表新增。
- `internal/tmpl/templates/skills/property-testing/SKILL.md` 与
  `internal/tmpl/templates/skills/mutation-testing/SKILL.md` —— 加
  `hydrate_references:` frontmatter，使用 Wave-1 PR-1 确立的 typed record
  形状（`name`、`reason`）。不得重构 frontmatter 契约。
- `internal/engine/capability/registry_b4.go` —— 为 Wave-2 中的
  `property-testing` 与 `mutation-testing` 填入 `Skill.HydrateReferences`，
  形式沿用 Wave-1 PR-4a。`variant-analysis` 与 `performance-profiling`
  在 Wave-2 仍然会把 references 写盘，但在出现具体 routed consumer 之前，
  **不**预先挂 dormant 的 registry/frontmatter hydrate 元数据。

### 要加 / 扩的测试

- `internal/toolgen/toolgen_test.go::TestCatalogSkillHasReferences` —— 把
  输入列表扩到包含 4 个 Wave-2 skill ID（`variant-analysis`、
  `property-testing`、`mutation-testing`、`performance-profiling`）。
- `internal/toolgen/toolgen_test.go::TestHydrateReferencesResolveToFiles`
  —— 随 registry 扩展自动覆盖；断言 2 个新增的 hydrate-bearing 技能都能解析到文件。
- `internal/engine/capability/gates_test.go::TestFrontmatterMirrorsRegistryHydrateReferences`
  （Wave-1 PR-4a 新增）—— 自动扩展到 `property-testing` /
  `mutation-testing`。
- `internal/toolgen/toolgen_test.go::TestReferenceFileSizeBudget` —— 上限
  维持 24 KB / 文件、64 KB / 技能。

### 验收

- 每份 reference ≤ 24 KB；每技能合计 ≤ 64 KB。
- rendered-tree diff 展示 4 个技能的新 `references/` 目录，以及受影响技能
  预期出现的 hydrate/frontmatter 变化。
- PR notes 包含每技能的源深度字节比表
  （`rendered_reference_bytes / selected_source_bytes`），按命名源段落，
  外加 "mapped / collapsed / deferred" 日志。
- 任一 reference 文件不得逐行复现其所属 `SKILL.md` 正文 ≥ 50%
  （人工复审规则，沿用 Wave-1）。
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  通过。

## 4. PR-2 —— 诚实的 helper lifts

**目标。** 交付 2 个契约真实、且都能回溯到上游材料的 helper lifts：
一个是 `performance-profiling` 的重命名 repo-level performance scan，
另一个是 `variant-analysis` 的模板脚手架生成器。

| 脚本 | 所属技能 | 用途 | Lift 来源 |
|------|----------|------|-----------|
| `scripts/repo-performance-scan.py` | `performance-profiling` | 对 `alirezarezvani/performance-profiler/scripts/performance_profiler.py` 的窄化且重命名的 lift。输入：项目目录路径。输出：围绕大文件、依赖数量、bundle/build 指标的确定性 text / JSON 报告。 | `alirezarezvani/performance-profiler/scripts/performance_profiler.py` |
| `scripts/find-variant.sh` | `variant-analysis` | 基于上游 `resources/codeql/*` 与 `resources/semgrep/*` 的模板脚手架 helper。输入：engine + language + seed metadata。输出：带所选上游模板正文和 TODO 占位的稳定 starter query / rule scaffold；不是最终可运行查询。 | `trailofbits/variant-analysis/resources/codeql/*.ql`；`trailofbits/variant-analysis/resources/semgrep/*.yaml` |

### 约束

- 沿用 Wave-1 PR-2 的脚本契约：Python 脚本通过 `python3 -m py_compile`、
  shell 脚本通过 `bash -n`、缺失 runtime 时 fail-fast、且不新增导出管线。
- 必须保留上游 helper 的真实契约面。这个 lift 仍然是面向目录路径的 repo
  scanner；本波**不**允许它长出 target process、binary、profile mode、
  flamegraph 启动或 load-test orchestration 之类的参数。
- PR notes 必须同时记录 lift 来源、从 `performance_profiler.py`
  改名为 `repo-performance-scan.py` 的事实，以及任何进一步窄化
  （例如输出形状清理）。
- `find-variant.sh` 必须严格锚定上游 resource shelves。它可以按
  engine/language 选择并输出对应的 CodeQL / Semgrep 模板，再补稳定的 seed
  位置与抽象说明 TODO 占位，但**不能**硬编码 Slipway
  `sast-orchestration` ruleset names，也不能宣称会合成最终查询。

### 代码改动

- `internal/tmpl/templates/skills/performance-profiling/scripts/repo-performance-scan.py`
  —— 新增重命名后的 repo-scan helper。
- `internal/tmpl/templates/skills/performance-profiling/SKILL.md`
  —— 把 helper 入口明确写成 repository scan，说明可接受输入、JSON/text
  输出模式与失败姿态；不要再把它写成 process profiler。
- `internal/tmpl/templates/skills/variant-analysis/scripts/find-variant.sh`
  —— 新增模板脚手架 helper。
- `internal/tmpl/templates/skills/variant-analysis/SKILL.md`
  —— 把 helper 入口明确写成 starter scaffold generator，说明它面向上游模板架，
  不是最终查询生成器。

### 要加的测试

- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit` —— 扩展。
- `internal/toolgen/toolgen_test.go::TestScriptStaticChecks` —— 扩展。
- `internal/toolgen/toolgen_test.go::TestScriptFixtureContracts`：
  - `repo-performance-scan.py`：给定 fixture project tree，断言 text 与
    `--json` 输出的报告形状稳定。
  - invalid-path 用例：断言输入路径不存在或不是目录时，报错稳定且可操作。
  - `find-variant.sh`：给定 `--engine=codeql --language=python` 与 seed
    metadata，断言输出包含 Python CodeQL 模板正文和稳定的 seed/TODO 占位；
    给定 `--engine=semgrep --language=go` 时，断言输出对应的 Semgrep scaffold。
  - `find-variant.sh` 的 invalid engine/language 用例：断言 usage 或校验错误
    输出稳定。

### 验收

- `repo-performance-scan.py` 与 `find-variant.sh` 都通过静态检查，且各自至少有
  1 个 fixture 或失败契约测试。
- `init --tools codex --refresh` 会把两个脚本都写进生成的 skill 树。
- 本波任何文字都不再把 `repo-performance-scan.py` 写成 process / binary
  profiling launcher。
- 本波任何文字都不再把 `find-variant.sh` 写成最终查询生成器或
  Slipway-ruleset adapter。

## 5. PR-3 —— 命中 selection path 的 hydrate wiring

**目标。** 在重构后的 surface model 上，验证并在必要时做最小修复，使 2 个
Wave-2 explicit-focus 技能在公共 selection path 上正确暴露 hydrate；
2 个 suggested-only 技能在 PR-1 只把 references 落盘，本波不引入 routed
hydrate 元数据，也不上线 routed-output 的 hydrate surface。不引入任何新
基础设施：route-surface PR-2 已经通过 surface-policy 解析公共 alias，
Wave-1 PR-4a / PR-4b 已经负责 hydrate 渲染。

### 首轮 binding 表

在写测试之前必须对照 b4 registry 构造函数与 surface-policy registry 核对；
两者是权威。

| 技能 | 重构后暴露 | selection path | 初始 hydrate refs | 首次暴露面 |
|------|-----------|----------------|-------------------|------------|
| `variant-analysis` | suggested-only（重构 plan §5.2） | resolver `SuggestedCapabilities[]` on `review` / `repair`；无公共 explicit selector | 本波为空；references 仅 file-backed 落盘 | 在当前 surface model 下本波不接线 |
| `property-testing` | `validate` 的 `--focus property`（重构 plan §5.3） | explicit focus alias 通过 surface-policy 解析到 backing skill | `design.md`、`generating.md`、`strategies.md`、`libraries.md`、`interpreting-failures.md`、`refactoring.md`、`reviewing.md` | `validate --focus property` |
| `mutation-testing` | `validate` 的 `--focus mutation`（重构 plan §5.3） | explicit focus alias 通过 surface-policy 解析到 backing skill | `optimization-strategies.md`、`configuration.md` | `validate --focus mutation` |
| `performance-profiling` | `validate` / `status` 的 suggested-only（重构 plan §5.2）；`--view=performance-profiling` 已被重构删除（§5.5） | resolver `SuggestedCapabilities[]`；无公共 explicit selector，也无 `--view` 面 | 本波为空；references 仅 file-backed 落盘 | 本波不接线 |

说明：两个 suggested-only 技能在 Wave-2 只停留在 file-backed references。
本计划**不**接受“先补 `Skill.HydrateReferences` / `hydrate_references:`，但当前
没有任何 routed consumer”的 dormant 元数据。在当前 surface model 下，
suggested-only 技能保持 file-backed 即可。

### 代码改动

- PR-3 不再新增 `Skill.HydrateReferences` 声明。`property-testing` /
  `mutation-testing` 的显式 focus hydrate 记录已在 PR-1 与
  reference/frontmatter 一起落下，保证现有 frontmatter-vs-registry gate
  一开始就是一致的。
- 默认预期：`internal/engine/capability/` 与 `cmd/` 无需新增生产代码。
  若重构后的 `--focus` path 没有把 PR-1 已声明的 refs 正确暴露出来，PR-3
  只携带恢复该路径所需的最小 resolver / command 修复。
- Wave-1 PR-4b 的 32 KB hydrate 输出上限仍生效。`property-testing` 7 份
  ref 最贴近上限，PR notes 需核算总字节估算。若超 32 KB，按 reference
  分层处理：只把主 refs 列进 `hydrate_references:`，其余保留为 file-backed，
  按需 on-demand 读取。

### 要加的测试

- `internal/engine/capability/gates_test.go::TestFrontmatterMirrorsRegistryHydrateReferences`
  —— 自动扩展。
- `internal/engine/capability/resolver_test.go` —— 新增 case 证明
  `property-testing` 与 `mutation-testing` 走 surface-policy 的 `--focus`
  alias 解析到预期 hydrate 切片；并证明 `variant-analysis` /
  `performance-profiling` 不会暴露 hydrate keys，也不会因无关触发而进入
  `Supports`（重构的 `TestCommandScopedBindingsDoNotAutoPopulateSupports`
  已覆盖该方向，Wave-2 补具体 signal fixture）。
- `cmd/hydrate_view_test.go` —— golden 用例：
  - `validate --focus property` 列出 `property-testing/design.md`
    （以及在 32 KB 上限内的其它接线 ref）。
  - `validate --focus mutation` 列出
    `mutation-testing/optimization-strategies.md`。
  - 否定 golden：`validate --focus property` 不会列出
    `variant-analysis/*` 或 `performance-profiling/*` 的 hydrate key，
    因为它们在本波是 suggested-only、未接线。

### 验收

- `property-testing` 与 `mutation-testing` 在 `--focus` alias path 上暴露
  hydrate keys。
- `variant-analysis` 与 `performance-profiling` 的 references 写盘，但在
  重构后的 surface model 下任何公共命令面都不会产出 routed hydrate keys。
- 每次调用选中的 hydrate body 总字节 ≤ 32 KB。
- 既有 `cmd/...` / `capability` golden 测试无回归。
- `validate --mode=property-testing`（或任何其它 raw skill-id selector）
  必须走重构的 `unknown_route_mode` usage 错误，不允许静默 fallback。

## 6. 执行顺序与门禁

1. **PR-1 先。** references + 必要的 hydrate frontmatter + registry 记录。
   这是唯一会改 authoring surface 的 PR。
2. **PR-2 第二。** 交付 `repo-performance-scan.py` 及其对应的 `SKILL.md` /
   script 契约，并交付 `find-variant.sh` 及 `variant-analysis` 的配套契约更新。
3. **PR-3 第三。** hydrate wiring 依赖 PR-1 已把 references 与
   frontmatter 落盘。
4. 每个 PR 合并前跑 Wave-1 §8.5 的同三道硬门禁，按重构后的 surface model
   适配：
   - `go test ./... -count=1`
   - 本仓库 `init --tools codex --refresh`，diff 审查
     `.codex/skills/slipway/` 生成树。
   - 针对改动面的命令烟测：
     `validate --focus property --json`、
     `validate --focus mutation --json`、
     `validate --list-focuses --format=json`（必须包含 `property` 与
     `mutation`），以及一条否定烟测：`validate --mode=property-testing`
     （或其它 raw skill-id selector）必须返回重构的 `unknown_route_mode`
     usage 错误。
5. **英文与 zh-CN 同步。** 任何改动本计划的 PR 必须在同批次更新两份文件，
   同 Wave-1 §8.6 的规则。
6. **Wave-2 结项门禁。** Wave-2 PR-3 合并后 7 天内产出一份短的结项 /
   metrics report，覆盖：4 个技能的 rendered reference 字节比、任何
   warning-band body 大小、`--focus property` / `--focus mutation`
   的 hydrate 烟测结果、`variant-analysis` 跨波引用是否在不复制的前提下
   成功链接到 `sast-orchestration`。只有该报告通过评审后，Wave-3 范围才算
   确认；在此之前不得开启任何 Wave-3 implementation PR。

## 7. 不在范围

- 改写 `skills_ref/`；本波只向其追加指针。
- 不在本波交付 `performance-profiling/scripts/profiling-recipes.py`，也不交付
  任何 process / binary profiling launcher contract。被 lift 的上游 helper
  是 repo scanner，必须一直按这个事实描述。
- 本波不交付 `find-variant.sh` 的 Slipway-ruleset adapter 变体。这个 helper
  必须始终锚定上游 CodeQL / Semgrep 模板，而不是本地 ruleset 命名。
- 给 4 个技能中的任何一个新增 typed partials。
- 把任何 Wave-3 技能塞进 Wave-2，即便其源语料恰好已齐。Wave-3 依赖
  Wave-2 的结项门禁（见 §6 第 6 条），混在一起会打穿该门禁。
- 任何改动 Wave-1 确立的 hydrate 契约形状、selection path 或基础设施的
  行为。若 Wave-2 的实施 PR 需要这类改动，停下来升级讨论，不要扩范围。
