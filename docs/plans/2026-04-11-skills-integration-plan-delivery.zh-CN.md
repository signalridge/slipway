# Skills 集成方案 - 交付

## 验收标准

### Kernel 与 catalog 分层

1. `ResolveNextSkill` 继续作为唯一 progression authority。
2. 当前 governed host 继续作为 runtime kernel：
   `intake-clarification`、`research-orchestration`、`plan-audit`、
   `worktree-preflight`、`wave-orchestration`、`tdd-governance`、
   `spec-compliance-review`、`code-quality-review`、`goal-verification`、
   `final-closeout`；其中 9 个是 registry-backed governance definition，
   `worktree-preflight` 继续保持 kernel-owned standalone surface。
3. 新的 25-skill catalog 被定义成独立 catalog layer，而不是第二个 runtime
   state machine。
4. Capability pack 降级成 tag 与文档视图，不再是主架构。
5. Slipway 在正常 runtime flow 中不要求手动调用被吸收的 skill。
6. `review`、`validate`、`repair`、`status`、`health` 继续是 command
   surface，不会变成第二个 workflow engine。

### Catalog 结构

7. `skills_ref/` 继续作为权威 source corpus；如果 rollout batching 采用更窄
   working set，也不会改变 disposition / provenance 覆盖口径。
8. 方案定义了 9 个 domain 下的 25 个独立 Slipway skill。
9. 方案定义了 6 个 source skill 只作为 posture 吸收，不提升成 standalone
   target。
10. 方案定义了 non-catalog disposition matrix，显式覆盖
    `view-only`、`route-only`、`absorbed`、`deferred` source / surface，
    不留下模糊待定项。
11. 每个 target skill 都有明确的 `domain`、`function`、`tier`、
    `primary_attachment`、`summary`、受限 `trigger_signals`、
    `evidence_contract`、`bindings`、`provenance_ref`。
12. 模型显式允许一个 source skill 喂给多个 target skill，也允许一个 target
    skill 吸收多个 source skill，并把 source 决策结构化记录在
    `provenance.yaml` 中。

### Binding registry 与 auto resolver

13. Runtime binding authority 由 Go 侧 binding registry 持有。
14. 生成出来的 `SKILL.md` frontmatter 只负责描述和导出，不是 runtime route
    source of truth。
15. 方案定义了 auto capability resolver 的输入、输出、guardrail，以及
    受限 trigger DSL。
16. Auto resolver 可以附着 support skill，也可以为 routed command 自动选
    路，但不能改变 `ResolveNextSkill` 选出来的下一个 governed host。
17. 显式 operator flag 优先于自动路由。

### 实现面

18. Catalog skill 的 source layout 被定义为：必备 `SKILL.md` 与
    `provenance.yaml`，可选 typed template（`PROSE.tmpl`、
    `CHECKLIST.tmpl`、`VERDICT.tmpl`），以及受约束的可选 support 目录
    （`references/`、`scripts/`）。
19. 方案定义了 assembler 或 toolgen 扩展，用来把 multi-file catalog source
    按固定顺序编译成 adapter 可见输出：
    frontmatter -> `SKILL.md` body -> 条件注入 typed template。
20. 蒸馏文档面被定义为：
    `docs/distillation/schema.md`、
    `docs/distillation/catalog.md`、`docs/distillation/by-source.md`、
    `docs/distillation/domains/*.md`、
    `docs/distillation/routed-surfaces.md`。

### Host 与 command binding

21. Governed kernel 的基础 host binding 已经在 plan 中定义。
22. `review` 被定义成质量、安全、变更形态 skill 的 routed surface，其中
    `second-opinion` 只作为 route，不再是 standalone catalog skill。
23. `validate` 被定义成 spec、coverage、property、mutation、performance
    skill 的 routed surface。
24. `repair` 被定义成 repair / CI specialist route 的 routed surface；
    `status` 与 `health` 被定义成 incident、observability、queue view 的
    routed surface，其中 incident 仅落在 `status` / `health`。
25. 当前方案不要求存在 `skill` command family；任何 factory tooling 都是
    延后的 future command family。

### Export 与产品边界

26. Export 的目标是 tool adapter，而不是 home-directory installation。
27. 不引入 mission/work-package/dashboard runtime。
28. Slipway 继续保持 multi-tool 能力，不收缩成某个工具专属 skill installer。

