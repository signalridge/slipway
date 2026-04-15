# Skills 强化计划 —— Wave 3（草案）

**状态。** 草案；依赖 Wave-2 的结项门禁（见
`2026-04-15-skills-wave2-plan*.md` §6 第 6 条）以及
`2026-04-15-route-surface-refactor-plan*.md` 的 PR-1 / PR-2 / PR-3
按 cross-plan 顺序全部落完。在 Wave-2 的 PR-1 / PR-2 / PR-3 全部合并、
其结项 / metrics report 通过评审且 route-surface 重构完整落地之前，不得
开启本波实施 PR。下面的每技能清单与 hydrate 绑定行为暂定，必须在
Wave-3 PR-1 合并前，根据 Wave-2 metrics 与重构后的 surface model 重新校核。

## 1. 背景

Wave-1 覆盖了 10 个源语料深的高杠杆技能；Wave-2 在 route-surface 重构
之后覆盖了 references 结构平行的分析族，并在同一套重构后 surface 上把
`coverage-analysis` 显式冻结为 no-op。因此 Wave-3 在同一套重构后的
surface model（`--focus` / `--view` alias，无 `BindingCommandManual`，
hosts-only 技能归 host/support-only）之上覆盖本轮 strengthening family
剩余的最后 9 个 scoped skills。这一波有三条属性与 Wave-1 / Wave-2 不同：

- **上游大多偏薄。** 本波涉及的源技能多半只有一份 `SKILL.md`，没有
  `references/` 架子。因此杠杆来自 *body 再平衡* + *选择性脚本 lift* +
  *工作流面板的 hydrate wiring*，而不是批量 references 迁移。
- **领域是工作流 / authoring / 协作**，不是安全调查。hydrate 用例服务于
  规划、评审、恢复流程。
- **脚本机会真实但窄。** `iterate-pr` 与 `gh-review-requests` 里有 Python
  助手（`fetch_pr_checks.py`、`fetch_pr_feedback.py`、`reply_to_thread.py`、
  `fetch_review_requests.py`）可以评估是否 lift，但
  `fetch_review_requests.py` 的上游解释器门槛更高（`>=3.12`）。只有在它被
  真实窄化到共享的 Wave-1 Python 契约后，本波才会把这组 helper 纳入交付；
  其它技能仍保持 prose-only。

本波价值因此集中在三处（见 §3–§5），由构造上小于 Wave-1 / Wave-2。

## 2. 非目标

- 不引入新的 hydrate 契约形状或 selection path；完全复用 Wave-1 PR-4a /
  PR-4b。
- 不再做 tier budget 上调。
- 不新增 catalog 技能。
- 不改写任何 `skills_ref/` 上游。
- 不强迫薄源技能塞进 reference-heavy 模板。若一个技能确实除了 `SKILL.md`
  就没东西，本波对它的工作只限 body 再平衡 + hydrate wiring；不必为了
  齐整而空建 `references/`。
- 不在 Wave-3 里回填 `coverage-analysis`。该技能已在 Wave-2 中被显式冻结：
  它已经落在 T1 target 内，没有值得单独开 PR 的上游 `references/` /
  `scripts` 架子，且当前 host 路径已与重构后 surface 对齐。

## 3. PR-1 —— body 再平衡 + 上游允许处的 references

**目标。** 对 9 个技能，要么 (a) 对照 Wave-1 已抬升的 T1/T2 target 把
body 收紧 / 扩厚，要么 (b) 在上游支持的情况下补少量 references。除非下表
逐行写明理由，否则本波不允许策展合成。

### 逐技能计划

