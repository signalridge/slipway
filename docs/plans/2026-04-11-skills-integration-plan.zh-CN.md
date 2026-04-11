# Skills 集成方案 — 大胃口建设版（2026-04-11，v5）

> 取代 v3（`2026-04-11-skills-integration-plan.md` 旧版）、v3-critique（`...v3-critique.md`）以及 v4（`...v4-merged.md`）。v3 拒绝了 v2 的幻觉是对的；v4 把每个论断都落到仓库事实上是对的。v5 同时保留这两点优势，并且不再保守：引擎、registry 契约、CLI、chezmoi 池、slipway 模板面 —— 全部一次协调改造。
>
> **状态：** 草案，等待 owner 在 §13 上签字。
> **审计时间：** 2026-04-11，针对本地机器实测。
> **目标形态：** 通过 `internal/engine/skill/skill.go` 注册 **19 个 gate + 12 个 reference + 4 个 technique**，全部经由扩展后的 `next` CLI 输出，复用并扩展现有的 `defaultGovernanceRegistry` 形状（不替换）。

---

## 1. 执行摘要

v4 矫枉过正了。v4 推迟掉的一半事项（"不加 gate、不扩展 frontmatter、不动 CLI 表面"）并不是根本约束 —— 它们是因为测试套件这么写而自我设的规则。测试套件是你写的；契约也是你定的。打破最小 frontmatter 契约、加入 gate 排序的最佳时机就是**现在**，在一次协调改造里完成，而不是分散在未来四份方案中。

v5 分四个 wave 落地。每个 wave 是独立的 PR，但四个 wave 合在一起构成一次连贯的契约变更：

- **Wave 1 — 引擎 + schema 改造。** `Definition` 新增 6 个字段（`Phase`、`Subject`、`Tier`、`References`、`OrderAfter`、若干条件标志）。`governanceFrontMatter` parser 扩展。同 state 内 gate 排序加入拓扑排序。新增 `defaultReferenceRegistry` map。新增 `slipway next --references` CLI flag。`chezmoi` 拉取 5 个缺失的上游源。测试契约迁移到新形状。零新 skill。现有 9 个 gate 行为零变化。
- **Wave 2 — 现有 skill 加固 + `checklist-quality` 拆分。** v4 §7.2 的 6 个增强合并落地（intake / code-review-protocol / codebase-mapping / spec-compliance-review / wave-orchestration / tdd-governance）。7 行的 `checklist-quality.md` sidecar 拆成 4 个领域 checklist（`intake`、`plan`、`review`、`test`）。所有被加固的 skill 获得新 frontmatter 字段。
- **Wave 3 — 新 gate（10 个）。** registry 落地 10 个新治理 gate，gate 总数从 9 提到 19。每个都来自经核实的上游 skill（完整引用见 §6）。每个都有 slipway 风格的 frontmatter、reason code、hard-gate 标记，并在适用的地方有条件触发规则。在 scratch 项目上做端到端冒烟。
- **Wave 4 — Reference shelf（12 个）。** 12 个 slipway 风格的 reference skill 落地到 `defaultReferenceRegistry`。每个 ~150–200 LoC，源自 v4 §4.1 已核实的上游内容。`next --references` 按 phase/subject 过滤展示。Reference shelf 就是 v4 想用文档交叉链做的"还应该读什么"层 —— 但作为结构化 registry 实现。

终态：slipway 的 skill 表面从 **9 gate + 4 technique** 扩到 **19 gate + 4 technique + 12 reference** = 35 个注册 skill。registry 契约一次性支持 phase × subject 过滤、gate 排序、条件触发和 reference 展示。

---

## 2. 落地此方案的事实依据

以下是 v4 审计得到的核实事实。v5 全部继承；v4 到 v5 之间这部分没有变化。

### 2.1 当前 `slipway` 运行时路由契约

`slipway` 通过这些 `Definition` 字段路由治理 skill（`internal/engine/skill/skill.go:10-20`）：

```go
type Definition struct {
    Name                string
    State               model.WorkflowState
    PlanSubStep         model.PlanSubStep
    Mitigation          string
    RunSummaryBound     bool
    DiscoveryOnly       bool
    GuardrailRequired   bool
    CloseoutConditional bool
    AgentHint           string
}
```

现有 9 条全部设置了 `AgentHint`。v5 对每个新 gate 也保留这个习惯。

### 2.2 当前 registry 大小：9 gate

| State | Skill | 备注 |
|---|---|---|
| `S0_INTAKE` | `intake-clarification` | `AgentHint: slipway-planner` |
| `S1_PLAN` | `research-orchestration` | `PlanSubStep: research`、`DiscoveryOnly: true`、`AgentHint: slipway-researcher` |
| `S1_PLAN` | `plan-audit` | `PlanSubStep: audit`、`AgentHint: slipway-auditor` |
| `S2_EXECUTE` | `wave-orchestration` | `RunSummaryBound: true`、`AgentHint: slipway-orchestrator` |
| `S2_EXECUTE` | `tdd-governance` | `GuardrailRequired: true`、`RunSummaryBound: true`、`AgentHint: slipway-orchestrator` |
| `S3_REVIEW` | `spec-compliance-review` | `RunSummaryBound: true`、`AgentHint: slipway-reviewer` |
| `S3_REVIEW` | `code-quality-review` | `RunSummaryBound: true`、`AgentHint: slipway-reviewer` |
| `S4_VERIFY` | `goal-verification` | `RunSummaryBound: true`、`AgentHint: slipway-verifier` |
| `S4_VERIFY` | `final-closeout` | `CloseoutConditional: true`、`RunSummaryBound: true`、`AgentHint: slipway-closer` |

### 2.3 仅作为 technique 的 skill（未注册为 gate）

`tdd`、`systematic-debugging`、`code-review-protocol`、`codebase-mapping`。v5 仍把这些保留为 technique skill —— 它们在 gate **内部**被引用，而不是作为 state checkpoint。v5 不把它们提升为 gate。

### 2.4 模板文件形态

- 静态 `SKILL.md`：大多数 skill。
- 由 `SKILL.md.tmpl` 渲染：`spec-compliance-review`、`wave-orchestration`、`tdd-governance`。改动落在 `.tmpl` 源；Wave 2 的结构断言要先渲染再断言。

### 2.5 `~/.agents/skills` 已经预装了 167 个 SKILL.md

`~/.local/share/chezmoi/.chezmoiexternal.toml.tmpl` 拉自 `wshobson/agents`、`anthropics/skills`、`getsentry/skills`、`openai/skills`、`trailofbits/skills`、`huggingface/skills`、`cloudflare`、`vercel-labs`、`supabase`、`expo`、`microsoft`、`signalridge`、多语言 humanizer 包、`nextlevelbuilder/ui-ux-pro-max-skill`、`sickn33/antigravity-awesome-skills`。v5 在 Wave 1 再加 5 个源（§9）。