### 文档与测试

29. 主 plan 与 delivery 文档保持 EN / zh-CN 同步。
30. 实施计划包含 catalog loading、provenance coverage、binding
    resolution、routed command selection、assembler/export behavior 的回归
    覆盖，以及 schema、binding、provenance coverage、size-budget discipline
    的 CI gate。

### Tier、attachment、rollout 纪律

31. 每个 catalog skill 都声明 `tier`（`T1` core capability、`T2` specialist
    route、`T3` diagnostic view）。`tier` 编码的是语义角色，而不是绑定数
    量；像 `threat-modeling` 这类绑定较窄的 skill 仍然是 T1。
32. 每个 catalog skill 都声明 `primary_attachment`，取值限于冻结的 5 种：
    `posture` / `procedure` / `checklist` / `tool-recipe` /
    `report-schema`；resolver 用 attachment mode 决定注入位置。
33. `technique-hint` binding 复用 `cmd/next_skill_view.go` 现有的
    `TechniqueHints` surface；Go 返回 skill id 与 hint 种类，文案由 host
    LLM 组织。
34. B1 必须端到端交付：`internal/engine/capability/` registry + trigger
    DSL + resolver + 对接 `TechniqueHints`，并且完整蒸馏 5 个 foundation
    skill（`scope-clarification`、`plan-authoring`、`tdd-proof`、
    `fresh-verification-evidence`、`independent-review`）。B1 证明端到端
    闭环之前，不得启动后续批次；也不得提前批量建 25 个 catalog skeleton。
35. 路由命令标志已落地：`review` / `validate` / `repair` 提供 `--mode`，
    `status` / `health` 提供 `--view`；显式标志优先于自动路由回退。
36. CI gate 自动化（`schema-lint`、`size-lint`、`binding-compare`、
    `provenance-coverage-scan`）已由
    `internal/engine/capability/gates_test.go` 与 `by_source_test.go` 强制执行。
    `size-lint` 按 tier 分预算并带告警区间与理由门槛。
    进入 warning band 只会记录提示日志；真正 fail 的条件是超过 hard-max 且
    缺少所需 rationale：
    T1 目标 <=2 KB（2-6 KB 告警；超过 6 KB 需理由），
    T2 目标 <=3 KB（3-8 KB 告警；超过 8 KB 需理由），
    T3 目标 <=1.5 KB（1.5-3 KB 告警；超过 3 KB 需理由）。

## 实施清单

落地按 B0-B8 线性批次组织；每批一个 PR，上一个批次未合并不得启动下一个
批次。批次之间的上下文交接只依赖已合并的 `provenance.yaml` 与持续维护的
`docs/distillation/by-source.md`。

### B0 - 契约冻结

1. [x] 冻结 `docs/distillation/schema.md`：tier 定义、冻结的 attachment
       mode 集合、trigger DSL 算子、`provenance.yaml` 形状、typed-template
       职责、support-目录规则、固定 assembler 顺序。
2. [x] 初始化 `catalog.md`、`by-source.md`、`routed-surfaces.md` 骨架。
3. [x] 落地 PR checklist：B0-B7 用来强制 schema / size / binding /
       provenance coverage 的人工 checklist。
4. [x] EN 与 zh-CN 文档同步。

### B1 - 端到端验证

5. [x] 基于 B0 冻结的 schema，实现
       `internal/engine/capability/{registry,trigger,resolver,provenance}.go`。
6. [x] Resolver 输出对接到 `cmd/next_skill_view.go` 现有 `TechniqueHints`
       surface；不新增平行的 hint 渲染路径。
7. [x] 完整蒸馏 5 个 foundation T1 skill：`scope-clarification`、
       `plan-authoring`、`tdd-proof`、`fresh-verification-evidence`、
       `independent-review`。
8. [x] 按 §5.2 / §5.3 把 5 个 B1 skill 绑到对应 governed host，以及
       `review` 命令（仅 `independent-review`）。
9. [x] 测试：registry load；B1 范围内的 resolver selection；
       technique-hint 发出；host binding；B1 source 的 provenance
       coverage。B1 不要求实现或测试 `hydrate_references[]` /
       `llm_tiebreak`。
10. [x] 本批次不得额外建 B1 五个之外的 catalog skeleton。

### B2 - 扩 foundation

