# 蒸馏契约（B0 冻结）

本文档冻结所有目录技能必须满足的编写契约。它被作者、Go 侧能力注册表
（`internal/engine/capability/`）以及计划在 B8 启用的 CI 门
（`schema-lint`、`size-lint`、`binding-compare`、`provenance-coverage-scan`）共同消费。

契约是描述性、面向导出的。运行时绑定权威位于 Go 注册表，而非前言。

## 1. 源文件布局

```
internal/tmpl/templates/skills/<skill-id>/
  SKILL.md              # 必需
  provenance.yaml       # 必需
  PROSE.tmpl            # 可选
  CHECKLIST.tmpl        # 可选
  VERDICT.tmpl          # 可选
  references/           # 可选，按需注入的示例材料
  scripts/              # 可选，确定性辅助脚本
```

规则：

1. `SKILL.md` 与 `provenance.yaml` 为每个目录技能的必需核心文件。
2. 类型化模板（`PROSE.tmpl`、`CHECKLIST.tmpl`、`VERDICT.tmpl`）为可选，
   仅当某个 binding 或 evidence 合约需要时才会消费。
3. `references/` 是单跳支持架，用于长例、反例与源注记；不是路由权威。
4. `scripts/` 仅用于具有明确 I/O 合约的确定性辅助；不是推进权威。
5. 引用或脚本必须先在技能主体或解析器输出中被命名，才会被消费。

## 2. SKILL.md 前言契约

```yaml
---
skill_id: <稳定 slipway 技能标识>
domain: <intake | execution | debugging | review-quality |
         review-security | review-change-shape | verification |
         repair-ci | ops-diagnostics 之一>
function: <一句话描述该技能的唯一职责>
tier: <T1 | T2 | T3>
primary_attachment: <posture | procedure | checklist | tool-recipe | report-schema>
summary: "Use when <条件>. Triggers on <信号>."
size_rationale: <可选字符串，仅当主体超出层级 hard-max 时必填>
trigger_signals:
  - <DSL 子句>
evidence_contract: <verdict | artifact | checklist>
bindings:
  - type: <host-embedded | command-auto | command-manual
           | technique-hint | command-view | export-only>
    target: <governed host 或命令 mode/view 标识>
    attachment: <posture | procedure | checklist | tool-recipe | report-schema>
provenance_ref: provenance.yaml
---
```

规则：

1. `skill_id` 必须与 `templates/skills/` 下目录名一致。
2. `summary` 必须使用 `Use when ... / Triggers on ...` 短语，使 description 即 dispatcher。
3. `tier` 标记语义角色，与绑定数量无关。T1 = 核心方法，T2 = 专家路由，T3 = 诊断视图。
4. `primary_attachment` 必填。每个 binding 可额外声明模式。
5. `trigger_signals[]` 仅限使用 §4 中的算子集。自由文本会被 schema-lint 拒绝。
6. `bindings[]` 必须与 Go 侧能力注册表一一对应，由 `binding-compare` 强制。
7. `evidence_contract` 标识该技能产出的证据形态。
8. `size_rationale` 默认可省略。仅当 `SKILL.md` 主体超过层级 hard-max
   警戒上界（6/8/3 KB）时必填。

## 3. 附着模式（冻结集合）

| 模式 | 含义 | 典型载体 |
|------|---------|-----------------|
| `posture` | 置于提示词顶部的持久姿态 | "enforce TDD" |
| `procedure` | 有序步骤 | `RED -> GREEN -> REFACTOR` |
| `checklist` | 离散检查项 | 安全评审列表 |
| `tool-recipe` | 工具或命令调用方案 | semgrep 配置 |
| `report-schema` | 结构化输出约束 | verdict 形状 |

典型模板映射：

- `PROSE.tmpl` → `posture` 或 `procedure`
- `CHECKLIST.tmpl` → `checklist`
- `VERDICT.tmpl` → `report-schema`
- `tool-recipe` → `scripts/` 或内联主体

解析器依据附着模式决定提示词注入位置。