### 2.6 v2 的所有错分都已确认

v4 §2.5 的完整证据表格无变化保留。v5 仍然依赖的几个要点：

- `agentic-actions-auditor` 是**只针对 GitHub Actions AI CI 的审计** —— 不是通用的 diff scanner。v5 不把它做成 gate；只在用户审视 `.github/workflows/*.yml` 时作为 reference 出现（`tier: reference`、`subject: safety`、`phase: review`）。
- `trailofbits/spec-to-code-compliance` 是区块链特定的。v5 的 `spec-compliance-review` gate 只借用其证据引用规则，绝不照搬整个流程。
- `trailofbits/testing-handbook-skills` 是 15 个模糊测试工具 skill，不是方法论 bundle。v5 不会注册 `testing-strategies` gate。
- Slipway 的 `spec-compliance-review/SKILL.md.tmpl` **比上游更严**。v5 §6.2.2 不会削弱它。
- `worktree-preflight、checkpoint、done、pivot、repair、cancel、abort` 是 CLI 命令，**不是注册的 skill**。v5 保留它们作为命令。
- `checklist-quality.md` 是 7 行 sidecar，不是 skill。v5 §7 把它拆成 4 个领域 checklist，仍然是 sidecar（仍然不注册为 skill）。

完整 file:line 引用见 §14。

---

## 3. 目标 Skill 表面

v5 的终态 = **35 个注册 skill**（19 gate + 4 technique + 12 reference），位于 `internal/tmpl/templates/skills/` 与 `internal/engine/skill/skill.go`。

### 3.1 v5 后按 state 划分的 gate

| State | Gate | 状态 | 来源 |
|---|---|---|---|
| `S0_INTAKE` | `intake-clarification` | 现有（Wave 2 加固） | + `trailofbits/ask-questions-if-underspecified` |
| `S1_PLAN` | `research-orchestration` | 现有 | — |
| `S1_PLAN` | `plan-audit` | 现有 | — |
| `S1_PLAN` | `threat-model-screen` | **Wave 3 新增** | `openai/.curated/security-threat-model`（已安装） |
| `S1_PLAN` | `adr-discipline` | **Wave 3 新增** | `wshobson/.../architecture-decision-records`（已安装） |
| `S2_EXECUTE` | `wave-orchestration` | 现有（Wave 2 加固） | + `wshobson/.../workflow-patterns` + `wshobson/.../task-coordination-strategies` |
| `S2_EXECUTE` | `tdd-governance` | 现有（Wave 2 加固） | + `wshobson/.../workflow-patterns`（wave 感知 TDD） |
| `S2_EXECUTE` | `code-simplification-review` | **Wave 3 新增** | `getsentry/.../code-simplifier`（已安装） |
| `S2_EXECUTE` | `error-handling-discipline` | **Wave 3 新增** | `wshobson/.../error-handling-patterns`（已安装） |
| `S3_REVIEW` | `spec-compliance-review` | 现有（Wave 2 加固） | 窄借 `trailofbits/spec-to-code-compliance` |
| `S3_REVIEW` | `code-quality-review` | 现有（Wave 2 加固） | + `developer-essentials/code-review-excellence`（已安装） |
| `S3_REVIEW` | `audit-context-readiness` | **Wave 3 新增** | `trailofbits/audit-context-building`（已安装） |
| `S3_REVIEW` | `differential-risk-review` | **Wave 3 新增** | `trailofbits/differential-review`（已安装） |
| `S3_REVIEW` | `security-review` | **Wave 3 新增** | `getsentry/security-review` + `openai/security-best-practices`（均已安装） |
| `S4_VERIFY` | `goal-verification` | 现有 | — |
| `S4_VERIFY` | `final-closeout` | 现有 | — |
| `S4_VERIFY` | `changelog-emission` | **Wave 3 新增** | `wshobson/.../changelog-automation`（已安装） |
| `S4_VERIFY` | `postmortem-readiness` | **Wave 3 新增** | `wshobson/.../postmortem-writing`（已安装） |

**总计 19 个 gate。** 9 现有 + 10 新增。

### 3.2 v5 后的 Reference

12 个 slipway 风格的 reference skill，每个 ~150–200 LoC，注册到新的 `defaultReferenceRegistry`。每个都是 `tier: reference`、`hard_gate: false`，通过 `next --references` 展示。

| # | Slipway 目标名 | Phase | Subject | 来源 |
|---|---|---|---|---|
| 1 | `audit-context-building` | review | safety | `~/.agents/skills/ecosystem/trailofbits/audit-context-building/SKILL.md` |
| 2 | `differential-review-methodology` | review | safety | `~/.agents/skills/ecosystem/trailofbits/differential-review/SKILL.md` |
| 3 | `property-based-testing` | execute | correctness | `~/.agents/skills/ecosystem/trailofbits/property-based-testing/SKILL.md` |
| 4 | `e2e-testing-patterns` | execute | correctness | `~/.agents/skills/developer-essentials/e2e-testing-patterns/SKILL.md` |
| 5 | `error-handling-patterns` | execute | refactor | `~/.agents/skills/developer-essentials/error-handling-patterns/SKILL.md` |
| 6 | `code-simplification` | execute | refactor | `~/.agents/skills/ecosystem/getsentry/code-simplifier/SKILL.md` |
| 7 | `architecture-decision-records` | plan | architecture | `~/.agents/skills/documentation-generation/architecture-decision-records/SKILL.md` |
| 8 | `threat-modeling` | plan | safety | `~/.agents/skills/ecosystem/openai/security-threat-model/SKILL.md` |
| 9 | `incident-runbooks` | verify | process | `wshobson/.../incident-runbook-templates`（Wave 1 chezmoi 拉取） |
| 10 | `postmortem-writing` | verify | process | `~/.agents/skills/.../postmortem-writing/SKILL.md` |
| 11 | `changelog-authoring` | verify | process | `~/.agents/skills/documentation-generation/changelog-automation/SKILL.md` |
| 12 | `debugging-strategies` | execute | debug | `~/.agents/skills/developer-essentials/debugging-strategies/SKILL.md` |

### 3.3 Technique（不变）

`tdd`、`systematic-debugging`、`code-review-protocol`、`codebase-mapping`。继续作为 technique-only `.md` 文件留在 `internal/tmpl/templates/skills/`。两个 registry 都不进。

---

## 4. 引擎 + Schema 变更（Wave 1）

### 4.1 新的 `Definition` 字段

