# 仅保留知识的重构方案

**状态.** 草案。本方案只能在
`2026-04-15-skills-wave3-plan*.md` 的 PR-1 / PR-2 / PR-3 以及 Wave-3 closeout
report 完成评审之后落地。当前前置全部落地后，本方案就是删除
`docs/distillation/`、provenance 元数据与剩余 checked-in dead source 的唯一
cleanup 方案。

## 1. 动机

本项目的最终产物是"蒸馏后的知识"——procedure、checklist、铁律、证据契约——
全部使用作者自己的表述。上游归属类元数据（`provenance.yaml`、
`provenance_ref` frontmatter、`docs/distillation/`）在第一轮蒸馏期间有用，
但现在已不属于目标终态。继续把这些制品保留在仓库里，会带来两项本项目不再
愿意承担的成本：

- **持续维护.** 每新增一个 skill、reference 或 script，都要补上对应的 provenance
  行、ledger 条目和 frontmatter 字段，这些都对最终行为毫无贡献。
- **源耦合.** 以源命名的元数据让蒸馏产物在心智上始终绑定上游仓库名，即便正文
  是独立撰写的。用户的明确目标是"只包含知识"的项目——"功能一样，表述不同"。

`skills_ref/` 保留在仓库中作为 curator 的"工作台"（未来再导入知识时的参考）。
它不是运行时依赖，本方案落地后也不再被任何测试读取。

## 2. 非目标

- 不修改任何 catalog skill 的正文、trigger 子句、bindings、hydrate references
  或 evidence 契约。知识表面维持不变。
- 不修改 `skills_ref/` 内容。
- 不修改 route-surface / `surfaces.go` 权威、catalog registry 契约、hydrate
  契约或 tier 预算。
- 不重命名任何现有 skill 或 host。
- **不引入新的替代性元数据面。** 最终状态是删除，而不是迁移。

## 3. 最终状态

本方案落地之后：

- `internal/tmpl/templates/skills/` 与 `.codex/skills/slipway/` 下都不存在
  `provenance.yaml`。
- 不存在 `docs/distillation/`。
- `internal/tmpl/templates/skills/` 与 `.codex/skills/slipway/` 下的任何
  `SKILL.md` frontmatter 中都不含 `provenance_ref`。
- 任何 Go 类型不携带 `ProvenanceRef` 字段；任何构造函数不再赋值。
- `internal/engine/capability/by_source_test.go` 不存在。
- 那些只为已删除元数据服务的 provenance 加载 / 覆盖辅助逻辑不再保留。
- `registry_b2.go`、`registry_b3.go`、`registry_b4.go`、`registry_b5.go`
  中不再出现 `docs/distillation/catalog.md` 注释。
- `differential-review` 的 checked-in dead source 会同时从模板树和
  checked-in generated mirror tree 中消失。
- 任何活跃的 `docs/plans/*.md` 都不再要求后续工作书写或维护 provenance 相关
  制品。
- `skills_ref/` 原样保留作为 curator 的参考材料。
- 不存在任何测试或 gate 继续断言已删除 provenance 制品的存在。

## 4. 范围

### 4.1 文件系统删除

| 路径 | 动作 |
|------|------|
| `internal/tmpl/templates/skills/*/provenance.yaml` | 删除（25 个文件）|
| `.codex/skills/slipway/*/provenance.yaml` | 删除（25 个 checked-in mirror 文件） |
| `docs/distillation/` | 递归删除 |
| `internal/tmpl/templates/skills/differential-review/` | 删除 dead checked-in source |
| `.codex/skills/slipway/differential-review/` | 删除 dead checked-in mirror |

### 4.2 Go 代码修改

- 从 `internal/engine/capability/registry.go` 移除 `Skill.ProvenanceRef` 字段。
- 从 `registry_default.go`、`registry_b2.go`、`registry_b3.go`、`registry_b4.go`、
  `registry_b5.go` 移除每一处 `ProvenanceRef: "provenance.yaml"` 赋值。
- 删除 `internal/engine/capability/by_source_test.go`。
- 移除 `internal/engine/capability/provenance.go` 及相邻测试里那些只为已删除
  provenance 制品服务的加载 / 覆盖辅助逻辑。**不保留替代性的 stub。**
- 移除 `registry_b2.go`、`registry_b3.go`、`registry_b4.go`、`registry_b5.go`
  中的 `// See docs/distillation/catalog.md …` 注释。改成简短批次说明
  （如 `// B3 security cluster.`）或直接去掉。

### 4.3 模板与 checked-in mirror 修改

- 从 `internal/tmpl/templates/skills/` 下所有 `SKILL.md` frontmatter 中移除
  `provenance_ref: provenance.yaml`。
- 在同一个 PR 里删除模板树中的所有 `provenance.yaml` 文件；不要留下空占位。
- 在同一个 PR 里刷新 checked-in 的 `.codex/skills/slipway/` mirror，让仓库内
  生成态 `SKILL.md` 也同步去掉 `provenance_ref`，并在同一次刷新里删除
  checked-in mirror 下的 `provenance.yaml` 文件；不要让 checked-in mirror
  落成比模板树更旧的脏状态。