## 4. 触发 DSL（冻结算子集）

归属 `internal/engine/capability/trigger.go`。`trigger_signals[]` 不允许任何该列表之外的算子：

- `all_of` / `any_of` / `not`
- `command` — 当前命令面（如 `review`、`validate`、`repair`、`status`、`health`）
- `host` — 当前 governed host
- `blocker_reason` — 规范化阻塞原因代码
- `changed_files_include` — 变更文件集合的 glob（支持 `*`、`?`、`**`，
  以及 `*.{yml,yaml}` 这类花括号备选）
- `path_includes` — 对引用路径的子串匹配
- `user_text_matches` — 对用户文本的大小写不敏感子串匹配

每条顶层子句都必须携带 `reason` 字段，解析器将其用于 `TechniqueHints`。

打分逻辑归 Go 所有。解析器每次最多返回一条路由以及至多三条排序后的支持附着。

## 5. provenance.yaml 契约

```yaml
sources:
  - source: <vendor>/<source-skill>
    absorbed_as: <standalone | posture-only | partial-only>
    extracted:
      - <该技能中保留的规则/流程>
    dropped:
      - <被丢弃的叙事或规则>
    conflicts_with: []
```

规则：

1. 只有在 `by-source.md` 中被标为 `standalone` 或 `partial-only` 的源，
   才必须出现在某个目录技能 provenance 文件的
   `extracted` / `dropped` / `conflicts_with` 之一，由
   `provenance-coverage-scan` 强制。`posture-only`、`absorbed`、
   `view-only`、`route-only` 与 `deferred` 仍记录在 `by-source.md`，
   但不作为 provenance gate。
2. `absorbed_as: standalone` 表示源实质贡献于目录技能；`posture-only` 表示仅保留姿态；
   `partial-only` 表示仅消费片段或模板分部。
3. 冲突默认保守合并；记录冲突与选择规则至 `conflicts_with`，并在 `reason` 简述原因。

## 6. 组装顺序（固定）

Toolgen 的多文件组装器现在按且仅按此顺序编译目录技能：

1. 前言
2. `SKILL.md` 主体
3. 条件类型化模板注入
   1. `PROSE.tmpl` 当 binding 声明 `posture` 或 `procedure`
   2. `CHECKLIST.tmpl` 当 binding 声明 `checklist`
   3. `VERDICT.tmpl` 当 binding 声明 `report-schema`
4. 由主体或解析器命名的 `references/<name>.md` 文件（`hydrate_references[]`）

规则：

1. Catalog skill 现在已经使用这套 assembler。非 catalog 的 governed host
   仍可能直接使用单文件或 `.tmpl` source。
2. Hydration 不得越过该技能自己的 `references/` 目录。
3. 脚本不在组装期内联；始终保持为文件系统工件，由用户工具调用。

## 7. CI 门（已在 Go 测试中落地，并预期由 CI 执行）

| 门 | 范围 |
|------|-------|
| `schema-lint` | 解析前言与类型化模板引用；校验算子白名单 |
| `size-lint` | 按层级预算度量主体大小（T1 ≤ 2.5KB, T2 ≤ 3.5KB, T3 ≤ 1.5KB；警戒 2.5-6 / 3.5-8 / 1.5-3KB；超上界需理由） |
| `binding-compare` | 前言 `bindings[]` 与 Go 注册表的差异；必须一一对应 |
| `provenance-coverage-scan` | 强制 `by-source` 中 `standalone` / `partial-only` 行被 `provenance.yaml` 覆盖，并做反向校验（每个 provenance source 必须出现在 `by-source.md`） |

## 8. 扩展输出（可选）

解析器除了主路由或支持列表，还可能返回：

- `hydrate_references[]`：建议注入的 `references/*.md` 文件路径。
  契约自 B2 起声明（`context-assembly` frontmatter）；resolver 发出当前仍为保留态，返回空。
- `llm_tiebreak`：DSL 打分并列时的候选 id 与决策标准。于 B7 首次启用；B1 既不实现也不测试。
