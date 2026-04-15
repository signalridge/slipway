# Distillation 硬切计划

**状态。** 提议中。若采纳，本计划将删除 `docs/distillation/` 这一 live
contract surface，并用 `internal/engine/capability/` 下的 code-local artifacts
以及现有 skill-local `SKILL.md` / `provenance.yaml` 源树承接其机器可读职责。

本计划将取代所有把 `docs/distillation/*` 视为必需 live authority 的 active
plan 表述，包括：

- `2026-04-11-skills-integration-plan*.md`
- `2026-04-11-skills-integration-plan-delivery*.md`
- `2026-04-14-skills-strengthening-plan*.md`

本计划**不**改变 Slipway 的 progression kernel。`ResolveNextSkill` 仍然是
唯一的 progression authority。

除非另有说明，本计划中所有 `*.md` 文档 / 计划 glob 都同时覆盖 EN 与 zh-CN
变体（若两者都存在）。

## 1. 问题

`docs/distillation/` 现在同时承担了三种角色：

- machine-readable contract input
- human-readable design/reference prose
- rollout bookkeeping

这带来四个具体问题：

- `internal/engine/capability/by_source_test.go` 会把
  `docs/distillation/by-source.md` 当成 markdown table 输入来解析，所以一次面向
  prose 的文档编辑也可能无意间改掉 CI gate。
- `docs/distillation/schema.md` 与 `docs/distillation/pr-checklist.md`
  在描述 live contract / gate 行为，但真正的 enforcement 已经落在 Go types、
  skill-local frontmatter 与 tests 里。
- `docs/distillation/catalog.md` 与 `docs/distillation/routed-surfaces.md`
  复制了 code 中已有的 registry / route truth，因此天然会漂移。
- 仓库已经明确希望最终删除 `docs/distillation/`，继续把它当 contract surface
  维护，只会制造一棵我们已经知道是临时态的第二 authority tree。

## 2. 目标

- 完整删除 `docs/distillation/`。
- 将 source corpus 的 reverse index 移到 code-owned 的 machine artifact。
- 保留每个 skill 的 source attribution 在各自的 `provenance.yaml` 中。
- 把 CI gates、comments、以及 active plan docs 全部回锚到 code-local
  authorities。
- 不引入新的 generated markdown mirror，也不保留任何兼容尾巴。

## 3. 非目标

- 不新增 catalog skills。
- 不改变 progression、routing、或 command 语义。
- 不大改 `provenance.yaml`；本计划只调整 repo-level reverse index 及其周围的
  docs surface。
- 不再造一棵只是把 `docs/distillation/` 换路径的新 documentation tree。
- 不做 “YAML -> markdown” 的生成链路去服务一个已经确定要删除的 docs surface。

## 4. 最终状态

### 4.1 Authority map

| 旧 surface | 最终 authority | 说明 |
|------------|----------------|------|
| `docs/distillation/by-source*.md` | `internal/engine/capability/source_index.yaml` | 机器可读的 source corpus reverse index |
| `docs/distillation/catalog*.md` | `internal/engine/capability/registry*.go` + registry tests | 不再保留人工维护的镜像表格 |
| `docs/distillation/routed-surfaces*.md` | `internal/engine/capability/surfaces.go` + command consumers/tests | route-surface authority 会先在代码里落地；本计划只删除重复文档 |
| `docs/distillation/schema*.md` | 拥有该 contract 的 Go types、validators 与 gate tests（`provenance.go`、`gates_test.go`、frontmatter compare tests、route tests） | contract 与 enforcing code 共址 |
| `docs/distillation/domains/*.md` | skill-local `SKILL.md`、`provenance.yaml`、typed templates、`references/` | 不再保留 repo-wide reverse mirror |
| `docs/distillation/pr-checklist*.md` | CI gates + active plan docs 内的 acceptance checklist | 不再保留单独的 distillation checklist 文档 |
| `docs/distillation/source-coverage-ledger-template*.md` | 需要时写入 PR description 的 artifact，而不是 checked-in contract file | 从仓库删除 |

当前 `by-source.md` 末尾附带的 coverage-snapshot table 只承担历史性
bookkeeping，不是 live authority 输入。本计划不会把它迁入
`source_index.yaml`；历史快照留在 git history 中，必要时再由 plan / PR
artifacts 承接。

### 4.2 新的 source index contract

引入 `internal/engine/capability/source_index.yaml`，作为 authoritative source
corpus 的唯一 repo-level reverse index。

初始形状：

```yaml
sources:
  - source: <vendor>/<source-skill>
    disposition: <standalone | posture-only | partial-only | view-only | route-only | absorbed | deferred>
    skills: [<catalog-skill-id>, ...]
    surfaces: [<non-catalog-surface-id>, ...]
    status: <B1 | B2 | B3 | B4 | B5 | B6 | shipped | n/a>
    notes: <optional human note>
```

