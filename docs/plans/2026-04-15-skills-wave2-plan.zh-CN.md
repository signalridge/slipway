# Skills 强化计划 —— Wave 2（草案）

**状态。** 草案；在 `2026-04-14-skills-strengthening-plan.md` §8.7 要求的
Wave-1 metrics 报告通过评审之前，不得开启本波的实施 PR。下面的每技能字节
目标、reference 清单、hydrate 绑定行均为暂定，必须在首个 Wave-2 PR 合并
之前，根据 Wave-1 metrics 重新校核。
Wave-1 的实测已经把 `gha-security-review` 推到 `63,645 / 65,536` 字节，
约占上限的 97.1%。后续任何 wave 若要再次改动该技能，必须先预算
collapse/defer，再谈新增内容。

## 1. 背景

Wave-1 完成了共享基础设施：PR-0 为非 catalog 技能打通支撑文件导出、PR-4a/4b
在三条 selection path 上打通 hydrate wiring、对 T1/T2 target budget 做了一次
上调。Wave-2 是第一个"模式应用"波次，把同一套处理（references + 可选
scripts + 可选 typed partials + hydrate wiring）落到 5 个上游 `references/`
或 `resources/` 结构高度平行的 catalog 技能上：

- 4 个 Trail of Bits 分析族（`differential-review`、`variant-analysis`、
  `property-testing`、`mutation-testing`）。每个都有边界清晰的上游
  reference 结构，reference 模板与 fixture 契约可跨技能复用。
- `performance-profiling`，上游（`alirezarezvani/performance-profiler` +
  `wshobson/distributed-tracing`）只带 1 份 reference + 1 个 Python 助手。
  其处理形态与 Trail of Bits 模式平行，故并入本波。

本波不引入任何新基础设施，只是 Wave-1 契约在一个族内的受控应用。

## 2. 非目标

- 不改 `ResolveNextSkill` 或 `capability.Resolve()` 的决策语义。本波加入的
  hydrate wiring 复用 Wave-1 PR-4a 字段。
- 不新增 catalog 技能，数量仍为 25。
- 本波不引入新的 typed partials（`PROSE.tmpl` / `CHECKLIST.tmpl` /
  `VERDICT.tmpl`）。5 个技能都是 reference-heavy，body 已在 Wave-1 预算
  上调后落在 T2 目标内。若后续审查发现某个技能确实需要 typed partial，
  另开追加 PR，不要塞进 Wave-2。
- 不再做 tier-budget 上调。T1 保持 2.5 KB、T2 保持 3.5 KB、T3 保持
  1.5 KB。若有技能在本波之后仍落在 warning-band，走 `references/` 再平衡，
  不再抬预算。
- 不改写 `skills_ref/`，本波只向其追加指针。

## 3. PR-1 —— 5 个技能的 references

**目标。** 按 Wave-1 PR-1 同一套蒸馏规则，把上游 reference 结构迁为条件
触发的 reference 内容：保留条件触发的操作性内容，叙事和长示例收拢或丢弃，
文件名尽可能对齐源语料。

### 计划中的 references