- 在同一个 PR 里删除 checked-in 的 `differential-review` 模板 / mirror 目录。
  route-surface PR-3 已经移除了 runtime authority；本方案负责删除 dead source。
- 若某个仍然存活的 skill 正文还把已删除的 provenance 制品当成交付契约
  （目前已知 `variant-analysis` 仍要求记录到 `provenance artifact`），则在
  同一个 PR 里做去掉该死契约所需的最小正文修订。
- 不把本 PR 扩大成正文重写；只做元数据删除本身所需的最小措辞清理。

### 4.4 测试修改

- 删除 by-source markdown coverage gate
  （`internal/engine/capability/by_source_test.go`）。
- 删除 `internal/toolgen` 与 `internal/engine/capability` 下那些只断言已删除
  provenance 制品存在性的测试 / helper。
- 保留覆盖 hydrate、bindings、surfaces、size 预算、脚本契约、frontmatter 对
  registry 镜像的测试，但需要从期望的 frontmatter 结构中剥离 `provenance_ref`。
- 只刷新这次 cleanup PR 实际触到的 golden。最低要求是文件系统删除完成后，用
  `UPDATE_GOLDEN=1` 刷新
  `internal/toolgen/testdata/skill_tree_inventory.codex.golden`；不把更早
  route-surface PR 引入的其它 golden 变化混进本方案责任边界。

### 4.5 方案文档修改

- `2026-04-15-route-surface-refactor-plan*.md`：删除那些在本 PR 落地后会过期
  的过渡性 cleanup 叙事，并把最终删除 / cleanup 步骤指向本方案。
- `2026-04-15-skills-wave2-plan*.md`、
  `2026-04-15-skills-wave3-plan*.md`：删除要求后续工作书写或更新 provenance
  制品的语句，并让 Wave-3 closeout 指向本方案。
- 英文与 zh-CN 镜像同步修改。

## 5. 执行顺序

在 route-surface、Wave-2、Wave-3 全部完成之后，以单个 bundled PR 落地：

1. **代码字段与注释。** 移除 `Skill.ProvenanceRef`、相关赋值、by-source
   coverage gate，以及 dead 的 provenance-only helpers。
2. **模板与 checked-in 生成树。** 从所有 `SKILL.md` 去掉 `provenance_ref`，
   删除 `provenance.yaml` 文件，完成删除 `provenance artifact` 契约所需的
   最小正文清理，刷新 checked-in 的 `.codex/skills/slipway/` mirror，并删除
   checked-in 的 `differential-review` dead source 目录。
3. **文件系统清理。** 删除 `docs/distillation/`。
4. **测试与 golden。** 删除 provenance-only 断言，调整 frontmatter fixture，
   只刷新这次 cleanup 实际触到的 golden（最低包含 codex skill-tree golden）。
5. **方案文档。** 把 route-surface / Wave-2 / Wave-3 计划族统一指向本
   cleanup PR，并删除那些仍把已删除元数据当成未来待办的过期 cleanup 叙事。
6. **验证。** `go vet ./...`、`go test ./... -count=1`，以及在临时目录下
   `init --tools codex --refresh` 确认生成的 skill 树不再输出被删除的文件。

## 6. 验收

- `rg -n "provenance.yaml" internal/tmpl/templates/skills .codex/skills/slipway`
  无命中。
- `rg -n "provenance_ref" internal/tmpl/templates/ .codex/skills/slipway/`
  无命中。
- `rg -n "provenance artifact" internal/tmpl/templates/skills .codex/skills/slipway`
  无命中。
- `rg -n "docs/distillation" internal/ docs/plans/2026-04-15-route-surface-refactor-plan*.md docs/plans/2026-04-15-skills-wave*.md`
  无命中。
- `rg -n "by-source.md|catalog.md" internal/engine/capability/` 无命中。
- `internal/tmpl/templates/skills/differential-review/` 与
  `.codex/skills/slipway/differential-review/` 均不存在。
- `go test ./... -count=1` 通过。
- 生成的 codex skill 树在任何 skill 目录下都不含 `provenance.yaml`。
- `skills_ref/` 未被触碰。

## 7. 与现有方案的关系

- **Route-surface / Wave-2 / Wave-3** —— 仍然是前置条件；本方案不改它们的顺序，
  只是它们 closeout 之后的 cleanup PR。
- **Wave-3 closeout** —— 一旦评审通过，本 bundled PR 就成为
  `docs/distillation/`、provenance 元数据与 `differential-review`
  剩余 dead source 的唯一授权清理步骤。
- **Task #10 golden 刷新** —— 并入本方案 §5.4。

## 8. 超出范围

- 除元数据删除本身所需的最小措辞清理外，不重写蒸馏正文。
- 不重新设计 catalog registry、hydrate 契约或 route surface。
- 不修改 `skills_ref/` 内容。
- 不引入替代性元数据格式。未来若再导入知识时需要 provenance，那是新方案，不
  在本方案之内。
