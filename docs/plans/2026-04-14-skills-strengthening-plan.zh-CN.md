# Skills 强化计划

## 1. 背景

Skills Integration 计划（B0-B8）已落地：25 个 catalog 技能通过
`.codex/skills/slipway/` 导出，adapter frontmatter 与 `provenance.yaml`
均已齐备。对照 `skills_ref` 后仍可见一个明确事实：catalog body 只保留了
源语料的很小一部分，仍属于个位数百分比量级。像
`gha-security-review`、`root-cause-tracing`、`independent-review`
这类当前导出主体，已经落在几十行量级，而对应源技能仍是数百到数千行。

蒸馏**本就**要求 body 轻量，但被裁掉的丰度没有迁移到 `references/` 或
`scripts/`，导致 catalog 目前缺失源技能依赖的"条件触发"语料。
仓库里唯一已经声明了 hydrate 的 owner——`context-assembly`——目前仍然指向
一个没有文件落地的 reference shelf，因此这次收紧契约时必须先把这个现有
owner 规范化，而不是在它旁边再叠出第二套 hydrate 形状。

这不是宣称 25 个 catalog 技能会在一轮里全部“补厚”，而是第一波强化。
Wave 1 只覆盖共享导出 / 运行时接线，以及 10 个最高杠杆的薄技能：
`context-assembly`、PR-1 里的 5 个 reference-heavy 技能，以及 PR-3 里的
4 个 typed-partial 技能。剩余 catalog 技能放到本轮 metrics 与 rendered-tree
evidence 审完后的后续波次。

按当前 inventory 规模估算，剩余 15 个 catalog 技能大概率还需要再做
2-3 个同量级强化波次。这只是计划级估算，不是锁死的排期；Wave 1 的 metrics
report 可能让这个估算收缩，也可能让它扩大。

交付上预期会拆成六个可独立合并的 PR，因为 PR-4 会刻意拆成 PR-4a / PR-4b，
保持运行时接线可审查、可回滚。顺序反映依赖关系：PR-0 解锁 PR-1/PR-2；
PR-3 虽然与 runtime wiring 相对正交，但仍属于 Wave 1 完成条件的一部分；
PR-4a 依赖 PR-1 的产物；PR-4b 依赖 PR-4a，以及 PR-3 先把 always-rendered /
on-demand 的边界冻结下来。

## 2. 非目标

- 不改动 `ResolveNextSkill` 或其契约。PR-4a 可以给
  `capability.Resolve()` 增加只读输出字段，但不会改变 `Resolve()`
  的决策语义，也不会改变 governed progression authority。
- 不新增 catalog 技能，数量仍为 25。
- 不改变生成侧 adapter frontmatter 契约
  （`.codex/skills/slipway/<id>/SKILL.md` 顶部）；PR-0 只扩大谁能拿到支撑
  文件。authoring 侧 `SKILL.md::hydrate_references` 会在 PR-1 / PR-4a 演化。
- 不修改 `skills_ref/` 内容。
- 本轮不会在 PR-3 的 T1/T2 调整之外再做第二次全局 target budget 放宽。
  若某个强化后的技能仍落在 warning band，优先通过显式 PR notes 或迁移到
  `references/` 处理，而不是再次整体抬升预算。

## 3. PR-0 — 支撑文件导出链（非 catalog 技能）

**目标。** 复用已经存在的 `emitSkillSupportFiles` 导出链，并放宽
`provenance.yaml` 的导出条件，让非 catalog 技能在模板侧存在支撑载荷时，
也能把 `provenance.yaml` 与 `references/` / `scripts/` 一起导出。

### 要改的文件
- `internal/toolgen/toolgen.go`：保留 governance / standalone / technique /
  catalog 各条现有 `emitSkillSupportFiles` 调用链，不把它描述成一次
  loop-hoist。真正要改的是把调用侧拥有的 `includeProvenance bool`
  gate 改成模板侧 presence check。为了对称，helper 可以统一使用
  "存在支撑载荷" 的判据（`provenance.yaml`、`references/`、`scripts/`），
  但本 PR 的实际行为变化只有：非 catalog 技能也能按模板侧情况导出
  `provenance.yaml`。
- `internal/tmpl/templates.go`：原则上无需生产代码变更；仅在模板侧判据需要
  额外覆盖时，补一条 `TemplateFS()` 枚举测试。

### 要补的测试
- `internal/toolgen/toolgen_test.go::TestEmitSupportFilesForNonCatalogSkills`
  - fixture：仅含 `provenance.yaml` 的 technique 技能模板。
  - 断言：渲染产物包含 `provenance.yaml`，`name:` 与目录一致。