| 技能 | 动作 | 计划产物 |
|------|------|----------|
| `scope-clarification` | 仅 body 再平衡 | 无。上游为 `trailofbits/ask-questions-if-underspecified/SKILL.md`（单文件）。若 body 已达标，记"no change"继续。 |
| `plan-authoring` | body 再平衡 + 1 个 reference | `references/plan-document-review-prompt.md`，lift 自 `superpowers/writing-plans/plan-document-reviewer-prompt.md`。这是一份评审 prompt，不是规划 prompt；命名反映角色。 |
| `tdd-proof` | body 再平衡 + 1 个 reference | `references/testing-anti-patterns.md`，lift 自 `superpowers/test-driven-development/testing-anti-patterns.md`（逐字或轻度裁剪）。 |
| `fresh-verification-evidence` | 仅 body 再平衡 | 无。上游为单文件 `SKILL.md`。 |
| `parallel-executor-contract` | 仅 body 再平衡 | 无。上游为单文件 `SKILL.md`。 |
| `multi-reviewer-calibration` | body 再平衡 + 1 个 reference | `references/review-dimensions.md`，lift 自 `wshobson/multi-reviewer-patterns/references/review-dimensions.md`。`alirezarezvani/pr-review-expert/SKILL.md` 并入所属 `SKILL.md` body，而非单独成 reference。 |
| `git-recovery` | 仅 body 再平衡 | 无。上游 `wshobson/git-advanced-workflows` 与 `wshobson/block-no-verify-hook` 都是单文件 `SKILL.md`，内容并入 body。 |
| `review-comment-triage` | PR-1 只做 body 再平衡；references 内容融入正文 | references 轨道无新增。本技能主要杠杆在 scripts 轨道（PR-2）。 |
| `ci-triage` | PR-1 只做 body 再平衡；PR-2 追加 1 个脚本 lift | 无 reference。`getsentry/iterate-pr/scripts/fetch_pr_checks.py` 在 PR-2 落到 `scripts/fetch-pr-checks.py`；PR-1 仍只收紧 body，不硬塞 reference。 |

### 蒸馏规则

与 Wave-1 PR-1 同：保留条件触发的操作性内容、丢弃叙事动机、优先与源对齐
的文件名、并把 collapsed/deferred 段落记录到 PR mapping log。本波严格执行
"curator 添加必须有理由"：除非上表逐行写明合成及理由，否则不得新增任何
无源对齐的 reference 文件。

### 代码改动

- `internal/tmpl/templates/skills/<id>/SKILL.md` —— 逐行按表编辑 body。
- `internal/tmpl/templates/skills/<id>/references/*.md` —— 仅为 3 个实际
  获得 references 的技能新增（plan-authoring、tdd-proof、
  multi-reviewer-calibration）。
- `internal/engine/capability/registry_default.go`、`registry_b4.go`
  —— 仅为 3 个现在带 references 的技能
  填入 `Skill.HydrateReferences`。薄源技能保持空的 hydrate 列表
  （Wave-1 PR-4a 测试已接受空）。

### 要加 / 扩的测试

- `internal/toolgen/toolgen_test.go::TestCatalogSkillHasReferences` ——
  输入扩到 `plan-authoring`、`tdd-proof`、`multi-reviewer-calibration`。
  **不要**扩到其余 6 个 Wave-3 技能，它们按设计就没有 references。
- `internal/engine/capability/gates_test.go::TestSizeBudgetsForRegisteredSkills`
  （Wave-1）自动覆盖 body 再平衡。
- Wave-3 PR-1 不加新的 hydrate 契约测试；Wave-1 的
  `TestFrontmatterMirrorsRegistryHydrateReferences` 与
  `TestHydrateReferencesResolveToFiles` 自动扩展。

### 验收

- 每个 Wave-3 技能的 body 落在 T1/T2 target 内（不仅是 hard-max）；
  warning-band 结果必须在 PR notes 明写。
- 3 份新 references 满足 24 KB / 文件、64 KB / 技能的上限。
- PR notes 逐薄源技能记录 body 再平衡是否吸收了额外上游素材（带 mapping
  notes 中的源链接），还是属于 no-op。
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  通过。

## 4. PR-2 —— 从 `iterate-pr` / `gh-review-requests` lift 脚本

**目标。** 把两个 getsentry 上游的 4 个 Python 助手 lift 到 Slipway
script 轨道，按 Slipway runtime 契约窄化，并按最接近的现有 skill 边界拆分
owner：`ci-triage` 承接 CI 状态抓取，`review-comment-triage` 承接评审反馈 /
线程回复 / review-request 辅助流。不再为了省事，把 4 个 helper 强行挂到
同一个技能上。

### 计划脚本