| 技能 | 计划 `references/` | 映射的源段落 | 策展合成（如有） | 备注 |
|------|--------------------|--------------|------------------|------|
| `differential-review` | `methodology.md`、`adversarial-comparison.md`、`patterns.md`、`reporting.md` | `trailofbits/differential-review/{methodology,adversarial,patterns,reporting}.md` | `adversarial.md` → `adversarial-comparison.md` 改名，避免与未来的 adversarial-review 语料歧义；在 provenance 记录改名。 | 上游文件是平铺（没有 `references/` 子目录），authoring 要从源根读取，不能假设存在 `references/`。 |
| `variant-analysis` | `methodology.md`、`codeql-variant-queries.md`、`semgrep-variant-rules.md`、`variant-report-template.md` | `trailofbits/variant-analysis/METHODOLOGY.md`；`resources/codeql/*`；`resources/semgrep/*`；`resources/variant-report-template.md` | `codeql-variant-queries.md` 与 `semgrep-variant-rules.md` 是对 `resources/codeql` / `resources/semgrep` 子树的多文件合成摘要；在 provenance 标"多文件合成"。 | 与 Wave-1 的 `sast-orchestration/{codeql-*,semgrep-*}.md` 有交叠。两边交叉引用，不在本波重复 CodeQL / Semgrep 基础知识 —— 公共部分指向 sast-orchestration 的 refs。 |
| `property-testing` | `design.md`、`generating.md`、`strategies.md`、`libraries.md`、`interpreting-failures.md`、`refactoring.md`、`reviewing.md` | `trailofbits/property-based-testing/references/*`（7 份，1:1） | 无 | 源已对齐，文件名逐字保留。 |
| `mutation-testing` | `optimization-strategies.md`、`configuration.md` | `trailofbits/mutation-testing/references/optimization-strategies.md`；`trailofbits/mutation-testing/workflows/configuration.md` | 无 | workflow 内容并入同级 reference；在 provenance 记录这次扁平化映射。 |
| `performance-profiling` | `profiling-recipes.md`、`distributed-tracing-playbook.md` | `alirezarezvani/performance-profiler/references/profiling-recipes.md`；对 `wshobson/distributed-tracing/SKILL.md` 的策展合成 | `distributed-tracing-playbook.md` 为策展撰写（上游无对应 reference）；理由：上游只发 SKILL.md，且 tracing 内容不应进 body。 | body 聚焦 profiling 工作流，tracing 内容变成 on-demand。 |

### 跨波 overlap 处理

- `variant-analysis` vs `sast-orchestration`（Wave-1）：两者都会引用 CodeQL
  与 Semgrep 相关内容。Wave-2 的 variant-analysis references 需要链接到
  已存在的 Wave-1 `sast-orchestration/codeql-*.md` 与
  `sast-orchestration/semgrep-*.md` 作为基础知识来源，只保留 variant
  发现 / 规则演化这一特有维度。不允许逐行复制。PR notes 必须列出被链接
  的 sast-orchestration refs。
- 若跨技能链接需要 runtime 支持（例如跨 skill 的 hydrate references），
  **停下来升级讨论** —— 本波不引入此类功能。reference 正文里的人写
  跨引用（prose 层面的指针）是唯一允许的机制。

### 代码改动

- `internal/tmpl/templates/skills/<id>/references/*.md` —— 按上表新增。
- `internal/tmpl/templates/skills/<id>/provenance.yaml` —— 扩 `inputs:`
  覆盖每份新 reference 与所有策展合成。
- `internal/tmpl/templates/skills/<id>/SKILL.md` —— 加
  `hydrate_references:` frontmatter，使用 Wave-1 PR-1 确立的 typed record
  形状（`name`、`reason`）。不得重构 frontmatter 契约。
- `internal/engine/capability/registry_b4.go` —— 为 4 个 b4 技能填入
  `Skill.HydrateReferences`，形式沿用 Wave-1 PR-4a。

### 要加 / 扩的测试

- `internal/toolgen/toolgen_test.go::TestCatalogSkillHasReferences` —— 把
  输入列表扩到包含 5 个 Wave-2 skill ID。
- `internal/toolgen/toolgen_test.go::TestHydrateReferencesResolveToFiles`
  —— 随 registry 扩展自动覆盖；断言 5 个新技能都能解析到文件。
- `internal/engine/capability/gates_test.go::TestHydrateReferencesMirrorRegistry`
  （Wave-1 PR-4a 新增）—— 自动扩展。
- `internal/toolgen/toolgen_test.go::TestReferenceFileSizeBudget` —— 上限
  维持 24 KB / 文件、64 KB / 技能。

### 验收

- 每份 reference ≤ 24 KB；每技能合计 ≤ 64 KB。
- rendered-tree diff 展示 5 个技能的新 `references/` 目录与扩充后的
  `provenance.yaml inputs:`。
- PR notes 包含每技能的源深度字节比表
  （`rendered_reference_bytes / selected_source_bytes`），按命名源段落，
  外加 "mapped / collapsed / deferred" 日志。
- 任一 reference 文件不得逐行复现其所属 `SKILL.md` 正文 ≥ 50%
  （人工复审规则，沿用 Wave-1）。
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  通过。