- `internal/toolgen/toolgen_test.go::TestCatalogSkillsRetainProvenanceOnPresenceCheckMigration`
  - fixture：在模板侧 presence check 迁移前后分别生成 catalog 技能树。
  - 断言：当前本来就携带 `provenance.yaml` 的 catalog 技能，在迁移后仍然
    全部保留；PR-0 不得让现有 catalog coverage 回退。
- `internal/toolgen/toolgen_test.go::TestEmitSupportFilesWithoutProvenanceStillCopiesReferences`
  - fixture：技能模板含 `references/` 或 `scripts/`，但不含
    `provenance.yaml`。
  - 断言：支撑文件被复制；`provenance.yaml` 不会凭空生成。
- `internal/toolgen/toolgen_test.go::TestEmitSupportFilesSkipsEmpty`
  - fixture：无支撑文件的技能模板。
  - 断言：不创建空目录；不报错。
- `internal/toolgen/toolgen_test.go::TestGeneratedSkillTreeInventoryManifest`
  - 在临时目录生成 `.codex/skills/slipway/` 结果树，并将标准化后的
    `path -> file_kind,executable` inventory 与仓库内受控 golden manifest
    对比。`executable` 只在 POSIX 上断言；Windows 统一归一到平台稳定的
    non-exec sentinel。
  - 目标：让 CI 抓住意外的结构性漂移（文件缺失、意外新增、权限位变化），
    而不把每一次预期中的内容修改都变成长期 hash churn。语义层的内容漂移
    仍然交给 rendered-tree diff 审查和 feature 级 fixture 测试。

### 验收
- `init --tools codex --refresh` 后，凡模板侧存在 `provenance.yaml`
  的非 catalog 技能都能在 `.codex/skills/slipway/<id>/provenance.yaml`
  生成对应文件。
- 当前本来就会导出 `provenance.yaml` 的 catalog 技能，在模板侧
  presence-check 迁移后仍然全部保留该文件。
- catalog / governance / standalone / technique 四类技能现有的
  `references/` / `scripts/` 导出行为保持不变。
- `go test ./internal/toolgen/... -count=1` 通过。
- 生成技能树的结构性漂移可被受控 inventory manifest 在 CI 中拦住；
  语义层的内容漂移仍然通过 rendered-tree diff 审查与定向 fixture 覆盖把关。

---

## 4. PR-1 — 五个源超厚技能补 references/ + hydrate owner 规范化

**目标。** 将源深度的 95%+ 以条件触发的参考材料形式回收，
同时保持技能 body 在 tier 预算内。这里的“95%+”只是 authoring review
metric：要在 PR notes 里记录每个技能的 byte-ratio
（`rendered_reference_bytes / selected_source_bytes`）以及"被拒收 / 被压缩"
的源段落清单，而不是 CI gate。并行完成已声明的 `context-assembly`
hydrate owner 规范化，让 PR-4a 从一套 canonical、file-backed 契约起步。

### 蒸馏 rubric
- 保留条件触发的操作性内容：修复步骤、决策表、失败特征、工具调用 recipe、
  会改变操作者动作的 anti-pattern。
- 删除或压缩 narrative motivation、长案例、vendor overview，以及已经由
  精简版 `SKILL.md` 主体表达过的 prose。
- 与 PR-3 typed partial 的边界：凡是每次渲染都必须出现的 schema /
  checklist / procedure，留在 `SKILL.md` 或 typed partial；体量大且仅在
  特定条件下才需要的内容，移入 `references/`。

### 在 `internal/tmpl/templates/skills/<id>/references/` 下新增

| 技能 | References | 主要源 |
|------|------------|--------|
| `gha-security-review` | `pinning.md`, `oidc-trust-boundaries.md`, `self-hosted-runners.md`, `secrets-exfil-patterns.md` | getsentry/gha-security-review, trailofbits/agentic-actions-auditor |
| `root-cause-tracing` | `five-whys.md`, `parallel-hypotheses.md`, `triage-playbook.md` | superpowers/systematic-debugging, wshobson/parallel-debugging, trailofbits/debug-buttercup |
| `sast-orchestration` | `codeql-recipes.md`, `semgrep-recipes.md`, `sarif-merge.md` | trailofbits/codeql, trailofbits/semgrep, trailofbits/sarif-parsing, trailofbits/audit-augmentation |
| `supply-chain-audit` | `sbom-checklist.md`, `typosquat-patterns.md`, `transitive-pinning.md` | trailofbits/supply-chain-risk-auditor, alirezarezvani/dependency-auditor |
| `incident-response` | `severity-matrix.md`, `comms-template.md`, `postmortem-outline.md` | alirezarezvani/incident-commander + incident-response, sickn33/acceptance-orchestrator |