扩展 `internal/engine/skill/skill.go:10-20`：

```go
type Definition struct {
    // 现有
    Name                string              `json:"name"`
    State               model.WorkflowState `json:"state"`
    PlanSubStep         model.PlanSubStep   `json:"plan_substep,omitempty"`
    Mitigation          string              `json:"mitigation"`
    RunSummaryBound     bool                `json:"run_summary_bound"`
    DiscoveryOnly       bool                `json:"discovery_only,omitempty"`
    GuardrailRequired   bool                `json:"guardrail_required,omitempty"`
    CloseoutConditional bool                `json:"closeout_conditional,omitempty"`
    AgentHint           string              `json:"agent_hint,omitempty"`

    // v5 新增
    Phase           Phase    `json:"phase,omitempty"`            // intake|plan|execute|review|verify|meta
    Subject         Subject  `json:"subject,omitempty"`          // correctness|safety|architecture|refactor|process|debug|authoring
    Tier            Tier     `json:"tier"`                       // gate|reference|technique
    References      []string `json:"references,omitempty"`       // 池路径（如 "shared:ecosystem/trailofbits/differential-review"）
    OrderAfter      string   `json:"order_after,omitempty"`      // 同 state 内 gate 排序
    ReasonCode      string   `json:"reason_code,omitempty"`      // internal/model/reason_code.go 中的 key
    HardGate        bool     `json:"hard_gate,omitempty"`        // 推进 state 需要显式用户批准
    SubjectGated    bool     `json:"subject_gated,omitempty"`    // 仅当 change.Subject 与 this.Subject 匹配时触发
    PivotConditional      bool `json:"pivot_conditional,omitempty"`       // 仅当执行日志含 pivot/repair/abort 时触发
    UserFacingConditional bool `json:"user_facing_conditional,omitempty"` // 仅当变更涉及用户面文件时触发
    ErrorPathConditional  bool `json:"error_path_conditional,omitempty"`  // 仅当变更涉及错误/异常路径时触发
}
```

配套三个枚举（`Phase`、`Subject`、`Tier`），含常量与校验器。校验器在 registry 加载时跑，并拒绝非法组合（如 `Tier: reference` + `HardGate: true`）。

### 4.2 新的 `governanceFrontMatter` 字段

扩展 `internal/engine/skill/registry_loader.go:40-44`，解析 `phase`、`subject`、`tier`、`references`、`order_after`、`reason_code`、`hard_gate`、`subject_gated`、`pivot_conditional`、`user_facing_conditional`、`error_path_conditional`。更新 `parseGovernanceSkillFromFile()`（168–199 行），在 name 查表后填充新的 `Definition` 字段。

向后兼容：没有任何新字段的 frontmatter 仍然有效；如果 skill 在 `defaultGovernanceRegistry` 中，loader 默认设 `Tier: gate`；在 `defaultReferenceRegistry` 中默认 `Tier: reference`；否则默认 `Tier: technique`。

### 4.3 新的 `defaultReferenceRegistry`

加入 `internal/engine/skill/skill.go`：

```go
var defaultReferenceRegistry = map[string]Definition{
    "audit-context-building": {
        Name:       "audit-context-building",
        Tier:       TierReference,
        Phase:      PhaseReview,
        Subject:    SubjectSafety,
        References: []string{"shared:ecosystem/trailofbits/audit-context-building"},
    },
    // ... §3.2 还有 11 个 ...
}
```

Reference skill 没有 `State`。它们不被 progression 引擎迭代；只通过 `next --references`（§4.6）和其它 skill 内部 `References` 交叉链来展示。

### 4.4 `OrderAfter` 拓扑排序

`RequiredSkillsForStateWithRegistry`（`skill.go:115-158`）当前按 state 迭代 registry 但没有顺序保证。把这次迭代换成：

1. 按 state 过滤为 `Definition` 切片。
2. 用 `OrderAfter` 边构造 DAG。
3. 拓扑排序。环或未知依赖在 registry 加载阶段直接 fail loud。
4. 返回排好序的切片。

这样像 `differential-risk-review` 这种必须在 S3 中跑在 `code-quality-review` **之后**的 gate 才能解锁。

### 4.5 条件触发逻辑

加入 `internal/engine/progression/skill_resolution.go`：

```go
type ChangeContext struct {
    Subject        Subject       // intake 时打 tag
    GuardrailDomain bool         // 现有
    ExecutionLog   []ExecutionEvent  // pivot、repair、abort
    TouchedFiles   []string      // 用于 user-facing / error-path 检测
}

func (d Definition) ShouldFire(ctx ChangeContext) bool {
    if d.SubjectGated && d.Subject != ctx.Subject {
        return false
    }
    if d.PivotConditional && !hasPivot(ctx.ExecutionLog) {
        return false
    }
    if d.UserFacingConditional && !touchesUserFacing(ctx.TouchedFiles) {
        return false
    }
    if d.ErrorPathConditional && !touchesErrorPath(ctx.TouchedFiles) {
        return false
    }
    return true
}
```

检测 helper（`hasPivot`、`touchesUserFacing`、`touchesErrorPath`）是简单的 file glob 匹配 —— `touchesUserFacing` 检测 `cmd/`、`internal/cli/`、`web/`、`frontend/` 下的变化；`touchesErrorPath` 检测含 `errors.go`、`*_error.*`、`recovery.*` 等的文件。这些 pattern 在 `internal/model/config.go` 中可配置。

`ChangeContext` 由现有的变更解析器构造，而不是每个 gate 自行构造。Gate 只通过条件标志声明它在意什么。

### 4.6 新 CLI flag：`slipway next --references`

扩展 `cmd/next.go` 支持 `--references`（短别名 `--refs`）。开启时：

- `slipway next` 先打印正常的 gate 输出。
- gate 输出之后追加 `## References` 段，列出 `defaultReferenceRegistry` 中所有 `Phase` 匹配当前 state 且 `Subject` 匹配变更主题（已知则匹配，未知则任意）的条目。
- 每行：`slipway-<name>: <一行说明> → <池路径>`。
- JSON 输出（`--json`）增加 `references` 数组。

flag 是 opt-in；`next` 默认行为不变。

### 4.7 测试契约迁移

`internal/tmpl/templates_test.go` 当前断言最小 frontmatter。Wave 1 把断言迁移成：

- 必须字段：`name`、`description`、`tier`。
- 对于 `tier: gate`：还要求 `phase`、`subject`、`reason_code`。
- 对于 `tier: reference`：还要求 `phase`、`subject`、`references`（非空）。
- 对于 `tier: technique`：仅要求 `name` 和 `description`。
- 拓扑排序断言：`TestGateOrderingNoCycles` 遍历 `defaultGovernanceRegistry` 并确认 `OrderAfter` 形成 DAG。
- Reference 断言：`TestReferenceTargetsExist` 遍历 `defaultReferenceRegistry` 并确认每个 `References` 条目都能解析到 `~/.agents/skills/` 下真实文件（在 CI 环境无 chezmoi 池时可软告警跳过）。