规则：

- `source` 全文件唯一。
- `disposition` 是 enum，不允许自由文本。
- `skills[]` 只能引用已注册的 catalog skill IDs。
- `surfaces[]` 只能引用仍然存在、且由代码拥有 authority 的 non-catalog
  landing，例如 `review-queue`、`observability-query`。
- 如果某一行当前的落点只是 dead override，或只是尚未成为真实 surface 的未来
  设想，则必须写进 `notes`，而不是强行塞进 `surfaces[]`。按当前代码事实，
  `trailofbits/second-opinion` 与 `openai/sentry` 这类行属于该范畴。
- `skills[]`、`surfaces[]`、`notes` 至少有一个非空，保证每一行都有明确落点或
  defer 理由。
- 文件按 `source` 做确定性排序。
- `notes` 只允许短小、单行、英文说明。更长的 rationale 或双语解释应写在
  plans 或 skill-local references 中，而不是塞进 machine-owned index。
- `notes` 不是 prose 抢救通道。distillation-only 的 domain 指南和本地化理由
  不会迁入 `source_index.yaml`。

### 4.3 Provenance coverage 语义

`provenance-coverage-scan` 只换输入源，不换策略：

- `standalone` 与 `partial-only` 的 source-index 条目必须至少出现在一个
  shipped catalog skill 的 `provenance.yaml` 中。
- `posture-only`、`absorbed`、`view-only`、`route-only`、`deferred`
  仍然记录在 `source_index.yaml`，但不作为 provenance gate。
- 任一 shipped catalog skill `provenance.yaml` 中出现的 source，也必须出现在
  `source_index.yaml` 中。

本计划**不**把当前 gate 从 “presence” 收紧为 “exactly-one-owner”。
ownership exclusivity 可以后续再讨论，但不属于这次 hard cut。

### 4.4 Docs 删除后的姿态

本计划落地后：

- `docs/distillation/` 在仓库中不存在。
- 不再有任何 code/test 读取 `docs/` 下的 markdown 文档来决定 contract truth。
- 不再生成任何 markdown mirror 来替代已删除目录。
- `source_index.yaml` 保持英文、machine-owned；它不会变成承载双语 prose 的
  镜像索引。
- distillation-only 的 EN / zh-CN prose 不属于本次 hard cut 的保留目标。若某条
  claim 没有被存活的 code-local 或 skill-local authority 复写，它就随被删除的树
  一起丢弃，而不是迁入新的 appendix 或 reference surface。
- 历史 rationale 保留在 plan docs；live contract 与 enforcing code/tests
  共址。

### 4.5 与 route-surface plan 的先后关系

固定落地顺序：

- 先落 `2026-04-15-route-surface-refactor-plan*.md` 的 PR-1
- 再落本计划的 PR-1 / PR-2
- 最后落 `2026-04-15-route-surface-refactor-plan*.md` 的 PR-2 / PR-3

`main` 上不允许存在其它交错顺序。route-surface plan 的 PR-1 落下的
`surfaces.go` 是本计划直接消费的唯一 public-surface authority；两份计划之间
不允许再插入临时 `surfaces[]` bridge allowlist 或第二份手工维护的 surface 表。

## 5. Rollout

### PR-1 —— Source Index 抽离

**目标。** 用 code-local machine artifact 替换 markdown 驱动的 reverse-index
gate。

#### 代码范围

- 新增：`internal/engine/capability/source_index.yaml`
- 新增：`internal/engine/capability/source_index.go`
- 新增：`internal/engine/capability/source_index_test.go`
- 删除：`internal/engine/capability/by_source_test.go`
- 更新：`internal/engine/capability/provenance.go` 中仍把
  `docs/distillation/by-source.md` 当 authority 的注释

#### 实现

- 在冻结 schema 之前，先审计当前 `by-source.md` 的每一行，明确它应落到
  `skills[]`、`surfaces[]` 还是 `notes`。
- 任何当前落点既不是已注册 catalog skill、也不是 live code-owned
  non-catalog surface 的行，都必须收敛进 `notes`，不能强塞进 `surfaces[]`。
- 把当前 `by-source` 的 source corpus 行迁入 `source_index.yaml`，并固定
  schema。
- 在 `capability` package 中实现 `source_index.yaml` 的 loader / validator。
- `surfaces[]` 直接对 route-surface plan PR-1 引入的 surface-policy
  registry 做校验。`source_index` 可以消费这份 registry，但不得再引入自己的
  bridge allowlist 或第二份 surface table。
- 将 provenance coverage gate 改为读取 `source_index.yaml`，不再用 regex 解析
  markdown。
- 除 §4.3 说明的输入源替换外，保持 gate 语义不变。
- 冻结 legacy gated source set（`standalone` + `partial-only`），并断言迁移后
  仍是同一组 provenance-gated sources。hard cut 可以改存储格式，但不能静默
  缩小 gate coverage。