### 既有 owner 规范化
- 落地 `internal/tmpl/templates/skills/context-assembly/references/codebase-map.md`，
  让仓库里已经声明过 hydrate 的 owner 不再指向一个没有文件落地的 shelf。
  这属于契约规范化，不属于上面五个源深度回收轨道的一部分。

### 其他代码变更
- 各涉及技能 `SKILL.md`：对这 5 个新进入 hydrate 的技能，采用
  `context-assembly` 已经在用的描述性 `hydrate_references:` record 形状
  （`name`、`reason`），让 authoring/export metadata 在进入任何 registry
  compare gate 之前就先收敛成同一套 canonical form。
- 在 PR-4a 落地前，`hydrate_references:` 还没有 registry-backed compare
  gate 兜底。临时防线只有 `TestHydrateReferencesResolveToFiles` 与
  provenance 覆盖补强。由于 PR-4a 前没有任何 runtime surface 消费该字段，
  这个短暂 drift window 是可接受的；PR-1 要做的是先补上现有
  `context-assembly` owner 的悬空问题，而不是把它继续放大。PR-4a 落地后，
  同一条测试继续保留，但角色转为正交的文件存在性 gate；它不再是唯一的
  hydrate contract 检查，却仍然是唯一能证明 file-backed resolution 的检查。
- 各涉及 `provenance.yaml`：扩展 `inputs:`，每个新增 reference 指向其源
  行范围；`context-assembly` 在本 PR 里变成 file-backed 之后也纳入同样规则。
- `context-assembly/references/codebase-map.md` 在 PR-1 里保持 method-first。
  如果本轮需要可执行 guidance，直接指向仓库里已经存在的
  `slipway codebase-map` 命令，而不是先引入第二个 context-assembly
  入口。
- PR-1 收口说明必须写明：下一个允许改动 `hydrate_references:` frontmatter
  形状的 PR 是 `PR-4a`。PR-2 / PR-3 可以只读消费，但不能顺手改形状。

### 要补的测试
- `internal/toolgen/toolgen_test.go::TestCatalogSkillHasReferences`
  - 输入：上述 5 个技能 ID。
  - 断言：渲染后 `.codex/skills/slipway/<id>/references/` 非空；
    每个 `.md` 均以 `# ` 开头。
- `internal/toolgen/toolgen_test.go::TestHydrateReferencesResolveToFiles`
  - 输入：所有在 `SKILL.md` 中声明了 `hydrate_references:` 的 catalog 技能。
  - 断言：清单中每个条目均为对象；`name` 非空且在该技能内唯一；`reason`
    如出现则必须非空；`name` 必须是 basename，不得包含路径分隔符或 `..`；
    且必须能解析到该技能 `references/` 目录下的实际 `.md` 文件。
- Body 体积：继续复用
  `internal/engine/capability/gates_test.go::TestSizeBudgetsForRegisteredSkills`；
  该 gate 只约束 `SKILL.md`，不覆盖 references。
- References 体积：新增
  `internal/toolgen/toolgen_test.go::TestReferenceFileSizeBudget`，
  强制单文件 <= 24 KB、单技能合计 <= 64 KB。总量上限是字节预算，
  不是文件数量上限；它仍给多个中等体量 reference 留出空间，同时逼迫真实
  distillation 发生，避免在 `SKILL.md` 旁边再长出第二份 body。

### 验收
- 单个 reference 文件 <= 24 KB；单技能 references 合计 <= 64 KB（由
  `TestReferenceFileSizeBudget` 强制）。
- 渲染树 diff 能直接看见新增的 `references/` 目录，以及五个技能扩大的
  `provenance.yaml inputs:` 覆盖面；另外还能看见规范化后的
  `context-assembly/codebase-map.md`。
- 人工审查规则：任一 reference 文件不得与所属 `SKILL.md` body 形成
  >=50% 的逐行重复。这条保留在 rendered-tree diff 审查中执行，不进入 CI gate。
- PR notes 除 byte-ratio 表外，还要附一份简短的"源段落拒收 / 压缩清单"，
  说明哪些内容被有意留在外面。
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  通过。

---

## 5. PR-2 — 确定性脚本

**目标。** 把源技能以散文描述的可执行动作，落成仓库脚本供 agent 直接调用。

### 新增文件