这就是 v3/v4 想避开的契约破裂点。v5 一次完成迁移，然后锁住新形状。

---

## 5. 现有 Skill 加固（Wave 2）

v4 §7.2 的 6 个加固，重述于此。每个都在 Wave 2 与 Wave 1 的新 frontmatter 字段一起落地。

### 5.1 `intake-clarification` ← `trailofbits/ask-questions-if-underspecified`

借用：5 类提问分类法（scope / acceptance / constraint / risk / stakeholder）、显式停止条件（"用 1–3 句话复述需求 + 关键约束"）。在现有 "Clarification Loop" 步骤下加 `## Question Taxonomy` 与 `## Stop Condition` 子节。不动现有的 Rationalization Red Flags 或 Scope Boundary Precision Rules。

新 frontmatter：

```yaml
---
name: slipway-intake-clarification
description: "Verify scope, acceptance criteria, and constraints before planning"
tier: gate
phase: intake
subject: correctness
reason_code: intake_clarification_required
hard_gate: true
---
```

### 5.2 `code-quality-review`（v4 中叫 `code-review-protocol`）← `wshobson/multi-reviewer-patterns` + `developer-essentials/code-review-excellence`

借用 `multi-reviewer-patterns`：reviewer 维度分配表（Security / Performance / Architecture / Testing / Accessibility）、findings 去重协议。借用 `code-review-excellence`：「Goals vs Not Goals」反向清单、建设性反馈量规。在现有 iron-law 段后添加 `## Reviewer Role Splits` 与 `## Feedback Discipline` 节。

注意：`code-review-protocol` 仍然作为 technique skill 存在（被 `code-quality-review` 引用）。`code-quality-review` 这个 gate 才是注册的执行点。

### 5.3 `wave-orchestration` ← `wshobson/workflow-patterns` + `wshobson/task-coordination-strategies`

借用 workflow-patterns：11 步 TDD 生命周期（仅生命周期清单）、phase 完成协议、quality gates checkpoint 结构。借用 task-coordination-strategies：依赖图原则（parallel-safe vs sequential-required 判定）。添加 `## Coordination Strategies` 与 `## Lifecycle Checkpoints` 节。

`SKILL.md.tmpl` 源 —— 改动落在模板，后续渲染。

### 5.4 `tdd-governance` ← `wshobson/workflow-patterns`（延续）

新增 `## Wave-Aware TDD` 节：何时跨 wave 拆测试，何时 fail-fast。交叉链至 `wave-orchestration`。`SKILL.md.tmpl` 源。

### 5.5 `codebase-mapping` ← `trailofbits/entry-point-analyzer`（仅借概念）

借用 5 类入口分类（CLI、HTTP、定时任务、消息队列消费者、事件处理器），改写为语言无关。添加 `## Entry-Point Discovery` 子节。`codebase-mapping` 仍是 technique skill。

### 5.6 `spec-compliance-review` ← `trailofbits/spec-to-code-compliance`（窄借）

只借：Phase 0 证据引用规则（每个论断都必须引用 `file:line`），以及 changed-files 完备性 phase（要读每个变更文件，而不是只看 diff hunk）。在现有 "Independent Verification Mandate" 段内添加 `## Evidence Citation` 子节。**不删不弱化**现有的 Iron Law、Mandatory Checklist、Review Layers 或 Rationalization Red Flags。`SKILL.md.tmpl` 源。

### 5.7 `checklist-quality` 拆分

把单文件 7 行的 `internal/tmpl/templates/skills/checklist-quality.md` 替换为目录：

```
internal/tmpl/templates/skills/checklist-quality/
├── intake.md   — 澄清完备性 checklist（5 类提问）
├── plan.md     — 计划审计 checklist
├── review.md   — 审查/审计 checklist（现有内容放这里并扩展）
└── test.md     — 测试充分性 checklist
```

每个仍然是 sidecar（无 frontmatter，无 registry 注册）。Skill 通过新的 `references:` frontmatter 字段引用具体 checklist。Wave 2 把 `spec-compliance-review/SKILL.md.tmpl:35` 改为引用 `checklist-quality/review.md` 而不是整目录。

---

## 6. 新 Gate 规格（Wave 3）

每个新 gate 一个子节：来源引用、slipway frontmatter、body skeleton、reason code、排序、条件逻辑。

### 6.1 `S1_PLAN` 新增

#### 6.1.1 `threat-model-screen`

**来源：** `~/.agents/skills/ecosystem/openai/security-threat-model/SKILL.md`（已安装；82 LoC；STRIDE 级别 workflow）。

**Frontmatter：**
```yaml
---
name: slipway-threat-model-screen
description: "STRIDE-lite threat model screen for safety-subject changes during planning"
tier: gate
phase: plan
subject: safety
reason_code: threat_model_screen_required
hard_gate: true
subject_gated: true       # 仅在 change.Subject == "safety" 时触发
order_after: plan-audit
references:
  - "shared:ecosystem/openai/security-threat-model"
agent_hint: slipway-auditor
---
```

**Body skeleton：** 6 节 STRIDE 走查（边界 / 资产 / 入口 / 滥用路径 / 缓解 / 开放问题）。验证 YAML 写入 `artifacts/changes/{slug}/verification/threat-model-screen.yaml`。

**Reason code：** `internal/model/reason_code.go` 中 `threat_model_screen:safety_subject_unscreened`。

**排序：** 在 `subject == safety` 时，于同 state 中 `plan-audit` 之后执行。

#### 6.1.2 `adr-discipline`

**来源：** `~/.agents/skills/documentation-generation/architecture-decision-records/SKILL.md`（已安装；441 LoC）。

**Frontmatter：**
```yaml
---
name: slipway-adr-discipline
description: "Material design changes require an ADR"
tier: gate
phase: plan
subject: architecture
reason_code: adr_discipline_required
hard_gate: false
order_after: plan-audit
references:
  - "shared:documentation-generation/architecture-decision-records"
agent_hint: slipway-auditor
---
```

**条件触发：** Wave 1 检测 helper `touchesArchitecture()` 匹配 `internal/engine/`、`internal/model/`、`cmd/root.go` 下的变化或重命名导出类型的变化。仅在 `touchesArchitecture(ctx.TouchedFiles)` 时触发。实现为新 flag `MaterialDesignConditional bool`（v5 终稿在 §4.1 struct 中加入）。

