# Skills 强化计划 —— Wave 3（草案）

**状态。** 草案；依赖 Wave-2 的结果（见
`2026-04-15-skills-wave2-plan.md` §6.5）。在 Wave-2 PR-1 已合并且其 metrics
report 通过评审之前，不得开启本波实施 PR。下面的每技能清单与 hydrate 绑定
行为暂定，必须在 Wave-3 PR-1 合并前，根据 Wave-2 metrics 重新校核。

## 1. 背景

Wave-1 覆盖了 10 个源语料深的高杠杆技能；Wave-2 覆盖了 references 结构平行
的分析族；Wave-3 覆盖剩余 9 个 catalog 技能。这一波有三条属性与
Wave-1 / Wave-2 不同：

- **上游大多偏薄。** 本波涉及的源技能多半只有一份 `SKILL.md`，没有
  `references/` 架子。因此杠杆来自 *body 再平衡* + *选择性脚本 lift* +
  *工作流面板的 hydrate wiring*，而不是批量 references 迁移。
- **领域是工作流 / authoring / 协作**，不是安全调查。hydrate 用例服务于
  规划、评审、恢复流程。
- **脚本机会真实但窄。** `iterate-pr` 与 `gh-review-requests` 里有 Python
  助手（`fetch_pr_checks.py`、`fetch_pr_feedback.py`、`reply_to_thread.py`、
  `fetch_review_requests.py`）可按 Wave-1 Python 脚本契约 lift。其他技能
  没有脚本机会，保持 prose。

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
| `ci-triage` | 仅 body 再平衡；标"无强上游" | 无。若再平衡后 body 仍落 warning-band，在 PR notes 明确记录；不得为了齐整而发明 references。 |

### 蒸馏规则

与 Wave-1 PR-1 同：保留条件触发的操作性内容、丢弃叙事动机、优先与源对齐
的文件名、provenance 里记录 collapsed/deferred 段落。本波严格执行"curator
添加必须有理由"：除非上表逐行写明合成及理由，否则不得新增任何无源对齐
的 reference 文件。

### 代码改动

- `internal/tmpl/templates/skills/<id>/SKILL.md` —— 逐行按表编辑 body。
- `internal/tmpl/templates/skills/<id>/references/*.md` —— 仅为 3 个实际
  获得 references 的技能新增（plan-authoring、tdd-proof、
  multi-reviewer-calibration）。
- `internal/tmpl/templates/skills/<id>/provenance.yaml` —— 为新 reference
  扩 `inputs:`；当 body 再平衡吸收额外上游素材时也要更新 body 源链接。
- `internal/engine/capability/registry_default.go`、`registry_b2.go`、
  `registry_b4.go`、`registry_b5.go` —— 仅为 3 个现在带 references 的技能
  填入 `Skill.HydrateReferences`。薄源技能保持空的 hydrate 列表
  （Wave-1 PR-4a 测试已接受空）。

### 要加 / 扩的测试

- `internal/toolgen/toolgen_test.go::TestCatalogSkillHasReferences` ——
  输入扩到 `plan-authoring`、`tdd-proof`、`multi-reviewer-calibration`。
  **不要**扩到其余 6 个 Wave-3 技能，它们按设计就没有 references。
- `internal/engine/capability/gates_test.go::TestSizeBudgetsForRegisteredSkills`
  （Wave-1）自动覆盖 body 再平衡。
- Wave-3 PR-1 不加新的 hydrate 契约测试；Wave-1 的
  `TestHydrateReferencesMirrorRegistry` 与
  `TestHydrateReferencesResolveToFiles` 自动扩展。

### 验收

- 每个 Wave-3 技能的 body 落在 T1/T2 target 内（不仅是 hard-max）；
  warning-band 结果必须在 PR notes 明写。
- 3 份新 references 满足 24 KB / 文件、64 KB / 技能的上限。
- PR notes 逐薄源技能记录 body 再平衡是否吸收了额外上游素材（带
  `provenance.yaml` 的源链接），还是属于 no-op。
- `go test ./internal/toolgen/... ./internal/engine/capability/... -count=1`
  通过。

## 4. PR-2 —— 从 `iterate-pr` / `gh-review-requests` lift 脚本

**目标。** 把两个 getsentry 上游的 4 个 Python 助手 lift 到 Slipway
script 轨道，按 Slipway runtime 契约窄化。`review-comment-triage` 是 4 个
脚本唯一的所属技能——它正是 PR feedback / review-request 流的承接者。

### 计划脚本

| 脚本 | 所属技能 | 用途 | Lift 来源 |
|------|----------|------|-----------|
| `scripts/fetch-pr-checks.py` | `review-comment-triage` | 抓取 PR 的 CI check run 状态；确定性 JSON 输出；无副作用。 | `getsentry/iterate-pr/scripts/fetch_pr_checks.py` |
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
- 4 个脚本都依赖 `gh` 或 GitHub token。所属 `SKILL.md` 必须描述 token
  要求；脚本在缺凭证或 `gh` 不可用时必须 fail-fast 并给出可操作信息。