| 脚本 | 所属技能 | 用途 | Lift 来源 |
|------|----------|------|-----------|
| `scripts/fetch-pr-checks.py` | `ci-triage` | 抓取 PR 的 CI check run 状态；确定性 JSON 输出；无副作用。 | `getsentry/iterate-pr/scripts/fetch_pr_checks.py` |
| `scripts/fetch-pr-feedback.py` | `review-comment-triage` | 抓取 PR 的评审评论 / 评审线程；确定性 JSON 输出。 | `getsentry/iterate-pr/scripts/fetch_pr_feedback.py` |
| `scripts/reply-to-thread.py` | `review-comment-triage` | 回复指定评审线程。**有写入副作用——必须显式 `--confirm`，并在提交前把完整请求体打到 stderr。** | `getsentry/iterate-pr/scripts/reply_to_thread.py` |
| `scripts/fetch-review-requests.py` | `review-comment-triage` | 列出某用户开放中的 review requests；确定性 JSON 输出。 | `getsentry/gh-review-requests/scripts/fetch_review_requests.py` |

### 约束

- 沿用 Wave-1 PR-2 Python runtime 契约。
- **`reply-to-thread.py` 可写。** 按 Slipway 用户级 CLAUDE.md 的
  blast-radius 规则（授权不越出 scope），这个脚本必须默认 dry-run：
  打印预期 HTTP 请求后以非零退出，除非传 `--confirm`。不设例外。
  这不仅是 Slipway 约定，也是所属技能（`review-comment-triage`）应为其
  调用者树立的"默认安全"姿态。
- 4 个脚本都依赖 `gh` 或 GitHub token。`ci-triage` 与
  `review-comment-triage` 两个所属 `SKILL.md` 都必须描述各自 helper 的 token
  要求；脚本在缺凭证或 `gh` 不可用时必须 fail-fast 并给出可操作信息。
- 不要在上游已有的依赖之外额外引入 `gh`。若上游脚本直接用 `requests`，
  保留原样，不为了统一而改走 `gh`。
- `fetch-review-requests.py` 是本范围内唯一一个上游显式声明更高解释器门槛
  （`requires-python >=3.12`）的 helper。本波**不**静默继承这个例外。
  committed 路径是先把它窄化到共享的 Wave-1 Python 契约，再把这次
  interpreter-floor narrowing 记入 PR notes；若无法干净完成，就应停下并修订
  计划，而不是硬塞一个 3.12 特例。
- Wave-3 **不**承接任何 provenance bookkeeping。任何元数据或
  source-coverage 清理都只属于结项收束点
  `2026-04-16-knowledge-only-refactor-plan*.md`。若某个候选 helper lift 不能
  在不触碰那个清理面的前提下干净落地，就应停下并修订范围，而不是把本波扩成
  元数据维护。
- PR notes 必须记录每个脚本的 lift 来源，以及任何窄化（例如"上游有
  `--verbose` debug 模式，本波未保留"）。

### 代码改动

- `internal/tmpl/templates/skills/ci-triage/scripts/fetch-pr-checks.py`
  —— 新增 CI 状态抓取脚本。
- `internal/tmpl/templates/skills/review-comment-triage/scripts/fetch-pr-feedback.py`
  —— 新增评审反馈抓取脚本。
- `internal/tmpl/templates/skills/review-comment-triage/scripts/reply-to-thread.py`
  —— 新增默认 dry-run 的线程回复脚本。
- `internal/tmpl/templates/skills/review-comment-triage/scripts/fetch-review-requests.py`
  —— 新增 review-request 辅助脚本。
- `internal/tmpl/templates/skills/ci-triage/SKILL.md` 与
  `internal/tmpl/templates/skills/review-comment-triage/SKILL.md`
  —— 补 helper 入口、凭证要求与读写姿态说明；不要把这些 runtime 前提只埋在
  脚本注释里。

### 要加的测试

- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit` —— 扩展。
- `internal/toolgen/toolgen_test.go::TestScriptStaticChecks` —— 扩展
  （只有在完成上面的共享契约窄化后，4 个脚本才统一过
  `python3 -m py_compile`；Wave-3 不保留更高解释器门槛的特例）。
- `internal/toolgen/toolgen_test.go::TestScriptFixtureContracts`：
  - `fetch-pr-checks.py` / `fetch-pr-feedback.py` /
    `fetch-review-requests.py`：`GH_TOKEN=invalid`（或等价）时断言
    凭证错误输出稳定；无网络回退。
  - `reply-to-thread.py`：不传 `--confirm` 时断言 dry-run 输出含预期
    请求行且脚本非零退出；传 `--confirm` 但 `GH_TOKEN=invalid` 时走
    凭证错误路径。

### 验收

- 4 个脚本都过静态检查与 fixture 契约测试。
- 测试验证：`reply-to-thread.py` 不传 `--confirm` 无法提交。
- `init --tools codex --refresh` 把脚本写进生成的 skill 树。
- Wave-3 交付出的脚本里，不允许残留高于共享 Wave-1 契约的一次性 Python
  runtime floor。
- 不允许为了镜像 helper-lift 的 source metadata，把 Wave-3 PR 扩成
  provenance bookkeeping；这类清理只属于
  `2026-04-16-knowledge-only-refactor-plan*.md`。

## 5. PR-3 —— 工作流面板的 hydrate wiring

**目标。** 在重构后的 surface model 上，验证并在必要时做最小修复，使 3 个
Wave-3 有 references 的技能在各自实际承载的 selection path 上正确暴露
hydrate：host/support-only 技能走 host-embedded 路径，
`multi-reviewer-calibration` 走 `--focus calibration` alias。带脚本的
`ci-triage` 与 `review-comment-triage` 本波都不获得 routed hydrate surface。
薄源技能（无 references、无脚本）在本波不获得 hydrate 行。真正的
`Skill.HydrateReferences` 声明在更早的 PR-1 与 reference/frontmatter
一起落地。

### 首轮 binding 表

在写测试之前必须对照 b0 / b2 / b4 / b5 的 registry 构造函数与
surface-policy registry 核对；两者是权威。

| 技能 | 重构后暴露 | selection path | 初始 hydrate refs | 首次暴露面 |
|------|-----------|----------------|-------------------|------------|
| `plan-authoring` | host/support-only（重构 plan §5.5） | host-embedded 在 `plan-audit` host（当前 `registry_default.go` 已是该形状）；无公共 `--focus` 或 `--mode` selector | `plan-document-review-prompt.md` | `plan-audit` host 激活时（planning host 路径） |
| `tdd-proof` | host/support-only（重构 plan §5.5） | host-embedded 在 `tdd-governance` / `wave-orchestration` + technique-hint 在 `tdd-governance`（当前 `registry_default.go` 已是该形状）；无公共 `--focus` 或 `--mode` selector | `testing-anti-patterns.md` | 上述 host 任一激活时 |
| `multi-reviewer-calibration` | `review` 的 `--focus calibration`（重构 plan §5.3）；保留 `code-quality-review` host-embedded attachment | explicit focus alias 通过 surface-policy 解析到 backing skill；host 路径继续把该技能作为 support 附着、不单独开 hydrate 面（重构的 `TestCalibrationHostAttachmentSurvivesFocusMigration` 保护该行为） | `review-dimensions.md` | `review --focus calibration` |
| `ci-triage` | `repair` / `status` 的 suggested-only（重构 plan §5.2）；无公共 explicit selector | resolver `SuggestedCapabilities[]`；scripts 轨道暴露 helper | 无（纯脚本技能，hydrate refs 空） | `repair` / `status` 上的 suggestion channel；hydrate 面保持空 |
| `review-comment-triage` | `repair` 的 suggested-only（重构 plan §5.2）；无公共 explicit selector | resolver `SuggestedCapabilities[]`；scripts 轨道暴露助手 | 无（纯脚本技能，hydrate refs 空） | `repair` 上的 suggestion channel；hydrate 面保持空 |

关于 script-only suggested skills：`ci-triage` 与 `review-comment-triage`
通过各自 trigger clauses 经 resolver 的 `SuggestedCapabilities[]` channel
承载，不存在公共 selector —— route-surface 重构 PR-3 已把
`BindingCommandManual` 从 taxonomy 中移除，因此这两个技能不再有任何
manual-selector 公共入口。它们在 Wave-3 的价值都在 scripts 轨道，不在
reference 语料。空 hydrate 行仍列出，是为了声明"空不是遗漏"。

### 代码改动

- PR-3 不再新增 `Skill.HydrateReferences` 声明。这些记录在 PR-1 就已落地，
  以保证现有 frontmatter-vs-registry gate 从第一个 reference-bearing PR
  开始就是一致的。
- 不为挂 hydrate 新增任何 binding。Wave-1 PR-4a 已经在 host-embedded 路径
  当 `Skill.HydrateReferences` 非空且 host 激活时渲染 hydrate，
  route-surface 重构 PR-2 已经把 `--focus calibration` 通过 surface-policy
  解析到 backing-skill hydrate。
- 默认预期：`cmd/*.go` 与生产 resolver 代码都无需改动。若重构后的
  host / focus path 没有把 PR-1 已声明的 refs 正确暴露出来，PR-3 只携带
  恢复该路径所需的最小修复。
- Wave-1 PR-4b 的 32 KB hydrate 输出上限仍生效。

### 要加的测试

- `internal/engine/capability/gates_test.go::TestFrontmatterMirrorsRegistryHydrateReferences`
  —— 自动扩展。
- `internal/engine/capability/resolver_test.go` —— 新增 case 证明：
  - `plan-authoring` 的 hydrate keys 在 `plan-audit` host path 上暴露。
  - `tdd-proof` 的 hydrate keys 在 `tdd-governance`（与
    `wave-orchestration`）host path 上暴露。
  - `multi-reviewer-calibration` 的 hydrate keys 通过 surface-policy 的
    `--focus calibration` 解析暴露；`code-quality-review` host path
    继续作为 support 附着而不单独开 hydrate 面。
  - `ci-triage` 与 `review-comment-triage` 在 suggestion path 上都不会暴露
    hydrate keys。
- `cmd/hydrate_view_test.go` —— golden 用例：`review --focus calibration`
  列出 `multi-reviewer-calibration/review-dimensions.md`；否定 golden：
  `review --mode=multi-reviewer-calibration` 返回重构的
  `unknown_route_mode` usage 错误，且 `review --focus calibration`
  不会列出 `ci-triage/*` 或 `review-comment-triage/*` 的 hydrate key。

### 验收

- `plan-authoring`、`tdd-proof` 与 `multi-reviewer-calibration` 在上面
  binding 表列出的重构后 selection path 上暴露 hydrate keys。
- `ci-triage` 与 `review-comment-triage` 在任何位置都不暴露 hydrate keys
  —— 由 `resolver_test.go` 与 `cmd/hydrate_view_test.go` 的否定用例强制。
- 不允许重新引入 raw `--mode=<skill-id>` selector；任何这类尝试必须走
  重构的 `unknown_route_mode` 路径。
- 既有 `cmd/...` / `capability` golden 无回归。

## 6. 执行顺序与门禁

1. **PR-1 先。** body 再平衡 + 3 份新 references。
2. **PR-2 次。** 脚本 lift。依赖 PR-1 只是为了所属清晰（哪个技能承接哪个
   脚本）；body 再平衡是改所属 `SKILL.md` 不产生无谓 churn 的前置条件。
3. **PR-3 最后。** hydrate wiring。依赖 PR-1 references 落盘，**不能**
   与 PR-1 并行。
4. 每个 PR 合并前跑与 Wave-1 §8.5 / Wave-2 §6.3 同三道硬门禁，按重构后
   的 surface model 适配。命令冒烟检查为：
   - `review --focus calibration --json`
   - `review --list-focuses --format=json`（必须含 `calibration`）
   - 一条否定冒烟：断言 `review --mode=multi-reviewer-calibration`
     （或任何其他 raw skill-id selector）返回 `unknown_route_mode`
   - host-path 验证：`plan-authoring` / `tdd-proof` 的 hydrate keys
     只在对应 host 激活时出现（由 §5 的 resolver 测试覆盖，而不是
     用户调用层面的命令冒烟）
5. **英文与 zh-CN 同步**，同规则。
6. **整波结项报告。** Wave-3 PR-3 合并后 7 天内产出一份短的结项 report，
   覆盖：哪些薄源技能在 PR-1 保持 no-op、`review-comment-triage` 的脚本
   是否覆盖预期操作流、是否有 warning-band body 持续存在、以及
   `plan-authoring` / `tdd-proof` 的 host-path hydrate 暴露是否与 §5
   描述一致。report 通过评审后，本轮强化范围对当前源语料视为
   "强化完成"，`2026-04-16-knowledge-only-refactor-plan*.md`
   方可开展。

## 7. 不在范围

- 改写 `skills_ref/`；本波只追加指针。
- 给任何 Wave-3 技能加 typed partials。`independent-review` 仍是 PR-3
  typed-partials 的 reference 示例；本波不让任何其他 Wave-3 技能承载
  typed partial。
- 改动 hydrate 契约、selection path、tier budget 或其他 Wave-1 基础设施。
  若某 Wave-3 实施 PR 需要这类改动，停下来升级讨论。
- 本波只覆盖 §§3–5 枚举的技能集合；不会把无关的 plugin-shape 或
  incident-responder 源语料重新纳入当前 strengthening family。