**Body skeleton：** 读现有 `docs/decisions/` 中 ADR，决定是否要新写一份，写或显式给出跳过的理由。

### 6.2 `S2_EXECUTE` 新增

#### 6.2.1 `code-simplification-review`

**来源：** `~/.agents/skills/ecosystem/getsentry/code-simplifier/SKILL.md`（已安装；119 LoC）。

**Frontmatter：**
```yaml
---
name: slipway-code-simplification-review
description: "Apply simplification heuristics before TDD lockdown"
tier: gate
phase: execute
subject: refactor
reason_code: code_simplification_review_required
hard_gate: false
order_after: wave-orchestration
references:
  - "shared:ecosystem/getsentry/code-simplifier"
  - "slipway:reference:code-simplification"
agent_hint: slipway-orchestrator
---
```

**Body skeleton：** 语言无关的 checklist（减少分支、扁平嵌套、命名、删除死代码），针对刚完成的 wave diff 执行。输出："已应用的简化"清单或显式的 "无需简化" 说明。

#### 6.2.2 `error-handling-discipline`

**来源：** `~/.agents/skills/developer-essentials/error-handling-patterns/SKILL.md`（已安装；632 LoC —— 较大；v5 只借决策矩阵，不照搬全文）。

**Frontmatter：**
```yaml
---
name: slipway-error-handling-discipline
description: "Touched error paths must declare fail-fast vs fallback vs retry vs circuit-break"
tier: gate
phase: execute
subject: refactor
reason_code: error_handling_discipline_required
hard_gate: true
guardrail_required: true
error_path_conditional: true
order_after: tdd-governance
references:
  - "shared:developer-essentials/error-handling-patterns"
agent_hint: slipway-orchestrator
---
```

**条件触发：** `error_path_conditional: true` —— 仅在变更文件匹配 error-path pattern 时触发（在 `internal/model/config.go` 中可配；默认 pattern：`*errors*.go`、`*recover*.go`、`*panic*.go`、含 `defer recover()` 的文件等）。

### 6.3 `S3_REVIEW` 新增

#### 6.3.1 `audit-context-readiness`

**来源：** `~/.agents/skills/ecosystem/trailofbits/audit-context-building/SKILL.md`（已安装；约 200 LoC）。

**Frontmatter：**
```yaml
---
name: slipway-audit-context-readiness
description: "Build line-by-line architectural context before risky review"
tier: gate
phase: review
subject: safety
reason_code: audit_context_readiness_required
hard_gate: true
subject_gated: true
order_after: ""           # 触发时在 S3 中先跑
references:
  - "shared:ecosystem/trailofbits/audit-context-building"
agent_hint: slipway-reviewer
---
```

**条件触发：** `subject_gated: true` —— 仅在 `subject == safety` 时触发。触发时在 `spec-compliance-review` **之前**跑（无 `order_after`，但拓扑排序把无依赖的 gate 按注册序放在前面）。

#### 6.3.2 `differential-risk-review`

**来源：** `~/.agents/skills/ecosystem/trailofbits/differential-review/SKILL.md`（已安装）。

**Frontmatter：**
```yaml
---
name: slipway-differential-risk-review
description: "Risk-aware diff review with blast-radius and rationalizations"
tier: gate
phase: review
subject: correctness
reason_code: differential_risk_review_required
hard_gate: true
order_after: code-quality-review
references:
  - "shared:ecosystem/trailofbits/differential-review"
agent_hint: slipway-reviewer
---
```

**Body skeleton：** 改编自上游的 6 阶段走查 —— Triage → Blast Radius → Test Coverage → Risk Classification → Adversarial → Report。严重性立场：首次部署 Important，1 个月观察后升级为 Critical。

#### 6.3.3 `security-review`

**来源：** `~/.agents/skills/ecosystem/getsentry/security-review/SKILL.md` + `~/.agents/skills/ecosystem/openai/security-best-practices/SKILL.md`（均已安装）。

**Frontmatter：**
```yaml
---
name: slipway-security-review
description: "Confidence-based security findings for safety-subject changes"
tier: gate
phase: review
subject: safety
reason_code: security_review_required
hard_gate: true
subject_gated: true
order_after: differential-risk-review
references:
  - "shared:ecosystem/getsentry/security-review"
  - "shared:ecosystem/openai/security-best-practices"
agent_hint: slipway-reviewer
---
```

**Body skeleton：** OWASP 基础 checklist + 语言/框架感知的被动检测。按置信度分级输出（High = 阻断；Medium = 告警；Low = 信息）。

### 6.4 `S4_VERIFY` 新增

#### 6.4.1 `changelog-emission`

**来源：** `~/.agents/skills/documentation-generation/changelog-automation/SKILL.md`（已安装；572 LoC）。

**Frontmatter：**
```yaml
---
name: slipway-changelog-emission
description: "User-facing changes require a changelog entry before closeout"
tier: gate
phase: verify
subject: process
reason_code: changelog_emission_required
hard_gate: false
user_facing_conditional: true
order_after: goal-verification
references:
  - "shared:documentation-generation/changelog-automation"
  - "slipway:reference:changelog-authoring"
agent_hint: slipway-closer
---
```

**条件触发：** `user_facing_conditional: true` —— 仅在变更文件涉及 `cmd/`、`docs/`、`README*` 或任何具有 semver 影响的内容时触发。

#### 6.4.2 `postmortem-readiness`

**来源：** `~/.agents/skills/.../postmortem-writing/SKILL.md`（已安装；390 LoC）。

**Frontmatter：**
```yaml
---
name: slipway-postmortem-readiness
description: "Pivots/repairs/aborts during execution require a blameless postmortem"
tier: gate
phase: verify
subject: process
reason_code: postmortem_readiness_required
hard_gate: false
pivot_conditional: true
order_after: goal-verification
references:
  - "slipway:reference:postmortem-writing"
agent_hint: slipway-closer
---
```

**条件触发：** `pivot_conditional: true` —— 仅在执行日志含 pivot、repair 或 abort 事件时触发。

### 6.5 Wave 3 新增的 reason code

加入 `internal/model/reason_code.go`：

```go
const (
    ReasonThreatModelScreen      ReasonCode = "threat_model_screen:safety_subject_unscreened"
    ReasonADRDiscipline          ReasonCode = "adr_discipline:material_design_undocumented"
    ReasonCodeSimplification     ReasonCode = "code_simplification_review:not_applied"
    ReasonErrorHandlingDiscipline ReasonCode = "error_handling_discipline:undeclared_strategy"
    ReasonAuditContextReadiness  ReasonCode = "audit_context_readiness:context_unbuilt"
    ReasonDifferentialRiskReview ReasonCode = "differential_risk_review:risk_unclassified"
    ReasonSecurityReview         ReasonCode = "security_review:safety_subject_unreviewed"
    ReasonChangelogEmission      ReasonCode = "changelog_emission:user_facing_undocumented"
    ReasonPostmortemReadiness    ReasonCode = "postmortem_readiness:pivot_unanalyzed"
)
```