- 不要在上游已有的依赖之外额外引入 `gh`。若上游脚本直接用 `requests`，
  保留原样，不为了统一而改走 `gh`。
- provenance 必须记录每个脚本的 lift 来源，以及任何窄化（例如"上游有
  `--verbose` debug 模式，本波未保留"）。

### 要加的测试

- `internal/toolgen/toolgen_test.go::TestScriptExecutableBit` —— 扩展。
- `internal/toolgen/toolgen_test.go::TestScriptStaticChecks` —— 扩展
  （4 个都过 `python3 -m py_compile`）。
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

## 5. PR-3 —— 工作流面板的 hydrate wiring

**目标。** 为 3 个 Wave-3 有 references 的技能 + 带脚本的
`review-comment-triage`，在它们实际参与的 selection path 上补 hydrate
references。薄源技能（无 references、无脚本）在本波不获得 hydrate 行。

### 首轮 binding 表（暂定）

在写测试之前必须对照 b0 / b2 / b4 / b5 的 registry 构造函数核对；
registry 是权威。若行里的 binding 面不支撑预期 hydrate surface，在
PR notes 标记，不要只为挂 hydrate 而新增 binding。

| 技能 | 已有 bindings（需复核） | selection path | 初始 hydrate refs | 首次暴露面 |
|------|-------------------------|----------------|-------------------|------------|
| `plan-authoring` | TBD | Manual explicit via `--mode=plan-authoring` | `plan-document-review-prompt.md` | manual `--mode` 允许处；可能是 `review` 或专属 plan surface |
| `tdd-proof` | TBD | Manual explicit via `--mode=tdd-proof` | `testing-anti-patterns.md` | `validate` 或 `review` |
| `multi-reviewer-calibration` | TBD | Manual explicit via `--mode=multi-reviewer-calibration` | `review-dimensions.md` | `review` |
| `review-comment-triage` | TBD | Manual explicit via `--mode=review-comment-triage` | 无（纯脚本技能，hydrate refs 空） | scripts 轨道暴露助手；hydrate 行保持空 |

注：`review-comment-triage` 刻意列为 **hydrate refs 空**。它的 Wave-3
价值在 scripts 轨道，不在 reference 语料。这一行之所以仍列，是为了声明
"空不是遗漏"。

### 代码改动

- `internal/engine/capability/<registry_file>.go` —— 为 3 个带 references
  的技能填 `Skill.HydrateReferences`。
- `cmd/*.go` 不改；Wave-1 PR-4a 渲染会自动处理。

### 要加的测试

- `internal/engine/capability/gates_test.go::TestHydrateReferencesMirrorRegistry`
  —— 自动扩展。
- `cmd/hydrate_view_test.go` —— 为 3 个带 references 的技能的
  manual-explicit 路径加 golden 用例。

### 验收

- 3 个带 references 的 Wave-3 技能至少在 1 个命令面暴露 hydrate keys。
- `review-comment-triage` 在任何位置都不暴露 hydrate keys ——
  由 `cmd/hydrate_view_test.go` 的否定 golden 强制。
- 既有 `cmd/...` / `capability` golden 无回归。

## 6. 执行顺序与门禁

1. **PR-1 先。** body 再平衡 + 3 份新 references。
2. **PR-2 次。** 脚本 lift。依赖 PR-1 只是为了所属清晰（哪个技能承接哪个
   脚本）；body 再平衡是改所属 `SKILL.md` 不产生无谓 churn 的前置条件。
3. **PR-3 最后。** hydrate wiring。依赖 PR-1 references 落盘，**不能**
   与 PR-1 并行。
4. 每个 PR 合并前跑与 Wave-1 §8.5 / Wave-2 §6.3 同三道硬门禁。
5. **英文与 zh-CN 同步**，同规则。
6. **整波结项报告。** Wave-3 PR-3 合并后 7 天内产出一份短的结项 report，
   覆盖：哪些薄源技能在 PR-1 保持 no-op、`review-comment-triage` 的脚本
   是否覆盖预期操作流、是否有 warning-band body 持续存在。report
   通过评审后，25 个 catalog 技能对当前源语料视为"强化完成"；后续工作
   只有在 `skills_ref/` 被重导入或操作者提出具体缺口时才启动。

## 7. 不在范围

- 改写 `skills_ref/` provenance；本波只追加指针。
- 给任何 Wave-3 技能加 typed partials。`independent-review` 仍是 PR-3
  typed-partials 的 reference 示例；本波不让任何其他 Wave-3 技能承载
  typed partial。
- 改动 hydrate 契约、selection path、tier budget 或其他 Wave-1 基础设施。
  若某 Wave-3 实施 PR 需要这类改动，停下来升级讨论。
- 适配 `alirezarezvani/prompt-governance` —— 仍按 Wave-1 §9 延期，
  待 plugin 形态支持。
- 从 §4 之外任何技能 lift 脚本。尤其
  `alirezarezvani/incident-response/scripts` 与
  `incident-commander/scripts`（Wave-1 PR-1 已备注延期）保持延期，
  其 Python 响应器目标族不同，不属于 Wave-3。