## 4. PR-2 —— Scripts

**目标。** 交付两个可执行助手，把上游以 prose 描述或以 Python 实现的内容
落地。

| 脚本 | 所属技能 | 用途 |
|------|----------|------|
| `scripts/profiling-recipes.py` | `performance-profiling` | 对 `alirezarezvani/performance-profiler/scripts/performance_profiler.py` 的窄化 lift。输入：目标进程/二进制 + profile 模式；输出：确定性的 recipe 调用命令，带按平台回退的错误信息。Python runtime 契约沿用 Wave-1 PR-2（`python3` 缺失需 fail-fast）。 |
| `scripts/find-variant.sh` | `variant-analysis` | 离线助手：给定一个种子发现（漏洞模式 + file:line），生成稳定的 CodeQL + Semgrep 查询模板骨架，规则集名锚定到已有的 `sast-orchestration`。不联网、不做 tag 解析。 |

### 约束

- 完全沿用 Wave-1 PR-2 契约：`.sh` 使用 `0o755`、shell 过 `bash -n`、
  Python 过 `python3 -m py_compile`、缺失 runtime 路径有可操作错误信息。
  不新增导出管线。
- `profiling-recipes.py` 是 lift，不是从头重写。provenance 必须标 lift
  来源。如需任何可选三方依赖，必须以脚本内声明（例如通过 `uv` 脚本
  metadata）或以可操作错误信息失败。
- `find-variant.sh` 不得嵌入任何具体漏洞 fixture。输出模板是骨架，不是
  "可以直接跑"的查询。由 reference 文件
  （`variant-analysis/codeql-variant-queries.md` 或
  `semgrep-variant-rules.md`）说明如何完成骨架。

### 要加的测试

- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit` —— 扩展。
- `internal/toolgen/toolgen_test.go::TestScriptStaticChecks` —— 扩展。
- `internal/toolgen/toolgen_test.go::TestScriptFixtureContracts`：
  - `profiling-recipes.py`：fixture 模式下断言 recipe 输出形状稳定，
    且在不支持平台上 fail-fast 时错误字符串稳定。
  - `find-variant.sh`：fixture 种子下断言输出含稳定占位，且缺 `--seed`
    时有 usage 错误文本。

### 验收

- 两个脚本都过静态检查，且各自至少有 1 个 fixture 或失败契约测试。
- `init --tools codex --refresh` 把脚本写进生成的 skill 树。

## 5. PR-3 —— 命中 selection path 的 hydrate wiring

**目标。** 为 5 个 Wave-2 技能在它们实际参与的 selection path 上补齐 hydrate
references。不引入任何新基础设施，只做 registry 填充 + surface 渲染；
helpers 全部来自 Wave-1 PR-4a / PR-4b。

### 首轮 binding 表（暂定）

在写测试之前必须对照 b4 registry 构造函数核对；registry 是权威。

| 技能 | 已有 bindings（需复核） | selection path | 初始 hydrate refs | 首次暴露面 |
|------|-------------------------|----------------|-------------------|------------|
| `differential-review` | TBD（`review`？） | Manual explicit via `--mode=differential-review` | `methodology.md`、`adversarial-comparison.md`、`patterns.md`、`reporting.md` | `review` |
| `variant-analysis` | TBD（`review`？） | Manual explicit via `--mode=variant-analysis` | `methodology.md`、`codeql-variant-queries.md`、`semgrep-variant-rules.md`、`variant-report-template.md` | `review` |
| `property-testing` | TBD（`validate`？`review`？） | Manual explicit via `--mode=property-testing` | `design.md`、`generating.md`、`strategies.md`、`libraries.md`、`interpreting-failures.md`、`refactoring.md`、`reviewing.md` | `validate`、`review` |
| `mutation-testing` | TBD（`validate`？） | Manual explicit via `--mode=mutation-testing` | `optimization-strategies.md`、`configuration.md` | `validate` |
| `performance-profiling` | TBD（`review`？`status`？） | 视 binding 而定；大概率 manual explicit（`review` 用 `--mode`，`status` 用 `--view`） | `profiling-recipes.md`、`distributed-tracing-playbook.md` | `review`、`status` |

**复核要求。** 写测试前，打开 `registry_b4.go` 读每个技能的真实
`Bindings:` 切片。修行，不修 registry。若没有 binding 面能承载首轮 hydrate
surface，在 PR notes 标记；不要只为了挂 hydrate 而新增 binding。

### 代码改动

- `internal/engine/capability/registry_b4.go` —— 按表填入
  `Skill.HydrateReferences`。
- `cmd/route_flags.go`、`cmd/hydrate_render.go`、`cmd/review.go`、
  `cmd/validate.go`、`cmd/status.go`、`cmd/health.go`、`cmd/next.go`
  **不改**：Wave-1 PR-4a 已经在解析器 / manual-explicit 查表返回非空切片
  时统一渲染 hydrate。Wave-2 只改变"查表返回什么"。
- Wave-1 PR-4b 的 32 KB hydrate 输出上限仍生效。`property-testing` 7 份
  ref 最贴近上限，PR notes 需核算总字节估算。若超 32 KB，按 reference 分
  层处理：只把主 refs 列进 `hydrate_references:`，其余保留为 file-backed，
  按需 on-demand 读取。

### 要加的测试

- `internal/engine/capability/gates_test.go::TestHydrateReferencesMirrorRegistry`
  —— 自动扩展。
- `internal/engine/capability/resolver_test.go` —— 为每个新 manual-explicit
  binding 加 case。
- `cmd/hydrate_view_test.go` —— golden 用例：
  - `review --mode=differential-review` 列出
    `differential-review/methodology.md`。
  - `validate --mode=property-testing` 列出
    `property-testing/design.md`。
  - `status --view=performance-profiling`（若 binding 允许）列出
    `performance-profiling/profiling-recipes.md`。

### 验收

- 5 个技能至少在 1 个命令面暴露 hydrate keys（manual explicit 为下限；
  若既有 binding 允许，change-selected 或 support path 同样可加）。
- 每次调用选中的 hydrate body 总字节 ≤ 32 KB。
- 既有 `cmd/...` / `capability` golden 测试无回归。

## 6. 执行顺序与门禁

1. **PR-1 先。** references + hydrate frontmatter + registry 记录。
   这是唯一会改 authoring surface 的 PR。
2. **PR-2 与 PR-3 可在 PR-1 之后并行。** scripts 与 hydrate wiring 正交，
   都消费 PR-1 产出但彼此不依赖。
3. 每个 PR 合并前跑 Wave-1 §8.5 的同三道硬门禁：
   - `go test ./... -count=1`
   - 本仓库 `init --tools codex --refresh`，diff 审查
     `.codex/skills/slipway/` 生成树。
   - 针对改动面的命令烟测（`review --mode=<id> --json`、
     `validate --mode=<id> --json`，以及 binding 允许时
     `status --view=<id> --json`）。
4. **英文与 zh-CN 同步。** 任何改动本计划的 PR 必须在同批次更新两份文件，
   同 Wave-1 §8.6 的规则。
5. **Wave-3 触发条件。** Wave-2 PR-1 合并后 7 天内产出一份短 metrics
   report，覆盖：5 个技能的 rendered reference 字节比、任何 warning-band
   body 大小、hydrate 烟测结果、`variant-analysis` 跨波引用是否在不复制
   的前提下成功链接到 `sast-orchestration`。Wave-3 范围在该报告通过
   评审后确认。

## 7. 不在范围

- 改写 `skills_ref/` provenance；本波只向其追加指针。
- 给 5 个技能中的任何一个新增 typed partials。若某技能确实需要
  `CHECKLIST.tmpl` 或 `VERDICT.tmpl`，另开后续波次。
- 把任何 Wave-3 技能塞进 Wave-2，即便其源语料恰好已齐。Wave-3 依赖
  Wave-2 metrics（见 §6.5），混在一起会打穿该门禁。
- 任何改动 Wave-1 确立的 hydrate 契约形状、selection path 或基础设施的
  行为。若 Wave-2 的实施 PR 需要这类改动，停下来升级讨论，不要扩范围。