外加一个用于排序字段冲突检测的第十个 code。

---

## 7. Reference Shelf 规格（Wave 4）

12 个 reference skill，每个 ~150–200 LoC。每个都是上游 skill 的 slipway 风格改编版，带 slipway frontmatter 和 `## Source` 节引用上游路径。

### 7.1 Reference frontmatter 模板

```yaml
---
name: slipway-<reference-name>
description: "<一行意图>"
tier: reference
phase: <intake|plan|execute|review|verify>
subject: <correctness|safety|architecture|refactor|process|debug|authoring>
references:
  - "shared:<池路径>"
hard_gate: false
---
```

### 7.2 Reference body 形态

```markdown
# <Title>

> **Source:** `<完整池路径>`（镜像上游）
> **Use when:** <一段触发说明>
> **Surfaced via:** `slipway next --references`，当 phase=<phase> 且 subject=<subject>

## Quick Reference

<5–10 行上游方法论摘要>

## When to invoke

<3–5 个 bullet>

## When NOT to invoke

<3–5 个 bullet>

## Borrowed essentials

<slipway 关心的 100–150 LoC —— 启发式、决策矩阵、反模式>

## See also

- <相关 slipway gate>
- <其它 reference shelf 条目>
```

### 7.3 12 个 reference

| # | Slipway 目标名 | 来源 | Phase | Subject | LoC 目标 |
|---|---|---|---|---|---|
| 1 | `audit-context-building` | `ecosystem/trailofbits/audit-context-building` | review | safety | ~150 |
| 2 | `differential-review-methodology` | `ecosystem/trailofbits/differential-review` | review | safety | ~200 |
| 3 | `property-based-testing` | `ecosystem/trailofbits/property-based-testing` | execute | correctness | ~180 |
| 4 | `e2e-testing-patterns` | `developer-essentials/e2e-testing-patterns` | execute | correctness | ~150 |
| 5 | `error-handling-patterns` | `developer-essentials/error-handling-patterns` | execute | refactor | ~180 |
| 6 | `code-simplification` | `ecosystem/getsentry/code-simplifier` | execute | refactor | ~120 |
| 7 | `architecture-decision-records` | `documentation-generation/architecture-decision-records` | plan | architecture | ~150 |
| 8 | `threat-modeling` | `ecosystem/openai/security-threat-model` | plan | safety | ~150 |
| 9 | `incident-runbooks` | `incident-response/incident-runbook-templates`（Wave 1 chezmoi 拉取） | verify | process | ~180 |
| 10 | `postmortem-writing` | `incident-response/postmortem-writing` | verify | process | ~150 |
| 11 | `changelog-authoring` | `documentation-generation/changelog-automation` | verify | process | ~150 |
| 12 | `debugging-strategies` | `developer-essentials/debugging-strategies` | execute | debug | ~180 |

Reference 总 LoC 约 1,950，每个都可以独立 review。

### 7.4 命名注意

12 个 reference 的目标名与 v2 一些发明的别名（`code-simplification`、`adr-authoring` 风格）有重叠。v5 在能对齐时尽量使用与上游一致的真实名（`code-simplification`、`architecture-decision-records`、`changelog-authoring`），不发明新身份。slipway 的 `slipway-` 前缀用来把本地适配版与上游来源区分开。

---

## 8. CLI 表面变更

### 8.1 `slipway next --references`（新 flag）

```
slipway next --references           # 在文本输出中追加 References 段
slipway next --references --json    # 在 JSON 输出中追加 "references" 数组
slipway next --refs                  # 短别名
```

实现：`cmd/next.go` 从变更 context 读取当前 state，按匹配 `Phase` 与 `Subject`（subject 取变更打 tag 的 subject，未知则取 "any"）查询 `defaultReferenceRegistry`，打印结果。

### 8.2 不做 `slipway preset --references`（明确不在范围）

`preset` 仍然是 workflow preset 选择器。Reference shelf 通过 `next` 表达，不通过 `preset`。v5 不把 `preset` 改成 catalog 浏览器。

### 8.3 不做 `slipway next --skill <name>`（明确不在范围）

v2 的 `--skill` 选择器想法仍然不在范围。skill 选择仍由现有的 state 驱动机制决定。Reference 是**追加性展示**，而不是可选择的目的地。

---

## 9. chezmoi 扩展（Wave 1）

加入 `~/.local/share/chezmoi/.chezmoiexternal.toml.tmpl`：

| 来源 | 路径 |
|---|---|
| `wshobson/agents` plugin `conductor` skill `workflow-patterns` | `~/.agents/skills/conductor/workflow-patterns/` |
| `wshobson/agents` plugin `agent-teams` skill `task-coordination-strategies` | `~/.agents/skills/agent-teams/task-coordination-strategies/` |
| `wshobson/agents` plugin `agent-teams` skill `multi-reviewer-patterns` | `~/.agents/skills/agent-teams/multi-reviewer-patterns/` |
| `trailofbits/skills` plugin `ask-questions-if-underspecified` | `~/.agents/skills/ecosystem/trailofbits/ask-questions-if-underspecified/` |
| `trailofbits/skills` plugin `workflow-skill-design` skill `designing-workflow-skills` | `~/.agents/skills/ecosystem/trailofbits/designing-workflow-skills/` |
| `wshobson/agents` plugin `incident-response` skill `incident-runbook-templates` | `~/.agents/skills/incident-response/incident-runbook-templates/` |
| `wshobson/agents` plugin `incident-response` skill `postmortem-writing` | `~/.agents/skills/incident-response/postmortem-writing/` |

7 个新的 chezmoi 条目。Wave 1 `chezmoi apply` 之后，Waves 2–4 所需的全部 35 个源都在本地 `~/.agents/skills/` 这一稳定路径下可用。

---

## 10. 实施 Wave

### Wave 1 — 引擎 + Schema 改造（基础）

**目标：** 落地契约变更，对现有 9 个 gate 行为零影响。