- **不要**新增 checked-in migration snapshot 或 migration allowlist。本次
  hard cut 接受直接把 corpus 重写进 `source_index.yaml`；保护依赖的是下面的
  gate-preservation 与 registry/provenance tests，而不是另一棵审计工件树。
- 删除 gate 中的 markdown table parsing 逻辑。
- 一旦 `source_index_test.go` 覆盖了旧 gate 的双向校验，`by_source_test.go`
  必须直接删除，不保留“同名但语义已变”的残留测试文件。

#### 测试

- `TestSourceIndexValid`
- `TestSourceIndexCoverageMatchesProvenance`
- `TestProvenanceSourcesAppearInSourceIndex`
- `TestSourceIndexLegacyGatedSourceSetPreserved`
- `TestSourceIndexSkillsResolveInRegistry`
- `TestSourceIndexSurfacesUseKnownNonCatalogIDs`

#### 验收

- 不再有任何测试读取 `docs/distillation/by-source.md`。
- provenance coverage gate 仅由 `source_index.yaml` 与 `provenance.yaml`
  驱动。
- gated source set（`standalone` + `partial-only`）在存储格式切换前后保持不变。
- reverse-index 语义在输入源切换后保持稳定。

### PR-2 —— Distillation Surface 删除

**目标。** 删除 `docs/distillation/`，并把所有 live references 回锚到
code-local authorities。

#### 代码 / 文档范围

- 删除：`docs/distillation/` 及其全部 EN / zh-CN 变体
- 更新：`internal/engine/capability/registry_b*.go` 中引用
  `docs/distillation/catalog.md` 的注释
- 更新：
  - `docs/plans/2026-04-11-skills-integration-plan*.md`
  - `docs/plans/2026-04-11-skills-integration-plan-delivery*.md`
  - `docs/plans/2026-04-14-skills-strengthening-plan*.md`
  - `docs/plans/2026-04-15-route-surface-refactor-plan*.md`

#### 实现

- 一次性删除整个 `docs/distillation/` 目录；不保留任何兼容子集。
- 不增加单独的 prose-rescue batch、appendix 迁移或 skill-reference 抢救流程。
  distillation-only 的 EN / zh-CN 文本若未被其它存活 authority 复写，就随该目录
  一并删除。
- 将 active plan 中当前仍把 `docs/distillation/*` 写成 live authority 的文本，
  改写为指向：
  - `internal/engine/capability/source_index.yaml`
  - `internal/engine/capability/registry*.go`
  - `internal/engine/capability/{provenance,gates}_*.go`
  - `internal/tmpl/templates/skills/<skill>/SKILL.md`
  - `internal/tmpl/templates/skills/<skill>/provenance.yaml`
- 在同一刀切 PR 中删掉
  `docs/plans/2026-04-14-skills-strengthening-plan*.md` 对
  `source-coverage-ledger-template*.md` 的残留引用，而不是另起一个 rescue PR。
- 删除把 `catalog.md` 行号当稳定 authority 的 code comments。
- **不要**再创建新的 repo-wide markdown replacement 来接这个目录。

#### 测试

- `go test ./internal/engine/capability/... -count=1`
- `go test ./internal/toolgen/... -count=1`
- 若删除 routed-surface doc references 时动到命令断言，则跑 targeted `./cmd`
  tests
- residue checks：
  - `rg -n "docs/distillation/" internal cmd docs/plans`
    在本 hard-cut plan family 与明确早于本 hard cut 的 historical plans 之外归零
  - `find docs -type d -name distillation` 归零
  - `rg -n "by-source\\.md|catalog\\.md|routed-surfaces\\.md|schema\\.md|pr-checklist\\.md" internal`
    不再命中 live-authority references

#### 验收

- `docs/distillation/` 已从仓库删除。
- 不再有任何 live test 或 code path 依赖 `docs/` 下的 markdown 文档。
- Active plan docs 不再把 `docs/distillation/*` 写成 frozen contract surface。
- 没有为了缓和删除而新增 rescue-only appendix、迁移 ledger 或第二棵
  reference tree。

## 6. 门禁

本计划内每个 PR 都要跑：

- `go test ./internal/engine/capability/... -count=1`
- `git diff --check`

PR-2 额外跑：

- `go test ./internal/toolgen/... -count=1`
- `go test ./... -count=1`
- `go vet ./...`
- §5 PR-2 中列出的 residue `rg` checks

## 7. 不在范围

- 用另一棵 repo-wide documentation tree 替代 `docs/distillation/`。
- 将 provenance ownership 从 presence-based 收紧到 exclusive ownership。
- 借删除 docs 之机修改 routed surface 行为。
- 重做 tool export shape 或 adapter-visible manifests。