| 脚本 | 归属技能 | 作用 |
|------|---------|------|
| `scripts/find-polluter-go.sh` | `root-cause-tracing` | Go 测试顺序污染二分 |
| `scripts/merge-sarif.sh`   | `sast-orchestration` | 基于 `jq` 的多源 SARIF 聚合 |
| `scripts/pin-actions.sh`   | `gha-security-review`| 将 `uses: foo@v1` 替换为 `uses: foo@<sha>` |

### 代码变更
- `internal/toolgen/toolgen.go`：若 helper 仍然全部采用 `.sh`，原则上不需要
  再做权限链路重构；`writeDeterministic` 已经是 `.sh` 输出的权限边界。
  因此本 PR 聚焦在补脚本与补回归测试。只有未来出现无 `.sh` 后缀但仍需
  可执行的 helper 时，才再引入显式 chmod 逻辑。
- `pin-actions.sh` 必须保持 offline-only。它依赖调用方显式传入一个受控、
  checked-in 的映射文件（例如 `--mapping <path>`）；未解析到的 ref 必须产
  生稳定报告并以非零退出，不允许退回 GitHub API、`gh` 或任何 tag-resolution
  网络请求。
- `pin-actions.sh` 不自带 Slipway 官方维护的通用 tag -> SHA 数据库。映射文
  件由调用它的仓库自己维护；Slipway 只提供 example schema / fixture 与
  usage contract。
- `pin-actions.sh` 是确定性的重写 helper，不是一个通用开箱即用的 pinning
  服务。若调用仓库没有 checked-in mapping，primary guidance surface
  仍然应该是 references；脚本必须显式失败，而不是假装自己具备普适可执行性。
- 若 helper 行为天然带语言约束，脚本命名应显式语言化。
  `root-cause-tracing` 本体仍保持语言中立；Go 专属 helper 只是其中一条
  可执行 recipe。未来同类 helper 统一占用
  `scripts/find-polluter-<lang>.sh` 命名空间，避免 Go 版本变成一次性特例。
- `context-assembly` 在本轮直接复用仓库里已有的
  `slipway codebase-map` 命令；PR-2 不新增 `scripts/codebase-map.sh`，
  避免同时存在两个 repo-mapping 入口。
- 带外部依赖的脚本必须在依赖缺失时 fail fast，并给出可操作的报错
  （例如 `merge-sarif.sh` 对 `jq` 的要求）。
- Wave 1 的 helper script 一律按 POSIX-only 处理。归属它们的 `SKILL.md` /
  `references/` 必须明确标注“这是可选的 POSIX helper”，并给非 POSIX agent
  保留 prose fallback，不能暗示它们是跨平台通用入口。