11. [x] 蒸馏剩余 foundation T1 skill：`context-assembly`、
        `parallel-executor-contract`、`root-cause-tracing`、
        `security-review`、`spec-trace`。
12. [x] 测试：多 skill 并存下 resolver 稳定性；binding compare；B2 source 的
        provenance coverage。`context-assembly` 在 frontmatter 中声明
        `hydrate_references[]`，但 resolver 发出仍处于保留态（当前无运行时输出）。

### B3 - 安全集群

13. [x] 蒸馏 T1 `threat-modeling`。
14. [x] 蒸馏 T2 `sast-orchestration`、`gha-security-review`、
        `supply-chain-audit`；把 Semgrep / CodeQL / SARIF 三件套的调用差异
        内联保留在 `SKILL.md` 中，只有确实需要更长示例时才使用
        `references/`。
15. [x] 测试：T2 command-route binding 行为；tool-recipe attachment 注入。

### B4 - 变更形态与 verification

16. [x] 蒸馏 T1 `multi-reviewer-calibration`、`differential-review`、
        `variant-analysis`、`coverage-analysis`、`property-testing`、
        `mutation-testing`、`performance-profiling`。
17. [x] 测试：host binding 覆盖；B4 source 的 provenance coverage。

### B5 - Repair/CI 与 ops

18. [x] 蒸馏 T2 `ci-triage`、`review-comment-triage`、`git-recovery`。
19. [x] 蒸馏 T3 `incident-response`；仅绑定到 `status` / `health` /
        export，不走 `repair` 路由。
20. [x] 起草 `sentry`、`skill-scanner` 等 `view-only` surface 的最小 view
        schema，保证与 T3 `incident-response` 的风格不漂移（落在
        `docs/distillation/routed-surfaces.md`）。
21. [x] 测试：T3 view-only binding 与显式 view selector 行为。

### B6 - 非 catalog disposition 清账

22. [x] 定稿 `routed-surfaces.md`，覆盖 `view-only`、`route-only`、
        `deferred` 条目及其命令落点。
23. [x] 6 条 posture-only 吸收（`using-superpowers`、`executing-plans`、
        `mission-system`、`runtime-next`、`agent-orchestrator`、
        `error-handling-patterns`）逐条标注目标 catalog skill 与
        attachment mode。
24. [x] 手工跑一次 provenance-coverage 扫描，按 `provenance.yaml` 的
        `extracted` / `dropped` / `conflicts_with` 复核 plan 列出的 source。
        `internal/engine/capability/by_source_test.go` 的自动化范围当前为
        `standalone` / `partial-only` 行，并附带反向一致性校验
        （`provenance` source 必须出现在 `by-source.md`）。

### B7 - Routed command rollout

25. [x] 实现 `review` auto routing 与 `--mode` 覆盖 flag。
26. [x] 实现 `validate` auto routing 与 `--mode` 覆盖 flag。
27. [x] 实现 `repair` auto routing 与 `--mode` 覆盖 flag。
28. [x] 实现 `status` auto view routing 与 `--view` 覆盖 flag。
29. [x] 实现 `health` diagnostics view routing 与 `--view` 覆盖 flag。
30. [x] 测试：route selection、view selection、显式 override 优先级、
        fallback 行为。若出现真实 DSL tie，再验证 `llm_tiebreak`
        hand-off 行为。保持 scanner-heavy execution 不进入 governed kernel。

### B8 - Export 与 gate 自动化

31. [x] 扩展 toolgen / assembler：按固定顺序（frontmatter -> `SKILL.md`
        body -> 条件 typed template）编译 multi-file catalog source，并带
        上 provenance metadata。
32. [x] 为外部 agent 输出 `using-slipway-catalog.md` export target
        （`capability.BuildCatalogManifest` +
        `toolgen.CatalogManifestPath`）。
33. [x] 自动化 CI gate：`schema-lint`、按 tier 的 `size-lint`、
        `binding-compare`、范围受限的 `provenance-coverage-scan`（由
        `internal/engine/capability/gates_test.go` 与
        `by_source_test.go` 在 `go test ./...` 中强制执行）。
34. [x] 确认本轮交付中 repo-local `skill` command family 继续保持 deferred。
        若未来引入，范围只限 authoring / audit tooling；install-to-home
        继续 out of scope。