**改动文件：**
- `internal/engine/skill/skill.go`（Definition 字段、defaultReferenceRegistry、Phase/Subject/Tier 枚举）
- `internal/engine/skill/registry_loader.go`（governanceFrontMatter parser）
- `internal/engine/progression/skill_resolution.go`（ChangeContext、ShouldFire、条件 helper）
- `internal/engine/progression/advance_governed.go`（在 gate 选择中调用 ShouldFire）
- `internal/model/config.go`（error-path / user-facing / architecture 文件 pattern 配置）
- `internal/tmpl/templates_test.go`（迁移到新契约）
- `cmd/next.go`（--references flag）
- `~/.local/share/chezmoi/.chezmoiexternal.toml.tmpl`（7 个新源）
- `docs/plans/2026-04-11-skills-integration-plan.zh-CN.md`（移植 v5 内容）

**新建文件：**
- `internal/engine/skill/phase_subject_tier.go`（枚举 + 校验器）

**完成判据：**
- `go test ./...` 全绿
- `go vet ./...` 干净
- 在 scratch 项目上 `slipway next` 输出与之前一致（现有 9 个 gate 的新字段全部是默认值）
- `slipway next --references` 输出空的 References 段（还没有 reference skill 注册）
- `chezmoi apply` 成功；`~/.agents/skills/conductor/workflow-patterns/SKILL.md` 存在

### Wave 2 — 现有 skill 加固 + checklist-quality 拆分

**目标：** 落地 v4 §7.2 的 6 个合并规格并拆分 `checklist-quality.md`。

**改动文件：**
- `internal/tmpl/templates/skills/intake-clarification/SKILL.md`（5.1）
- `internal/tmpl/templates/skills/code-quality-review/SKILL.md`（或 `.tmpl`）（5.2）
- `internal/tmpl/templates/skills/wave-orchestration/SKILL.md.tmpl`（5.3）
- `internal/tmpl/templates/skills/tdd-governance/SKILL.md.tmpl`（5.4）
- `internal/tmpl/templates/skills/codebase-mapping/SKILL.md`（5.5）
- `internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl`（5.6）
- `internal/engine/skill/skill.go`（给现有 9 条加新字段：phase、subject、tier、reason_code、hard_gate、references、order_after 在适用处）

**新建文件：**
- `internal/tmpl/templates/skills/checklist-quality/intake.md`
- `internal/tmpl/templates/skills/checklist-quality/plan.md`
- `internal/tmpl/templates/skills/checklist-quality/review.md`
- `internal/tmpl/templates/skills/checklist-quality/test.md`

**删除文件：**
- `internal/tmpl/templates/skills/checklist-quality.md`（被目录替换）

**完成判据：**
- 6 个加固 skill 都有新节；结构测试扩展为对新节标识的断言
- 现有 9 条 registry 条目都有完整的 v5 frontmatter
- `spec-compliance-review` 引用的是 `checklist-quality/review.md` 而不是整目录

### Wave 3 — 新 gate（10 个）

**目标：** 落地 10 个新 gate，总数到 19。

**新建文件：**
- 10 个新的 `internal/tmpl/templates/skills/<gate-name>/SKILL.md`（必要时 `.tmpl`）
- 每个含完整 frontmatter、body skeleton、强制 checklist、失败处理

**改动文件：**
- `internal/engine/skill/skill.go`（在 `defaultGovernanceRegistry` 加 10 条新条目）
- `internal/model/reason_code.go`（§6.5 的 10 个新 reason code 常量）
- `internal/engine/progression/advance_governed.go`（state 特定路由微调）
- `internal/tmpl/templates_test.go`（10 个新 skill 的结构断言）
- `cmd/<various>_test.go`（条件触发的端到端覆盖）

**完成判据：**
- 19 个 gate 注册，19 个 gate 通过拓扑排序循环检查
- 每个条件 gate 仅在其声明条件下触发（用合成 ChangeContext 的表驱动测试覆盖）
- 冒烟：在 scratch 项目上 `slipway init → new → next → done`，覆盖 10 个条件触发场景

### Wave 4 — Reference Shelf（12 个）

**目标：** 落地 12 个 slipway 风格的 reference skill。

**新建文件：**
- 12 个新的 `internal/tmpl/templates/skills/<reference-name>/SKILL.md`，每个 ~150–200 LoC
- 每个有 `tier: reference` frontmatter 和 §7.2 的 body 形态

**改动文件：**
- `internal/engine/skill/skill.go`（在 `defaultReferenceRegistry` 加 12 条）
- `internal/tmpl/templates_test.go`（12 个 reference skill 的结构断言）
- `cmd/next.go`（--references 输出非空；集成测试）

**完成判据：**
- 在 scratch 项目上 `slipway next --references` 对每个 state 输出正确子集
- `TestReferenceTargetsExist` 通过（每个 `references:` 条目都解析到真实 `~/.agents/skills/` 路径）
- `docs/skills/INDEX.md` 由 registry 重新生成（每个 skill 一行，按 phase 分组）

---

## 11. 实施风险

1. **模板渲染 vs 静态 markdown。** `spec-compliance-review`、`wave-orchestration`、`tdd-governance` 是 `.tmpl`。Wave 2 的结构测试必须先渲染再断言，或者直接对 `.tmpl` 源断言。
2. **`checklist-quality.md` 引用如果不原子迁移会断。** Wave 2 的 `spec-compliance-review/SKILL.md.tmpl:35` 引用必须在同一个 commit 内随目录创建一起更新。
3. **`AgentHint` 对 gate 是必填。** 10 个新 gate 都必须设置。复用现有值（`slipway-planner`、`slipway-auditor`、`slipway-orchestrator`、`slipway-reviewer`、`slipway-verifier`、`slipway-closer`）；v5 不引入新 hint。
4. **拓扑排序在 registry 加载时 fail loud。** 加单测确认 19 gate registry 无环。如果未来某 plan 添加的 gate 形成环，binary 拒绝启动。
5. **条件触发依赖 `ChangeContext` 的准确性。** 检测 helper（`touchesUserFacing`、`touchesErrorPath`、`touchesArchitecture`）是 file-glob 启发式，会有假阳和假阴。缓解：每个 helper 在 `internal/model/config.go` 里有项目专属 pattern 的 config 覆盖。
6. **`subject` 在 intake 时打 tag。** v5 要求变更 context 携带 `Subject` 值。Wave 1 必须扩展变更解析器，从 intake artifact（`artifacts/changes/{slug}/intake/subject.yaml` 或类似路径）读取 subject。如果 subject 未设，条件 gate 默认"触发"（safe-by-default）。
7. **Reason code 是稳定标识。** §6.5 的 10 个 reason code 一旦发布就不能没有 deprecation 流程地改名。Wave 3 起名时要小心。
8. **chezmoi 拉取仅限本地。** Wave 1 的 `.chezmoiexternal.toml.tmpl` 改动只在能 `chezmoi apply` 到源的机器上工作。CI 应该回退到读 `~/ghq/.../<repo>/...` 路径或跳过 reference-existence 断言。
9. **`zh-CN.md` 兄弟文件漂移。** 当前 zh-CN 内容是空文件。Wave 1 必须移植 v5 内容（即本文件），并保持同步。
10. **Wave 3 的 gate 数量是拐点。** 从 9 涨到 19 是 schema 价值的核心兑现处 —— 但也是 review 负担尖峰处。如果 review 速率是瓶颈，把 Wave 3 拆成每 2–3 个 gate 一个 PR（共 5 个子 PR）。