### 要补的测试
- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit`
  - 断言：渲染后的 `scripts/*.sh` 具有 `0o111` 位。
- `internal/toolgen/toolgen_test.go::TestScriptSyntaxCheck`
  - 对每个脚本执行 `bash -n <script>`；非零退出即失败。
- `internal/toolgen/toolgen_test.go::TestScriptFixtureContracts`
  - `merge-sarif.sh`：合并 fixture SARIF 输入，并断言输出结构稳定。
  - `pin-actions.sh`：对 fixture workflow 做 tag->SHA 改写并断言结果；
    同时断言缺少 `--mapping` 时 usage 报错稳定且可操作。
  - `find-polluter-go.sh`：至少断言缺少必要输入时，usage / dependency
    报错是稳定且可操作的。

### 验收
- 三个脚本全部通过 `bash -n`。
- POSIX 文件系统上可执行位存在（Windows 上跳过该断言）。
- 每个脚本至少有一条确定性的 fixture 或 failure-contract 测试；
  PR-2 不接受仅靠 syntax check 的覆盖。
- `init --tools codex --refresh` 会把这些脚本写入生成后的技能树。

---

## 6. PR-3 — 扩展 typed partial + 放宽 target budget

**目标。** 在 *已经落地* 的 typed-template 装配器
（`internal/toolgen/toolgen.go` 的 `renderCatalogSkill`）基础上，给四个源
超厚技能扩写 body，并放宽 T1/T2 的 target budget；现有 hard-max +
`size_rationale` 规则不动。

### 已在仓库中的基线
- 装配顺序 `SKILL.md` -> `PROSE.tmpl` -> `CHECKLIST.tmpl` ->
  `VERDICT.tmpl` 已经在 `renderCatalogSkill` 实现，并有
  `internal/toolgen/toolgen_test.go` 覆盖。
- `independent-review` 已经携带 `PROSE.tmpl` + `CHECKLIST.tmpl` +
  `VERDICT.tmpl`，是参考例；PR-3 不动它。

### 要扩写的分部模板

| 技能 | 要新增的分部 | 作用 |
|------|--------------|------|
| `spec-trace` | `CHECKLIST.tmpl` | spec->code 覆盖矩阵 |
| `threat-modeling` | `PROSE.tmpl`, `CHECKLIST.tmpl` | STRIDE 散文 + 资产表 |
| `coverage-analysis` | `VERDICT.tmpl` | gap-report schema |
| `security-review` | `CHECKLIST.tmpl` | 合并 insecure-defaults + sharp-edges 项 |

### 与 references 的边界
- Typed partial 一旦 attachment mode 命中，就始终会被拼装进最终
  `SKILL.md`。
- `references/` 仍然是文件级、按需读取的内容，不替代必需的
  checklist / report-schema / procedure。
- `security-review` 的 `CHECKLIST.tmpl` 是有意保持 always-rendered 的：
  它属于正常 review attachment surface，而不是条件 reference。

### 预算策略调整
- 在 schema 文档、distillation checklist、2026-04-11 plan/delivery
  双语文档，以及 capability size gate 中同步放宽 target budget：
  - T1：2 KB -> 2.5 KB
  - T2：3 KB -> 3.5 KB
  - T3：保持 1.5 KB（T3 技能仍维持轻量；T3 的 body 强化靠 references，
    不靠 target 放宽）。
- hard-max 仍维持 6 / 8 / 3 KB，且 **仅在超过 hard-max 时** 才必须提供
  `size_rationale`。PR-3 不把 `size_rationale` 提升到 warning band，也不
  承诺新的 runtime warn/critical surface。
- `security-review` 是 T1 技能，因此本轮真正要看的压力点是 T1 的 2.5 KB
  target，而不是 T2。若它在 `CHECKLIST.tmpl` 落地后仍然处于 warning band，
  本轮不再继续抬 tier budget；要么明确接受 warning-band 并写进 PR notes，
  要么把溢出内容移入 `references/`。

### 要补的测试
- `internal/engine/capability/gates_test.go`
  - 更新 `tierSizeBudget` 与 `TestSizeBudgetsForRegisteredSkills`，使其反映
    放宽后的 target。warning-band 仍然只走 `t.Logf`，不升级为错误。
  - `TestTierSizeBudgetRequiresRationaleAboveHardMax` 语义不变：rationale
    仍然只在越过 hard-max 时必填。
- `internal/toolgen/toolgen_test.go::TestTypedPartsRendered`
  - 对上述 4 个技能，断言 CHECKLIST / PROSE / VERDICT 章节标题
    出现在拼装后的 `SKILL.md` 中。

### 验收
- size / schema / binding / provenance 四类 capability gate 在放宽 target 后
  仍保持 green。
- 没有任何技能在缺少 `size_rationale` 的情况下越过 hard-max。进入
  warning band 的技能只记录日志、不阻塞，与现有 gate 语义一致。
- `security-review` 在 `CHECKLIST.tmpl` 落地后的最终渲染字节数必须被记录；
  若它仍然落在 warning band，PR 说明里要显式写出来，而不是默认认为 target
  放宽已经吸收了这部分体量。

---

## 7. PR-4 — 三条选择路径上的 hydrate 接线

**目标。** 把静态 `references/` 升级为条件触发的 hydration。代码里实际
存在 **三条不同的选择路径**，PR-4 必须在三条路径上都接通 hydrate，而不
只是 auto-route。

### 既有选择路径（非本计划新造）
1. **Auto-route** — `capability.Resolve()` -> `pickRoute()` 仅对
   `BindingCommandAuto` / `BindingCommandView` 绑定触发。当前经由
   `resolveEffectiveRouteMode` / `resolveEffectiveRouteView` 对外浮现。
2. **Manual explicit** — 用户传 `--mode=<skill-id>` 或
   `--view=<skill-id>`，由 `validateRouteMode` / `validateRouteView` 校验。
   对只读 surface 来说，`ValidViewsForCommand` 会同时放行
   `BindingCommandManual` 与 `BindingCommandView`。这条路径保留用户显式
   选择的 skill id，本身不依赖 `pickRoute()`。
3. **Support / host-hint** — `next` 由 `appendCatalogHints` 从
   `resolution.Supports` 渲染 technique hints。并非 routed surface。

首波技能覆盖了全部三条路径，且部分技能本身横跨多条路径：
`gha-security-review`、`sast-orchestration`、`supply-chain-audit`
覆盖 manual-explicit；`root-cause-tracing` 首先通过 support / host-hint
在 `next` 上落第一批 hydrate，同时保留现有 `repair` auto-route 绑定不变；
`incident-response` 则是本轮主要的 `status` / `health` command-default view
案例。按当前已发货代码，这条 route 是由 command context 选出来的；
change-selected surface 只决定是否会走 auto-route。

### 交付拆分
- **PR-4a**：在三条路径上填充 hydrate references，并在受影响的命令上只读
  浮现；暂不引入 `--hydrate` 展开。
- **PR-4b**：PR-4a 输出稳定后，再增加 `--hydrate` 正文展开能力。

### Authority 与 compare gate（单一真相）
- Registry 仍是 binding metadata 的 runtime authority；PR-4a 之后，
  hydrate metadata 也一并收敛到 registry。为保持与现有
  `context-assembly` owner 一致，PR-4a 不把它压扁成裸字符串，而是引入
  typed `Skill.HydrateReferences []HydrateReference`（至少含 `Name`、
  `Reason`）作为运行时权威。
- Frontmatter `hydrate_references:` 退化为这些 typed records 的导出镜像。
- `HydrateReference.Name` 在 authoring 阶段仍是 skill 内 basename，但
  runtime 输出与等价比较一律使用防冲突的 skill-relative key
  `<skill-id>/<name>`；去重与排序也只在这个完整 key 上做，不能只看 basename。
- 在 `internal/engine/capability/gates_test.go` 新增
  `TestHydrateReferencesMirrorRegistry`，仿照已有的 binding-compare gate
  （`TestFrontmatterMirrorsRegistryBindings`）：对每个 catalog 技能，要求
  `SKILL.md::hydrate_references` 与 `registry.HydrateReferences`
  在按 `name` 排序后逐记录相等。这条是之前缺失的 1:1 契约。
- PR-4a 之后继续保留 `TestHydrateReferencesResolveToFiles`，把它作为正交的
  file-backed resolution gate。mirror-registry equality 只能证明契约同步，
  不能证明被引用的文件还真实存在于磁盘上。

### 代码变更
- `internal/engine/capability/`
  - 新增 `HydrateReference` 结构体与
    `Skill.HydrateReferences []HydrateReference`；初值取自 PR-1 规范化后的
    SKILL.md frontmatter，并覆盖已存在的 `context-assembly` owner。
  - 在 `Resolve()` 发出 route 或 support 时填充
    `Resolution.HydrateReferences`：选中 skill + 所有 `Supports` 条目的
    refs 展平成稳定排序、去重后的 skill-relative key
    `<skill-id>/<name>`。Resolver 对内核仍然只读。
  - 新增 helper
    `capability.HydrateReferenceKeysForSkill(reg, skillID)`，让
    manual-explicit 路径与 `next` hint 渲染都能按 skill id 直接查
    skill-relative key，不必经过 `Resolve()`。
  - 更新 resolver 测试：PR-4a 之后，reserved 的只剩 `llm_tiebreak`；
    auto-route 场景的 hydrate 断言变成真实断言。
  - 增加 routing-invariant 测试矩阵：PR-4a 可以新增
    `HydrateReferences`，但不得改变既有绿线场景下的 `Route` / `Supports`
    输出。
- `cmd/route_flags.go`
  - 新增 `resolveEffectiveRouteHydrate(command, explicitMode, signals...)`
    与 `resolveEffectiveViewHydrate(...)`。优先级：`explicit != ""`
    且已通过校验时，走 `HydrateReferenceKeysForSkill` 查 registry；如果该显
    式 skill 的 hydrate refs 本来就是空，就直接返回空切片，**不得**回退到
    `Resolve()`。只有 `explicit == ""` 的路径才退回 resolver 输出。这是
    manual-explicit gap 的修复点。
- `cmd/hydrate_render.go`
  - 新增一个共享的 hydrate render helper，同时服务 text / JSON surface。
    `review`、`status`、`health`、`validate`、`next` 都必须复用它，不能各自
    手写 `Hydrate:` 文本、JSON 字段填充、排序规则或空切片省略逻辑。PR-4a
    结束后，delimiter、排序以及后续 size-cap 行为都应该只有这一处权威实现。
- `cmd/next_skill_view.go`、`cmd/next.go`
  - 扩展 `appendCatalogHints` 与 `techniqueHint`，让每个 support hint
    可携带 `hydrate_references[]`；`next --json` 继续把 hydrate keys 嵌在
    各自的 `technique_hint` 对象下，不产生顶层聚合字段；`cmd/next.go`
    只在所属 hint 下方输出稳定的缩进 `Hydrate:` 行，而不是另造一个顶层
    `next` surface。
- `cmd/review.go`、`cmd/status.go`、`cmd/health.go`
  - 当该命令面的有效 resolution 返回非空 references 时，渲染
    `Hydrate:` 行（文本）与字段（JSON）。JSON 形状：
    `"hydrate_references": ["gha-security-review/pinning.md", ...]`；文本形状
    （单行）：
    `Hydrate: gha-security-review/pinning.md, gha-security-review/oidc-trust-boundaries.md`。
- `cmd/validate.go`
  - PR-4a 对 `validate` 保持 JSON-only：当前它本来就是 JSON surface，因此
    本轮只新增 `"hydrate_references": [...]` 字段，不新造 text renderer。
- `cmd/review.go`、`cmd/status.go`、`cmd/health.go`
  - PR-4b：新增 `--hydrate`，逐文件原样打印 reference 正文；每个文件前
  加 `===== SLIPWAY HYDRATE: <skill-id>/<name> =====` 分隔符，方便
  golden test 固定锚点且降低正文误撞的概率。helper 在渲染前校验
  hydrate key 不含 `=` 或换行。单次命令输出的 hydrate body 总量上限为
  32 KB；超过该上限时，命令必须用稳定错误返回被选 keys 与字节估算，
  而不是把超大上下文直接倾倒给消费它的 agent。

### 首批 binding table（含选择路径）

| 技能 | 绑定类型 | 选择路径 | 初始 hydrated refs | 首批呈现命令面 |
|------|---------|----------|--------------------|----------------|
| `gha-security-review` | CommandManual on `review`、`repair` | Manual explicit via `--mode=gha-security-review` | `pinning.md`, `oidc-trust-boundaries.md`, `self-hosted-runners.md`, `secrets-exfil-patterns.md` | `review` |
| `root-cause-tracing` | `repair` 上的 CommandAuto；`wave-orchestration` 上的 TechniqueHint / HostEmbedded | 第一批 hydrated output 先通过 `appendCatalogHints` 的 support / host-hint 路径落在 `next`；现有 `repair` auto-route 绑定在本轮保持不变 | `five-whys.md`, `parallel-hypotheses.md`, `triage-playbook.md` | `next`（`wave-orchestration` host）；`repair` 属于 existing, unchanged |
| `sast-orchestration` | CommandManual on `review`、`validate`、`repair` | Manual explicit via `--mode=sast-orchestration` | `codeql-recipes.md`, `semgrep-recipes.md`, `sarif-merge.md` | `review`、`validate` |
| `supply-chain-audit` | CommandManual on `review`、`repair`、`status` | `review` / `repair` 通过 `--mode`，`status` 通过 `--view` 显式选择（read-only surface 通过 `ValidViewsForCommand` 接纳 manual binding） | `sbom-checklist.md`, `typosquat-patterns.md`, `transitive-pinning.md` | `status`、`review` |
| `incident-response` | CommandView on `status`、`health` | 在 change-selected `status` / `health` surface 上，通过 `pickRoute` 走 command-default auto view | `severity-matrix.md`, `comms-template.md`, `postmortem-outline.md` | change-selected `status`、`health` |

编写测试前先用 registry constructor（`sastOrchestration`、
`ghaSecurityReview`、`supplyChainAudit`、`rootCauseTracing`、
`incidentResponse`）核对每一行当前绑定集合；如有出入，改表格、不改
registry（registry 是权威）。

### 要补的测试
- `internal/engine/capability/gates_test.go::TestHydrateReferencesMirrorRegistry`
  - 对每个 catalog 技能：frontmatter `hydrate_references:` 与
    `Skill.HydrateReferences` 作为按 `name` 排序的 typed records 相等。
- `internal/engine/capability/resolver_test.go`
  - Auto-route case：`incident-response` 在 `status` view 上通过
    `Resolution.HydrateReferences` 返回
    `incident-response/severity-matrix.md`。
  - Support case：`wave-orchestration` host 上通过 support 路径返回
    `root-cause-tracing` 的 refs。
  - 稳定性 case：返回的 references 已排序、已去重。
  - Invariant case：代表性 routed 场景的 `Route` / `Supports` 与 PR-4a
    之前完全一致；新增的只有 `HydrateReferences`。
- `cmd/route_flags_test.go`
  - `resolveEffectiveRouteHydrate` 对 `--mode=gha-security-review`、
    `--mode=sast-orchestration` 返回预期 refs，且不经过 `Resolve()`。
  - `resolveEffectiveViewHydrate` 对 `--view=supply-chain-audit`、
    `--view=incident-response` 在只读命令面上返回预期 refs，且不经过
    `Resolve()`。
  - 显式 skill 若本身没有 hydrate refs，返回空切片，且不得回退到
    resolver 的 auto-route 输出。
- `cmd/hydrate_view_test.go`（新建，命名与 `route_flags_test.go` 对齐）
  - 黄金测试：`review --mode=gha-security-review` 列出
    `gha-security-review/pinning.md`。
  - 黄金测试：`status --view=incident-response` 列出
    `incident-response/severity-matrix.md`。
  - 黄金测试：`next` 在 `wave-orchestration` host 上在 technique-hint
    区块以缩进 `Hydrate:` 行列出 `root-cause-tracing` 的 refs，并在 JSON 的
    该 hint 对象下保留 `hydrate_references[]`。
  - 黄金测试：`next` 的 hint 区块渲染顺序稳定：已有 technique hint 不被重排，
    新追加的 support hint 顺序确定，且每个 hint 内的 hydrate key
    均为稳定排序。
- `cmd/hydrate_flag_test.go`（新建，PR-4b 才落地）
  - `--hydrate` 输出包含每个 reference 的 H1，以及
    `===== SLIPWAY HYDRATE: <skill-id>/<name> =====` 分隔符。
  - 超量 case：若被选 hydrate bodies 合计超过 32 KB，命令必须以稳定的
    `hydrate_output_too_large` 契约失败，而不是输出部分正文或截断正文。

### 验收
- Manual-explicit：`review --mode=gha-security-review`、
  `validate --mode=sast-orchestration`、`status --view=supply-chain-audit`
  均能浮现 hydrate key（PR-4a）；其中 `validate` 仅在 JSON 输出里浮现。
- Auto-route：change-selected `status` / `health` surface 继续沿用现有
  command-default 的 `incident-response` route；PR-4a 只是在这条既有 route
  上新增 hydrate refs，不宣称引入新的 change-derived routing signal。
  diagnostics-only 输出仍然只保留显式 `--view`。
- Support / host-hint：`wave-orchestration` host 场景下 `next` 浮现
  `root-cause-tracing` 的 refs，并保持 technique-hint 区块渲染顺序稳定；
  JSON 里按 hint 嵌套 `hydrate_references[]`，文本里用缩进 `Hydrate:` 行呈现。
- `--hydrate` 在所选 body 总量 <= 32 KB 时按 reference body 原样打印；
  超过上限时，以稳定方式返回 keys 与 size diagnostics，而不是输出超大上下文
  （PR-4b）。
- `TestHydrateReferencesMirrorRegistry` 通过；registry 保持单一权威。
- 现有 `cmd/...` / `capability` golden test 无回归。

---

## 8. 执行顺序与门禁

1. **PR-0 最先。** 影响面最小；所有后续 PR 依赖其支撑文件链。
2. **PR-1 与 PR-2 可并行。** references 与 scripts 互不相干。
   在这个窗口里，`hydrate_references:` frontmatter 只能由 PR-1 改；下一个
   允许改它的 PR 是 `PR-4a`。
3. **PR-3 保持正交，但仍属于 Wave 1 的必做项。** 需在 `PR-4b` 之前完成，
   这样 always-rendered 与 on-demand 的边界会先冻结，再去定 hydration UX。
4. **PR-4a 在 PR-1 之后。** `PR-4b` 只在 `PR-4a` 输出与测试稳定后再做。
5. 每个 PR 合并前跑三道硬门禁：
   - `go test ./... -count=1`
   - 在本仓库执行 `init --tools codex --refresh`，diff 审查
     `.codex/skills/slipway/` 结果树。受控 inventory manifest 会自动拦截
     意外的结构性漂移；人工 diff 审查仍保留，用于语义层面的确认。
   - 针对改动面的命令烟测（`next --preview --json`、`review --json`、
     `validate --json`、显式 manual-view 检查
     （`status --json --view <id>`、
     `health --json --governance --view <id>`），以及 change-selected
     default-route 检查
     （`status --json --change <slug>`、
     `health --json --governance --change <slug>`）能给出预期 routed /
     hydrate surface，且无意外回归。
6. 任何修改这组 plan family 的 PR，都必须在同一批次同步更新英文版与
   `zh-CN` 版；Wave 1 不接受翻译漂移。
7. **Wave 2 的触发条件写死。** Wave 1 merge 后 7 天内，必须产出一份短 metrics
   report，覆盖 rendered reference byte ratio、源段落拒收清单、warning-band
   skills、hydrate smoke 结果，以及 support-file drift。只有这份报告和
   rendered-tree evidence 一起审完后，才决定 Wave 2 的范围。

## 9. 不在范围

- Claude adapter 导出刷新（待 Codex adapter 稳定后单独跟进）。
- 重写 `skills_ref/` 的 provenance；本计划仅向其追加指针。
- 延期源 `alirezarezvani/prompt-governance` 维持延期状态。