---

## 12. 显式不做

- 不做 `slipway next --skill <name>` 选择器。skill 选择仍由 state 驱动。
- 不做 `slipway preset` 重构。preset 仍是 workflow preset 选择器。
- 不把 technique（`tdd`、`systematic-debugging`、`code-review-protocol`、`codebase-mapping`）提升为 gate。technique 在 gate **内部**被引用。
- `defaultReferenceRegistry` 不包含 `agentic-actions-auditor`、`dimensional-analysis`、`mutation-testing`、`entry-point-analyzer`、`spec-to-code-compliance`、`testing-handbook-skills/*`。这些按 §2.6 显式排除。
- 不引入新 `AgentHint` 值。复用现有 6 个。
- 不为旧 `governanceFrontMatter` 形状写向后兼容 shim。Wave 1 一次性迁移测试契约并锁定。

---

## 13. 等 Owner 拍板的开放决策

以下 6 项需要在 Wave 1 启动前 yes/no。v5 对每项给出推荐。

1. **采纳 6 个新 `Definition` 字段及 `Tier`/`Phase`/`Subject` 枚举（§4.1）？** *推荐：是 —— 这是其它一切的基础。*
2. **采纳带 `OrderAfter` 的拓扑排序 gate 排序（§4.4）？** *推荐：是 —— 没有它，新的 S3 gate 没有定义良好的顺序。*
3. **采纳条件触发机制（`SubjectGated`、`PivotConditional`、`UserFacingConditional`、`ErrorPathConditional`、`MaterialDesignConditional`），而非另选条件 DSL（§4.5）？** *推荐：是 —— 显式 bool flag 比条件 DSL 更简单也更易测。*
4. **加入 `slipway next --references`（§4.6、§8.1），但**不**加 `--skill`、**不**重做 `preset`（§8.2、§8.3）？** *推荐：是 —— 用最小 CLI 表面解锁 reference shelf。*
5. **在 Wave 2 而非另开方案中拆分 `checklist-quality.md` 为 4 个领域 checklist（§5.7、§7）？** *推荐：是 —— 四 checklist 拆分是个小的 Wave 2 增量，避免 sidecar 与 v5 词汇不一致。*
6. **Wave 3 拆 5 个子 PR（每 2–3 gate 一个）还是一个大 PR？** *推荐：5 个子 PR —— Wave 3 的瓶颈是 review 负担，而不是引擎工作。*

---

## 14. 验证索引（继承自 v4 §13）

审计中端到端读过的文件。§2 的每条论断都映射到这里某一项。任意一项都可以重新读以再验证。

**Trailofbits：**
- `~/ghq/github.com/trailofbits/skills/plugins/agentic-actions-auditor/skills/agentic-actions-auditor/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/differential-review/skills/differential-review/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/property-based-testing/skills/property-based-testing/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/spec-to-code-compliance/skills/spec-to-code-compliance/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/ask-questions-if-underspecified/skills/ask-questions-if-underspecified/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/entry-point-analyzer/skills/entry-point-analyzer/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/workflow-skill-design/skills/designing-workflow-skills/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/skill-improver/skills/skill-improver/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/testing-handbook-skills/skills/`（15 项目录列表）
- `~/ghq/github.com/trailofbits/skills/plugins/mutation-testing/skills/mutation-testing/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/dimensional-analysis/skills/dimensional-analysis/SKILL.md`
- `~/ghq/github.com/trailofbits/skills/plugins/audit-context-building/skills/audit-context-building/SKILL.md`

**Anthropics / OpenAI / Getsentry：**
- `~/ghq/github.com/anthropics/skills/skills/skill-creator/SKILL.md`
- `~/ghq/github.com/openai/skills/skills/.curated/security-threat-model/SKILL.md`
- `~/ghq/github.com/openai/skills/skills/.curated/security-best-practices/SKILL.md`
- `~/ghq/github.com/openai/skills/skills/.curated/security-ownership-map/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/skill-scanner/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/code-simplifier/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/claude-settings-audit/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/security-review/SKILL.md`
- `~/ghq/github.com/getsentry/skills/plugins/sentry-skills/skills/find-bugs/SKILL.md`

**wshobson：**
- `~/ghq/github.com/wshobson/agents/plugins/agent-teams/skills/multi-reviewer-patterns/SKILL.md`（约 127 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/conductor/skills/workflow-patterns/SKILL.md`（约 623 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/agent-teams/skills/task-coordination-strategies/SKILL.md`（约 163 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/developer-essentials/skills/error-handling-patterns/SKILL.md`（约 632 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/framework-migration/skills/dependency-upgrade/SKILL.md`（约 368 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/developer-essentials/skills/e2e-testing-patterns/SKILL.md`（约 535 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/documentation-generation/skills/architecture-decision-records/SKILL.md`（约 441 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/incident-response/skills/incident-runbook-templates/SKILL.md`（约 471 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/incident-response/skills/postmortem-writing/SKILL.md`（约 390 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/documentation-generation/skills/changelog-automation/SKILL.md`（约 572 LoC）
- `~/ghq/github.com/wshobson/agents/plugins/developer-essentials/skills/code-review-excellence/SKILL.md`
- `~/ghq/github.com/wshobson/agents/plugins/developer-essentials/skills/debugging-strategies/SKILL.md`

**Slipway 内部：**
- `~/ghq/github.com/signalridge/slipway/internal/engine/skill/skill.go:1-100`
- `~/ghq/github.com/signalridge/slipway/internal/engine/skill/registry_loader.go:40-44, 168-199`
- `~/ghq/github.com/signalridge/slipway/internal/engine/progression/skill_resolution.go`
- `~/ghq/github.com/signalridge/slipway/internal/tmpl/templates/skills/spec-compliance-review/SKILL.md.tmpl`
- `~/ghq/github.com/signalridge/slipway/internal/tmpl/templates/skills/intake-clarification/SKILL.md`
- `~/ghq/github.com/signalridge/slipway/internal/tmpl/templates/skills/checklist-quality.md`
- `~/.local/share/chezmoi/.chezmoiexternal.toml.tmpl`
- `~/.agents/skills/`（顶层 + ecosystem/ 遍历；确认 167 个 SKILL.md）
